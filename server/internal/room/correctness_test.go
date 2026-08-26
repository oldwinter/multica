package room

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCreateRejectsRoomConfigBeyondDatabaseJSONLimit(t *testing.T) {
	fixture := newServiceFixture(t)
	tests := []struct {
		name   string
		value  string
		count  int
		accept bool
	}{
		{name: "ascii below limit", value: strings.Repeat("a", 2000), count: 32, accept: true},
		{name: "ascii above limit", value: strings.Repeat("a", 2000), count: 33},
		{name: "CJK below limit", value: strings.Repeat("界", 2000), count: 10, accept: true},
		{name: "CJK above limit", value: strings.Repeat("界", 2000), count: 11},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			criteria := make([]string, test.count)
			for index := range criteria {
				criteria[index] = test.value
			}
			_, err := fixture.service.Create(context.Background(), CreateInput{
				WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
				Title: "Config size " + test.name, FacilitatorAgentID: fixture.leaderID,
				SuccessCriteria: criteria,
			})
			if test.accept && err != nil {
				t.Fatalf("accepted boundary failed: %v", err)
			}
			if !test.accept && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("oversize boundary error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestWakeReplayIgnoresSynthesisTurnsAndValidatesPersistedRequest(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	original := WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:v2-outcome",
	}
	replay, err := fixture.service.Wake(ctx, original)
	if err != nil {
		t.Fatalf("replay after synthesis turn: %v", err)
	}
	if countTurns(replay.Turns, "participant") != 2 || countTurns(replay.Turns, "synthesis") != 1 {
		t.Fatalf("replayed turns = %+v", replay.Turns)
	}
	changedSource := original
	changedSource.Source = "schedule"
	if _, err := fixture.service.Wake(ctx, changedSource); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed source replay error = %v, want ErrIdempotencyConflict", err)
	}
	changedTrigger := original
	changedTrigger.TriggeringEntryID = fixture.leaderID
	if _, err := fixture.service.Wake(ctx, changedTrigger); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed trigger replay error = %v, want ErrIdempotencyConflict", err)
	}
}

func TestSynthesisValidationInfrastructureErrorRollsBack(t *testing.T) {
	fixture := setupPendingOutcome(t)
	revision := fixture.detail.MemoryRevisions[0]
	turn, err := db.New(fixture.pool).GetRoomTurn(context.Background(), db.GetRoomTurnParams{
		ID: revision.SynthesisTurnID, WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, created, err := fixture.service.tryCreateRoomRevision(
		ctx, db.New(fixture.pool), fixture.detail.Room, fixture.detail.Cycles[0], turn, revision.Synthesis, false,
	)
	if created || !errors.Is(err, context.Canceled) || errors.Is(err, ErrInvalidSynthesis) {
		t.Fatalf("infrastructure validation = created %t error %v", created, err)
	}
}

func TestSynthesisBudgetBlocksInitialAndRetryWithoutNewTurn(t *testing.T) {
	fixture := newServiceFixture(t)
	enableRoomOutcomesV2(t, fixture)
	ctx := context.Background()
	limit := int32(3)
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Transactional synthesis budget", FacilitatorSquadID: fixture.squadID,
		DailyTurnLimit: &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:synthesis-budget",
	})
	if err != nil {
		t.Fatal(err)
	}
	completeRoomTask(t, fixture, wake.Tasks[0].ID, "First budgeted result.")
	if changed, err := fixture.service.SyncTask(ctx, wake.Tasks[0].ID); err != nil || !changed {
		t.Fatalf("sync first participant = %t, %v", changed, err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET daily_turn_limit = 2 WHERE id = $1`, created.Room.ID); err != nil {
		t.Fatal(err)
	}
	completeRoomTask(t, fixture, wake.Tasks[1].ID, "Second budgeted result.")
	if changed, err := fixture.service.SyncTask(ctx, wake.Tasks[1].ID); err != nil || !changed {
		t.Fatalf("sync second participant = %t, %v", changed, err)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	var synthesisFailure synthesisError
	if len(detail.Cycles) != 1 || detail.Cycles[0].Status != "running" || detail.Cycles[0].Phase != "awaiting_review" ||
		json.Unmarshal(detail.Cycles[0].SynthesisError, &synthesisFailure) != nil || synthesisFailure.Code != "budget_exhausted" || !synthesisFailure.Retryable {
		t.Fatalf("budget-blocked cycle = %+v error %+v", detail.Cycles, synthesisFailure)
	}
	if countTurns(detail.Turns, "participant") != 2 || countTurns(detail.Turns, "synthesis") != 0 || fixture.notifier.count() != 2 {
		t.Fatalf("budget-blocked work = turns %+v notifications %d", detail.Turns, fixture.notifier.count())
	}
	retry := RetrySynthesisInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, CycleID: detail.Cycles[0].ID,
		ActorUserID: fixture.userID, IdempotencyKey: "synthesis:budget-retry",
	}
	if _, err := fixture.service.RetrySynthesis(ctx, retry); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("budget retry error = %v, want ErrBudgetExhausted", err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET daily_turn_limit = 3 WHERE id = $1`, created.Room.ID); err != nil {
		t.Fatal(err)
	}
	if result, err := fixture.service.RetrySynthesis(ctx, retry); err != nil || result.Turn.TurnKind != "synthesis" {
		t.Fatalf("restored budget retry = %+v, %v", result, err)
	}
	detail, err = fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if countTurns(detail.Turns, "participant") != 2 || countTurns(detail.Turns, "synthesis") != 1 || fixture.notifier.count() != 3 {
		t.Fatalf("restored synthesis work = turns %+v notifications %d", detail.Turns, fixture.notifier.count())
	}
}

func TestRoomCycleFailsWhenNoParticipantProducesAResult(t *testing.T) {
	fixture := newServiceFixture(t)
	enableRoomOutcomesV2(t, fixture)
	ctx := context.Background()
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "No participant results", FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.New(fixture.pool).AddRoomEntry(ctx, db.AddRoomEntryParams{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		EntryType: "message", AuthorType: "member", AuthorID: fixture.userID,
		Body: "An older transcript entry must not be synthesized.", Mentions: []byte("[]"),
	}); err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:no-results",
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, task := range wake.Tasks {
		if _, err := fixture.pool.Exec(ctx, `
			UPDATE agent_task_queue
			SET status = 'failed', error = 'participant failed', failure_reason = 'unknown', completed_at = now()
			WHERE id = $1
		`, task.ID); err != nil {
			t.Fatal(err)
		}
		if changed, err := fixture.service.SyncTask(ctx, task.ID); err != nil || !changed {
			t.Fatalf("sync failed participant %d = %t, %v", index, changed, err)
		}
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	var cycleError synthesisError
	if len(detail.Cycles) != 1 || detail.Cycles[0].Status != "failed" || detail.Cycles[0].Phase != "failed" ||
		json.Unmarshal(detail.Cycles[0].SynthesisError, &cycleError) != nil || cycleError.Code != "participant_results_unavailable" || cycleError.Retryable {
		t.Fatalf("failed no-result cycle = %+v error %+v", detail.Cycles, cycleError)
	}
	if detail.Room.ActiveCycleID.Valid || countTurns(detail.Turns, "participant") != 2 || countTurns(detail.Turns, "synthesis") != 0 || len(detail.Entries) != 1 || fixture.notifier.count() != 2 {
		t.Fatalf("no-result terminal state = room %+v turns %+v entries %+v notifications %d", detail.Room, detail.Turns, detail.Entries, fixture.notifier.count())
	}
}

func TestPromotionReplaySurvivesAcceptedPointerAdvancing(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	if _, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:promotion-replay",
	}); err != nil {
		t.Fatal(err)
	}
	promotion := recommendationPromotion(fixture, "promotion:pointer-advanced")
	mismatched := promotion
	mismatched.IdempotencyKey = "promotion:mismatched-citations"
	mismatched.CitationEntryIDs = []pgtype.UUID{fixture.leaderID}
	if _, err := fixture.service.Promote(ctx, mismatched); !errors.Is(err, ErrPromotionSourceMismatch) {
		t.Fatalf("mismatched promotion citations error = %v, want ErrPromotionSourceMismatch", err)
	}
	first, err := fixture.service.Promote(ctx, promotion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET accepted_memory_revision_id = NULL WHERE id = $1`, fixture.detail.Room.ID); err != nil {
		t.Fatal(err)
	}
	replay, err := fixture.service.Promote(ctx, promotion)
	if err != nil || replay.Created || replay.Artifact.ID != first.Artifact.ID {
		t.Fatalf("promotion replay after pointer advance = %+v, %v", replay, err)
	}
	changed := promotion
	changed.Title = "Changed replay payload"
	if _, err := fixture.service.Promote(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed promotion replay error = %v, want ErrIdempotencyConflict", err)
	}
}

func enableRoomOutcomesV2(t *testing.T, fixture serviceFixture) {
	t.Helper()
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
}
