package room

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const EventTaskLifecycle = "room:task_lifecycle"

type TaskEnqueuer interface {
	EnqueueRoomTurn(context.Context, *db.Queries, RoomTaskEnqueueInput) (db.AgentTaskQueue, error)
	LookupRoomRuntime(context.Context, *db.Queries, pgtype.UUID) (db.AgentRuntime, error)
}

type ArtifactTargetCreator interface {
	CreateRoomArtifactTarget(context.Context, pgx.Tx, *db.Queries, db.RoomArtifact) (pgtype.UUID, error)
	RoomArtifactTargetCreated(context.Context, db.RoomArtifact)
}

type RecommendationTarget string

const (
	RecommendationTargetKnowledge            RecommendationTarget = "knowledge"
	RecommendationTargetPreference           RecommendationTarget = "preference"
	RecommendationTargetConstraint           RecommendationTarget = "constraint"
	RecommendationTargetExecutableProcedure  RecommendationTarget = "executable_procedure"
	RecommendationTargetImplementationDefect RecommendationTarget = "implementation_defect"
	RecommendationTargetDecision             RecommendationTarget = "decision"
	RecommendationTargetUnsupported          RecommendationTarget = "unsupported"
)

func (target RecommendationTarget) Valid() bool {
	switch target {
	case RecommendationTargetKnowledge, RecommendationTargetPreference,
		RecommendationTargetConstraint, RecommendationTargetExecutableProcedure,
		RecommendationTargetImplementationDefect, RecommendationTargetDecision,
		RecommendationTargetUnsupported:
		return true
	default:
		return false
	}
}

type ArtifactTargetContributor interface {
	CreateRoomArtifactTarget(context.Context, pgx.Tx, *db.Queries, db.RoomArtifact) (pgtype.UUID, error)
	RoomArtifactTargetCreated(context.Context, db.RoomArtifact)
}

type ArtifactTargetRouter struct {
	contributors map[RecommendationTarget]ArtifactTargetContributor
}

func NewArtifactTargetRouter() *ArtifactTargetRouter {
	router := &ArtifactTargetRouter{contributors: make(map[RecommendationTarget]ArtifactTargetContributor)}
	router.contributors[RecommendationTargetDecision] = roomDecisionTarget{}
	return router
}

func (router *ArtifactTargetRouter) Register(target RecommendationTarget, contributor ArtifactTargetContributor) error {
	if router == nil || !target.Valid() || target == RecommendationTargetUnsupported || target == RecommendationTargetDecision || contributor == nil {
		return ErrInvalidTargetRegistration
	}
	if _, exists := router.contributors[target]; exists {
		return ErrDuplicateTargetRegistration
	}
	router.contributors[target] = contributor
	return nil
}

func (router *ArtifactTargetRouter) CreateRoomArtifactTarget(ctx context.Context, tx pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	target := canonicalRecommendationTarget(artifact.Kind)
	if target == RecommendationTargetUnsupported || !target.Valid() {
		return pgtype.UUID{}, &RecommendationTargetRefusal{Target: artifact.Kind, Reason: "unsupported_target"}
	}
	if router == nil {
		return pgtype.UUID{}, &RecommendationTargetRefusal{Target: artifact.Kind, Reason: "target_unavailable"}
	}
	contributor := router.contributors[target]
	if contributor == nil {
		return pgtype.UUID{}, &RecommendationTargetRefusal{Target: artifact.Kind, Reason: "target_unavailable"}
	}
	return contributor.CreateRoomArtifactTarget(ctx, tx, queries, artifact)
}

func (router *ArtifactTargetRouter) RoomArtifactTargetCreated(ctx context.Context, artifact db.RoomArtifact) {
	if router == nil {
		return
	}
	if contributor := router.contributors[canonicalRecommendationTarget(artifact.Kind)]; contributor != nil {
		contributor.RoomArtifactTargetCreated(ctx, artifact)
	}
}

func canonicalRecommendationTarget(kind string) RecommendationTarget {
	switch kind {
	case "issue":
		return RecommendationTargetImplementationDefect
	case "wiki":
		return RecommendationTargetKnowledge
	default:
		return RecommendationTarget(kind)
	}
}

func validPromotionKind(kind string) bool {
	return kind == "issue" || kind == "wiki" || canonicalRecommendationTarget(kind).Valid()
}

type roomDecisionTarget struct{}

func (roomDecisionTarget) CreateRoomArtifactTarget(_ context.Context, _ pgx.Tx, _ *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	return artifact.ID, nil
}

func (roomDecisionTarget) RoomArtifactTargetCreated(context.Context, db.RoomArtifact) {}

type RoomTaskEnqueueInput struct {
	Agent               db.Agent
	RoomTurnID          pgtype.UUID
	SquadID             pgtype.UUID
	Context             []byte
	OriginatorUserID    pgtype.UUID
	AccountableUserID   pgtype.UUID
	OriginatorSource    string
	TriggerEvidenceKind string
	TriggerEvidenceID   pgtype.UUID
	SessionID           pgtype.Text
	WorkDir             pgtype.Text
}

var (
	ErrNotFound                    = errors.New("room not found")
	ErrInvalidInput                = errors.New("invalid room input")
	ErrInvalidParticipant          = errors.New("invalid room participant")
	ErrInvocationNotAllowed        = errors.New("room agent invocation not allowed")
	ErrIdempotencyConflict         = errors.New("room idempotency key conflicts with an earlier request")
	ErrStaleReview                 = errors.New("room memory review is stale")
	ErrSynthesisNotRetryable       = errors.New("room synthesis is not retryable")
	ErrBudgetExhausted             = errors.New("room budget is exhausted")
	ErrBudgetPermissionDenied      = errors.New("room budget update is not allowed")
	ErrBudgetBelowCommitted        = errors.New("room budget is below committed work")
	ErrBudgetHasUncostedUsage      = errors.New("room budget cannot be capped while usage is uncosted")
	ErrSpendLimitUnsupported       = errors.New("room spend limit execution is unsupported")
	ErrRecommendationReviewed      = errors.New("room recommendation was already reviewed")
	ErrPromotionSourceMismatch     = errors.New("room promotion source does not match the accepted outcome")
	ErrInvalidTargetRegistration   = errors.New("invalid Room target registration")
	ErrDuplicateTargetRegistration = errors.New("Room target already registered")
	ErrRecommendationTargetRefused = errors.New("Room recommendation target refused")
)

type RecommendationTargetRefusal struct {
	Target string
	Reason string
}

func (e *RecommendationTargetRefusal) Error() string {
	return ErrRecommendationTargetRefused.Error() + ": " + e.Reason
}

func (e *RecommendationTargetRefusal) Unwrap() []error {
	return []error{ErrRecommendationTargetRefused, ErrInvalidInput}
}

type ParticipantInput struct {
	Type string
	ID   pgtype.UUID
	Role string
}

type CreateInput struct {
	WorkspaceID             pgtype.UUID
	ActorUserID             pgtype.UUID
	Title                   string
	Instructions            string
	Objective               string
	SuccessCriteria         []string
	StopConditions          []string
	TemplateID              string
	FacilitatorAgentID      pgtype.UUID
	FacilitatorSquadID      pgtype.UUID
	Participants            []ParticipantInput
	DailyTurnLimit          *int32
	MaxCostTicks            *int64
	ScheduleIntervalMinutes *int32
	StartPaused             bool
}

type MessageInput struct {
	WorkspaceID    pgtype.UUID
	RoomID         pgtype.UUID
	ActorUserID    pgtype.UUID
	Body           string
	MentionAgents  []pgtype.UUID
	IdempotencyKey string
}

type WakeInput struct {
	WorkspaceID       pgtype.UUID
	RoomID            pgtype.UUID
	ActorUserID       pgtype.UUID
	Source            string
	WakeKey           string
	TriggeringEntryID pgtype.UUID
	TargetAgentIDs    []pgtype.UUID
	PlannedAt         pgtype.Timestamptz
}

type Detail struct {
	Room                  db.Room
	Participants          []db.RoomParticipant
	Entries               []db.RoomEntry
	Cycles                []db.RoomCycle
	Turns                 []db.RoomTurn
	Artifacts             []db.RoomArtifact
	MemoryRevisions       []db.RoomMemoryRevision
	RecommendationReviews []db.RoomRecommendationReview
}

type WakeResult struct {
	Cycle             db.RoomCycle
	Turns             []db.RoomTurn
	Tasks             []db.AgentTaskQueue
	attentionItems    []db.InboxItem
	archivedAttention []db.ArchiveRoomInboxItemsRow
	replayed          bool
}

type MessageResult struct {
	Entry db.RoomEntry
	WakeResult
}

type PromotionInput struct {
	WorkspaceID       pgtype.UUID
	RoomID            pgtype.UUID
	ActorUserID       pgtype.UUID
	EntryID           pgtype.UUID
	CycleID           pgtype.UUID
	MemoryRevisionID  pgtype.UUID
	RecommendationKey string
	CitationEntryIDs  []pgtype.UUID
	Kind              string
	IdempotencyKey    string
	Title             string
	Rationale         string
	Body              string
}

type PromotionResult struct {
	Artifact db.RoomArtifact
	Created  bool
}

type DueResult struct {
	RoomsAdvanced int
	CyclesQueued  int
	CyclesRefused int
	TasksQueued   int
}

type PreflightAgent struct {
	AgentID           pgtype.UUID
	Ready             bool
	InvocationAllowed bool
	Reason            string
}

type BudgetSummary struct {
	DailyTurnLimit     *int32
	UsedTurns          int64
	MaxCostTicks       *int64
	UsedCostTicks      int64
	RemainingCostTicks *int64
	ReservedCostTicks  int64
	UncostedTurns      int64
}

type UpdateBudgetInput struct {
	WorkspaceID    pgtype.UUID
	RoomID         pgtype.UUID
	ActorUserID    pgtype.UUID
	DailyTurnLimit pgtype.Int4
	MaxCostTicks   pgtype.Int8
}

type PreflightInput struct {
	WorkspaceID    pgtype.UUID
	RoomID         pgtype.UUID
	ActorUserID    pgtype.UUID
	Source         string
	TargetAgentIDs []pgtype.UUID
}

type PreflightResult struct {
	Source                   string
	Allowed                  bool
	RefusalReason            string
	TargetAgents             []PreflightAgent
	ExpectedMaxTurns         int32
	SynthesisRequired        bool
	CapabilityVersion        int32
	RequiredDaemonCapability string
	CapabilityReady          bool
	SpendLimitSupported      bool
	RequiredCostCapability   string
	Budget                   BudgetSummary
}

type UsageSummary struct {
	TurnsTotal                    int64
	CostTicks                     int64
	UncostedTurns                 int64
	Failures                      int64
	AcceptedSyntheses             int64
	PromotedArtifacts             int64
	RepeatRunCount                int64
	ActiveWeeks                   int64
	MedianReviewLatencySeconds    float64
	AcceptedOutcomesPerActiveWeek float64
	PromotionRate                 float64
	FailedCycles                  int64
	RefusedCycles                 int64
	CostTicksPerAcceptedOutcome   float64
}

type ValueSignal struct {
	RoomID                        pgtype.UUID
	LastAcceptedRevisionID        pgtype.UUID
	LastAcceptedAt                pgtype.Timestamptz
	LastCycleID                   pgtype.UUID
	LastRunStatus                 string
	LastRunPhase                  string
	LastRunReason                 pgtype.Text
	LastRunAt                     pgtype.Timestamptz
	LastRunCostTicks              int64
	RepeatRunCount                int64
	AcceptedOutcomes              int64
	ActiveWeeks                   int64
	AcceptedOutcomesPerActiveWeek float64
	MedianReviewLatencySeconds    float64
	PromotionRate                 float64
	FailedCycles                  int64
	RefusedCycles                 int64
}

type RetrySynthesisInput struct {
	WorkspaceID    pgtype.UUID
	RoomID         pgtype.UUID
	CycleID        pgtype.UUID
	ActorUserID    pgtype.UUID
	IdempotencyKey string
}

type RetrySynthesisResult struct {
	Cycle    db.RoomCycle
	Turn     db.RoomTurn
	Task     db.AgentTaskQueue
	Replayed bool
}

type ReviewInput struct {
	WorkspaceID           pgtype.UUID
	RoomID                pgtype.UUID
	CycleID               pgtype.UUID
	ActorUserID           pgtype.UUID
	Action                string
	ExpectedMemoryVersion int64
	Correction            []byte
	IdempotencyKey        string
}

type ReviewResult struct {
	Room           db.Room
	MemoryRevision db.RoomMemoryRevision
	Replayed       bool
}

type CancelInput struct {
	WorkspaceID    pgtype.UUID
	RoomID         pgtype.UUID
	CycleID        pgtype.UUID
	ActorUserID    pgtype.UUID
	IdempotencyKey string
}

type RecommendationReviewInput struct {
	WorkspaceID       pgtype.UUID
	RoomID            pgtype.UUID
	MemoryRevisionID  pgtype.UUID
	RecommendationKey string
	ActorUserID       pgtype.UUID
	Action            string
	IdempotencyKey    string
}

type Rooms interface {
	List(context.Context, pgtype.UUID) ([]db.Room, error)
	ListValueSignals(context.Context, pgtype.UUID) ([]ValueSignal, error)
	Get(context.Context, pgtype.UUID, pgtype.UUID) (Detail, error)
	Create(context.Context, CreateInput) (Detail, error)
	PostMessage(context.Context, MessageInput) (MessageResult, error)
	Wake(context.Context, WakeInput) (WakeResult, error)
	SetStatus(context.Context, pgtype.UUID, pgtype.UUID, string) (db.Room, error)
	UpdateBudget(context.Context, UpdateBudgetInput) (db.Room, error)
	Promote(context.Context, PromotionInput) (PromotionResult, error)
	Preflight(context.Context, PreflightInput) (PreflightResult, error)
	Usage(context.Context, pgtype.UUID, pgtype.UUID) (UsageSummary, error)
	RetrySynthesis(context.Context, RetrySynthesisInput) (RetrySynthesisResult, error)
	Review(context.Context, ReviewInput) (ReviewResult, error)
	Cancel(context.Context, CancelInput) (db.RoomCycle, error)
	ReviewRecommendation(context.Context, RecommendationReviewInput) (db.RoomRecommendationReview, error)
}

type Runtime interface {
	SyncTask(context.Context, pgtype.UUID) (bool, error)
	Reconcile(context.Context, int32) (int, error)
}

type Maintenance interface {
	DispatchDue(context.Context, time.Time, int32) (DueResult, error)
	Reconcile(context.Context, int32) (int, error)
}
