package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

const (
	DefaultReplayTimeout    = 2 * time.Minute
	DefaultOverviewLimit    = 50
	lifecycleCleanupTimeout = 5 * time.Second
)

var (
	ErrLifecycleInvalidInput = errors.New("invalid skill evolution lifecycle input")
	ErrHumanActorRequired    = errors.New("skill evolution decision requires a human actor")
	ErrEvolutionDisabled     = errors.New("skill evolution loop is disabled")
	ErrEvolutionPaused       = errors.New("skill evolution loop is paused")
	ErrEvolutionObserveOnly  = errors.New("skill evolution loop is observe-only")
	ErrEvolutionCooldown     = errors.New("skill evolution loop is cooling down")
	ErrInsufficientSignals   = errors.New("skill evolution has insufficient eligible signals")
	ErrGenerationActive      = errors.New("skill evolution generation is already active")
	ErrGenerationFailed      = errors.New("skill evolution proposal generation failed")
	ErrEvaluationFailed      = errors.New("skill evolution candidate evaluation did not pass")
	ErrDecisionConflict      = errors.New("skill evolution proposal cannot accept that decision")
	ErrReleaseNotRetryable   = errors.New("skill evolution publication outcome requires inspection")
	ErrSkillForkUnavailable  = errors.New("workspace Skill fork adapter is unavailable")
)

type ActorKind string

const (
	ActorKindHuman   ActorKind = "human"
	ActorKindMachine ActorKind = "machine"
)

type DecisionActor struct {
	ID   pgtype.UUID
	Kind ActorKind
}

func (a DecisionActor) human() bool { return a.Kind == ActorKindHuman && validUUID(a.ID) }

// SkillEvolution is the only application-facing module contract. Callers do
// not reproduce collection, validation, evaluation, or publication ordering.
type SkillEvolution interface {
	Overview(context.Context, pgtype.UUID, pgtype.UUID, int) (EvolutionOverview, error)
	ReadProposal(context.Context, pgtype.UUID, pgtype.UUID) (ProposalView, error)
	Configure(context.Context, DecisionActor, LoopConfig) (db.SkillEvolutionLoop, error)
	Enable(context.Context, DecisionActor, LoopConfig) (db.SkillEvolutionLoop, error)
	Pause(context.Context, DecisionActor, pgtype.UUID, pgtype.UUID) (db.SkillEvolutionLoop, error)
	Observe(context.Context, ObserveRequest) (Observation, error)
	Generate(context.Context, GenerateRequest) (Generation, error)
	CreateProposalFromRoomRecommendation(context.Context, RoomRecommendationRequest) (Generation, error)
	Reject(context.Context, RejectRequest) (db.SkillEvolutionProposal, error)
	Publish(context.Context, PublishRequest) (Publication, error)
	Rollback(context.Context, RollbackRequest) (Publication, error)
	Fork(context.Context, ForkRequest) (Fork, error)
}

type lifecycleStore interface {
	ConfigureLoop(context.Context, LoopConfig) (db.SkillEvolutionLoop, error)
	GetLoop(context.Context, pgtype.UUID, pgtype.UUID) (db.SkillEvolutionLoop, error)
	RecordLoopObservation(context.Context, pgtype.UUID, pgtype.UUID, time.Time, time.Time) (db.SkillEvolutionLoop, error)
	SaveRevision(context.Context, RevisionInput) (RevisionSnapshot, error)
	GetRevisionSnapshot(context.Context, pgtype.UUID, pgtype.UUID) (RevisionSnapshot, error)
	getRevisionByHash(context.Context, pgtype.UUID, pgtype.UUID, Digest) (RevisionSnapshot, error)
	getProposalByKey(context.Context, pgtype.UUID, pgtype.UUID, string) (db.SkillEvolutionProposal, error)
	CreateProposal(context.Context, ProposalInput) (db.SkillEvolutionProposal, error)
	TransitionProposal(context.Context, ProposalTransition) (db.SkillEvolutionProposal, error)
	GetProposal(context.Context, pgtype.UUID, pgtype.UUID) (db.SkillEvolutionProposal, error)
	RecordEvidence(context.Context, pgtype.UUID, EvidenceRef) (db.SkillEvolutionEvidence, error)
	RecordEvaluation(context.Context, EvaluationInput) (db.SkillEvolutionEvaluation, error)
	RecordReview(context.Context, ReviewInput) (db.SkillEvolutionReview, error)
	CreateRelease(context.Context, ReleaseInput) (db.SkillEvolutionRelease, error)
	TransitionRelease(context.Context, ReleaseTransition) (db.SkillEvolutionRelease, error)
	getRelease(context.Context, pgtype.UUID, pgtype.UUID) (db.SkillEvolutionRelease, error)
	getReleaseByKey(context.Context, pgtype.UUID, string) (db.SkillEvolutionRelease, error)
	GetProposalDetail(context.Context, pgtype.UUID, pgtype.UUID) (ProposalDetail, error)
	GetOverview(context.Context, pgtype.UUID, pgtype.UUID, int) (Overview, error)
}

type WorkspaceSkillLoader interface {
	Load(context.Context, pgtype.UUID, pgtype.UUID) (WorkspaceSkillSnapshot, error)
}

// SkillForker owns creation of a new workspace Skill through the existing
// Skill creation transaction. It must remove external/plugin ownership,
// preserve a content-free source audit (source ID, ownership and exact hash),
// copy the complete bundle, and return a distinct new Skill ID. T8 composes the
// existing product creation workflow behind this seam.
type SkillForker interface {
	ForkSkill(context.Context, ForkSkillInput) (ForkSkillResult, error)
}

type ForkSkillInput struct {
	WorkspaceID        pgtype.UUID
	SourceSkillID      pgtype.UUID
	ExpectedSourceHash Digest
	NewName            string
	ActorID            pgtype.UUID
	IdempotencyKey     string
}

type ForkSkillResult struct {
	SkillID           pgtype.UUID
	SourceAuditDigest Digest
}

type Lifecycle struct {
	store           lifecycleStore
	skills          WorkspaceSkillLoader
	publisher       SkillPublisher
	signals         *signalSet
	improver        Improver
	evaluator       BehavioralEvaluator
	recommendations ImprovementRecommendationSource
	forker          SkillForker
	policy          CandidatePolicy
	replayTime      time.Duration
	now             func() time.Time
}

// SetSkillForker is a composition hook for T8's existing Skill creation
// workflow. It is intentionally not part of SkillEvolution's caller surface.
func (l *Lifecycle) SetSkillForker(forker SkillForker) {
	if l != nil {
		l.forker = forker
	}
}

// SetImprovementRecommendationSource composes the Room-owned accepted outcome
// reader without importing Room orchestration into this module.
func (l *Lifecycle) SetImprovementRecommendationSource(source ImprovementRecommendationSource) {
	if l != nil {
		l.recommendations = source
	}
}

func NewLifecycle(
	store *Store,
	skills WorkspaceSkillLoader,
	publisher SkillPublisher,
	improver Improver,
	evaluator BehavioralEvaluator,
	sources ...SignalSource,
) (*Lifecycle, error) {
	signalSet, err := newSignalSet(sources)
	if err != nil {
		return nil, err
	}
	if store == nil || skills == nil || publisher == nil || improver == nil || evaluator == nil {
		return nil, ErrLifecycleInvalidInput
	}
	return &Lifecycle{
		store: store, skills: skills, publisher: publisher, signals: signalSet,
		improver: improver, evaluator: evaluator, policy: DefaultCandidatePolicy(),
		replayTime: DefaultReplayTimeout, now: time.Now,
	}, nil
}

type EvolutionOverview struct {
	Skill     WorkspaceSkillSnapshot
	Loop      *db.SkillEvolutionLoop
	Revisions []db.SkillEvolutionRevision
	Proposals []db.SkillEvolutionProposal
	Releases  []db.SkillEvolutionRelease
}

func (l *Lifecycle) Overview(ctx context.Context, workspaceID, skillID pgtype.UUID, limit int) (EvolutionOverview, error) {
	if l == nil || l.store == nil || l.skills == nil || !validUUID(workspaceID) || !validUUID(skillID) || !validPageSize(limit) {
		return EvolutionOverview{}, ErrLifecycleInvalidInput
	}
	skill, err := l.skills.Load(ctx, workspaceID, skillID)
	if err != nil {
		return EvolutionOverview{}, err
	}
	if skill.Skill.WorkspaceID != workspaceID || skill.Skill.ID != skillID {
		return EvolutionOverview{}, ErrWorkspaceSkillNotFound
	}
	stored, err := l.store.GetOverview(ctx, workspaceID, skillID, limit)
	if errors.Is(err, ErrPersistenceNotFound) {
		return EvolutionOverview{Skill: skill}, nil
	}
	if err != nil {
		return EvolutionOverview{}, err
	}
	return EvolutionOverview{
		Skill: skill, Loop: &stored.Loop, Revisions: stored.Revisions,
		Proposals: stored.Proposals, Releases: stored.Releases,
	}, nil
}

type ProposalView struct {
	Detail    ProposalDetail
	Base      RevisionSnapshot
	Candidate *RevisionSnapshot
	Rationale *ImprovementRationale
}

type ImprovementRationale struct {
	ObservedPattern string `json:"observed_pattern"`
	ExpectedBenefit string `json:"expected_benefit"`
	RegressionRisk  string `json:"regression_risk"`
}

func (l *Lifecycle) ReadProposal(ctx context.Context, workspaceID, proposalID pgtype.UUID) (ProposalView, error) {
	if l == nil || l.store == nil {
		return ProposalView{}, ErrLifecycleInvalidInput
	}
	detail, err := l.store.GetProposalDetail(ctx, workspaceID, proposalID)
	if err != nil {
		return ProposalView{}, err
	}
	base, err := l.store.GetRevisionSnapshot(ctx, workspaceID, detail.Proposal.BaseRevisionID)
	if err != nil {
		return ProposalView{}, err
	}
	view := ProposalView{Detail: detail, Base: base}
	if detail.Proposal.CandidateRevisionID.Valid {
		candidate, err := l.store.GetRevisionSnapshot(ctx, workspaceID, detail.Proposal.CandidateRevisionID)
		if err != nil {
			return ProposalView{}, err
		}
		view.Candidate = &candidate
	}
	view.Rationale = readImprovementRationale(detail)
	return view, nil
}

func (l *Lifecycle) Configure(ctx context.Context, actor DecisionActor, config LoopConfig) (db.SkillEvolutionLoop, error) {
	if l == nil || l.store == nil || !actor.human() {
		return db.SkillEvolutionLoop{}, ErrHumanActorRequired
	}
	if !validLifecycleLoopConfig(config) {
		return db.SkillEvolutionLoop{}, ErrLifecycleInvalidInput
	}
	if config.Enabled {
		if _, err := l.snapshotLiveRevision(ctx, config.WorkspaceID, config.SkillID, "base"); err != nil {
			return db.SkillEvolutionLoop{}, err
		}
	}
	return l.store.ConfigureLoop(ctx, config)
}

func (l *Lifecycle) Enable(ctx context.Context, actor DecisionActor, config LoopConfig) (db.SkillEvolutionLoop, error) {
	config.Enabled = true
	return l.Configure(ctx, actor, config)
}

func (l *Lifecycle) Pause(ctx context.Context, actor DecisionActor, workspaceID, skillID pgtype.UUID) (db.SkillEvolutionLoop, error) {
	if l == nil || !actor.human() {
		return db.SkillEvolutionLoop{}, ErrHumanActorRequired
	}
	loop, err := l.store.GetLoop(ctx, workspaceID, skillID)
	if err != nil {
		return db.SkillEvolutionLoop{}, err
	}
	config, err := loopConfigFromRow(loop)
	if err != nil {
		return db.SkillEvolutionLoop{}, err
	}
	config.Mode = LoopModePaused
	return l.store.ConfigureLoop(ctx, config)
}

type ObserveRequest struct {
	WorkspaceID pgtype.UUID
	SkillID     pgtype.UUID
	ActorID     pgtype.UUID
}

type Observation struct {
	Loop       db.SkillEvolutionLoop
	References []EvidenceRef
}

func (l *Lifecycle) Observe(ctx context.Context, request ObserveRequest) (Observation, error) {
	loop, refs, err := l.discover(ctx, request.WorkspaceID, request.SkillID, request.ActorID, false)
	if err != nil {
		return Observation{}, err
	}
	now := l.now().UTC()
	observed, err := l.store.RecordLoopObservation(ctx, request.WorkspaceID, loop.ID, now, now.Add(time.Duration(loop.CooldownSeconds)*time.Second))
	if err != nil {
		return Observation{}, err
	}
	return Observation{Loop: observed, References: refs}, nil
}

type GenerateRequest struct {
	WorkspaceID   pgtype.UUID
	SkillID       pgtype.UUID
	RequestedByID pgtype.UUID
	GenerationKey string
}

type Generation struct {
	Proposal   db.SkillEvolutionProposal
	Candidate  RevisionSnapshot
	Validation db.SkillEvolutionEvaluation
	Replay     db.SkillEvolutionEvaluation
	Replayed   bool
}

type RoomRecommendationRequest struct {
	WorkspaceID      pgtype.UUID
	SkillID          pgtype.UUID
	RecommendationID string
	IdempotencyKey   string
}

type AcceptedImprovementRecommendation struct {
	WorkspaceID      pgtype.UUID
	SkillID          pgtype.UUID
	RecommendationID string
	ExpectedBaseHash Digest
	AcceptedByID     pgtype.UUID
	Candidate        ImprovementCandidate
	Evidence         []ResolvedEvidence
}

// ImprovementRecommendationSource loads only a human-accepted Improvement
// Room executable_procedure outcome and revalidates its exact revision.
type ImprovementRecommendationSource interface {
	LoadAcceptedImprovement(context.Context, RoomRecommendationRequest) (AcceptedImprovementRecommendation, error)
}

func (l *Lifecycle) Generate(ctx context.Context, request GenerateRequest) (Generation, error) {
	if l == nil || !validUUID(request.WorkspaceID) || !validUUID(request.SkillID) ||
		!validOptionalUUID(request.RequestedByID) || !boundedToken(request.GenerationKey, MaxIdempotencyKeyBytes) {
		return Generation{}, ErrLifecycleInvalidInput
	}
	var queuedProposal db.SkillEvolutionProposal
	if existing, err := l.store.getProposalByKey(ctx, request.WorkspaceID, request.SkillID, request.GenerationKey); err == nil {
		switch ProposalState(existing.State) {
		case ProposalStateQueued:
			queuedProposal = existing
		case ProposalStateRunning:
			return Generation{Proposal: existing, Replayed: true}, ErrGenerationActive
		default:
			return Generation{Proposal: existing, Replayed: true}, nil
		}
	} else if !errors.Is(err, ErrPersistenceNotFound) {
		return Generation{}, err
	}
	live, err := l.loadEligibleSkill(ctx, request.WorkspaceID, request.SkillID)
	if err != nil {
		return Generation{}, err
	}
	loop, refs, err := l.discover(ctx, request.WorkspaceID, request.SkillID, request.RequestedByID, true)
	if err != nil {
		return Generation{}, err
	}
	if len(refs) < int(loop.MinimumSignals) {
		return Generation{}, ErrInsufficientSignals
	}
	base, err := l.ensureRevision(ctx, live, "base")
	if err != nil {
		return Generation{}, err
	}
	proposal := queuedProposal
	if !validUUID(proposal.ID) {
		proposal, err = l.store.CreateProposal(ctx, ProposalInput{
			WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, LoopID: loop.ID,
			BaseRevisionID: base.Revision.ID, BaseHash: Digest(live.Manifest.Hash),
			GenerationKey: request.GenerationKey, RequestedByID: request.RequestedByID,
		})
		if errors.Is(err, ErrPersistenceConflict) {
			return Generation{}, ErrGenerationActive
		}
		if err != nil {
			return Generation{}, err
		}
	} else if proposal.BaseRevisionID != base.Revision.ID || proposal.BaseHash != live.Manifest.Hash || proposal.LoopID != loop.ID {
		return Generation{Proposal: proposal, Replayed: true}, ErrPersistenceConflict
	}
	if ProposalState(proposal.State) != ProposalStateQueued {
		if ProposalState(proposal.State) == ProposalStateRunning {
			return Generation{Proposal: proposal, Replayed: true}, ErrGenerationActive
		}
		return Generation{Proposal: proposal, Replayed: true}, nil
	}
	proposal, err = l.store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID,
		ExpectedState: ProposalStateQueued, NextState: ProposalStateRunning,
	})
	if errors.Is(err, ErrPersistenceConflict) {
		current, loadErr := l.store.GetProposal(ctx, request.WorkspaceID, proposal.ID)
		if loadErr != nil {
			return Generation{}, loadErr
		}
		return Generation{Proposal: current, Replayed: true}, ErrGenerationActive
	}
	if err != nil {
		return Generation{}, err
	}
	now := l.now().UTC()
	if _, err := l.store.RecordLoopObservation(ctx, request.WorkspaceID, loop.ID, now, now.Add(time.Duration(loop.CooldownSeconds)*time.Second)); err != nil {
		failed, transitionErr := l.failProposal(ctx, proposal, "cooldown_persistence_failed")
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err)
	}

	query := SignalQuery{WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, ActorID: request.RequestedByID, Limit: int(loop.MaxEvidenceRefs)}
	resolved, err := l.signals.resolve(ctx, query, refs)
	if err != nil || len(resolved) < int(loop.MinimumSignals) {
		failed, transitionErr := l.failProposal(ctx, proposal, "evidence_revalidation_failed")
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		if err == nil {
			err = ErrInsufficientSignals
		}
		return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err)
	}
	for _, evidence := range resolved {
		if _, err := l.store.RecordEvidence(ctx, proposal.ID, evidence.Ref); err != nil {
			failed, transitionErr := l.failProposal(ctx, proposal, "evidence_persistence_failed")
			if transitionErr != nil {
				return Generation{}, transitionErr
			}
			return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err)
		}
	}

	candidate, err := l.improver.Improve(ctx, ImprovementRequest{
		Base: live.Bundle, Evidence: resolved, PolicyVersion: loop.PolicyVersion,
		MaxCostUSDTicks: loop.MaxCostUsdTicks, MaxChangedFiles: l.policy.MaxChangedFiles,
		MaxPrimaryGrowth: l.policy.MaxPrimaryGrowth,
	})
	if err != nil {
		failed, transitionErr := l.failProposal(ctx, proposal, "improver_failed")
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err)
	}
	return l.completeCandidate(ctx, request.WorkspaceID, request.GenerationKey, loop, proposal, live, resolved, candidate)
}

func (l *Lifecycle) completeCandidate(
	ctx context.Context,
	workspaceID pgtype.UUID,
	generationKey string,
	loop db.SkillEvolutionLoop,
	proposal db.SkillEvolutionProposal,
	live WorkspaceSkillSnapshot,
	resolved []ResolvedEvidence,
	candidate ImprovementCandidate,
) (Generation, error) {
	rationaleDigest, err := improvementRationaleDigest(candidate)
	if err != nil {
		failed, transitionErr := l.failProposal(ctx, proposal, "rationale_invalid")
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err)
	}
	validationPolicy := l.policy
	validationPolicy.MaxCostUSDTicks = loop.MaxCostUsdTicks
	validation := ValidateCandidatePolicy(live.Bundle, candidate, resolved, validationPolicy)
	validationRow, err := l.store.RecordEvaluation(ctx, EvaluationInput{
		WorkspaceID: workspaceID, ProposalID: proposal.ID, Kind: "deterministic_validation",
		Result: validation.Result, Adapter: DeterministicValidatorName, AdapterVersion: DeterministicValidatorVersion,
		PolicyVersion: loop.PolicyVersion, ResultDigest: validation.Digest, SafeMetrics: candidateValidationMetrics(validation, candidate),
		CostUSDTicks: evaluationCost(candidate.CostUSDTicks), IdempotencyKey: lifecycleKey("validation", generationKey),
	})
	if err != nil {
		failed, transitionErr := l.failProposal(ctx, proposal, "validation_persistence_failed")
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err)
	}
	if validation.Result != EvaluationResultPassed {
		failed, transitionErr := l.failProposal(ctx, proposal, "deterministic_validation_failed")
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed, Validation: validationRow}, ErrEvaluationFailed
	}

	candidateRevision, err := l.ensureCandidateRevision(ctx, live, candidate.Bundle)
	if err != nil {
		failed, transitionErr := l.failProposal(ctx, proposal, "candidate_persistence_failed")
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed, Validation: validationRow}, errors.Join(ErrGenerationFailed, err)
	}
	remainingCost := loop.MaxCostUsdTicks - candidate.CostUSDTicks
	replay, err := l.evaluator.Evaluate(ctx, ReplayRequest{
		Base: live.Bundle, Candidate: candidate.Bundle, Evidence: resolved,
		Limits: ReplayLimits{Timeout: l.replayTime, MaxSamples: int(loop.MaxReplaySamples), MaxCostUSDTicks: remainingCost, PolicyVersion: loop.PolicyVersion},
	})
	if err != nil {
		replay = finalizeReplayOutcome(ReplayOutcome{
			ReplayResult: ReplayResult{Result: EvaluationResultUnknown, ReasonCode: "adapter_unavailable"},
			Adapter:      "behavioral-replay", AdapterVersion: "v1",
		})
	}
	replay = enforceReplayLimits(replay, ReplayLimits{
		Timeout: l.replayTime, MaxSamples: int(loop.MaxReplaySamples), MaxCostUSDTicks: remainingCost, PolicyVersion: loop.PolicyVersion,
	})
	replayRow, recordErr := l.store.RecordEvaluation(ctx, EvaluationInput{
		WorkspaceID: workspaceID, ProposalID: proposal.ID, Kind: "behavioral_replay",
		Result: replay.Result, Adapter: replay.Adapter, AdapterVersion: replay.AdapterVersion,
		PolicyVersion: loop.PolicyVersion, ResultDigest: replay.Digest, SafeMetrics: replay.SafeMetrics(),
		CostUSDTicks: evaluationCost(replay.CostUSDTicks), Duration: replay.Duration,
		IdempotencyKey: lifecycleKey("replay", generationKey),
	})
	if recordErr != nil {
		failed, transitionErr := l.failProposalWithCandidate(ctx, proposal, candidateRevision, "evaluation_persistence_failed", rationaleDigest)
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed, Candidate: candidateRevision, Validation: validationRow}, errors.Join(ErrGenerationFailed, recordErr)
	}
	if replay.Result != EvaluationResultPassed {
		failed, transitionErr := l.failProposalWithCandidate(ctx, proposal, candidateRevision, "behavioral_evaluation_failed", rationaleDigest)
		if transitionErr != nil {
			return Generation{}, transitionErr
		}
		return Generation{Proposal: failed, Candidate: candidateRevision, Validation: validationRow, Replay: replayRow}, ErrEvaluationFailed
	}
	ready, err := l.store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: workspaceID, ProposalID: proposal.ID,
		ExpectedState: ProposalStateRunning, NextState: ProposalStateReady,
		CandidateRevisionID: candidateRevision.Revision.ID, CandidateHash: Digest(candidateRevision.Revision.BundleHash),
		RationaleDigest: rationaleDigest,
	})
	if err != nil {
		failed, cleanupErr := l.failProposalWithCandidate(ctx, proposal, candidateRevision, "ready_persistence_failed", rationaleDigest)
		return Generation{Proposal: failed, Candidate: candidateRevision, Validation: validationRow, Replay: replayRow}, errors.Join(err, cleanupErr)
	}
	return Generation{Proposal: ready, Candidate: candidateRevision, Validation: validationRow, Replay: replayRow}, nil
}

func (l *Lifecycle) CreateProposalFromRoomRecommendation(ctx context.Context, request RoomRecommendationRequest) (Generation, error) {
	if l == nil || l.recommendations == nil || !validUUID(request.WorkspaceID) || !validUUID(request.SkillID) ||
		!boundedToken(request.RecommendationID, 160) || !boundedToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return Generation{}, ErrLifecycleInvalidInput
	}
	generationKey := lifecycleKey("room-recommendation", request.IdempotencyKey)
	var queued db.SkillEvolutionProposal
	if existing, err := l.store.getProposalByKey(ctx, request.WorkspaceID, request.SkillID, generationKey); err == nil {
		switch ProposalState(existing.State) {
		case ProposalStateQueued:
			queued = existing
		case ProposalStateRunning:
			return Generation{Proposal: existing, Replayed: true}, ErrGenerationActive
		default:
			return Generation{Proposal: existing, Replayed: true}, nil
		}
	} else if !errors.Is(err, ErrPersistenceNotFound) {
		return Generation{}, err
	}
	accepted, err := l.recommendations.LoadAcceptedImprovement(ctx, request)
	if err != nil {
		return Generation{}, err
	}
	if accepted.WorkspaceID != request.WorkspaceID || accepted.SkillID != request.SkillID ||
		accepted.RecommendationID != request.RecommendationID || !accepted.ExpectedBaseHash.Valid() ||
		!validUUID(accepted.AcceptedByID) {
		return Generation{}, ErrLifecycleInvalidInput
	}
	live, err := l.loadEligibleSkill(ctx, request.WorkspaceID, request.SkillID)
	if err != nil {
		return Generation{}, err
	}
	if Digest(live.Manifest.Hash) != accepted.ExpectedBaseHash {
		return Generation{}, &StaleBaseError{Expected: accepted.ExpectedBaseHash, Current: Digest(live.Manifest.Hash)}
	}
	loop, err := l.requireGenerationLoop(ctx, request.WorkspaceID, request.SkillID)
	if err != nil {
		return Generation{}, err
	}
	if err := validateAcceptedImprovement(request, accepted, int(loop.MinimumSignals), int(loop.MaxEvidenceRefs)); err != nil {
		return Generation{}, err
	}
	base, err := l.ensureRevision(ctx, live, "base")
	if err != nil {
		return Generation{}, err
	}
	proposal := queued
	if !validUUID(proposal.ID) {
		proposal, err = l.store.CreateProposal(ctx, ProposalInput{
			WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, LoopID: loop.ID,
			BaseRevisionID: base.Revision.ID, BaseHash: accepted.ExpectedBaseHash,
			GenerationKey: generationKey, RequestedByID: accepted.AcceptedByID,
		})
		if errors.Is(err, ErrPersistenceConflict) {
			return Generation{}, ErrGenerationActive
		}
		if err != nil {
			return Generation{}, err
		}
	} else if proposal.BaseRevisionID != base.Revision.ID || proposal.BaseHash != string(accepted.ExpectedBaseHash) ||
		proposal.LoopID != loop.ID || proposal.RequestedByID != accepted.AcceptedByID {
		return Generation{Proposal: proposal, Replayed: true}, ErrPersistenceConflict
	}
	proposal, err = l.store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID,
		ExpectedState: ProposalStateQueued, NextState: ProposalStateRunning,
	})
	if err != nil {
		return Generation{Proposal: proposal}, err
	}
	now := l.now().UTC()
	if _, err := l.store.RecordLoopObservation(ctx, request.WorkspaceID, loop.ID, now, now.Add(time.Duration(loop.CooldownSeconds)*time.Second)); err != nil {
		failed, cleanupErr := l.failProposal(ctx, proposal, "cooldown_persistence_failed")
		return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err, cleanupErr)
	}
	for _, evidence := range accepted.Evidence {
		if _, err := l.store.RecordEvidence(ctx, proposal.ID, evidence.Ref); err != nil {
			failed, cleanupErr := l.failProposal(ctx, proposal, "evidence_persistence_failed")
			return Generation{Proposal: failed}, errors.Join(ErrGenerationFailed, err, cleanupErr)
		}
	}
	return l.completeCandidate(ctx, request.WorkspaceID, generationKey, loop, proposal, live, accepted.Evidence, accepted.Candidate)
}

type RejectRequest struct {
	WorkspaceID    pgtype.UUID
	ProposalID     pgtype.UUID
	Actor          DecisionActor
	Reason         string
	IdempotencyKey string
}

func (l *Lifecycle) Reject(ctx context.Context, request RejectRequest) (db.SkillEvolutionProposal, error) {
	if l == nil || !request.Actor.human() {
		return db.SkillEvolutionProposal{}, ErrHumanActorRequired
	}
	if !validUUID(request.WorkspaceID) || !validUUID(request.ProposalID) ||
		!boundedToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) || len([]rune(request.Reason)) > MaxReviewReasonRunes {
		return db.SkillEvolutionProposal{}, ErrLifecycleInvalidInput
	}
	proposal, err := l.store.GetProposal(ctx, request.WorkspaceID, request.ProposalID)
	if err != nil {
		return db.SkillEvolutionProposal{}, err
	}
	if ProposalState(proposal.State) == ProposalStateRejected {
		detail, detailErr := l.store.GetProposalDetail(ctx, request.WorkspaceID, request.ProposalID)
		if detailErr == nil && reviewMatches(detail.Reviews, proposal.CandidateRevisionID, "rejected", request.Actor.ID, request.Reason, request.IdempotencyKey) {
			return proposal, nil
		}
		if detailErr != nil {
			return db.SkillEvolutionProposal{}, detailErr
		}
		return db.SkillEvolutionProposal{}, ErrPersistenceConflict
	}
	if ProposalState(proposal.State) != ProposalStateReady || !validUUID(proposal.CandidateRevisionID) || !proposal.CandidateHash.Valid {
		return db.SkillEvolutionProposal{}, ErrDecisionConflict
	}
	if _, err := l.store.RecordReview(ctx, ReviewInput{
		WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID, CandidateRevisionID: proposal.CandidateRevisionID,
		Decision: "rejected", ActorID: request.Actor.ID, Reason: request.Reason, IdempotencyKey: request.IdempotencyKey,
	}); err != nil {
		return db.SkillEvolutionProposal{}, err
	}
	return l.store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID,
		ExpectedState: ProposalStateReady, NextState: ProposalStateRejected,
	})
}

type PublishRequest struct {
	WorkspaceID    pgtype.UUID
	ProposalID     pgtype.UUID
	Actor          DecisionActor
	Reason         string
	IdempotencyKey string
}

type RollbackRequest struct {
	WorkspaceID     pgtype.UUID
	SkillID         pgtype.UUID
	SourceReleaseID pgtype.UUID
	Actor           DecisionActor
	IdempotencyKey  string
}

type Publication struct {
	Proposal db.SkillEvolutionProposal
	Release  db.SkillEvolutionRelease
	Result   PublishSkillResult
}

func (l *Lifecycle) Publish(ctx context.Context, request PublishRequest) (Publication, error) {
	if l == nil || !request.Actor.human() {
		return Publication{}, ErrHumanActorRequired
	}
	if !validUUID(request.WorkspaceID) || !validUUID(request.ProposalID) ||
		!boundedToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) || len([]rune(request.Reason)) > MaxReviewReasonRunes {
		return Publication{}, ErrLifecycleInvalidInput
	}
	existing, existingErr := l.store.getReleaseByKey(ctx, request.WorkspaceID, request.IdempotencyKey)
	if existingErr != nil && !errors.Is(existingErr, ErrPersistenceNotFound) {
		return Publication{}, existingErr
	}
	proposal, err := l.store.GetProposal(ctx, request.WorkspaceID, request.ProposalID)
	if err != nil {
		return Publication{}, err
	}
	if existingErr == nil {
		if !releaseMatchesInput(existing, ReleaseInput{
			WorkspaceID: request.WorkspaceID, SkillID: proposal.SkillID, ProposalID: proposal.ID,
			RevisionID: proposal.CandidateRevisionID, Kind: ReleaseKindPublish,
			ExpectedBaseHash: Digest(proposal.BaseHash), ActorID: request.Actor.ID,
			IdempotencyKey: request.IdempotencyKey,
		}) || !publishReviewMatches(ctx, l.store, proposal, request) {
			return Publication{}, ErrPersistenceConflict
		}
		return Publication{Proposal: proposal, Release: existing}, releaseReplayError(existing)
	}
	if ProposalState(proposal.State) != ProposalStateReady || !validUUID(proposal.CandidateRevisionID) || !proposal.CandidateHash.Valid {
		return Publication{}, ErrDecisionConflict
	}
	loop, err := l.store.GetLoop(ctx, request.WorkspaceID, proposal.SkillID)
	if err != nil {
		return Publication{}, err
	}
	if !loop.Enabled {
		return Publication{}, ErrEvolutionDisabled
	}
	if LoopMode(loop.Mode) == LoopModePaused {
		return Publication{}, ErrEvolutionPaused
	}
	candidate, err := l.store.GetRevisionSnapshot(ctx, request.WorkspaceID, proposal.CandidateRevisionID)
	if err != nil {
		return Publication{}, err
	}
	live, err := l.loadEligibleSkill(ctx, request.WorkspaceID, proposal.SkillID)
	if err != nil {
		return Publication{}, err
	}
	if Digest(live.Manifest.Hash) != Digest(proposal.BaseHash) {
		stale, transitionErr := l.store.TransitionProposal(ctx, ProposalTransition{
			WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID,
			ExpectedState: ProposalStateReady, NextState: ProposalStateStale, StaleReason: "live_bundle_changed",
		})
		if transitionErr != nil {
			return Publication{}, transitionErr
		}
		return Publication{Proposal: stale}, &StaleBaseError{Expected: Digest(proposal.BaseHash), Current: Digest(live.Manifest.Hash)}
	}
	if _, err := l.store.RecordReview(ctx, ReviewInput{
		WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID, CandidateRevisionID: proposal.CandidateRevisionID,
		Decision: "publish", ActorID: request.Actor.ID, Reason: request.Reason, IdempotencyKey: lifecycleKey("review", request.IdempotencyKey),
	}); err != nil {
		return Publication{}, err
	}
	publishing, err := l.store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID,
		ExpectedState: ProposalStateReady, NextState: ProposalStatePublishing,
	})
	if err != nil {
		return Publication{}, err
	}
	release, err := l.store.CreateRelease(ctx, ReleaseInput{
		WorkspaceID: request.WorkspaceID, SkillID: proposal.SkillID, ProposalID: proposal.ID,
		RevisionID: candidate.Revision.ID, Kind: ReleaseKindPublish, ExpectedBaseHash: Digest(proposal.BaseHash),
		ActorID: request.Actor.ID, IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		recovered, recoveryErr := l.store.TransitionProposal(ctx, ProposalTransition{
			WorkspaceID: request.WorkspaceID, ProposalID: proposal.ID,
			ExpectedState: ProposalStatePublishing, NextState: ProposalStateReady,
		})
		if recoveryErr == nil {
			publishing = recovered
		}
		return Publication{Proposal: publishing}, errors.Join(err, recoveryErr)
	}
	result, publishErr := l.publisher.Publish(ctx, PublishSkillRequest{
		WorkspaceID: request.WorkspaceID, SkillID: proposal.SkillID, ExpectedBaseHash: Digest(proposal.BaseHash),
		Bundle: revisionBundle(candidate),
	})
	return l.finishPublication(ctx, publishing, release, result, publishErr)
}

func (l *Lifecycle) Rollback(ctx context.Context, request RollbackRequest) (Publication, error) {
	if l == nil || !request.Actor.human() {
		return Publication{}, ErrHumanActorRequired
	}
	if !validUUID(request.WorkspaceID) || !validUUID(request.SkillID) || !validUUID(request.SourceReleaseID) ||
		!boundedToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return Publication{}, ErrLifecycleInvalidInput
	}
	source, err := l.store.getRelease(ctx, request.WorkspaceID, request.SourceReleaseID)
	if err != nil {
		return Publication{}, err
	}
	if source.SkillID != request.SkillID || source.Outcome != string(ReleaseOutcomeSucceeded) || !source.PreHash.Valid || !source.PostHash.Valid {
		return Publication{}, ErrDecisionConflict
	}
	preHash := Digest(source.PreHash.String)
	postHash := Digest(source.PostHash.String)
	target, err := l.store.getRevisionByHash(ctx, request.WorkspaceID, request.SkillID, preHash)
	if err != nil {
		return Publication{}, err
	}
	input := ReleaseInput{
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, SourceReleaseID: source.ID,
		RevisionID: target.Revision.ID, Kind: ReleaseKindRollback, ExpectedBaseHash: postHash,
		ActorID: request.Actor.ID, IdempotencyKey: request.IdempotencyKey,
	}
	if existing, replayErr := l.store.getReleaseByKey(ctx, request.WorkspaceID, request.IdempotencyKey); replayErr == nil {
		if !releaseMatchesInput(existing, input) {
			return Publication{}, ErrPersistenceConflict
		}
		return Publication{Release: existing}, releaseReplayError(existing)
	} else if !errors.Is(replayErr, ErrPersistenceNotFound) {
		return Publication{}, replayErr
	}
	live, err := l.loadEligibleSkill(ctx, request.WorkspaceID, request.SkillID)
	if err != nil {
		return Publication{}, err
	}
	release, err := l.store.CreateRelease(ctx, input)
	if err != nil {
		return Publication{}, err
	}
	if Digest(live.Manifest.Hash) != postHash {
		failed, transitionErr := l.store.TransitionRelease(ctx, ReleaseTransition{
			WorkspaceID: request.WorkspaceID, ReleaseID: release.ID, ExpectedOutcome: ReleaseOutcomePending,
			NextOutcome: ReleaseOutcomeFailed, ErrorCode: "stale_base",
		})
		if transitionErr != nil {
			return Publication{Release: release}, transitionErr
		}
		return Publication{Release: failed}, &StaleBaseError{Expected: postHash, Current: Digest(live.Manifest.Hash)}
	}
	result, publishErr := l.publisher.Publish(ctx, PublishSkillRequest{
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, ExpectedBaseHash: postHash,
		Bundle: revisionBundle(target),
	})
	return l.finishPublication(ctx, db.SkillEvolutionProposal{}, release, result, publishErr)
}

type ForkRequest struct {
	WorkspaceID    pgtype.UUID
	SourceSkillID  pgtype.UUID
	NewName        string
	Actor          DecisionActor
	IdempotencyKey string
}

type Fork struct {
	SourceOwnership Ownership
	SourceHash      Digest
	SourceAudit     Digest
	Skill           WorkspaceSkillSnapshot
	Base            RevisionSnapshot
}

func (l *Lifecycle) Fork(ctx context.Context, request ForkRequest) (Fork, error) {
	if l == nil || !request.Actor.human() {
		return Fork{}, ErrHumanActorRequired
	}
	if l.forker == nil {
		return Fork{}, ErrSkillForkUnavailable
	}
	if !validUUID(request.WorkspaceID) || !validUUID(request.SourceSkillID) ||
		!boundedToken(request.NewName, 255) || !boundedToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return Fork{}, ErrLifecycleInvalidInput
	}
	source, err := l.skills.Load(ctx, request.WorkspaceID, request.SourceSkillID)
	if err != nil {
		return Fork{}, err
	}
	if source.Skill.WorkspaceID != request.WorkspaceID || source.Skill.ID != request.SourceSkillID {
		return Fork{}, ErrWorkspaceSkillNotFound
	}
	if source.Ownership.DirectEvolution || !source.Ownership.ForkRequired {
		return Fork{}, ErrDecisionConflict
	}
	sourceHash := Digest(source.Manifest.Hash)
	result, err := l.forker.ForkSkill(ctx, ForkSkillInput{
		WorkspaceID: request.WorkspaceID, SourceSkillID: request.SourceSkillID, ExpectedSourceHash: sourceHash,
		NewName: request.NewName, ActorID: request.Actor.ID, IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		return Fork{}, err
	}
	if !validUUID(result.SkillID) || result.SkillID == request.SourceSkillID || !result.SourceAuditDigest.Valid() {
		return Fork{}, ErrLifecycleInvalidInput
	}
	forked, err := l.loadEligibleSkill(ctx, request.WorkspaceID, result.SkillID)
	if err != nil {
		return Fork{}, err
	}
	base, err := l.ensureRevision(ctx, forked, "base")
	if err != nil {
		return Fork{}, err
	}
	return Fork{
		SourceOwnership: source.Ownership, SourceHash: sourceHash, SourceAudit: result.SourceAuditDigest,
		Skill: forked, Base: base,
	}, nil
}

func (l *Lifecycle) finishPublication(
	ctx context.Context,
	proposal db.SkillEvolutionProposal,
	release db.SkillEvolutionRelease,
	result PublishSkillResult,
	publishErr error,
) (Publication, error) {
	publication := Publication{Proposal: proposal, Release: release, Result: result}
	if publishErr != nil {
		outcome := ReleaseOutcomeFailed
		errorCode := publicationErrorCode(publishErr)
		if errors.Is(publishErr, ErrPublicationUnknown) {
			outcome = ReleaseOutcomePublicationUnknown
		}
		updated, transitionErr := l.store.TransitionRelease(ctx, ReleaseTransition{
			WorkspaceID: release.WorkspaceID, ReleaseID: release.ID, ExpectedOutcome: ReleaseOutcomePending,
			NextOutcome: outcome, ErrorCode: errorCode,
		})
		if transitionErr != nil {
			return publication, errors.Join(publishErr, transitionErr)
		}
		publication.Release = updated
		if validUUID(proposal.ID) {
			next := ProposalStateReady
			staleReason := ""
			if outcome == ReleaseOutcomePublicationUnknown {
				next = ProposalStatePublicationUnknown
			} else if errors.Is(publishErr, ErrStaleBase) {
				next = ProposalStateStale
				staleReason = "publisher_stale_base"
			}
			updatedProposal, proposalErr := l.store.TransitionProposal(ctx, ProposalTransition{
				WorkspaceID: proposal.WorkspaceID, ProposalID: proposal.ID,
				ExpectedState: ProposalStatePublishing, NextState: next, StaleReason: staleReason,
			})
			if proposalErr != nil {
				return publication, errors.Join(publishErr, proposalErr)
			}
			publication.Proposal = updatedProposal
		}
		return publication, publishErr
	}
	updated, err := l.store.TransitionRelease(ctx, ReleaseTransition{
		WorkspaceID: release.WorkspaceID, ReleaseID: release.ID, ExpectedOutcome: ReleaseOutcomePending,
		NextOutcome: ReleaseOutcomeSucceeded, PreHash: result.PreHash, PostHash: result.PostHash,
	})
	if err != nil {
		unknown, unknownErr := l.store.TransitionRelease(ctx, ReleaseTransition{
			WorkspaceID: release.WorkspaceID, ReleaseID: release.ID, ExpectedOutcome: ReleaseOutcomePending,
			NextOutcome: ReleaseOutcomePublicationUnknown, ErrorCode: "release_recording_failed",
		})
		if unknownErr == nil {
			publication.Release = unknown
		}
		var proposalErr error
		if validUUID(proposal.ID) {
			publication.Proposal, proposalErr = l.store.TransitionProposal(ctx, ProposalTransition{
				WorkspaceID: proposal.WorkspaceID, ProposalID: proposal.ID,
				ExpectedState: ProposalStatePublishing, NextState: ProposalStatePublicationUnknown,
			})
		}
		return publication, &PublicationUnknownError{ExpectedPostHash: result.PostHash, Cause: errors.Join(err, unknownErr, proposalErr)}
	}
	publication.Release = updated
	if validUUID(proposal.ID) {
		published, proposalErr := l.store.TransitionProposal(ctx, ProposalTransition{
			WorkspaceID: proposal.WorkspaceID, ProposalID: proposal.ID,
			ExpectedState: ProposalStatePublishing, NextState: ProposalStatePublished,
		})
		if proposalErr != nil {
			unknown, unknownErr := l.store.TransitionProposal(ctx, ProposalTransition{
				WorkspaceID: proposal.WorkspaceID, ProposalID: proposal.ID,
				ExpectedState: ProposalStatePublishing, NextState: ProposalStatePublicationUnknown,
			})
			if unknownErr == nil {
				publication.Proposal = unknown
			}
			return publication, &PublicationUnknownError{ExpectedPostHash: result.PostHash, Cause: errors.Join(proposalErr, unknownErr)}
		}
		publication.Proposal = published
	}
	return publication, nil
}

func (l *Lifecycle) discover(ctx context.Context, workspaceID, skillID, actorID pgtype.UUID, generation bool) (db.SkillEvolutionLoop, []EvidenceRef, error) {
	if l == nil || l.store == nil || l.signals == nil || !validUUID(workspaceID) || !validUUID(skillID) || !validOptionalUUID(actorID) {
		return db.SkillEvolutionLoop{}, nil, ErrLifecycleInvalidInput
	}
	loop, err := l.store.GetLoop(ctx, workspaceID, skillID)
	if err != nil {
		return db.SkillEvolutionLoop{}, nil, err
	}
	if !loop.Enabled {
		return db.SkillEvolutionLoop{}, nil, ErrEvolutionDisabled
	}
	mode := LoopMode(loop.Mode)
	if mode == LoopModePaused {
		return db.SkillEvolutionLoop{}, nil, ErrEvolutionPaused
	}
	if generation && mode != LoopModePropose {
		return db.SkillEvolutionLoop{}, nil, ErrEvolutionObserveOnly
	}
	now := l.now().UTC()
	if generation && loop.NextEligibleAt.Valid && now.Before(loop.NextEligibleAt.Time) {
		return db.SkillEvolutionLoop{}, nil, ErrEvolutionCooldown
	}
	refs, err := l.signals.discover(ctx, SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, ActorID: actorID, Limit: int(loop.MaxEvidenceRefs)})
	return loop, refs, err
}

func (l *Lifecycle) requireGenerationLoop(ctx context.Context, workspaceID, skillID pgtype.UUID) (db.SkillEvolutionLoop, error) {
	loop, err := l.store.GetLoop(ctx, workspaceID, skillID)
	if err != nil {
		return db.SkillEvolutionLoop{}, err
	}
	if !loop.Enabled {
		return db.SkillEvolutionLoop{}, ErrEvolutionDisabled
	}
	if LoopMode(loop.Mode) == LoopModePaused {
		return db.SkillEvolutionLoop{}, ErrEvolutionPaused
	}
	if LoopMode(loop.Mode) != LoopModePropose {
		return db.SkillEvolutionLoop{}, ErrEvolutionObserveOnly
	}
	if loop.NextEligibleAt.Valid && l.now().UTC().Before(loop.NextEligibleAt.Time) {
		return db.SkillEvolutionLoop{}, ErrEvolutionCooldown
	}
	return loop, nil
}

func validateAcceptedImprovement(request RoomRecommendationRequest, accepted AcceptedImprovementRecommendation, minimum, maximum int) error {
	if len(accepted.Evidence) < minimum || len(accepted.Evidence) > maximum || len(accepted.Evidence) > MaxEvidenceRefs {
		return ErrInsufficientSignals
	}
	workspaceID := uuid.UUID(request.WorkspaceID.Bytes).String()
	skillID := uuid.UUID(request.SkillID.Bytes).String()
	seen := make(map[string]struct{}, len(accepted.Evidence))
	totalBytes := 0
	for _, evidence := range accepted.Evidence {
		ref := evidence.Ref
		identity := string(ref.Kind) + "\x00" + ref.SourceID + "\x00" + ref.SourceRevisionID
		if ref.Validate() != nil || ref.WorkspaceID != workspaceID || ref.Eligibility != EvidenceEligibilityEligible ||
			(ref.TargetSkillID != "" && ref.TargetSkillID != skillID) || len(evidence.Payload) > MaxResolvedEvidenceBytes {
			return ErrSignalSourceInvalid
		}
		if _, duplicate := seen[identity]; duplicate {
			return ErrSignalSourceInvalid
		}
		seen[identity] = struct{}{}
		totalBytes += len(evidence.Payload)
		if totalBytes > MaxResolvedEvidenceBytes {
			return ErrSignalSourceInvalid
		}
	}
	if !candidateClaimsResolvedEvidence(accepted.Candidate.EvidenceDigests, accepted.Evidence) {
		return ErrImproverOutput
	}
	return nil
}

func (l *Lifecycle) loadEligibleSkill(ctx context.Context, workspaceID, skillID pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	snapshot, err := l.skills.Load(ctx, workspaceID, skillID)
	if err != nil {
		return WorkspaceSkillSnapshot{}, err
	}
	if snapshot.Skill.WorkspaceID != workspaceID || snapshot.Skill.ID != skillID ||
		snapshot.Ownership.Class != OwnershipWorkspace || !snapshot.Ownership.DirectEvolution {
		return WorkspaceSkillSnapshot{}, &ForkRequiredError{Ownership: snapshot.Ownership}
	}
	return snapshot, nil
}

func (l *Lifecycle) snapshotLiveRevision(ctx context.Context, workspaceID, skillID pgtype.UUID, kind string) (RevisionSnapshot, error) {
	live, err := l.loadEligibleSkill(ctx, workspaceID, skillID)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	return l.ensureRevision(ctx, live, kind)
}

func (l *Lifecycle) ensureRevision(ctx context.Context, snapshot WorkspaceSkillSnapshot, kind string) (RevisionSnapshot, error) {
	hash := Digest(snapshot.Manifest.Hash)
	existing, err := l.store.getRevisionByHash(ctx, snapshot.Skill.WorkspaceID, snapshot.Skill.ID, hash)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrPersistenceNotFound) {
		return RevisionSnapshot{}, err
	}
	metadata, err := revisionMetadataDigest(snapshot.Bundle, hash)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	return l.store.SaveRevision(ctx, revisionInput(snapshot.Skill.WorkspaceID, snapshot.Skill.ID, snapshot.Skill.CreatedBy, kind, snapshot.Ownership.Class, snapshot.Bundle, hash, metadata))
}

func (l *Lifecycle) ensureCandidateRevision(ctx context.Context, base WorkspaceSkillSnapshot, bundle skillbundle.Skill) (RevisionSnapshot, error) {
	manifest, err := ValidateCandidateBundle(bundle)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	hash := Digest(manifest.Hash)
	existing, err := l.store.getRevisionByHash(ctx, base.Skill.WorkspaceID, base.Skill.ID, hash)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrPersistenceNotFound) {
		return RevisionSnapshot{}, err
	}
	metadata, err := revisionMetadataDigest(bundle, hash)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	return l.store.SaveRevision(ctx, revisionInput(base.Skill.WorkspaceID, base.Skill.ID, base.Skill.CreatedBy, "candidate", OwnershipWorkspace, bundle, hash, metadata))
}

func (l *Lifecycle) readyProposal(ctx context.Context, workspaceID, proposalID pgtype.UUID) (db.SkillEvolutionProposal, error) {
	proposal, err := l.store.GetProposal(ctx, workspaceID, proposalID)
	if err != nil {
		return db.SkillEvolutionProposal{}, err
	}
	if ProposalState(proposal.State) != ProposalStateReady || !validUUID(proposal.CandidateRevisionID) || !proposal.CandidateHash.Valid {
		return db.SkillEvolutionProposal{}, ErrDecisionConflict
	}
	return proposal, nil
}

func (l *Lifecycle) failProposal(ctx context.Context, proposal db.SkillEvolutionProposal, reason string) (db.SkillEvolutionProposal, error) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lifecycleCleanupTimeout)
	defer cancel()
	return l.store.TransitionProposal(cleanupCtx, ProposalTransition{
		WorkspaceID: proposal.WorkspaceID, ProposalID: proposal.ID,
		ExpectedState: ProposalStateRunning, NextState: ProposalStateFailed, FailureReason: reason,
	})
}

func (l *Lifecycle) failProposalWithCandidate(ctx context.Context, proposal db.SkillEvolutionProposal, candidate RevisionSnapshot, reason string, rationale Digest) (db.SkillEvolutionProposal, error) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lifecycleCleanupTimeout)
	defer cancel()
	return l.store.TransitionProposal(cleanupCtx, ProposalTransition{
		WorkspaceID: proposal.WorkspaceID, ProposalID: proposal.ID,
		ExpectedState: ProposalStateRunning, NextState: ProposalStateFailed, FailureReason: reason,
		CandidateRevisionID: candidate.Revision.ID, CandidateHash: Digest(candidate.Revision.BundleHash), RationaleDigest: rationale,
	})
}

func revisionInput(workspaceID, skillID, creatorID pgtype.UUID, kind string, ownership OwnershipClass, bundle skillbundle.Skill, hash, metadata Digest) RevisionInput {
	files := make([]RevisionFileInput, len(bundle.Files))
	for index, file := range bundle.Files {
		files[index] = RevisionFileInput{Path: file.Path, Content: file.Content}
	}
	return RevisionInput{
		WorkspaceID: workspaceID, SkillID: skillID, Kind: kind, Ownership: ownership,
		Source: bundle.Source, BundleHash: hash, MetadataDigest: metadata,
		Name: bundle.Name, Description: bundle.Description, PrimaryContent: bundle.Content,
		Files: files, CreatedByID: creatorID,
	}
}

func revisionBundle(snapshot RevisionSnapshot) skillbundle.Skill {
	files := make([]skillbundle.File, len(snapshot.Files))
	for index, file := range snapshot.Files {
		files[index] = skillbundle.File{Path: file.Path, Content: file.Content}
	}
	return skillbundle.Skill{
		ID: uuid.UUID(snapshot.Revision.SkillID.Bytes).String(), Source: snapshot.Revision.Source,
		Name: snapshot.Revision.Name, Description: snapshot.Revision.Description,
		Content: snapshot.Revision.PrimaryContent, Files: files,
	}
}

func revisionMetadataDigest(bundle skillbundle.Skill, hash Digest) (Digest, error) {
	return CanonicalEvidenceDigest("skill_revision_metadata", []DigestPart{
		{Key: "bundle_hash", Value: string(hash)}, {Key: "description", Value: bundle.Description},
		{Key: "name", Value: bundle.Name}, {Key: "source", Value: bundle.Source},
	})
}

func improvementRationaleDigest(candidate ImprovementCandidate) (Digest, error) {
	digests := append([]Digest(nil), candidate.EvidenceDigests...)
	sort.Slice(digests, func(i, j int) bool { return digests[i] < digests[j] })
	parts := []DigestPart{
		{Key: "expected_benefit", Value: candidate.ExpectedBenefit},
		{Key: "observed_pattern", Value: candidate.ObservedPattern},
		{Key: "regression_risk", Value: candidate.RegressionRisk},
	}
	for index, digest := range digests {
		parts = append(parts, DigestPart{Key: fmt.Sprintf("evidence.%03d", index), Value: string(digest)})
	}
	return CanonicalEvidenceDigest("improvement_rationale", parts)
}

type candidateReviewMetrics struct {
	RuleCodes          []string             `json:"rule_codes"`
	ChangedFiles       int                  `json:"changed_files"`
	AddedFiles         int                  `json:"added_files"`
	DeletedFiles       int                  `json:"deleted_files"`
	PrimaryGrowthBytes int                  `json:"primary_growth_bytes"`
	Rationale          ImprovementRationale `json:"rationale,omitempty"`
	EvidenceDigests    []Digest             `json:"evidence_digests,omitempty"`
}

func candidateValidationMetrics(validation ValidationOutcome, candidate ImprovementCandidate) json.RawMessage {
	metrics := candidateReviewMetrics{
		RuleCodes: validation.RuleCodes, ChangedFiles: validation.ChangedFiles,
		AddedFiles: validation.AddedFiles, DeletedFiles: validation.DeletedFiles,
		PrimaryGrowthBytes: validation.PrimaryGrowthBytes,
	}
	if validRationale(candidate.ObservedPattern) && validRationale(candidate.ExpectedBenefit) && validRationale(candidate.RegressionRisk) {
		metrics.Rationale = ImprovementRationale{
			ObservedPattern: candidate.ObservedPattern, ExpectedBenefit: candidate.ExpectedBenefit,
			RegressionRisk: candidate.RegressionRisk,
		}
		metrics.EvidenceDigests = append([]Digest(nil), candidate.EvidenceDigests...)
	}
	raw, _ := json.Marshal(metrics)
	return raw
}

func readImprovementRationale(detail ProposalDetail) *ImprovementRationale {
	if !detail.Proposal.RationaleDigest.Valid {
		return nil
	}
	for _, evaluation := range detail.Evaluations {
		if evaluation.Kind != "deterministic_validation" {
			continue
		}
		var metrics candidateReviewMetrics
		if json.Unmarshal(evaluation.SafeMetrics, &metrics) != nil ||
			!validRationale(metrics.Rationale.ObservedPattern) ||
			!validRationale(metrics.Rationale.ExpectedBenefit) ||
			!validRationale(metrics.Rationale.RegressionRisk) {
			continue
		}
		candidate := ImprovementCandidate{
			ObservedPattern: metrics.Rationale.ObservedPattern,
			ExpectedBenefit: metrics.Rationale.ExpectedBenefit,
			RegressionRisk:  metrics.Rationale.RegressionRisk,
			EvidenceDigests: metrics.EvidenceDigests,
		}
		digest, err := improvementRationaleDigest(candidate)
		if err == nil && string(digest) == detail.Proposal.RationaleDigest.String {
			rationale := metrics.Rationale
			return &rationale
		}
	}
	return nil
}

func loopConfigFromRow(loop db.SkillEvolutionLoop) (LoopConfig, error) {
	mode := LoopMode(loop.Mode)
	if !mode.Valid() {
		return LoopConfig{}, ErrLifecycleInvalidInput
	}
	return LoopConfig{
		WorkspaceID: loop.WorkspaceID, SkillID: loop.SkillID, Enabled: loop.Enabled, Mode: mode,
		Cooldown: time.Duration(loop.CooldownSeconds) * time.Second, MinimumSignals: int(loop.MinimumSignals),
		MaxEvidenceRefs: int(loop.MaxEvidenceRefs), MaxReplaySamples: int(loop.MaxReplaySamples),
		MaxCostUSDTicks: loop.MaxCostUsdTicks, PolicyVersion: loop.PolicyVersion,
		NextEligibleAt: optionalDatabaseTime(loop.NextEligibleAt),
	}, nil
}

func validLifecycleLoopConfig(config LoopConfig) bool {
	return validUUID(config.WorkspaceID) && validUUID(config.SkillID) && config.Mode.Valid() &&
		config.Cooldown >= time.Minute && config.Cooldown <= 30*24*time.Hour && config.Cooldown%time.Second == 0 &&
		config.MinimumSignals >= 1 && config.MinimumSignals <= MaxEvidenceRefs &&
		config.MaxEvidenceRefs >= config.MinimumSignals && config.MaxEvidenceRefs <= MaxEvidenceRefs &&
		config.MaxReplaySamples >= 1 && config.MaxReplaySamples <= 32 &&
		config.MaxCostUSDTicks >= 0 && config.MaxCostUSDTicks <= 1_000_000_000 &&
		boundedToken(config.PolicyVersion, 80)
}

func optionalDatabaseTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func lifecycleKey(namespace, key string) string {
	return namespace + ":" + string(digestSafeValue("lifecycle-idempotency-v1", key))[len("sha256:"):]
}

func publicationErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrPublicationUnknown):
		return "publication_unknown"
	case errors.Is(err, ErrStaleBase):
		return "stale_base"
	case errors.Is(err, ErrSkillNameConflict):
		return "name_conflict"
	case errors.Is(err, ErrForkRequired):
		return "fork_required"
	case errors.Is(err, ErrConcurrentBundleDrift):
		return "concurrent_drift"
	default:
		return "publisher_failed"
	}
}

func releaseReplayError(release db.SkillEvolutionRelease) error {
	switch ReleaseOutcome(release.Outcome) {
	case ReleaseOutcomeSucceeded:
		return nil
	case ReleaseOutcomeFailed:
		return ErrDecisionConflict
	case ReleaseOutcomePending, ReleaseOutcomePublicationUnknown:
		return errors.Join(ErrReleaseNotRetryable, ErrPublicationUnknown)
	default:
		return ErrDecisionConflict
	}
}

func releaseMatchesInput(release db.SkillEvolutionRelease, input ReleaseInput) bool {
	return release.WorkspaceID == input.WorkspaceID && release.SkillID == input.SkillID &&
		release.ProposalID == input.ProposalID && release.SourceReleaseID == input.SourceReleaseID &&
		release.RevisionID == input.RevisionID && release.Kind == string(input.Kind) &&
		release.ExpectedBaseHash == string(input.ExpectedBaseHash) && release.ActorID == input.ActorID &&
		release.IdempotencyKey == input.IdempotencyKey
}

func publishReviewMatches(ctx context.Context, store lifecycleStore, proposal db.SkillEvolutionProposal, request PublishRequest) bool {
	detail, err := store.GetProposalDetail(ctx, request.WorkspaceID, request.ProposalID)
	if err != nil {
		return false
	}
	return reviewMatches(detail.Reviews, proposal.CandidateRevisionID, "publish", request.Actor.ID, request.Reason, lifecycleKey("review", request.IdempotencyKey))
}

func reviewMatches(reviews []db.SkillEvolutionReview, candidateID pgtype.UUID, decision string, actorID pgtype.UUID, reason, key string) bool {
	for _, review := range reviews {
		if review.IdempotencyKey == key {
			return review.CandidateRevisionID == candidateID && review.Decision == decision &&
				review.ActorID == actorID && review.Reason == optionalText(reason)
		}
	}
	return false
}

var _ SkillEvolution = (*Lifecycle)(nil)
