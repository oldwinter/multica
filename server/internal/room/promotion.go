package room

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *Service) Promote(ctx context.Context, input PromotionInput) (PromotionResult, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Rationale = strings.TrimSpace(input.Rationale)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.WorkspaceID.Valid || !input.RoomID.Valid || !input.ActorUserID.Valid ||
		(input.Kind != "issue" && input.Kind != "wiki" && input.Kind != "decision") ||
		input.IdempotencyKey == "" || input.Title == "" || len([]rune(input.Title)) > 300 ||
		len([]rune(input.Rationale)) > 20000 || input.EntryID.Valid == input.CycleID.Valid {
		return PromotionResult{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return PromotionResult{}, fmt.Errorf("begin Room promotion: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err := queries.LockRoomWorkspaceForWrite(ctx, input.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PromotionResult{}, ErrNotFound
		}
		return PromotionResult{}, fmt.Errorf("lock Room workspace: %w", err)
	}
	roomRow, err := queries.GetRoomForUpdate(ctx, db.GetRoomForUpdateParams{ID: input.RoomID, WorkspaceID: input.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return PromotionResult{}, ErrNotFound
	}
	if err != nil {
		return PromotionResult{}, fmt.Errorf("lock Room for promotion: %w", err)
	}
	if _, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: input.ActorUserID, WorkspaceID: input.WorkspaceID}); err != nil {
		return PromotionResult{}, ErrInvalidParticipant
	}

	body, cycleID, turnID, entryID, err := promotionSource(ctx, queries, input)
	if err != nil {
		return PromotionResult{}, err
	}
	if len([]rune(body)) > 200000 {
		return PromotionResult{}, ErrInvalidInput
	}
	digest := promotionDigest(input, body)
	existing, existingErr := queries.GetRoomArtifactByKey(ctx, db.GetRoomArtifactByKeyParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
		Kind: input.Kind, IdempotencyKey: input.IdempotencyKey,
	})
	if existingErr == nil {
		if existing.SourceDigest != digest {
			return PromotionResult{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return PromotionResult{}, fmt.Errorf("commit repeated Room promotion: %w", err)
		}
		return PromotionResult{Artifact: existing}, nil
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return PromotionResult{}, fmt.Errorf("check Room promotion identity: %w", existingErr)
	}

	artifactID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	artifact, err := queries.CreateRoomArtifact(ctx, db.CreateRoomArtifactParams{
		ID: artifactID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
		CycleID: cycleID, TurnID: turnID, EntryID: entryID, Kind: input.Kind,
		IdempotencyKey: input.IdempotencyKey,
		Title:          input.Title, Body: body,
		Rationale:    pgtype.Text{String: input.Rationale, Valid: input.Rationale != ""},
		SourceDigest: digest, CreatedByUserID: input.ActorUserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		artifact, err = queries.GetRoomArtifactByKey(ctx, db.GetRoomArtifactByKeyParams{
			WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
			Kind: input.Kind, IdempotencyKey: input.IdempotencyKey,
		})
		if err == nil && artifact.SourceDigest != digest {
			return PromotionResult{}, ErrIdempotencyConflict
		}
		if err == nil {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return PromotionResult{}, fmt.Errorf("commit concurrent Room promotion: %w", commitErr)
			}
			return PromotionResult{Artifact: artifact}, nil
		}
	}
	if err != nil {
		return PromotionResult{}, fmt.Errorf("create Room artifact: %w", err)
	}
	targetID := artifact.ID
	switch input.Kind {
	case "issue", "wiki":
		if s.targets == nil {
			return PromotionResult{}, fmt.Errorf("Room artifact target creator is unavailable")
		}
		targetID, err = s.targets.CreateRoomArtifactTarget(ctx, tx, queries, artifact)
		if err != nil {
			return PromotionResult{}, fmt.Errorf("create Room %s target: %w", input.Kind, err)
		}
	}
	artifact, err = queries.SetRoomArtifactTarget(ctx, db.SetRoomArtifactTargetParams{
		TargetID: targetID, ID: artifact.ID, WorkspaceID: artifact.WorkspaceID,
	})
	if err != nil {
		return PromotionResult{}, fmt.Errorf("finalize Room artifact target: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PromotionResult{}, fmt.Errorf("commit Room promotion: %w", err)
	}
	s.publish("room:artifact", roomRow, input.ActorUserID, map[string]any{
		"room_id": util.UUIDToString(roomRow.ID), "artifact": artifact,
	})
	if s.targets != nil {
		s.targets.RoomArtifactTargetCreated(ctx, artifact)
	}
	return PromotionResult{Artifact: artifact, Created: true}, nil
}

func promotionSource(ctx context.Context, queries *db.Queries, input PromotionInput) (string, pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	if input.EntryID.Valid {
		entry, err := queries.GetRoomEntry(ctx, db.GetRoomEntryParams{ID: input.EntryID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
		if err != nil || entry.EntryType != "result" || !entry.TurnID.Valid || !entry.CycleID.Valid {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, ErrInvalidInput
		}
		turn, err := queries.GetRoomTurn(ctx, db.GetRoomTurnParams{ID: entry.TurnID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
		if err != nil || turn.CycleID != entry.CycleID || turn.Status != "completed" {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, ErrInvalidInput
		}
		return entry.Body, entry.CycleID, entry.TurnID, entry.ID, nil
	}
	cycle, err := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: input.CycleID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if err != nil || cycle.Status != "completed" {
		return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, ErrInvalidInput
	}
	entries, err := queries.ListRoomResultEntriesByCycle(ctx, db.ListRoomResultEntriesByCycleParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, CycleID: input.CycleID})
	if err != nil {
		return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, fmt.Errorf("list Room promotion entries: %w", err)
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, entry.Body)
	}
	if len(parts) == 0 {
		return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, ErrInvalidInput
	}
	return strings.Join(parts, "\n\n"), cycle.ID, pgtype.UUID{}, pgtype.UUID{}, nil
}

func promotionDigest(input PromotionInput, body string) string {
	value := strings.Join([]string{input.Kind, input.Title, input.Rationale, body, util.UUIDToString(input.EntryID), util.UUIDToString(input.CycleID)}, "\x00")
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
