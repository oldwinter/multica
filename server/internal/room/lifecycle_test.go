package room

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type pendingOutcomeFixture struct {
	serviceFixture
	detail         Detail
	participantIDs []string
	recommendation ArtifactRecommendation
}

func setupPendingOutcome(t *testing.T) pendingOutcomeFixture {
	t.Helper()
	fixture := newServiceFixture(t)
	ctx := context.Background()
	if _, err := fixture.pool.Exec(ctx, `UPDATE workspace SET settings = '{"room_outcomes_v2":true}'::jsonb WHERE id = $1`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `
		UPDATE agent_runtime
		SET metadata = '{"capabilities":["room-tasks-v1","room-outcomes-v2"]}'::jsonb
		WHERE workspace_id = $1
	`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Outcome review", Objective: "Choose a safe rollout.",
		SuccessCriteria: []string{"Cited outcome"}, StopConditions: []string{"No evidence"},
		TemplateID: "decision", FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Room.CapabilityVersion != 2 {
		t.Fatalf("capability version = %d, want 2", created.Room.CapabilityVersion)
	}
	preflight, err := fixture.service.Preflight(ctx, PreflightInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.Allowed || !preflight.SynthesisRequired || preflight.ExpectedMaxTurns != 3 || !preflight.CapabilityReady {
		t.Fatalf("v2 preflight = %+v", preflight)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:v2-outcome",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wake.Tasks) != 2 || len(wake.Turns) != 2 {
		t.Fatalf("participant tasks/turns = %d/%d, want 2/2", len(wake.Tasks), len(wake.Turns))
	}
	completeRoomTask(t, fixture, wake.Tasks[0].ID, "First participant evidence.")
	if changed, syncErr := fixture.service.SyncTask(ctx, wake.Tasks[0].ID); syncErr != nil || !changed {
		t.Fatalf("sync first participant = %t, %v", changed, syncErr)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "gathering" || countTurns(detail.Turns, "synthesis") != 0 {
		t.Fatalf("first participant advanced synthesis: phase=%q turns=%+v", detail.Cycles[0].Phase, detail.Turns)
	}
	completeRoomTask(t, fixture, wake.Tasks[1].ID, "Second participant evidence.")
	if changed, syncErr := fixture.service.SyncTask(ctx, wake.Tasks[1].ID); syncErr != nil || !changed {
		t.Fatalf("sync second participant = %t, %v", changed, syncErr)
	}
	detail, err = fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "synthesizing" || countTurns(detail.Turns, "participant") != 2 || countTurns(detail.Turns, "synthesis") != 1 {
		t.Fatalf("participant barrier result: cycle=%+v turns=%+v", detail.Cycles[0], detail.Turns)
	}
	participantIDs := make([]string, 0, 2)
	for _, entry := range detail.Entries {
		if entry.TurnID.Valid {
			participantIDs = append(participantIDs, util.UUIDToString(entry.ID))
		}
	}
	if len(participantIDs) != 2 {
		t.Fatalf("participant result entries = %d, want 2", len(participantIDs))
	}
	synthesisTask := latestTaskForKind(t, fixture, created.Room.ID, "synthesis")
	parsedContext, err := protocol.ParseRoomTaskContext(synthesisTask.Context)
	if err != nil {
		t.Fatal(err)
	}
	if parsedContext.SchemaVersion != protocol.RoomTaskContextSchemaV2 || parsedContext.TurnKind != "synthesis" || len(parsedContext.Transcript) != 2 {
		t.Fatalf("synthesis task context = %+v", parsedContext)
	}
	raw := testSynthesis(t, participantIDs, "Promote the rollout decision.")
	completeRoomTask(t, fixture, synthesisTask.ID, string(raw))
	if changed, syncErr := fixture.service.SyncTask(ctx, synthesisTask.ID); syncErr != nil || !changed {
		t.Fatalf("sync synthesis = %t, %v", changed, syncErr)
	}
	detail, err = fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "awaiting_review" || len(detail.MemoryRevisions) != 1 || detail.MemoryRevisions[0].ReviewStatus != "pending" {
		t.Fatalf("pending outcome = cycle %+v revisions %+v", detail.Cycles[0], detail.MemoryRevisions)
	}
	var stored Synthesis
	if err := json.Unmarshal(detail.MemoryRevisions[0].Synthesis, &stored); err != nil {
		t.Fatal(err)
	}
	return pendingOutcomeFixture{
		serviceFixture: fixture, detail: detail, participantIDs: participantIDs,
		recommendation: stored.Recommendations[0],
	}
}

func TestOutcomeRejectReplaysAndRetryDoesNotRerunParticipants(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	cycle := fixture.detail.Cycles[0]
	revision := fixture.detail.MemoryRevisions[0]
	reviewInput := ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: cycle.ID,
		ActorUserID: fixture.userID, Action: "reject", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:reject",
	}
	first, err := fixture.service.Review(ctx, reviewInput)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := fixture.service.Review(ctx, reviewInput)
	if err != nil {
		t.Fatal(err)
	}
	if first.MemoryRevision.ID != revision.ID || !replay.Replayed || replay.MemoryRevision.ID != revision.ID {
		t.Fatalf("reject replay = first %+v replay %+v", first, replay)
	}
	conflict := reviewInput
	conflict.Action = "accept"
	if _, err := fixture.service.Review(ctx, conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed review replay error = %v, want idempotency conflict", err)
	}
	if _, err := fixture.service.Promote(ctx, recommendationPromotion(fixture, "promotion:rejected")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("rejected recommendation promotion error = %v, want invalid input", err)
	}

	retryInput := RetrySynthesisInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: cycle.ID,
		ActorUserID: fixture.userID, IdempotencyKey: "synthesis:retry-1",
	}
	retry, err := fixture.service.RetrySynthesis(ctx, retryInput)
	if err != nil {
		t.Fatal(err)
	}
	retryReplay, err := fixture.service.RetrySynthesis(ctx, retryInput)
	if err != nil {
		t.Fatal(err)
	}
	if !retryReplay.Replayed || retryReplay.Turn.ID != retry.Turn.ID {
		t.Fatalf("retry replay = first %+v replay %+v", retry, retryReplay)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "synthesizing" || countTurns(detail.Turns, "participant") != 2 || countTurns(detail.Turns, "synthesis") != 2 {
		t.Fatalf("targeted retry state: cycle=%+v turns=%+v", detail.Cycles[0], detail.Turns)
	}
	completeRoomTask(t, fixture.serviceFixture, retry.Task.ID, "ordinary malformed contribution")
	if changed, syncErr := fixture.service.SyncTask(ctx, retry.Task.ID); syncErr != nil || !changed {
		t.Fatalf("sync malformed retry = %t, %v", changed, syncErr)
	}
	detail, err = fixture.service.Get(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "awaiting_review" || len(detail.Cycles[0].SynthesisError) == 0 || len(detail.Entries) != 4 {
		t.Fatalf("malformed retry state: cycle=%+v entries=%d", detail.Cycles[0], len(detail.Entries))
	}
}

func TestAcceptedRecommendationPromotionRequiresCurrentCompletedOutcomeAndIsIdempotent(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	promotion := recommendationPromotion(fixture, "promotion:accepted")
	if _, err := fixture.service.Promote(ctx, promotion); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("pending recommendation promotion error = %v, want invalid input", err)
	}
	accepted, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:accept",
	})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:accept",
	})
	if err != nil || !replayed.Replayed || accepted.MemoryRevision.ID != replayed.MemoryRevision.ID {
		t.Fatalf("accept replay = %+v, %v", replayed, err)
	}
	wrongKind := promotion
	wrongKind.Kind = "wiki"
	wrongKind.IdempotencyKey = "promotion:wrong-kind"
	if _, err := fixture.service.Promote(ctx, wrongKind); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("wrong recommendation kind error = %v, want invalid input", err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET accepted_memory_revision_id = NULL WHERE id = $1`, fixture.detail.Room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Promote(ctx, promotion); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("non-current recommendation error = %v, want invalid input", err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET accepted_memory_revision_id = $2 WHERE id = $1`, fixture.detail.Room.ID, accepted.MemoryRevision.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room_cycle SET status = 'running', phase = 'awaiting_review' WHERE id = $1`, fixture.detail.Cycles[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Promote(ctx, promotion); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("non-completed recommendation error = %v, want invalid input", err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room_cycle SET status = 'completed', phase = 'completed' WHERE id = $1`, fixture.detail.Cycles[0].ID); err != nil {
		t.Fatal(err)
	}
	first, err := fixture.service.Promote(ctx, promotion)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.Promote(ctx, promotion)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || first.Artifact.ID != second.Artifact.ID || first.Artifact.RecommendationKey.String != fixture.recommendation.Key {
		t.Fatalf("recommendation promotion replay = first %+v second %+v", first, second)
	}
	if string(first.Artifact.CitationEntryIds) == "[]" || first.Artifact.Body != promotion.Body {
		t.Fatalf("promotion provenance/body = %+v", first.Artifact)
	}
	var reviewCount int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT count(*) FROM room_recommendation_review
		WHERE room_id = $1 AND memory_revision_id = $2 AND recommendation_key = $3
		  AND status = 'approved' AND artifact_id = $4
	`, fixture.detail.Room.ID, accepted.MemoryRevision.ID, fixture.recommendation.Key, first.Artifact.ID).Scan(&reviewCount); err != nil {
		t.Fatal(err)
	}
	if reviewCount != 1 {
		t.Fatalf("approved recommendation reviews = %d, want 1", reviewCount)
	}
}

func TestCorrectReviewIsConcurrentSafeAndUsesExactCitationOwnership(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	if _, err := fixture.pool.Exec(ctx, `
		WITH allocated AS (
			UPDATE room SET last_entry_ordinal = last_entry_ordinal + 205 WHERE id = $1
			RETURNING last_entry_ordinal - 205 AS base
		)
		INSERT INTO room_entry (workspace_id, room_id, ordinal, entry_type, author_type, author_id, body, mentions)
		SELECT $2, $1, allocated.base + value, 'message', 'member', $3, 'later message', '[]'::jsonb
		FROM allocated, generate_series(1, 205) AS value
	`, fixture.detail.Room.ID, fixture.workspaceID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	correction := testSynthesis(t, fixture.participantIDs, "Corrected recommendation body.")
	input := ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "correct", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		Correction: correction, IdempotencyKey: "review:correct",
	}
	results := make(chan ReviewResult, 2)
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := fixture.service.Review(ctx, input)
			if err != nil {
				errorsChannel <- err
				return
			}
			results <- result
		}()
	}
	wait.Wait()
	close(results)
	close(errorsChannel)
	for err := range errorsChannel {
		t.Errorf("concurrent correction: %v", err)
	}
	var correctedID pgtype.UUID
	resultCount := 0
	for result := range results {
		resultCount++
		if !correctedID.Valid {
			correctedID = result.MemoryRevision.ID
		} else if correctedID != result.MemoryRevision.ID {
			t.Fatalf("concurrent correction created different revisions: %v and %v", correctedID, result.MemoryRevision.ID)
		}
	}
	if resultCount != 2 {
		t.Fatalf("successful concurrent corrections = %d, want 2", resultCount)
	}
	replay, err := fixture.service.Review(ctx, input)
	if err != nil || !replay.Replayed || replay.MemoryRevision.ID != correctedID {
		t.Fatalf("correction replay = %+v, %v", replay, err)
	}
	var revisionCount int
	if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM room_memory_revision WHERE room_id = $1`, fixture.detail.Room.ID).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if revisionCount != 2 {
		t.Fatalf("memory revisions after concurrent correction = %d, want 2", revisionCount)
	}
	changed := input
	changed.Correction = testSynthesis(t, fixture.participantIDs, "Different corrected body.")
	if _, err := fixture.service.Review(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed correction replay error = %v, want idempotency conflict", err)
	}
	stale := input
	stale.IdempotencyKey = "review:stale"
	stale.ExpectedMemoryVersion++
	if _, err := fixture.service.Review(ctx, stale); !errors.Is(err, ErrStaleReview) {
		t.Fatalf("stale correction error = %v, want stale review", err)
	}
}

func completeRoomTask(t *testing.T, fixture serviceFixture, taskID pgtype.UUID, output string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"output": output})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'completed', result = $2::jsonb, started_at = now(), completed_at = now()
		WHERE id = $1
	`, taskID, payload); err != nil {
		t.Fatal(err)
	}
}

func latestTaskForKind(t *testing.T, fixture serviceFixture, roomID pgtype.UUID, kind string) db.AgentTaskQueue {
	t.Helper()
	var taskID pgtype.UUID
	if err := fixture.pool.QueryRow(context.Background(), `
		SELECT task.id
		FROM agent_task_queue task
		JOIN room_turn turn ON turn.id = task.room_turn_id
		WHERE turn.room_id = $1 AND turn.turn_kind = $2
		ORDER BY turn.attempt DESC, task.attempt DESC
		LIMIT 1
	`, roomID, kind).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	task, err := db.New(fixture.pool).GetAgentTask(context.Background(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func testSynthesis(t *testing.T, citations []string, recommendationBody string) []byte {
	t.Helper()
	value := Synthesis{
		SchemaVersion: RoomSynthesisSchemaVersion, Summary: "The participants supplied a cited outcome.",
		Facts:     []SynthesisItem{{Text: "Both participant outputs were considered.", CitationEntryIDs: citations, Confidence: 0.9}},
		Decisions: []SynthesisItem{}, OpenQuestions: []SynthesisItem{}, Disagreements: []SynthesisItem{}, ActionItems: []SynthesisItem{},
		Recommendations: []ArtifactRecommendation{{
			Kind: "decision", Title: "Rollout decision", Body: recommendationBody,
			Rationale: "Preserve the reviewed outcome.", CitationEntryIDs: citations, Confidence: 0.85,
		}},
		Confidence: 0.88,
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func recommendationPromotion(fixture pendingOutcomeFixture, key string) PromotionInput {
	return PromotionInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, ActorUserID: fixture.userID,
		MemoryRevisionID: fixture.detail.MemoryRevisions[0].ID, RecommendationKey: fixture.recommendation.Key,
		Kind: "decision", IdempotencyKey: key, Title: "Edited rollout decision",
		Body: "Human-edited decision body.", Rationale: "Accepted by the Room owner.",
	}
}

func countTurns(turns []db.RoomTurn, kind string) int {
	count := 0
	for _, turn := range turns {
		if turn.TurnKind == kind {
			count++
		}
	}
	return count
}
