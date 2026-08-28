package room

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	input.RecommendationKey = strings.TrimSpace(input.RecommendationKey)
	input.Body = strings.TrimSpace(input.Body)
	isRecommendation := input.MemoryRevisionID.Valid && input.RecommendationKey != "" && !input.EntryID.Valid && !input.CycleID.Valid
	isLegacySource := input.EntryID.Valid != input.CycleID.Valid && !input.MemoryRevisionID.Valid && input.RecommendationKey == ""
	if !input.WorkspaceID.Valid || !input.RoomID.Valid || !input.ActorUserID.Valid ||
		(input.Kind != "issue" && input.Kind != "wiki" && input.Kind != "decision") ||
		input.IdempotencyKey == "" || input.Title == "" || len([]rune(input.Title)) > 300 ||
		len([]rune(input.Rationale)) > 20000 || len([]rune(input.Body)) > 200000 || (!isRecommendation && !isLegacySource) {
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

	existing, existingErr := queries.GetRoomArtifactByKey(ctx, db.GetRoomArtifactByKeyParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
		Kind: input.Kind, IdempotencyKey: input.IdempotencyKey,
	})
	if existingErr == nil {
		replayBody := existing.Body
		if isRecommendation && input.Body != "" {
			replayBody = input.Body
		}
		digest := promotionDigest(input, replayBody)
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
	body, cycleID, turnID, entryID, memoryRevisionID, recommendationKey, citations, err := promotionSource(ctx, queries, roomRow, input)
	if err != nil {
		return PromotionResult{}, err
	}
	if len([]rune(body)) > 200000 {
		return PromotionResult{}, ErrInvalidInput
	}
	digest := promotionDigest(input, body)

	artifactID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	artifact, err := queries.CreateRoomArtifact(ctx, db.CreateRoomArtifactParams{
		ID: artifactID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
		CycleID: cycleID, TurnID: turnID, EntryID: entryID, Kind: input.Kind,
		IdempotencyKey: input.IdempotencyKey,
		Title:          input.Title, Body: body,
		Rationale:    pgtype.Text{String: input.Rationale, Valid: input.Rationale != ""},
		SourceDigest: digest, CreatedByUserID: input.ActorUserID,
		MemoryRevisionID:  memoryRevisionID,
		RecommendationKey: pgtype.Text{String: recommendationKey, Valid: recommendationKey != ""},
		CitationEntryIds:  citations,
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
	var recommendationReview *db.RoomRecommendationReview
	var archivedAttention []db.ArchiveRoomInboxItemsRow
	if isRecommendation {
		reviewDigest := lifecycleDigest("approve", recommendationKey, digest)
		review, reviewErr := queries.CreateRoomRecommendationReview(ctx, db.CreateRoomRecommendationReviewParams{
			WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, MemoryRevisionID: memoryRevisionID,
			RecommendationKey: recommendationKey, Status: "approved", IdempotencyKey: input.IdempotencyKey,
			RequestDigest: reviewDigest, ArtifactID: artifact.ID, ReviewedByUserID: input.ActorUserID,
		})
		if errors.Is(reviewErr, pgx.ErrNoRows) {
			review, reviewErr = queries.GetRoomRecommendationReview(ctx, db.GetRoomRecommendationReviewParams{
				WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, MemoryRevisionID: memoryRevisionID,
				RecommendationKey: recommendationKey,
			})
			if reviewErr == nil && (review.Status != "approved" || !review.ArtifactID.Valid || review.ArtifactID != artifact.ID) {
				return PromotionResult{}, ErrRecommendationReviewed
			}
		}
		if reviewErr != nil {
			return PromotionResult{}, fmt.Errorf("approve Room recommendation: %w", reviewErr)
		}
		recommendationReview = &review
		archivedAttention, err = archiveRoomAttention(
			ctx, queries, roomRow, cycleID, RoomInboxRecommendationReviewRequired, recommendationKey,
		)
		if err != nil {
			return PromotionResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return PromotionResult{}, fmt.Errorf("commit Room promotion: %w", err)
	}
	s.recordRoomArtifactPromoted(roomRow, artifact, input.ActorUserID)
	if recommendationReview != nil {
		s.publishRoomAttentionArchived(roomRow.WorkspaceID, archivedAttention)
		s.publish(EventRoomRecommendationReview, roomRow, input.ActorUserID, roomRecommendationReviewEventPayload(*recommendationReview))
	}
	s.publish(EventRoomArtifact, roomRow, input.ActorUserID, roomArtifactEventPayload(artifact))
	if s.targets != nil {
		s.targets.RoomArtifactTargetCreated(ctx, artifact)
	}
	return PromotionResult{Artifact: artifact, Created: true}, nil
}

func promotionSource(ctx context.Context, queries *db.Queries, roomRow db.Room, input PromotionInput) (string, pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, string, []byte, error) {
	if input.MemoryRevisionID.Valid {
		outcome, err := loadAcceptedCurrentOutcome(ctx, queries, roomRow, input.WorkspaceID, input.RoomID, input.MemoryRevisionID)
		if err != nil {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, err
		}
		recommendation, ok := FindRecommendation(outcome.synthesis, input.RecommendationKey)
		if !ok || recommendation.Kind != input.Kind {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, ErrInvalidInput
		}
		if len(input.CitationEntryIDs) > 0 && !samePromotionCitations(input.CitationEntryIDs, recommendation.CitationEntryIDs) {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, ErrPromotionSourceMismatch
		}
		if existing, existingErr := queries.GetRoomRecommendationReview(ctx, db.GetRoomRecommendationReviewParams{
			WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, MemoryRevisionID: outcome.revision.ID,
			RecommendationKey: recommendation.Key,
		}); existingErr == nil && existing.Status != "approved" {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, ErrRecommendationReviewed
		} else if existingErr != nil && !errors.Is(existingErr, pgx.ErrNoRows) {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, existingErr
		}
		body := input.Body
		if body == "" {
			body = recommendation.Body
		}
		citationJSON, err := json.Marshal(recommendation.CitationEntryIDs)
		if err != nil {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, fmt.Errorf("encode Room recommendation citations: %w", err)
		}
		return body, outcome.revision.CycleID, outcome.revision.SynthesisTurnID, pgtype.UUID{}, outcome.revision.ID, recommendation.Key, citationJSON, nil
	}
	if input.EntryID.Valid {
		entry, err := queries.GetRoomEntry(ctx, db.GetRoomEntryParams{ID: input.EntryID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
		if err != nil || entry.EntryType != "result" || !entry.TurnID.Valid || !entry.CycleID.Valid {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, ErrInvalidInput
		}
		turn, err := queries.GetRoomTurn(ctx, db.GetRoomTurnParams{ID: entry.TurnID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
		if err != nil || turn.CycleID != entry.CycleID || turn.Status != "completed" {
			return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, ErrInvalidInput
		}
		return entry.Body, entry.CycleID, entry.TurnID, entry.ID, pgtype.UUID{}, "", []byte("[]"), nil
	}
	cycle, err := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: input.CycleID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if err != nil || cycle.Status != "completed" {
		return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, ErrInvalidInput
	}
	entries, err := queries.ListRoomResultEntriesByCycle(ctx, db.ListRoomResultEntriesByCycleParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, CycleID: input.CycleID})
	if err != nil {
		return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, fmt.Errorf("list Room promotion entries: %w", err)
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, entry.Body)
	}
	if len(parts) == 0 {
		return "", pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", nil, ErrInvalidInput
	}
	return strings.Join(parts, "\n\n"), cycle.ID, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "", []byte("[]"), nil
}

func samePromotionCitations(actual []pgtype.UUID, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	seen := make(map[pgtype.UUID]struct{}, len(actual))
	for _, citation := range actual {
		if !citation.Valid {
			return false
		}
		if _, duplicate := seen[citation]; duplicate {
			return false
		}
		seen[citation] = struct{}{}
	}
	for _, value := range expected {
		parsed, err := uuid.Parse(strings.TrimSpace(value))
		if err != nil {
			return false
		}
		if _, ok := seen[pgtype.UUID{Bytes: parsed, Valid: true}]; !ok {
			return false
		}
	}
	return true
}

func promotionDigest(input PromotionInput, body string) string {
	value := strings.Join([]string{
		input.Kind, input.Title, input.Rationale, body, util.UUIDToString(input.EntryID), util.UUIDToString(input.CycleID),
		util.UUIDToString(input.MemoryRevisionID), input.RecommendationKey,
	}, "\x00")
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
