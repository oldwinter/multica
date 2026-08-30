package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	RoomInboxOutcomeReviewRequired        = "room_outcome_review_required"
	RoomInboxRecommendationReviewRequired = "room_recommendation_review_required"
	RoomInboxCycleFailed                  = "room_cycle_failed"
	RoomInboxCycleBlocked                 = "room_cycle_blocked"
)

var (
	roomAttentionReviewIdentifier   = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,127}$`)
	roomAttentionMetadataIdentifier = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
)

type roomAttentionInput struct {
	RoomID            pgtype.UUID
	CycleID           pgtype.UUID
	MemoryRevisionID  pgtype.UUID
	RecommendationKey string
	Phase             string
	ReasonCode        string
}

type roomAttentionProjection struct {
	Type           string
	Severity       string
	Title          string
	RoomID         pgtype.UUID
	CycleID        pgtype.UUID
	ReviewIdentity string
	AttentionKey   string
	Details        []byte
}

func buildRoomAttentionProjection(kind string, input roomAttentionInput) (roomAttentionProjection, error) {
	roomID, ok := validRoomAttentionUUID(input.RoomID)
	if !ok {
		return roomAttentionProjection{}, fmt.Errorf("%w: room attention requires a room id", ErrInvalidInput)
	}

	details := map[string]string{"room_id": roomID}
	projection := roomAttentionProjection{
		Type:     kind,
		RoomID:   input.RoomID,
		Severity: "attention",
	}

	cycleID, hasCycle := validRoomAttentionUUID(input.CycleID)
	if hasCycle {
		projection.CycleID = input.CycleID
		details["cycle_id"] = cycleID
	}

	switch kind {
	case RoomInboxOutcomeReviewRequired:
		revisionID, ok := validRoomAttentionUUID(input.MemoryRevisionID)
		if !hasCycle || !ok {
			return roomAttentionProjection{}, fmt.Errorf("%w: outcome review attention requires cycle and revision ids", ErrInvalidInput)
		}
		projection.Severity = "action_required"
		projection.Title = "Room outcome needs review"
		projection.ReviewIdentity = revisionID
		projection.AttentionKey = fmt.Sprintf("room:%s:cycle:%s:outcome:%s", roomID, cycleID, revisionID)
		details["memory_revision_id"] = revisionID
		details["focus"] = "outcome_review"
		details["route"] = fmt.Sprintf("/rooms?room=%s&tab=outcome&focus=outcome_review&cycle_id=%s&memory_revision_id=%s", roomID, cycleID, revisionID)
	case RoomInboxRecommendationReviewRequired:
		revisionID, hasRevision := validRoomAttentionUUID(input.MemoryRevisionID)
		if !hasCycle || !hasRevision || !roomAttentionReviewIdentifier.MatchString(input.RecommendationKey) {
			return roomAttentionProjection{}, fmt.Errorf("%w: recommendation attention requires cycle and revision ids plus a bounded recommendation key", ErrInvalidInput)
		}
		projection.Severity = "action_required"
		projection.Title = "Room recommendation needs review"
		projection.ReviewIdentity = input.RecommendationKey
		projection.AttentionKey = fmt.Sprintf("room:%s:cycle:%s:recommendation:%s", roomID, cycleID, input.RecommendationKey)
		details["recommendation_key"] = input.RecommendationKey
		details["memory_revision_id"] = revisionID
		details["focus"] = "recommendation_review"
		details["route"] = fmt.Sprintf("/rooms?room=%s&tab=outcome&focus=recommendation_review&cycle_id=%s&memory_revision_id=%s&recommendation_key=%s", roomID, cycleID, revisionID, input.RecommendationKey)
	case RoomInboxCycleFailed:
		if !hasCycle {
			return roomAttentionProjection{}, fmt.Errorf("%w: failed cycle attention requires a cycle id", ErrInvalidInput)
		}
		projection.Title = "Room cycle needs attention"
		projection.AttentionKey = fmt.Sprintf("room:%s:cycle:%s:failed", roomID, cycleID)
		details["focus"] = "cycle_failed"
		details["route"] = fmt.Sprintf("/rooms?room=%s&tab=outcome&focus=cycle_failed&cycle_id=%s", roomID, cycleID)
	case RoomInboxCycleBlocked:
		projection.Title = "Room run is blocked"
		if hasCycle {
			projection.AttentionKey = fmt.Sprintf("room:%s:cycle:%s:blocked", roomID, cycleID)
		} else {
			projection.AttentionKey = fmt.Sprintf("room:%s:blocked", roomID)
		}
		details["focus"] = "cycle_blocked"
		projectionRoute := fmt.Sprintf("/rooms?room=%s&tab=outcome&focus=cycle_blocked", roomID)
		if hasCycle {
			projectionRoute += "&cycle_id=" + cycleID
		}
		details["route"] = projectionRoute
	default:
		return roomAttentionProjection{}, fmt.Errorf("%w: unsupported room attention type", ErrInvalidInput)
	}

	if roomAttentionMetadataIdentifier.MatchString(input.Phase) {
		details["phase"] = input.Phase
	}
	if roomAttentionMetadataIdentifier.MatchString(input.ReasonCode) {
		details["reason_code"] = input.ReasonCode
	}

	encoded, err := json.Marshal(details)
	if err != nil {
		return roomAttentionProjection{}, fmt.Errorf("marshal room attention details: %w", err)
	}
	projection.Details = encoded
	return projection, nil
}

func validRoomAttentionUUID(value pgtype.UUID) (string, bool) {
	if !value.Valid {
		return "", false
	}
	return value.String(), true
}

func (s *Service) upsertRoomAttention(
	ctx context.Context,
	queries *db.Queries,
	roomRow db.Room,
	kind string,
	input roomAttentionInput,
) ([]db.InboxItem, error) {
	projection, err := buildRoomAttentionProjection(kind, input)
	if err != nil {
		return nil, err
	}
	recipients, err := roomAttentionRecipients(ctx, queries, roomRow, kind)
	if err != nil {
		return nil, err
	}
	items := make([]db.InboxItem, 0, len(recipients))
	for _, recipientID := range recipients {
		item, createErr := queries.UpsertRoomInboxItem(ctx, db.UpsertRoomInboxItemParams{
			WorkspaceID: roomRow.WorkspaceID, RecipientID: recipientID,
			Type: projection.Type, Severity: projection.Severity, Title: projection.Title,
			Details: projection.Details, RoomID: projection.RoomID, RoomCycleID: projection.CycleID,
			RoomReviewIdentity: pgtype.Text{String: projection.ReviewIdentity, Valid: projection.ReviewIdentity != ""},
			RoomAttentionKey:   pgtype.Text{String: projection.AttentionKey, Valid: true},
		})
		if createErr != nil {
			return nil, fmt.Errorf("upsert Room attention item: %w", createErr)
		}
		items = append(items, item)
	}
	return items, nil
}

func roomAttentionRecipients(ctx context.Context, queries *db.Queries, roomRow db.Room, kind string) ([]pgtype.UUID, error) {
	recipients := map[pgtype.UUID]struct{}{}
	addCurrentMember := func(userID pgtype.UUID) error {
		if !userID.Valid {
			return nil
		}
		_, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
			UserID: userID, WorkspaceID: roomRow.WorkspaceID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("validate Room attention recipient: %w", err)
		}
		recipients[userID] = struct{}{}
		return nil
	}

	if kind == RoomInboxCycleBlocked {
		members, err := queries.ListMembers(ctx, roomRow.WorkspaceID)
		if err != nil {
			return nil, fmt.Errorf("list Room attention owners: %w", err)
		}
		for _, member := range members {
			if member.Role == "owner" || member.Role == "admin" {
				recipients[member.UserID] = struct{}{}
			}
		}
	} else {
		if err := addCurrentMember(roomRow.CreatedByUserID); err != nil {
			return nil, err
		}
		participants, err := queries.ListRoomParticipants(ctx, db.ListRoomParticipantsParams{
			WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("list Room attention participants: %w", err)
		}
		for _, participant := range participants {
			if participant.ParticipantType == "member" {
				if err := addCurrentMember(participant.ParticipantID); err != nil {
					return nil, err
				}
			}
		}
	}

	result := make([]pgtype.UUID, 0, len(recipients))
	for recipientID := range recipients {
		result = append(result, recipientID)
	}
	sort.Slice(result, func(i, j int) bool {
		return util.UUIDToString(result[i]) < util.UUIDToString(result[j])
	})
	return result, nil
}

func archiveRoomAttention(
	ctx context.Context,
	queries *db.Queries,
	roomRow db.Room,
	cycleID pgtype.UUID,
	kind string,
	reviewIdentity string,
) ([]db.ArchiveRoomInboxItemsRow, error) {
	archived, err := queries.ArchiveRoomInboxItems(ctx, db.ArchiveRoomInboxItemsParams{
		WorkspaceID:        roomRow.WorkspaceID,
		RoomID:             roomRow.ID,
		RoomCycleID:        cycleID,
		Type:               pgtype.Text{String: kind, Valid: kind != ""},
		RoomReviewIdentity: pgtype.Text{String: reviewIdentity, Valid: reviewIdentity != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("archive Room attention items: %w", err)
	}
	return archived, nil
}

func archiveSupersededRoomRunAttention(
	ctx context.Context,
	queries *db.Queries,
	roomRow db.Room,
) ([]db.ArchiveRoomInboxItemsRow, error) {
	var archived []db.ArchiveRoomInboxItemsRow
	for _, kind := range []string{RoomInboxCycleFailed, RoomInboxCycleBlocked} {
		items, err := archiveRoomAttention(ctx, queries, roomRow, pgtype.UUID{}, kind, "")
		if err != nil {
			return nil, err
		}
		archived = append(archived, items...)
	}
	return archived, nil
}

func (s *Service) publishRoomAttentionItems(items []db.InboxItem) {
	if s.events == nil {
		return
	}
	for _, item := range items {
		s.events.Publish(events.Event{
			Type:        protocol.EventInboxNew,
			WorkspaceID: util.UUIDToString(item.WorkspaceID),
			ActorType:   "system",
			Payload:     map[string]any{"item": roomInboxEventItem(item)},
		})
	}
}

func (s *Service) publishRoomAttentionArchived(workspaceID pgtype.UUID, archived []db.ArchiveRoomInboxItemsRow) {
	if s.events == nil || len(archived) == 0 {
		return
	}
	counts := map[pgtype.UUID]int{}
	for _, item := range archived {
		counts[item.RecipientID]++
	}
	for recipientID, count := range counts {
		s.events.Publish(events.Event{
			Type:        protocol.EventInboxBatchArchived,
			WorkspaceID: util.UUIDToString(workspaceID),
			ActorType:   "system",
			Payload: map[string]any{
				"recipient_id": util.UUIDToString(recipientID),
				"count":        count,
			},
		})
	}
}

func roomInboxEventItem(item db.InboxItem) map[string]any {
	return map[string]any{
		"id":                   util.UUIDToString(item.ID),
		"workspace_id":         util.UUIDToString(item.WorkspaceID),
		"recipient_type":       item.RecipientType,
		"recipient_id":         util.UUIDToString(item.RecipientID),
		"type":                 item.Type,
		"severity":             item.Severity,
		"issue_id":             util.UUIDToPtr(item.IssueID),
		"room_id":              util.UUIDToPtr(item.RoomID),
		"room_cycle_id":        util.UUIDToPtr(item.RoomCycleID),
		"room_review_identity": util.TextToPtr(item.RoomReviewIdentity),
		"title":                item.Title,
		"body":                 util.TextToPtr(item.Body),
		"read":                 item.Read,
		"archived":             item.Archived,
		"created_at":           util.TimestampToString(item.CreatedAt),
		"actor_type":           util.TextToPtr(item.ActorType),
		"actor_id":             util.UUIDToPtr(item.ActorID),
		"details":              json.RawMessage(item.Details),
	}
}
