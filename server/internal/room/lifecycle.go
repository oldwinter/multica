package room

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type synthesisError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (s *Service) RetrySynthesis(ctx context.Context, input RetrySynthesisInput) (RetrySynthesisResult, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.WorkspaceID.Valid || !input.RoomID.Valid || !input.CycleID.Valid || !input.ActorUserID.Valid || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return RetrySynthesisResult{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return RetrySynthesisResult{}, fmt.Errorf("begin Room synthesis retry: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	roomRow, err := lockRoomForMember(ctx, queries, input.WorkspaceID, input.RoomID, input.ActorUserID)
	if err != nil {
		return RetrySynthesisResult{}, err
	}
	if err := validateRoomAgent(ctx, queries, input.WorkspaceID, input.ActorUserID, roomRow.FacilitatorAgentID); err != nil {
		return RetrySynthesisResult{}, err
	}
	if existing, existingErr := queries.GetRoomSynthesisTurnByKey(ctx, db.GetRoomSynthesisTurnByKeyParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, CycleID: input.CycleID, IdempotencyKey: pgtype.Text{String: input.IdempotencyKey, Valid: true},
	}); existingErr == nil {
		cycle, cycleErr := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: input.CycleID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
		if cycleErr != nil {
			return RetrySynthesisResult{}, fmt.Errorf("load repeated synthesis cycle: %w", cycleErr)
		}
		task, taskErr := queries.GetLatestTaskForRoomTurn(ctx, existing.ID)
		if taskErr != nil {
			return RetrySynthesisResult{}, fmt.Errorf("load repeated synthesis task: %w", taskErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return RetrySynthesisResult{}, fmt.Errorf("commit repeated synthesis retry: %w", err)
		}
		return RetrySynthesisResult{Cycle: cycle, Turn: existing, Task: task, Replayed: true}, nil
	} else if !errors.Is(existingErr, pgx.ErrNoRows) {
		return RetrySynthesisResult{}, fmt.Errorf("check synthesis retry identity: %w", existingErr)
	}
	cycle, err := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: input.CycleID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if errors.Is(err, pgx.ErrNoRows) {
		return RetrySynthesisResult{}, ErrNotFound
	}
	if err != nil {
		return RetrySynthesisResult{}, fmt.Errorf("load Room cycle for retry: %w", err)
	}
	var currentError synthesisError
	if cycle.Phase != "awaiting_review" || len(cycle.SynthesisError) == 0 || json.Unmarshal(cycle.SynthesisError, &currentError) != nil || !currentError.Retryable {
		return RetrySynthesisResult{}, ErrSynthesisNotRetryable
	}
	turn, task, err := s.enqueueSynthesisTx(ctx, queries, roomRow, cycle, input.ActorUserID, input.IdempotencyKey, false)
	if err != nil {
		return RetrySynthesisResult{}, err
	}
	cycle, err = queries.SetRoomCycleSynthesisRetry(ctx, db.SetRoomCycleSynthesisRetryParams{
		SynthesisTurnID: turn.ID, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
	})
	if err != nil {
		return RetrySynthesisResult{}, fmt.Errorf("start Room synthesis retry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return RetrySynthesisResult{}, fmt.Errorf("commit Room synthesis retry: %w", err)
	}
	if notifier, ok := s.tasks.(TaskNotifier); ok {
		notifier.NotifyTaskEnqueued(ctx, task)
	}
	s.recordRoomSynthesisRetried(roomRow, cycle, input.ActorUserID)
	s.publish(EventRoomTurn, roomRow, input.ActorUserID, roomTurnEventPayload(turn))
	s.publish(EventRoomCycle, roomRow, input.ActorUserID, roomCycleEventPayload(cycle))
	return RetrySynthesisResult{Cycle: cycle, Turn: turn, Task: task}, nil
}

func (s *Service) Review(ctx context.Context, input ReviewInput) (ReviewResult, error) {
	input.Action = strings.TrimSpace(input.Action)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.WorkspaceID.Valid || !input.RoomID.Valid || !input.CycleID.Valid || !input.ActorUserID.Valid ||
		(input.Action != "accept" && input.Action != "reject" && input.Action != "correct") || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return ReviewResult{}, ErrInvalidInput
	}
	requestDigest := lifecycleDigest(input.Action, fmt.Sprintf("%d", input.ExpectedMemoryVersion), string(input.Correction))
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("begin Room memory review: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	roomRow, err := lockRoomForMember(ctx, queries, input.WorkspaceID, input.RoomID, input.ActorUserID)
	if err != nil {
		return ReviewResult{}, err
	}
	cycle, err := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: input.CycleID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewResult{}, ErrNotFound
	}
	if err != nil {
		return ReviewResult{}, fmt.Errorf("load Room cycle for review: %w", err)
	}
	existing, existingErr := queries.GetRoomMemoryRevisionByReviewKey(ctx, db.GetRoomMemoryRevisionByReviewKeyParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, CycleID: input.CycleID,
		ReviewIdempotencyKey: pgtype.Text{String: input.IdempotencyKey, Valid: true},
	})
	if existingErr == nil {
		if !existing.ReviewRequestDigest.Valid || existing.ReviewRequestDigest.String != requestDigest {
			return ReviewResult{}, ErrIdempotencyConflict
		}
		revision := existing
		if existing.ReviewStatus == "corrected" {
			revision, err = queries.GetCorrectedRoomMemoryRevision(ctx, db.GetCorrectedRoomMemoryRevisionParams{
				WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, CorrectedFromRevisionID: existing.ID,
			})
			if err != nil {
				return ReviewResult{}, fmt.Errorf("load corrected Room revision replay: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return ReviewResult{}, fmt.Errorf("commit repeated Room review: %w", err)
		}
		return ReviewResult{Room: roomRow, MemoryRevision: revision, Replayed: true}, nil
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return ReviewResult{}, fmt.Errorf("check Room review identity: %w", existingErr)
	}
	if !cycle.MemoryRevisionID.Valid || cycle.Phase != "awaiting_review" || cycle.Status != "running" {
		return ReviewResult{}, ErrStaleReview
	}
	revision, err := queries.GetRoomMemoryRevision(ctx, db.GetRoomMemoryRevisionParams{ID: cycle.MemoryRevisionID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if err != nil {
		return ReviewResult{}, fmt.Errorf("load Room memory revision for review: %w", err)
	}
	if revision.ReviewStatus != "pending" {
		return ReviewResult{}, ErrStaleReview
	}
	if roomRow.MemoryVersion != input.ExpectedMemoryVersion {
		return ReviewResult{}, ErrStaleReview
	}

	reviewStatus := map[string]string{
		"accept":  "accepted",
		"reject":  "rejected",
		"correct": "corrected",
	}
	revision, err = queries.ReviewRoomMemoryRevision(ctx, db.ReviewRoomMemoryRevisionParams{
		ReviewStatus: reviewStatus[input.Action], ReviewedByUserID: input.ActorUserID,
		ReviewIdempotencyKey: pgtype.Text{String: input.IdempotencyKey, Valid: true},
		ReviewRequestDigest:  pgtype.Text{String: requestDigest, Valid: true},
		ID:                   revision.ID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewResult{}, ErrStaleReview
	}
	if err != nil {
		return ReviewResult{}, fmt.Errorf("review Room memory revision: %w", err)
	}

	firstCompletedCycle := false
	switch input.Action {
	case "accept":
		roomRow, err = queries.AcceptRoomMemoryRevision(ctx, db.AcceptRoomMemoryRevisionParams{
			RevisionID: revision.ID, Synthesis: revision.Synthesis, CycleID: cycle.ID,
			ID: roomRow.ID, WorkspaceID: roomRow.WorkspaceID, ExpectedMemoryVersion: input.ExpectedMemoryVersion,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ReviewResult{}, ErrStaleReview
		}
		if err != nil {
			return ReviewResult{}, fmt.Errorf("accept Room memory revision: %w", err)
		}
		cycle, err = queries.SetRoomCycleReviewed(ctx, db.SetRoomCycleReviewedParams{
			Status: "completed", Phase: "completed", ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
		})
		if err != nil {
			return ReviewResult{}, fmt.Errorf("complete reviewed Room cycle: %w", err)
		}
		usage, usageErr := queries.GetRoomUsageSummary(ctx, db.GetRoomUsageSummaryParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID})
		if usageErr != nil {
			return ReviewResult{}, fmt.Errorf("load Room acceptance usage: %w", usageErr)
		}
		firstCompletedCycle = usage.AcceptedSyntheses == 1
	case "reject":
		errorData, _ := json.Marshal(synthesisError{Code: "review_rejected", Message: "Another synthesis was requested.", Retryable: true})
		cycle, err = queries.SetRoomCycleReviewRetryable(ctx, db.SetRoomCycleReviewRetryableParams{
			SynthesisError: errorData, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
		})
		if err != nil {
			return ReviewResult{}, fmt.Errorf("make rejected Room synthesis retryable: %w", err)
		}
	case "correct":
		_, canonical, digest, validateErr := validateRoomSynthesis(ctx, queries, roomRow.WorkspaceID, roomRow.ID, input.Correction)
		if validateErr != nil {
			return ReviewResult{}, validateErr
		}
		corrected, createErr := queries.CreateRoomMemoryRevision(ctx, db.CreateRoomMemoryRevisionParams{
			WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
			SynthesisTurnID: revision.SynthesisTurnID, SchemaVersion: RoomSynthesisSchemaVersion,
			Synthesis: canonical, Digest: digest, CorrectedFromRevisionID: revision.ID,
			CreatorType: "member", CreatorID: input.ActorUserID,
		})
		if createErr != nil {
			return ReviewResult{}, fmt.Errorf("create corrected Room memory revision: %w", createErr)
		}
		cycle, err = queries.SetRoomCyclePendingRevision(ctx, db.SetRoomCyclePendingRevisionParams{
			MemoryRevisionID: corrected.ID, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
		})
		if err != nil {
			return ReviewResult{}, fmt.Errorf("link corrected Room memory revision: %w", err)
		}
		revision = corrected
	}
	if err := tx.Commit(ctx); err != nil {
		return ReviewResult{}, fmt.Errorf("commit Room memory review: %w", err)
	}
	switch input.Action {
	case "accept":
		s.recordRoomSynthesisAccepted(roomRow, cycle, input.ActorUserID)
		if firstCompletedCycle {
			s.recordRoomFirstCycleCompleted(roomRow, cycle, input.ActorUserID)
		}
	case "reject":
		s.recordRoomSynthesisRejected(roomRow, cycle, input.ActorUserID)
	}
	s.publish(EventRoomReview, roomRow, input.ActorUserID, roomReviewEventPayload(roomRow, cycle, revision, input.Action))
	return ReviewResult{Room: roomRow, MemoryRevision: revision}, nil
}

func (s *Service) Cancel(ctx context.Context, input CancelInput) (db.RoomCycle, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.WorkspaceID.Valid || !input.RoomID.Valid || !input.CycleID.Valid || !input.ActorUserID.Valid || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return db.RoomCycle{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return db.RoomCycle{}, fmt.Errorf("begin Room cycle cancellation: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	roomRow, err := lockRoomForMember(ctx, queries, input.WorkspaceID, input.RoomID, input.ActorUserID)
	if err != nil {
		return db.RoomCycle{}, err
	}
	cycle, err := queries.CancelRoomCycle(ctx, db.CancelRoomCycleParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, CycleID: input.CycleID, IdempotencyKey: pgtype.Text{String: input.IdempotencyKey, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		cycle, err = queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: input.CycleID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
		if err != nil {
			return db.RoomCycle{}, ErrNotFound
		}
		if cycle.Status != "cancelled" || !cycle.CancelIdempotencyKey.Valid || cycle.CancelIdempotencyKey.String != input.IdempotencyKey {
			return db.RoomCycle{}, ErrIdempotencyConflict
		}
	} else if err != nil {
		return db.RoomCycle{}, fmt.Errorf("cancel Room cycle: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.RoomCycle{}, fmt.Errorf("commit Room cycle cancellation: %w", err)
	}
	s.publish(EventRoomCycle, roomRow, input.ActorUserID, roomCycleEventPayload(cycle))
	return cycle, nil
}

func (s *Service) ReviewRecommendation(ctx context.Context, input RecommendationReviewInput) (db.RoomRecommendationReview, error) {
	input.RecommendationKey = strings.TrimSpace(input.RecommendationKey)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.WorkspaceID.Valid || !input.RoomID.Valid || !input.MemoryRevisionID.Valid || !input.ActorUserID.Valid ||
		input.Action != "reject" || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return db.RoomRecommendationReview{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return db.RoomRecommendationReview{}, fmt.Errorf("begin Room recommendation review: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	roomRow, err := lockRoomForMember(ctx, queries, input.WorkspaceID, input.RoomID, input.ActorUserID)
	if err != nil {
		return db.RoomRecommendationReview{}, err
	}
	revision, err := queries.GetRoomMemoryRevision(ctx, db.GetRoomMemoryRevisionParams{ID: input.MemoryRevisionID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.RoomRecommendationReview{}, ErrNotFound
	}
	if err != nil {
		return db.RoomRecommendationReview{}, fmt.Errorf("load Room recommendation revision: %w", err)
	}
	var synthesis Synthesis
	if json.Unmarshal(revision.Synthesis, &synthesis) != nil {
		return db.RoomRecommendationReview{}, ErrInvalidSynthesis
	}
	if _, ok := FindRecommendation(synthesis, input.RecommendationKey); !ok {
		return db.RoomRecommendationReview{}, ErrInvalidInput
	}
	digest := lifecycleDigest(input.Action, input.RecommendationKey)
	review, err := queries.CreateRoomRecommendationReview(ctx, db.CreateRoomRecommendationReviewParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, MemoryRevisionID: input.MemoryRevisionID,
		RecommendationKey: input.RecommendationKey, Status: "rejected", IdempotencyKey: input.IdempotencyKey,
		RequestDigest: digest, ReviewedByUserID: input.ActorUserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		review, err = queries.GetRoomRecommendationReview(ctx, db.GetRoomRecommendationReviewParams{
			WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, MemoryRevisionID: input.MemoryRevisionID,
			RecommendationKey: input.RecommendationKey,
		})
		if err == nil && review.IdempotencyKey == input.IdempotencyKey && review.RequestDigest == digest && review.Status == "rejected" {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return db.RoomRecommendationReview{}, fmt.Errorf("commit repeated recommendation review: %w", commitErr)
			}
			return review, nil
		}
		if err == nil {
			return db.RoomRecommendationReview{}, ErrRecommendationReviewed
		}
	}
	if err != nil {
		return db.RoomRecommendationReview{}, fmt.Errorf("create Room recommendation review: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.RoomRecommendationReview{}, fmt.Errorf("commit Room recommendation review: %w", err)
	}
	s.publish(EventRoomRecommendationReview, roomRow, input.ActorUserID, roomRecommendationReviewEventPayload(review))
	return review, nil
}

func (s *Service) enqueueSynthesisTx(ctx context.Context, queries *db.Queries, roomRow db.Room, cycle db.RoomCycle, actorUserID pgtype.UUID, idempotencyKey string, initial bool) (db.RoomTurn, db.AgentTaskQueue, error) {
	costLimitTicks, err := s.ensureSynthesisBudget(ctx, queries, roomRow, cycle)
	if err != nil {
		return db.RoomTurn{}, db.AgentTaskQueue{}, err
	}
	agent, err := queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: roomRow.FacilitatorAgentID, WorkspaceID: roomRow.WorkspaceID})
	if err != nil || agent.ArchivedAt.Valid {
		return db.RoomTurn{}, db.AgentTaskQueue{}, ErrInvalidParticipant
	}
	requiredCapability := protocol.DaemonCapabilityRoomTasksV1
	if roomRow.CapabilityVersion >= 2 {
		requiredCapability = protocol.DaemonCapabilityRoomOutcomesV2
	}
	ready, err := roomAgentReadyForCapability(ctx, queries, agent, requiredCapability)
	if err != nil {
		return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("check Room facilitator readiness: %w", err)
	}
	if !ready {
		return db.RoomTurn{}, db.AgentTaskQueue{}, ErrSynthesisNotRetryable
	}
	if cycle.CostLimitTicks.Valid {
		costReady, costErr := roomAgentReadyForCapability(ctx, queries, agent, protocol.DaemonCapabilityRoomCostLimitsV1)
		if costErr != nil {
			return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("check Room facilitator spend-limit support: %w", costErr)
		}
		if !costReady {
			return db.RoomTurn{}, db.AgentTaskQueue{}, ErrSpendLimitUnsupported
		}
	}
	attempt, err := queries.NextRoomSynthesisAttempt(ctx, db.NextRoomSynthesisAttemptParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID, AgentID: agent.ID,
	})
	if err != nil {
		return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("allocate Room synthesis attempt: %w", err)
	}
	turn, err := queries.CreateRoomTurn(ctx, db.CreateRoomTurnParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
		AgentID: agent.ID, SquadID: roomRow.FacilitatorSquadID, TurnKind: "synthesis",
		Attempt: attempt, Status: "queued", IdempotencyKey: pgtype.Text{String: idempotencyKey, Valid: idempotencyKey != ""},
	})
	if err != nil {
		return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("create Room synthesis turn: %w", err)
	}
	entries, err := queries.ListRoomParticipantResultEntriesByCycle(ctx, db.ListRoomParticipantResultEntriesByCycleParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
	})
	if err != nil {
		return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("load Room participant outputs: %w", err)
	}
	if len(entries) == 0 {
		return db.RoomTurn{}, db.AgentTaskQueue{}, ErrSynthesisNotRetryable
	}
	contextData, err := protocol.EncodeRoomTaskContextV2(protocol.RoomTaskContextV1{
		WorkspaceID: util.UUIDToString(roomRow.WorkspaceID), RoomID: util.UUIDToString(roomRow.ID),
		CycleID: util.UUIDToString(cycle.ID), TurnID: util.UUIDToString(turn.ID), Title: roomRow.Title,
		Instructions: synthesisInstructions(roomRow), Memory: roomRow.Memory, Transcript: roomTaskTranscript(entries), TurnKind: "synthesis",
		CostLimitTicks: costLimitTicks,
	})
	if err != nil {
		return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("encode Room synthesis context: %w", err)
	}
	previous, previousErr := queries.GetLastCompletedRoomTurnForAgent(ctx, db.GetLastCompletedRoomTurnForAgentParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, AgentID: agent.ID,
	})
	if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
		return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("load Room facilitator continuity: %w", previousErr)
	}
	originator := actorUserID
	if initial {
		originator = pgtype.UUID{}
	}
	task, err := s.tasks.EnqueueRoomTurn(ctx, queries, RoomTaskEnqueueInput{
		Agent: agent, RoomTurnID: turn.ID, SquadID: roomRow.FacilitatorSquadID,
		Context: contextData, OriginatorUserID: originator, AccountableUserID: roomRow.CreatedByUserID,
		OriginatorSource: "trigger_owner", TriggerEvidenceKind: "room_cycle", TriggerEvidenceID: cycle.ID,
		SessionID: previous.SessionID, WorkDir: previous.WorkDir,
	})
	if err != nil {
		return db.RoomTurn{}, db.AgentTaskQueue{}, fmt.Errorf("enqueue Room synthesis turn: %w", err)
	}
	return turn, task, nil
}

func (s *Service) ensureSynthesisBudget(ctx context.Context, queries *db.Queries, roomRow db.Room, cycle db.RoomCycle) (*int64, error) {
	if roomRow.DailyTurnLimit.Valid {
		now := s.now().UTC()
		used, err := queries.CountRoomTurnsSince(ctx, db.CountRoomTurnsSinceParams{
			WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
			SinceAt: pgtype.Timestamptz{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("count Room synthesis budget: %w", err)
		}
		if used+1 > int64(roomRow.DailyTurnLimit.Int32) {
			return nil, ErrBudgetExhausted
		}
	}
	if roomRow.MaxCostTicks.Valid {
		usage, err := queries.GetRoomUsageSummary(ctx, db.GetRoomUsageSummaryParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID})
		if err != nil {
			return nil, fmt.Errorf("load Room synthesis budget: %w", err)
		}
		if usage.UncostedTurns > 0 {
			return nil, ErrBudgetExhausted
		}
		remaining := roomRow.MaxCostTicks.Int64 - usage.CostTicks
		if cycle.CostLimitTicks.Valid {
			cycleUsage, cycleUsageErr := queries.GetRoomCycleUsageSummary(ctx, db.GetRoomCycleUsageSummaryParams{
				WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
			})
			if cycleUsageErr != nil {
				return nil, fmt.Errorf("load Room cycle synthesis budget: %w", cycleUsageErr)
			}
			if cycleUsage.UncostedTurns > 0 {
				return nil, ErrBudgetExhausted
			}
			remaining = min(remaining, cycle.CostLimitTicks.Int64-cycleUsage.CostTicks)
		}
		if remaining <= 0 {
			return nil, ErrBudgetExhausted
		}
		return &remaining, nil
	}
	return nil, nil
}

func lockRoomForMember(ctx context.Context, queries *db.Queries, workspaceID, roomID, actorUserID pgtype.UUID) (db.Room, error) {
	if _, err := queries.LockRoomWorkspaceForWrite(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Room{}, ErrNotFound
		}
		return db.Room{}, fmt.Errorf("lock Room workspace: %w", err)
	}
	roomRow, err := queries.GetRoomForUpdate(ctx, db.GetRoomForUpdateParams{ID: roomID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Room{}, ErrNotFound
	}
	if err != nil {
		return db.Room{}, fmt.Errorf("lock Room: %w", err)
	}
	if _, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actorUserID, WorkspaceID: workspaceID}); err != nil {
		return db.Room{}, ErrInvalidParticipant
	}
	return roomRow, nil
}

func roomEntryIDSet(entries []db.RoomEntry) map[string]struct{} {
	result := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		result[util.UUIDToString(entry.ID)] = struct{}{}
	}
	return result
}

func lifecycleDigest(parts ...string) string {
	value := strings.Join(parts, "\x00")
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}
