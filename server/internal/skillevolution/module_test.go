package skillevolution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/room"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type attributionSinkStub struct {
	accepted bool
}

type productionSkillLoaderStub struct {
	snapshot WorkspaceSkillSnapshot
}

func (stub productionSkillLoaderStub) Load(context.Context, pgtype.UUID, pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	return stub.snapshot, nil
}

type productionRoomsStub struct {
	room.Rooms
	pool     *pgxpool.Pool
	roomID   pgtype.UUID
	creates  int
	messages int
}

func (stub *productionRoomsStub) Create(ctx context.Context, input room.CreateInput) (room.Detail, error) {
	stub.creates++
	_, err := stub.pool.Exec(ctx, `
INSERT INTO room (id, workspace_id, template_id, objective, status, created_at)
VALUES ($1, $2, $3, $4, 'active', now())`, stub.roomID, input.WorkspaceID, input.TemplateID, input.Objective)
	if err != nil {
		return room.Detail{}, err
	}
	return room.Detail{Room: db.Room{ID: stub.roomID, WorkspaceID: input.WorkspaceID, TemplateID: pgtype.Text{String: input.TemplateID, Valid: true}}}, nil
}

func (stub *productionRoomsStub) PostMessage(ctx context.Context, input room.MessageInput) (room.MessageResult, error) {
	stub.messages++
	entryID, cycleID := publisherUUID(), publisherUUID()
	if _, err := stub.pool.Exec(ctx, `
INSERT INTO room_entry (
    id, workspace_id, room_id, ordinal, entry_type, author_type, author_id, body, mentions, created_at
) VALUES ($1, $2, $3, 1, 'message', 'member', $4, $5, '[]'::jsonb, now())`,
		entryID, input.WorkspaceID, input.RoomID, input.ActorUserID, input.Body); err != nil {
		return room.MessageResult{}, err
	}
	if _, err := stub.pool.Exec(ctx, `
INSERT INTO room_cycle (
    id, workspace_id, room_id, sequence, source, wake_key, triggering_entry_id,
    status, phase, expected_max_turns, created_at
) VALUES ($1, $2, $3, 1, 'message', $4, $5, 'queued', 'discussion', 1, now())`,
		cycleID, input.WorkspaceID, input.RoomID, "message:"+input.IdempotencyKey, entryID); err != nil {
		return room.MessageResult{}, err
	}
	return room.MessageResult{}, nil
}

func (stub attributionSinkStub) OfferTaskDispatch(handler.TaskDispatchEvent) bool {
	return stub.accepted
}

func (stub attributionSinkStub) OfferTaskCompletion(handler.TaskCompletionEvent) bool {
	return stub.accepted
}

func TestProductionBehavioralEvaluatorUsesAcceptedRoomEngineAndFailsClosed(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	evaluator, ok := newProductionBehavioralEvaluator(fixture.source).(*ProductionReplayEvaluator)
	if !ok {
		t.Fatalf("production evaluator type = %T", newProductionBehavioralEvaluator(fixture.source))
	}
	if evaluator.engine != fixture.source || evaluator.adapter != productionReplayAdapter || evaluator.version != productionReplayVersion {
		t.Fatalf("production evaluator = %+v", evaluator)
	}
}

func TestProductionMetricsAdaptersRecordRepresentativeOperations(t *testing.T) {
	metrics := NewMetrics()
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(metrics.Collectors()...)
	review := recordSuccessfulTaskReview(metrics, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	review(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/tasks/id/review", strings.NewReader(`{}`)))
	if got := testutil.ToFloat64(metrics.FeedbackCoveredRuns); got != 1 {
		t.Fatalf("feedback covered = %v, want 1", got)
	}

	contributor := &attributionMetricsContributor{delegate: attributionSinkStub{accepted: true}, metrics: metrics}
	accepted := contributor.OfferTaskCompletion(handler.TaskCompletionEvent{
		CapabilityProven:       true,
		SkillExecutionManifest: []byte(`{"malformed-for-attribution":true}`),
	})
	if !accepted {
		t.Fatal("completion was not delegated")
	}
	if got := testutil.ToFloat64(metrics.ManifestEligibleRuns); got != 1 {
		t.Fatalf("manifest eligible = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ManifestAttributedRuns); got != 0 {
		t.Fatalf("manifest attributed before persistence = %v, want 0", got)
	}
	if got := testutil.ToFloat64(metrics.FeedbackEligibleRuns); got != 0 {
		t.Fatalf("feedback eligible before persistence = %v, want 0", got)
	}

	store := &metricsAttributionStore{delegate: newFakeAttributionWorkerStore(), metrics: metrics}
	if err := store.recordAttributionBatch(context.Background(), []TaskAttributionInput{{}}); err != nil {
		t.Fatalf("record attributed batch: %v", err)
	}
	if got := testutil.ToFloat64(metrics.ManifestAttributedRuns); got != 1 {
		t.Fatalf("manifest attributed after persistence = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.FeedbackEligibleRuns); got != 1 {
		t.Fatalf("feedback eligible after persistence = %v, want 1", got)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather production metrics: %v", err)
	}
	want := map[string]float64{
		"multica_skill_evolution_feedback_covered_runs_total":    1,
		"multica_skill_evolution_feedback_eligible_runs_total":   1,
		"multica_skill_evolution_manifest_attributed_runs_total": 1,
		"multica_skill_evolution_manifest_eligible_runs_total":   1,
	}
	for _, family := range families {
		expected, ok := want[family.GetName()]
		if !ok || len(family.Metric) != 1 || family.Metric[0].Counter == nil {
			continue
		}
		if got := family.Metric[0].Counter.GetValue(); got != expected {
			t.Fatalf("gathered %s = %v, want %v", family.GetName(), got, expected)
		}
		delete(want, family.GetName())
	}
	if len(want) != 0 {
		t.Fatalf("missing gathered metrics: %v", want)
	}
}

func TestProductionTaskReviewRouteRegistration(t *testing.T) {
	router := chi.NewRouter()
	handlerFor := func(marker string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Route", marker)
			w.WriteHeader(http.StatusNoContent)
		}
	}
	registerTaskReviewRoutes(router, taskReviewRouteHandlers{
		create: handlerFor("create"), list: handlerFor("list"), get: handlerFor("get"),
		listManualReruns: handlerFor("manual-list"), getManualRerun: handlerFor("manual-get"),
	})
	for _, test := range []struct {
		method string
		path   string
		marker string
	}{
		{http.MethodPost, "/api/tasks/task-id/review", "create"},
		{http.MethodGet, "/api/task-run-reviews", "list"},
		{http.MethodGet, "/api/task-run-reviews/review-id", "get"},
		{http.MethodGet, "/api/task-run-reviews/manual-reruns", "manual-list"},
		{http.MethodGet, "/api/task-run-reviews/manual-reruns/task-id", "manual-get"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != http.StatusNoContent || recorder.Header().Get("X-Route") != test.marker {
			t.Fatalf("%s %s status=%d marker=%q", test.method, test.path, recorder.Code, recorder.Header().Get("X-Route"))
		}
	}
}

func TestProductionImprovementRoomQueuerCreatesVisibleRoomAndReplaysWake(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	setupProductionRoomQueueSchema(t, pool)
	workspaceID, skillID, actorID, agentID := publisherUUID(), publisherUUID(), publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: skillID, WorkspaceID: workspaceID, CreatorID: actorID, Name: "deploy",
		Description: "Deploy safely", Content: "---\nname: deploy\ndescription: Deploy safely\n---\n\nVerify the release.", Config: `{}`,
	})
	mustExecPublisher(t, pool, `INSERT INTO agent (id, workspace_id) VALUES ($1, $2)`, agentID, workspaceID)
	mustExecPublisher(t, pool, `INSERT INTO agent_skill (agent_id, skill_id, enabled) VALUES ($1, $2, TRUE)`, agentID, skillID)
	mustExecPublisher(t, pool, `
INSERT INTO skill_evolution_loop (
    workspace_id, skill_id, enabled, mode, cooldown_seconds, minimum_signals,
    max_evidence_refs, max_replay_samples, max_cost_usd_ticks, policy_version
) VALUES ($1, $2, TRUE, 'propose', 60, 1, 5, 1, 0, 'v1')`, workspaceID, skillID)

	snapshot := mustLoadPublisherSkill(t, NewWorkspaceSkillRepository(db.New(pool)), workspaceID, skillID)
	ref := lifecycleEvidenceRef(workspaceID, skillID, "production-room")
	source := NewSignalAdapter(EvidenceKindTaskReview,
		func(context.Context, SignalQuery) ([]EvidenceRef, error) { return []EvidenceRef{ref}, nil },
		func(_ context.Context, _ SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			return ResolvedEvidence{Ref: expected, Payload: []byte(`{"outcome":"needs_correction","correction":"Verify the release.","reason":"A check was skipped."}`)}, nil
		},
	)
	signals, err := newSignalSet([]SignalSource{source})
	if err != nil {
		t.Fatal(err)
	}
	rooms := &productionRoomsStub{pool: pool, roomID: publisherUUID()}
	queuer := &productionImprovementRoomQueuer{
		pool: pool, queries: db.New(pool), rooms: rooms, store: NewStore(db.New(pool), pool),
		skills: productionSkillLoaderStub{snapshot: snapshot}, signals: signals,
	}

	first, err := queuer.EnsureImprovementRoom(context.Background(), workspaceID, skillID, actorID, "request-1")
	if err != nil {
		t.Fatalf("first EnsureImprovementRoom: %v", err)
	}
	second, err := queuer.EnsureImprovementRoom(context.Background(), workspaceID, skillID, actorID, "request-1")
	if err != nil {
		t.Fatalf("repeated EnsureImprovementRoom: %v", err)
	}
	if first != second || first.RoomID != rooms.roomID || first.EligibleSignals != 1 || rooms.creates != 1 || rooms.messages != 1 {
		t.Fatalf("queue results first=%+v second=%+v creates=%d messages=%d", first, second, rooms.creates, rooms.messages)
	}
	var templateID, objective, status string
	if err := pool.QueryRow(context.Background(), `SELECT template_id, objective, status FROM room WHERE id = $1`, rooms.roomID).
		Scan(&templateID, &objective, &status); err != nil {
		t.Fatalf("load visible Improvement Room: %v", err)
	}
	if templateID != improvementRoomTemplateID || objective != "Improve workspace Skill "+uuidText(skillID) || status != "active" {
		t.Fatalf("visible Improvement Room = template %q objective %q status %q", templateID, objective, status)
	}

	mustExecPublisher(t, pool, `UPDATE skill_evolution_loop SET mode = 'observe', next_eligible_at = NULL WHERE workspace_id = $1 AND skill_id = $2`, workspaceID, skillID)
	var observedActor pgtype.UUID
	observeSource := NewSignalAdapter(EvidenceKindTaskReview,
		func(_ context.Context, query SignalQuery) ([]EvidenceRef, error) {
			observedActor = query.ActorID
			return []EvidenceRef{ref}, nil
		},
		func(context.Context, SignalQuery, EvidenceRef) (ResolvedEvidence, error) {
			return ResolvedEvidence{}, nil
		},
	)
	skills := &memorySkillLoader{current: snapshot}
	lifecycle, err := NewLifecycle(
		NewStore(db.New(pool), pool), skills, &memoryPublisher{skills: skills},
		&DeterministicImprover{}, &DeterministicReplayEvaluator{}, observeSource,
	)
	if err != nil {
		t.Fatalf("compose observe lifecycle: %v", err)
	}
	loop, err := NewStore(db.New(pool), pool).GetLoop(context.Background(), workspaceID, skillID)
	if err != nil {
		t.Fatalf("load scheduled loop: %v", err)
	}
	outcome, err := (&Module{lifecycle: lifecycle, requester: &RoomProposalRequester{}}).
		RunScheduledLoop(context.Background(), loop, "scheduled:observe")
	if err != nil {
		t.Fatalf("run scheduled observe: %v", err)
	}
	if observedActor != actorID || outcome.Action != ScheduledLoopObserved || outcome.EligibleSignals != 1 {
		t.Fatalf("scheduled observe actor=%s outcome=%+v", uuidText(observedActor), outcome)
	}
}

func setupProductionRoomQueueSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	mustExecPublisher(t, pool, `
CREATE TABLE agent (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    archived_at TIMESTAMPTZ NULL
);
CREATE TABLE skill_evolution_loop (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    mode TEXT NOT NULL DEFAULT 'observe',
    cooldown_seconds INTEGER NOT NULL DEFAULT 86400,
    minimum_signals INTEGER NOT NULL DEFAULT 3,
    max_evidence_refs INTEGER NOT NULL DEFAULT 20,
    max_replay_samples INTEGER NOT NULL DEFAULT 5,
    max_cost_usd_ticks BIGINT NOT NULL DEFAULT 0,
    policy_version TEXT NOT NULL DEFAULT 'v1',
    last_observed_at TIMESTAMPTZ NULL,
    last_proposal_at TIMESTAMPTZ NULL,
    next_eligible_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE room (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    template_id TEXT NOT NULL,
    objective TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE room_entry (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    cycle_id UUID NULL,
    turn_id UUID NULL,
    ordinal BIGINT NOT NULL,
    entry_type TEXT NOT NULL,
    author_type TEXT NOT NULL,
    author_id UUID NULL,
    body TEXT NOT NULL,
    mentions JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE room_cycle (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    sequence BIGINT NOT NULL,
    source TEXT NOT NULL,
    wake_key TEXT NOT NULL,
    triggering_entry_id UUID NULL,
    status TEXT NOT NULL,
    refusal_reason TEXT NULL,
    planned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    phase TEXT NOT NULL DEFAULT 'discussion',
    synthesis_error JSONB NULL,
    synthesis_turn_id UUID NULL,
    memory_revision_id UUID NULL,
    expected_max_turns INTEGER NOT NULL DEFAULT 1,
    cancel_idempotency_key TEXT NULL,
    cost_limit_ticks BIGINT NULL
);
CREATE UNIQUE INDEX room_cycle_wake_key_uidx ON room_cycle (room_id, wake_key);
`)
}

var _ WorkspaceSkillLoader = productionSkillLoaderStub{}
var _ room.Rooms = (*productionRoomsStub)(nil)
