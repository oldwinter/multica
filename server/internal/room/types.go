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
}

type ArtifactTargetCreator interface {
	CreateRoomArtifactTarget(context.Context, pgx.Tx, *db.Queries, db.RoomArtifact) (pgtype.UUID, error)
	RoomArtifactTargetCreated(context.Context, db.RoomArtifact)
}

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
	ErrNotFound             = errors.New("room not found")
	ErrInvalidInput         = errors.New("invalid room input")
	ErrInvalidParticipant   = errors.New("invalid room participant")
	ErrInvocationNotAllowed = errors.New("room agent invocation not allowed")
	ErrIdempotencyConflict  = errors.New("room idempotency key conflicts with an earlier request")
)

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
	FacilitatorAgentID      pgtype.UUID
	FacilitatorSquadID      pgtype.UUID
	Participants            []ParticipantInput
	DailyTurnLimit          *int32
	ScheduleIntervalMinutes *int32
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
	Room         db.Room
	Participants []db.RoomParticipant
	Entries      []db.RoomEntry
	Cycles       []db.RoomCycle
	Turns        []db.RoomTurn
	Artifacts    []db.RoomArtifact
}

type WakeResult struct {
	Cycle    db.RoomCycle
	Turns    []db.RoomTurn
	Tasks    []db.AgentTaskQueue
	replayed bool
}

type MessageResult struct {
	Entry db.RoomEntry
	WakeResult
}

type PromotionInput struct {
	WorkspaceID    pgtype.UUID
	RoomID         pgtype.UUID
	ActorUserID    pgtype.UUID
	EntryID        pgtype.UUID
	CycleID        pgtype.UUID
	Kind           string
	IdempotencyKey string
	Title          string
	Rationale      string
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

type Rooms interface {
	List(context.Context, pgtype.UUID) ([]db.Room, error)
	Get(context.Context, pgtype.UUID, pgtype.UUID) (Detail, error)
	Create(context.Context, CreateInput) (Detail, error)
	PostMessage(context.Context, MessageInput) (MessageResult, error)
	Wake(context.Context, WakeInput) (WakeResult, error)
	SetStatus(context.Context, pgtype.UUID, pgtype.UUID, string) (db.Room, error)
	Promote(context.Context, PromotionInput) (PromotionResult, error)
}

type Runtime interface {
	SyncTask(context.Context, pgtype.UUID) (bool, error)
	Reconcile(context.Context, int32) (int, error)
}

type Maintenance interface {
	DispatchDue(context.Context, time.Time, int32) (DueResult, error)
	Reconcile(context.Context, int32) (int, error)
}
