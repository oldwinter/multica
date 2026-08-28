package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/scheduler"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	productionReplayAdapter           = "accepted-room-bounded-replay"
	productionReplayVersion           = "v1"
	productionAttributionQueueSize    = 256
	productionAttributionWriteTimeout = 5 * time.Second
	maxImprovementRoomMessageRunes    = 100000
)

var (
	ErrProductionModuleUnavailable  = errors.New("production skill evolution module is unavailable")
	ErrImprovementRoomUnavailable   = errors.New("Improvement Room is unavailable")
	ErrImprovementRoomContextTooBig = errors.New("Improvement Room context exceeds the Room message limit")
)

// ProductionRegistration owns the one production module instance shared by
// HTTP, task contributors, Room callbacks, metrics, and the scheduler.
type ProductionRegistration struct {
	pool    *pgxpool.Pool
	metrics *Metrics

	mu     sync.RWMutex
	module *Module
}

func NewProductionRegistration(pool *pgxpool.Pool) (*ProductionRegistration, error) {
	if pool == nil {
		return nil, ErrProductionModuleUnavailable
	}
	return &ProductionRegistration{pool: pool, metrics: NewMetrics()}, nil
}

func (registration *ProductionRegistration) Metrics() *Metrics {
	if registration == nil {
		return nil
	}
	return registration.metrics
}

// RegisterRoutes matches cmd/server.RouterOptions.RegisterSkillEvolution.
// Construction failures are startup invariants, so the callback panics before
// the server begins accepting requests instead of installing a partial module.
func (registration *ProductionRegistration) RegisterRoutes(router chi.Router, root *handler.Handler, queries *db.Queries) {
	if registration == nil || router == nil || root == nil || queries == nil {
		panic(ErrProductionModuleUnavailable)
	}
	module, err := newProductionModule(registration.pool, root, queries, registration.metrics)
	if err != nil {
		panic(fmt.Errorf("compose production skill evolution: %w", err))
	}
	if err := module.register(router, root); err != nil {
		module.Close()
		panic(fmt.Errorf("register production skill evolution: %w", err))
	}
	registration.mu.Lock()
	defer registration.mu.Unlock()
	if registration.module != nil {
		module.Close()
		panic("production skill evolution registered more than once")
	}
	registration.module = module
}

func (registration *ProductionRegistration) JobSpec() (scheduler.JobSpec, error) {
	if registration == nil {
		return scheduler.JobSpec{}, ErrProductionModuleUnavailable
	}
	registration.mu.RLock()
	defer registration.mu.RUnlock()
	if registration.module == nil {
		return scheduler.JobSpec{}, ErrProductionModuleUnavailable
	}
	return SkillEvolutionJob(registration.module.store, registration.module, registration.metrics), nil
}

func (registration *ProductionRegistration) Close() {
	if registration == nil {
		return
	}
	registration.mu.Lock()
	defer registration.mu.Unlock()
	if registration.module != nil {
		registration.module.Close()
		registration.module = nil
	}
}

type Module struct {
	store       *Store
	lifecycle   *Lifecycle
	requester   *RoomProposalRequester
	roomTarget  *RoomSkillProposalTarget
	reviewHTTP  *handler.TaskRunReviewHTTPHandler
	attribution *AttributionWorker
	metrics     *Metrics
}

func newProductionModule(pool *pgxpool.Pool, root *handler.Handler, queries *db.Queries, metrics *Metrics) (*Module, error) {
	if pool == nil || root == nil || queries == nil || metrics == nil || root.Rooms == nil || root.RoomArtifactTargets == nil {
		return nil, ErrProductionModuleUnavailable
	}
	outcomes, ok := root.Rooms.(room.AcceptedOutcomeSignals)
	if !ok {
		return nil, ErrProductionModuleUnavailable
	}

	store := NewStore(queries, pool)
	skills := NewWorkspaceSkillRepository(queries)
	reviewAccess := handler.NewTaskRunReviewTaskAccess(root)
	reviews := service.NewTaskRunReviewService(service.NewDBTaskRunReviewRepository(queries), reviewAccess)
	exact := NewExactSkillTaskIndex(queries)
	twinFactory := TwinSignalsFactory(func(actorID pgtype.UUID) TwinSignals {
		if !validUUID(actorID) {
			return nil
		}
		return service.NewTwinSignalAdapter(queries, service.TwinSignalAuthorizerFunc(func(ctx context.Context, workspaceID, taskID pgtype.UUID) error {
			_, err := reviewAccess.LoadAuthorizedTask(ctx, uuidText(workspaceID), uuidText(actorID), uuidText(taskID))
			if err != nil {
				return service.ErrTwinSignalUnauthorized
			}
			return nil
		}))
	})
	sources, err := NewProductionSourceAdapters(
		reviews, reviews, service.NewWikiReviewedProposalSignalAdapter(pool), outcomes, twinFactory, exact,
	)
	if err != nil {
		return nil, err
	}
	candidates, err := NewRoomCandidateEngine(outcomes, queries, sources...)
	if err != nil {
		return nil, err
	}
	lifecycle, err := NewLifecycle(
		store,
		skills,
		NewWorkspaceSkillPublisher(queries, pool),
		NewProductionImprover(candidates, DefaultReplayTimeout),
		newProductionBehavioralEvaluator(candidates),
		sources...,
	)
	if err != nil {
		return nil, err
	}
	lifecycle.SetImprovementRecommendationSource(candidates)
	lifecycle.SetSkillForker(NewWorkspaceSkillForker(queries, pool))
	target := NewRoomSkillProposalTarget(lifecycle, candidates, metrics)
	queuer := &productionImprovementRoomQueuer{
		pool: pool, queries: queries, rooms: root.Rooms, store: store, skills: skills, signals: candidates.signals,
	}
	requester := NewRoomProposalRequester(candidates, target, queuer)
	attribution, err := newAttributionWorker(
		&metricsAttributionStore{delegate: store, metrics: metrics},
		productionAttributionQueueSize,
		productionAttributionWriteTimeout,
	)
	if err != nil {
		return nil, err
	}
	return &Module{
		store: store, lifecycle: lifecycle, requester: requester, roomTarget: target,
		reviewHTTP: handler.NewTaskRunReviewHTTPHandler(root, reviews), attribution: attribution, metrics: metrics,
	}, nil
}

func newProductionBehavioralEvaluator(source *RoomCandidateSource) BehavioralEvaluator {
	return NewProductionReplayEvaluator(source, productionReplayAdapter, productionReplayVersion)
}

func (module *Module) register(router chi.Router, root *handler.Handler) error {
	if module == nil || module.lifecycle == nil || module.requester == nil || module.reviewHTTP == nil ||
		module.roomTarget == nil || module.attribution == nil || root == nil || root.RoomArtifactTargets == nil {
		return ErrProductionModuleUnavailable
	}
	registrations := []struct {
		target  room.RecommendationTarget
		create  service.RoomArtifactTargetCreator
		publish service.RoomArtifactTargetPublisher
	}{
		{room.RecommendationTargetExecutableProcedure, module.roomTarget.CreateRoomArtifactTarget, module.roomTarget.RoomArtifactTargetCreated},
		{room.RecommendationTargetKnowledge, service.NewRoomWikiProposalTarget(), nil},
		{room.RecommendationTargetPreference, service.NewRoomTwinProposalTarget(), nil},
		{room.RecommendationTargetConstraint, service.NewRoomTwinProposalTarget(), nil},
	}
	for _, registration := range registrations {
		if err := root.RoomArtifactTargets.RegisterProposalTarget(registration.target, registration.create, registration.publish); err != nil {
			return err
		}
	}
	attribution := &attributionMetricsContributor{delegate: module.attribution, metrics: module.metrics}
	root.RegisterTaskDispatchContributor(attribution)
	root.RegisterTaskCompletionContributor(attribution)
	root.RegisterWorkspaceCleanupContributor(NewWorkspaceCleanup())

	router.Mount("/api/skill-evolution", NewHTTP(module.lifecycle, module.lifecycle.skills, module.requester, module.metrics).Routes())
	registerTaskReviewRoutes(router, taskReviewRouteHandlers{
		create: recordSuccessfulTaskReview(module.metrics, module.reviewHTTP.Create),
		list:   module.reviewHTTP.List, get: module.reviewHTTP.Get,
		listManualReruns: module.reviewHTTP.ListManualReruns, getManualRerun: module.reviewHTTP.GetManualRerun,
	})
	return nil
}

type taskReviewRouteHandlers struct {
	create           http.HandlerFunc
	list             http.HandlerFunc
	get              http.HandlerFunc
	listManualReruns http.HandlerFunc
	getManualRerun   http.HandlerFunc
}

func registerTaskReviewRoutes(router chi.Router, handlers taskReviewRouteHandlers) {
	router.Post("/api/tasks/{taskId}/review", handlers.create)
	router.Get("/api/task-run-reviews", handlers.list)
	router.Get("/api/task-run-reviews/manual-reruns", handlers.listManualReruns)
	router.Get("/api/task-run-reviews/manual-reruns/{taskId}", handlers.getManualRerun)
	router.Get("/api/task-run-reviews/{reviewId}", handlers.get)
}

type attributionMetricsContributor struct {
	delegate interface {
		OfferTaskDispatch(handler.TaskDispatchEvent) bool
		OfferTaskCompletion(handler.TaskCompletionEvent) bool
	}
	metrics *Metrics
}

func (contributor *attributionMetricsContributor) OfferTaskDispatch(event handler.TaskDispatchEvent) bool {
	return contributor != nil && contributor.delegate != nil && contributor.delegate.OfferTaskDispatch(event)
}

func (contributor *attributionMetricsContributor) OfferTaskCompletion(event handler.TaskCompletionEvent) bool {
	if contributor == nil || contributor.delegate == nil {
		return false
	}
	accepted := contributor.delegate.OfferTaskCompletion(event)
	if event.CapabilityProven && contributor.metrics != nil {
		contributor.metrics.RecordManifestCoverage(false)
	}
	return accepted
}

type metricsAttributionStore struct {
	delegate attributionWorkerStore
	metrics  *Metrics
}

func (store *metricsAttributionStore) RecordTaskDispatchSnapshot(ctx context.Context, input TaskDispatchSnapshotInput) (TaskDispatchSnapshot, error) {
	return store.delegate.RecordTaskDispatchSnapshot(ctx, input)
}

func (store *metricsAttributionStore) GetTaskDispatchSnapshot(
	ctx context.Context,
	workspaceID, taskID, agentID, runtimeID pgtype.UUID,
	dispatchedAt time.Time,
) (TaskDispatchSnapshot, error) {
	return store.delegate.GetTaskDispatchSnapshot(ctx, workspaceID, taskID, agentID, runtimeID, dispatchedAt)
}

func (store *metricsAttributionStore) resolveAttributionRevisions(ctx context.Context, match attributionRevisionMatch) ([]attributionRevision, error) {
	return store.delegate.resolveAttributionRevisions(ctx, match)
}

func (store *metricsAttributionStore) recordAttributionBatch(ctx context.Context, inputs []TaskAttributionInput) error {
	if err := store.delegate.recordAttributionBatch(ctx, inputs); err != nil {
		return err
	}
	if len(inputs) > 0 && store.metrics != nil {
		store.metrics.ManifestAttributedRuns.Inc()
		store.metrics.RecordFeedbackCoverage(false)
	}
	return nil
}

type evolutionStatusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *evolutionStatusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *evolutionStatusWriter) Write(payload []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(payload)
}

func recordSuccessfulTaskReview(metrics *Metrics, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writer := &evolutionStatusWriter{ResponseWriter: w}
		next(writer, r)
		if metrics != nil && writer.status >= 200 && writer.status < 300 {
			metrics.FeedbackCoveredRuns.Inc()
		}
	}
}

func (module *Module) Close() {
	if module != nil && module.attribution != nil {
		module.attribution.Close()
	}
}

func (module *Module) RunScheduledLoop(ctx context.Context, loop db.SkillEvolutionLoop, idempotencyKey string) (ScheduledLoopOutcome, error) {
	if module == nil || module.lifecycle == nil || module.lifecycle.skills == nil || module.requester == nil || !scheduledLoopRunnable(loop) ||
		!boundedToken(idempotencyKey, MaxIdempotencyKeyBytes) {
		return ScheduledLoopOutcome{}, ErrLifecycleInvalidInput
	}
	switch LoopMode(loop.Mode) {
	case LoopModeObserve:
		snapshot, err := module.lifecycle.skills.Load(ctx, loop.WorkspaceID, loop.SkillID)
		if err != nil {
			return ScheduledLoopOutcome{}, err
		}
		observation, err := module.lifecycle.Observe(ctx, ObserveRequest{
			WorkspaceID: loop.WorkspaceID, SkillID: loop.SkillID, ActorID: snapshot.Skill.CreatedBy,
		})
		if err != nil {
			return ScheduledLoopOutcome{}, err
		}
		return ScheduledLoopOutcome{Action: ScheduledLoopObserved, EligibleSignals: len(observation.References)}, nil
	case LoopModePropose:
		result, err := module.requester.RequestProposal(ctx, loop.WorkspaceID, loop.SkillID, pgtype.UUID{}, idempotencyKey)
		if err != nil {
			return ScheduledLoopOutcome{}, err
		}
		action := ScheduledLoopImprovementRoomQueued
		if result.Generation != nil {
			action = ScheduledLoopAcceptedRecommendationRecovered
		}
		return ScheduledLoopOutcome{Action: action, EligibleSignals: result.EligibleSignals}, nil
	default:
		return ScheduledLoopOutcome{Action: ScheduledLoopSkipped}, nil
	}
}

type productionImprovementRoomQueuer struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	rooms   room.Rooms
	store   *Store
	skills  WorkspaceSkillLoader
	signals *signalSet
}

func (queuer *productionImprovementRoomQueuer) EnsureImprovementRoom(
	ctx context.Context,
	workspaceID, skillID, actorID pgtype.UUID,
	idempotencyKey string,
) (ImprovementRoomQueueResult, error) {
	if queuer == nil || queuer.pool == nil || queuer.queries == nil || queuer.rooms == nil || queuer.store == nil ||
		queuer.skills == nil || queuer.signals == nil || !validUUID(workspaceID) || !validUUID(skillID) ||
		!validOptionalUUID(actorID) || !boundedToken(idempotencyKey, MaxIdempotencyKeyBytes) || queuer.pool.Config().MaxConns < 2 {
		return ImprovementRoomQueueResult{}, ErrImprovementRoomUnavailable
	}
	connection, err := queuer.pool.Acquire(ctx)
	if err != nil {
		return ImprovementRoomQueueResult{}, fmt.Errorf("acquire Improvement Room lock: %w", err)
	}
	defer connection.Release()
	lockKey := "skill-evolution-room:" + uuidText(workspaceID) + ":" + uuidText(skillID)
	if _, err := connection.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", lockKey); err != nil {
		return ImprovementRoomQueueResult{}, fmt.Errorf("lock Improvement Room: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = connection.Exec(unlockCtx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", lockKey)
	}()
	return queuer.ensureLocked(ctx, workspaceID, skillID, actorID, idempotencyKey)
}

func (queuer *productionImprovementRoomQueuer) ensureLocked(
	ctx context.Context,
	workspaceID, skillID, actorID pgtype.UUID,
	idempotencyKey string,
) (ImprovementRoomQueueResult, error) {
	loop, err := queuer.store.GetLoop(ctx, workspaceID, skillID)
	if err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	if !loop.Enabled {
		return ImprovementRoomQueueResult{}, ErrEvolutionDisabled
	}
	if LoopMode(loop.Mode) != LoopModePropose {
		return ImprovementRoomQueueResult{}, ErrEvolutionObserveOnly
	}
	snapshot, err := queuer.skills.Load(ctx, workspaceID, skillID)
	if err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	if !validUUID(actorID) {
		actorID = snapshot.Skill.CreatedBy
	}
	if !validUUID(actorID) {
		return ImprovementRoomQueueResult{}, ErrImprovementRoomUnavailable
	}
	roomID, err := queuer.findRoom(ctx, workspaceID, skillID)
	if err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	messageKey := "skill-evolution:" + idempotencyKey
	if validUUID(roomID) {
		if replay, ok, replayErr := queuer.replayedRoomRequest(ctx, workspaceID, skillID, roomID, messageKey); replayErr != nil {
			return ImprovementRoomQueueResult{}, replayErr
		} else if ok {
			return replay, nil
		}
	}
	if loop.NextEligibleAt.Valid && time.Now().UTC().Before(loop.NextEligibleAt.Time) {
		return ImprovementRoomQueueResult{}, ErrEvolutionCooldown
	}
	query := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, ActorID: actorID, Limit: int(loop.MaxEvidenceRefs)}
	refs, err := queuer.signals.discover(ctx, query)
	if err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	if len(refs) < int(loop.MinimumSignals) {
		return ImprovementRoomQueueResult{}, ErrInsufficientSignals
	}
	resolved, err := queuer.signals.resolve(ctx, query, refs)
	if err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	body, err := improvementRoomMessage(snapshot, resolved)
	if err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	if !validUUID(roomID) {
		facilitatorID, err := queuer.findFacilitator(ctx, workspaceID, skillID)
		if err != nil {
			return ImprovementRoomQueueResult{}, err
		}
		detail, err := queuer.rooms.Create(ctx, room.CreateInput{
			WorkspaceID: workspaceID, ActorUserID: actorID, Title: improvementRoomTitle(snapshot.Skill.Name),
			Instructions: "Use only the exact Skill bundle and evidence supplied in member messages. Keep the change bounded and emit the strict improvement executable_procedure envelope when the evidence supports one.",
			Objective:    "Improve workspace Skill " + uuidText(skillID), TemplateID: improvementRoomTemplateID,
			FacilitatorAgentID: facilitatorID,
			SuccessCriteria:    []string{"A reviewable, bounded Skill proposal with exact evidence provenance."},
			StopConditions:     []string{"Required evidence or exact Skill provenance is unavailable."},
		})
		if err != nil {
			return ImprovementRoomQueueResult{}, err
		}
		roomID = detail.Room.ID
	}
	if _, err := queuer.rooms.PostMessage(ctx, room.MessageInput{
		WorkspaceID: workspaceID, RoomID: roomID, ActorUserID: actorID, Body: body, IdempotencyKey: messageKey,
	}); err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	now := time.Now().UTC()
	if _, err := queuer.store.RecordLoopObservation(ctx, workspaceID, loop.ID, now, now.Add(time.Duration(loop.CooldownSeconds)*time.Second)); err != nil {
		return ImprovementRoomQueueResult{}, err
	}
	return ImprovementRoomQueueResult{RoomID: roomID, EligibleSignals: len(resolved)}, nil
}

func (queuer *productionImprovementRoomQueuer) findRoom(ctx context.Context, workspaceID, skillID pgtype.UUID) (pgtype.UUID, error) {
	rows, err := queuer.pool.Query(ctx, `
SELECT id
FROM room
WHERE workspace_id = $1
  AND template_id = 'improvement'
  AND objective = $2
  AND status <> 'archived'
ORDER BY created_at DESC, id DESC
LIMIT 2`, workspaceID, "Improve workspace Skill "+uuidText(skillID))
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("find Improvement Room: %w", err)
	}
	defer rows.Close()
	var ids []pgtype.UUID
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return pgtype.UUID{}, fmt.Errorf("scan Improvement Room: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return pgtype.UUID{}, fmt.Errorf("iterate Improvement Rooms: %w", err)
	}
	if len(ids) > 1 {
		return pgtype.UUID{}, ErrPersistenceConflict
	}
	if len(ids) == 0 {
		return pgtype.UUID{}, nil
	}
	return ids[0], nil
}

func (queuer *productionImprovementRoomQueuer) findFacilitator(ctx context.Context, workspaceID, skillID pgtype.UUID) (pgtype.UUID, error) {
	var agentID pgtype.UUID
	err := queuer.pool.QueryRow(ctx, `
SELECT assignment.agent_id
FROM agent_skill assignment
JOIN agent ON agent.id = assignment.agent_id AND agent.workspace_id = $1
WHERE assignment.skill_id = $2
  AND assignment.enabled = TRUE
  AND agent.archived_at IS NULL
ORDER BY assignment.agent_id
LIMIT 1`, workspaceID, skillID).Scan(&agentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, ErrImprovementRoomUnavailable
	}
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("select Improvement Room facilitator: %w", err)
	}
	if !validUUID(agentID) {
		return pgtype.UUID{}, ErrImprovementRoomUnavailable
	}
	return agentID, nil
}

func (queuer *productionImprovementRoomQueuer) replayedRoomRequest(
	ctx context.Context,
	workspaceID, skillID, roomID pgtype.UUID,
	messageKey string,
) (ImprovementRoomQueueResult, bool, error) {
	cycle, err := queuer.queries.GetRoomCycleByWakeKey(ctx, db.GetRoomCycleByWakeKeyParams{
		WorkspaceID: workspaceID, RoomID: roomID, WakeKey: "message:" + messageKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ImprovementRoomQueueResult{}, false, nil
	}
	if err != nil {
		return ImprovementRoomQueueResult{}, false, err
	}
	entry, err := queuer.queries.GetRoomEntry(ctx, db.GetRoomEntryParams{ID: cycle.TriggeringEntryID, WorkspaceID: workspaceID, RoomID: roomID})
	if err != nil {
		return ImprovementRoomQueueResult{}, false, err
	}
	var message improvementRoomContext
	if decodeStrictJSON([]byte(entry.Body), &message) != nil || message.SchemaVersion != 1 ||
		message.Skill.ID != uuidText(skillID) || len(message.Evidence) == 0 || len(message.Evidence) > MaxEvidenceRefs {
		return ImprovementRoomQueueResult{}, false, ErrPersistenceConflict
	}
	return ImprovementRoomQueueResult{RoomID: roomID, EligibleSignals: len(message.Evidence)}, true, nil
}

type improvementRoomContext struct {
	SchemaVersion int                       `json:"schema_version"`
	Skill         improvementRoomSkill      `json:"skill"`
	Evidence      []improvementRoomEvidence `json:"evidence"`
}

type improvementRoomSkill struct {
	ID          string                `json:"id"`
	Source      string                `json:"source"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Content     string                `json:"content"`
	Files       []improvementRoomFile `json:"files"`
	BundleHash  string                `json:"bundle_hash"`
}

type improvementRoomFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type improvementRoomEvidence struct {
	Kind             EvidenceKind    `json:"kind"`
	SourceID         string          `json:"source_id"`
	SourceRevisionID string          `json:"source_revision_id,omitempty"`
	SourceState      string          `json:"source_state"`
	Digest           Digest          `json:"digest"`
	ObservedAt       time.Time       `json:"observed_at"`
	Payload          json.RawMessage `json:"payload"`
}

func improvementRoomMessage(snapshot WorkspaceSkillSnapshot, evidence []ResolvedEvidence) (string, error) {
	if !validUUID(snapshot.Skill.ID) || snapshot.Bundle.ID == "" || snapshot.Manifest.Hash == "" || len(evidence) == 0 || len(evidence) > MaxEvidenceRefs {
		return "", ErrRoomCandidateInvalid
	}
	message := improvementRoomContext{SchemaVersion: 1, Skill: improvementRoomSkill{
		ID: snapshot.Bundle.ID, Source: snapshot.Bundle.Source, Name: snapshot.Bundle.Name,
		Description: snapshot.Bundle.Description, Content: snapshot.Bundle.Content,
		Files: make([]improvementRoomFile, len(snapshot.Bundle.Files)), BundleHash: snapshot.Manifest.Hash,
	}, Evidence: make([]improvementRoomEvidence, len(evidence))}
	for index, file := range snapshot.Bundle.Files {
		message.Skill.Files[index] = improvementRoomFile{Path: file.Path, Content: file.Content}
	}
	for index, item := range evidence {
		if item.Ref.Validate() != nil || !json.Valid(item.Payload) {
			return "", ErrSignalSourceInvalid
		}
		message.Evidence[index] = improvementRoomEvidence{
			Kind: item.Ref.Kind, SourceID: item.Ref.SourceID, SourceRevisionID: item.Ref.SourceRevisionID,
			SourceState: item.Ref.SourceState, Digest: item.Ref.Digest, ObservedAt: item.Ref.ObservedAt.UTC(),
			Payload: append(json.RawMessage(nil), item.Payload...),
		}
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return "", err
	}
	if utf8.RuneCount(raw) > maxImprovementRoomMessageRunes {
		return "", ErrImprovementRoomContextTooBig
	}
	return string(raw), nil
}

func improvementRoomTitle(skillName string) string {
	title := "Improve Skill: " + strings.TrimSpace(skillName)
	runes := []rune(title)
	if len(runes) > 160 {
		title = string(runes[:160])
	}
	return title
}

var _ ScheduledLoopRunner = (*Module)(nil)
