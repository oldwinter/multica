package room

import (
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	EventRoomCreated              = "room:created"
	EventRoomUpdated              = "room:updated"
	EventRoomEntry                = "room:entry"
	EventRoomCycle                = "room:cycle"
	EventRoomTurn                 = "room:turn"
	EventRoomMemoryRevision       = "room:memory_revision"
	EventRoomReview               = "room:review"
	EventRoomRecommendationReview = "room:recommendation_review"
	EventRoomArtifact             = "room:artifact"
)

// Room realtime events are invalidation signals, not alternate HTTP DTOs. Keep
// them bounded so task output and JSONB fields never cross the realtime bus.
type RoomEventPayload struct {
	RoomID        string  `json:"room_id"`
	Status        string  `json:"status"`
	MemoryVersion int64   `json:"memory_version"`
	ActiveCycleID *string `json:"active_cycle_id,omitempty"`
}

type RoomEntryEventPayload struct {
	RoomID  string  `json:"room_id"`
	EntryID string  `json:"entry_id"`
	CycleID *string `json:"cycle_id,omitempty"`
	TurnID  *string `json:"turn_id,omitempty"`
}

type RoomCycleEventPayload struct {
	RoomID  string `json:"room_id"`
	CycleID string `json:"cycle_id"`
	Status  string `json:"status"`
	Phase   string `json:"phase"`
}

type RoomTurnEventPayload struct {
	RoomID   string `json:"room_id"`
	CycleID  string `json:"cycle_id"`
	TurnID   string `json:"turn_id"`
	Status   string `json:"status"`
	TurnKind string `json:"turn_kind"`
	Attempt  int32  `json:"attempt"`
}

type RoomMemoryRevisionEventPayload struct {
	RoomID           string `json:"room_id"`
	CycleID          string `json:"cycle_id"`
	MemoryRevisionID string `json:"memory_revision_id"`
	ReviewStatus     string `json:"review_status"`
	Version          int64  `json:"version"`
}

type RoomReviewEventPayload struct {
	RoomID           string `json:"room_id"`
	CycleID          string `json:"cycle_id"`
	MemoryRevisionID string `json:"memory_revision_id"`
	ReviewStatus     string `json:"review_status"`
	Action           string `json:"action"`
	MemoryVersion    int64  `json:"memory_version"`
}

type RoomRecommendationReviewEventPayload struct {
	RoomID            string  `json:"room_id"`
	MemoryRevisionID  string  `json:"memory_revision_id"`
	RecommendationKey string  `json:"recommendation_key"`
	Status            string  `json:"status"`
	ArtifactID        *string `json:"artifact_id,omitempty"`
}

type RoomArtifactEventPayload struct {
	RoomID            string  `json:"room_id"`
	ArtifactID        string  `json:"artifact_id"`
	Kind              string  `json:"kind"`
	TargetID          *string `json:"target_id,omitempty"`
	MemoryRevisionID  *string `json:"memory_revision_id,omitempty"`
	RecommendationKey *string `json:"recommendation_key,omitempty"`
}

func roomEventPayload(roomRow db.Room) RoomEventPayload {
	return RoomEventPayload{
		RoomID: util.UUIDToString(roomRow.ID), Status: roomRow.Status,
		MemoryVersion: roomRow.MemoryVersion, ActiveCycleID: optionalUUID(roomRow.ActiveCycleID),
	}
}

func roomEntryEventPayload(entry db.RoomEntry) RoomEntryEventPayload {
	return RoomEntryEventPayload{
		RoomID: util.UUIDToString(entry.RoomID), EntryID: util.UUIDToString(entry.ID),
		CycleID: optionalUUID(entry.CycleID), TurnID: optionalUUID(entry.TurnID),
	}
}

func roomCycleEventPayload(cycle db.RoomCycle) RoomCycleEventPayload {
	return RoomCycleEventPayload{
		RoomID: util.UUIDToString(cycle.RoomID), CycleID: util.UUIDToString(cycle.ID),
		Status: cycle.Status, Phase: cycle.Phase,
	}
}

func roomTurnEventPayload(turn db.RoomTurn) RoomTurnEventPayload {
	return RoomTurnEventPayload{
		RoomID: util.UUIDToString(turn.RoomID), CycleID: util.UUIDToString(turn.CycleID),
		TurnID: util.UUIDToString(turn.ID), Status: turn.Status, TurnKind: turn.TurnKind,
		Attempt: turn.Attempt,
	}
}

func roomMemoryRevisionEventPayload(revision db.RoomMemoryRevision) RoomMemoryRevisionEventPayload {
	return RoomMemoryRevisionEventPayload{
		RoomID: util.UUIDToString(revision.RoomID), CycleID: util.UUIDToString(revision.CycleID),
		MemoryRevisionID: util.UUIDToString(revision.ID), ReviewStatus: revision.ReviewStatus,
		Version: revision.Version,
	}
}

func roomReviewEventPayload(roomRow db.Room, cycle db.RoomCycle, revision db.RoomMemoryRevision, action string) RoomReviewEventPayload {
	return RoomReviewEventPayload{
		RoomID: util.UUIDToString(roomRow.ID), CycleID: util.UUIDToString(cycle.ID),
		MemoryRevisionID: util.UUIDToString(revision.ID), ReviewStatus: revision.ReviewStatus,
		Action: action, MemoryVersion: roomRow.MemoryVersion,
	}
}

func roomRecommendationReviewEventPayload(review db.RoomRecommendationReview) RoomRecommendationReviewEventPayload {
	return RoomRecommendationReviewEventPayload{
		RoomID: util.UUIDToString(review.RoomID), MemoryRevisionID: util.UUIDToString(review.MemoryRevisionID),
		RecommendationKey: review.RecommendationKey, Status: review.Status,
		ArtifactID: optionalUUID(review.ArtifactID),
	}
}

func roomArtifactEventPayload(artifact db.RoomArtifact) RoomArtifactEventPayload {
	return RoomArtifactEventPayload{
		RoomID: util.UUIDToString(artifact.RoomID), ArtifactID: util.UUIDToString(artifact.ID),
		Kind: artifact.Kind, TargetID: optionalUUID(artifact.TargetID),
		MemoryRevisionID:  optionalUUID(artifact.MemoryRevisionID),
		RecommendationKey: optionalText(artifact.RecommendationKey),
	}
}

func optionalUUID(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	formatted := util.UUIDToString(value)
	return &formatted
}

func optionalText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
