package room

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type citationFailingTxStarter struct {
	base    TxStarter
	failure error
}

func (starter citationFailingTxStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := starter.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &citationFailingTx{Tx: tx, failure: starter.failure}, nil
}

type citationFailingTx struct {
	pgx.Tx
	failure error
}

func (tx *citationFailingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: ListRoomEntriesByIDs") {
		return nil, tx.failure
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func TestCancelCycleAtomicallyCancelsWorkAndProtectsTerminalCycles(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Cancellable Room", FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:cancel-atomic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wake.Tasks) != 2 || len(wake.Turns) != 2 {
		t.Fatalf("wake tasks/turns = %d/%d, want 2/2", len(wake.Tasks), len(wake.Turns))
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE agent_task_queue SET status = 'running', started_at = now() WHERE id = $1`, wake.Tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room_turn SET status = 'running', started_at = now() WHERE id = $1`, wake.Turns[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room_cycle SET status = 'running', started_at = now() WHERE id = $1`, wake.Cycle.ID); err != nil {
		t.Fatal(err)
	}

	input := CancelInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, CycleID: wake.Cycle.ID,
		ActorUserID: fixture.userID, IdempotencyKey: "cancel:atomic",
	}
	cancelled, err := fixture.service.Cancel(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" || cancelled.Phase != "cancelled" ||
		!cancelled.CancelIdempotencyKey.Valid || cancelled.CancelIdempotencyKey.String != input.IdempotencyKey {
		t.Fatalf("cancelled cycle = %+v", cancelled)
	}
	var cancelledTasks, completedTasks, cancelledTurns, completedTurns int
	var activeCycle pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM agent_task_queue task
			 JOIN room_turn turn ON turn.id = task.room_turn_id
			 WHERE turn.cycle_id = $1 AND task.status = 'cancelled'),
			(SELECT count(*) FROM agent_task_queue task
			 JOIN room_turn turn ON turn.id = task.room_turn_id
			 WHERE turn.cycle_id = $1 AND task.completed_at IS NOT NULL),
			(SELECT count(*) FROM room_turn WHERE cycle_id = $1 AND status = 'cancelled'),
			(SELECT count(*) FROM room_turn WHERE cycle_id = $1 AND completed_at IS NOT NULL),
			(SELECT active_cycle_id FROM room WHERE id = $2)
	`, wake.Cycle.ID, created.Room.ID).Scan(&cancelledTasks, &completedTasks, &cancelledTurns, &completedTurns, &activeCycle); err != nil {
		t.Fatal(err)
	}
	if cancelledTasks != 2 || completedTasks != 2 || cancelledTurns != 2 || completedTurns != 2 || activeCycle.Valid {
		t.Fatalf("cancelled state tasks=%d completed_tasks=%d turns=%d completed_turns=%d active=%v",
			cancelledTasks, completedTasks, cancelledTurns, completedTurns, activeCycle)
	}
	replay, err := fixture.service.Cancel(ctx, input)
	if err != nil || replay.ID != cancelled.ID {
		t.Fatalf("cancel replay = %+v, %v", replay, err)
	}
	conflict := input
	conflict.IdempotencyKey = "cancel:different"
	if _, err := fixture.service.Cancel(ctx, conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("different cancel key error = %v, want idempotency conflict", err)
	}

	terminal, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Completed Room", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	terminalWake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: terminal.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:complete-before-cancel",
	})
	if err != nil {
		t.Fatal(err)
	}
	completeRoomTask(t, fixture, terminalWake.Tasks[0].ID, "Completed contribution.")
	if changed, err := fixture.service.SyncTask(ctx, terminalWake.Tasks[0].ID); err != nil || !changed {
		t.Fatalf("sync terminal task = %t, %v", changed, err)
	}
	if _, err := fixture.service.Cancel(ctx, CancelInput{
		WorkspaceID: fixture.workspaceID, RoomID: terminal.Room.ID, CycleID: terminalWake.Cycle.ID,
		ActorUserID: fixture.userID, IdempotencyKey: "cancel:terminal",
	}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("terminal cancel error = %v, want idempotency conflict", err)
	}
}

func TestRecommendationRejectIsIdempotentAndBlocksPromotion(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	accepted, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:accept-before-recommendation-reject",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := RecommendationReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID,
		MemoryRevisionID: accepted.MemoryRevision.ID, RecommendationKey: fixture.recommendation.Key,
		ActorUserID: fixture.userID, Action: "reject", IdempotencyKey: "recommendation:reject",
	}
	rejected, err := fixture.service.ReviewRecommendation(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := fixture.service.ReviewRecommendation(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.ID != replay.ID || rejected.Status != "rejected" || rejected.ArtifactID.Valid {
		t.Fatalf("recommendation reject/replay = first %+v replay %+v", rejected, replay)
	}
	changedKey := input
	changedKey.IdempotencyKey = "recommendation:different-key"
	if _, err := fixture.service.ReviewRecommendation(ctx, changedKey); !errors.Is(err, ErrRecommendationReviewed) {
		t.Fatalf("changed recommendation review key error = %v, want already reviewed", err)
	}
	if _, err := fixture.service.Promote(ctx, recommendationPromotion(fixture, "promotion:after-reject")); !errors.Is(err, ErrRecommendationReviewed) {
		t.Fatalf("rejected recommendation promotion error = %v, want already reviewed", err)
	}
	missing := input
	missing.RecommendationKey = "missing-recommendation"
	missing.IdempotencyKey = "recommendation:missing"
	if _, err := fixture.service.ReviewRecommendation(ctx, missing); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing recommendation error = %v, want invalid input", err)
	}

	other := setupPendingOutcome(t)
	crossRoom := input
	crossRoom.MemoryRevisionID = other.detail.MemoryRevisions[0].ID
	crossRoom.IdempotencyKey = "recommendation:cross-room"
	if _, err := fixture.service.ReviewRecommendation(ctx, crossRoom); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-Room revision error = %v, want not found", err)
	}
}

func TestRecommendationRejectAfterApprovalIsRefused(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	if _, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:accept-before-approval",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Promote(ctx, recommendationPromotion(fixture, "promotion:approve-before-reject")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ReviewRecommendation(ctx, RecommendationReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID,
		MemoryRevisionID: fixture.detail.MemoryRevisions[0].ID, RecommendationKey: fixture.recommendation.Key,
		ActorUserID: fixture.userID, Action: "reject", IdempotencyKey: "recommendation:reject-after-approval",
	}); !errors.Is(err, ErrRecommendationReviewed) {
		t.Fatalf("reject after approval error = %v, want already reviewed", err)
	}
}

func TestPreflightFailsClosedForCapabilitiesAndReservesV2Synthesis(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	enableRoomOutcomes(t, fixture)
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Capability Room", FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		metadata string
	}{
		{name: "missing", metadata: `{}`},
		{name: "malformed shape", metadata: `{"capabilities":"room-outcomes-v2"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := fixture.pool.Exec(ctx, `UPDATE agent_runtime SET metadata = $2::jsonb WHERE workspace_id = $1`, fixture.workspaceID, test.metadata); err != nil {
				t.Fatal(err)
			}
			preflight, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, created.Room.ID, "manual"))
			if err != nil {
				t.Fatal(err)
			}
			if preflight.Allowed || preflight.CapabilityReady || preflight.RefusalReason != "agent_unavailable" ||
				preflight.ExpectedMaxTurns != 3 || !preflight.SynthesisRequired ||
				preflight.RequiredDaemonCapability != "room-outcomes-v2" {
				t.Fatalf("fail-closed preflight = %+v", preflight)
			}
			for _, agent := range preflight.TargetAgents {
				if agent.Ready || agent.Reason != "daemon_capability_unavailable" {
					t.Fatalf("fail-closed agent = %+v", agent)
				}
			}
		})
	}
	setRoomOutcomeRuntimeReady(t, fixture)
	ready, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, created.Room.ID, "manual"))
	if err != nil {
		t.Fatal(err)
	}
	if !ready.Allowed || !ready.CapabilityReady || ready.ExpectedMaxTurns != 3 || !ready.SynthesisRequired {
		t.Fatalf("ready v2 preflight = %+v", ready)
	}
	if _, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:active-preflight",
	}); err != nil {
		t.Fatal(err)
	}
	active, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, created.Room.ID, "manual"))
	if err != nil {
		t.Fatal(err)
	}
	if active.Allowed || active.RefusalReason != "cycle_active" {
		t.Fatalf("active cycle preflight = %+v", active)
	}
}

func TestRunAgainUsesFreshCapabilityPreflight(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	created := createSingleOutcomeRoom(t, fixture, "Fresh preflight Room", 0)
	first, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "manual:first-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Cycle.Status != "queued" || len(first.Tasks) != 1 {
		t.Fatalf("first run = %+v", first)
	}
	clearActiveCycle(t, fixture, created.Room.ID, first.Cycle.ID)

	if _, err := fixture.pool.Exec(ctx, `UPDATE agent_runtime SET metadata = '{}'::jsonb WHERE workspace_id = $1`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	currentPreflight, err := fixture.service.Preflight(
		ctx,
		roomTestPreflightInput(fixture, created.Room.ID, "manual"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if currentPreflight.Allowed || currentPreflight.RefusalReason != "agent_unavailable" || currentPreflight.CapabilityReady {
		t.Fatalf("current run-again preflight = %+v", currentPreflight)
	}
	blocked, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "manual:run-again-blocked",
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Cycle.Status != "refused" || blocked.Cycle.RefusalReason.String != "no_targets" || len(blocked.Tasks) != 0 {
		t.Fatalf("run again reused stale readiness: %+v", blocked)
	}
	var activeBlocked int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT count(*) FROM inbox_item
		WHERE workspace_id = $1 AND room_id = $2
		  AND type = 'room_cycle_blocked' AND archived = false
	`, fixture.workspaceID, created.Room.ID).Scan(&activeBlocked); err != nil {
		t.Fatal(err)
	}
	if activeBlocked != 1 {
		t.Fatalf("active blocked attention = %d, want 1", activeBlocked)
	}

	setRoomOutcomeRuntimeReady(t, fixture)
	ready, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "manual:run-again-ready",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ready.Cycle.Status != "queued" || ready.Cycle.Sequence != 3 || len(ready.Tasks) != 1 {
		t.Fatalf("fresh run again = %+v", ready)
	}
	var activeSuperseded int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT count(*) FROM inbox_item
		WHERE workspace_id = $1 AND room_id = $2
		  AND type IN ('room_cycle_blocked', 'room_cycle_failed') AND archived = false
	`, fixture.workspaceID, created.Room.ID).Scan(&activeSuperseded); err != nil {
		t.Fatal(err)
	}
	if activeSuperseded != 0 {
		t.Fatalf("superseded run attention still active = %d", activeSuperseded)
	}
}

func TestPreflightAndUsageEnforceCostAndUncostedBudgets(t *testing.T) {
	t.Run("cost limit fails closed without execution capability", func(t *testing.T) {
		fixture := newServiceFixture(t)
		ctx := context.Background()
		enableRoomOutcomes(t, fixture)
		setRoomOutcomeRuntimeReady(t, fixture)
		maxCost := int64(100)
		created, err := fixture.service.Create(ctx, CreateInput{
			WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
			Title: "Unsupported cost Room", FacilitatorAgentID: fixture.leaderID,
			MaxCostTicks: &maxCost,
		})
		if err != nil {
			t.Fatal(err)
		}
		preflight, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, created.Room.ID, "manual"))
		if err != nil {
			t.Fatal(err)
		}
		if preflight.Allowed || preflight.SpendLimitSupported ||
			preflight.RefusalReason != "spend_limit_unsupported" ||
			preflight.RequiredCostCapability != protocol.DaemonCapabilityRoomCostLimitsV1 {
			t.Fatalf("unsupported spend preflight = %+v", preflight)
		}
		wake, err := fixture.service.Wake(ctx, WakeInput{
			WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
			Source: "manual", WakeKey: "manual:unsupported-cost-limit",
		})
		if err != nil {
			t.Fatal(err)
		}
		if wake.Cycle.Status != "refused" || wake.Cycle.RefusalReason.String != "spend_limit_unsupported" ||
			len(wake.Tasks) != 0 {
			t.Fatalf("unsupported spend wake = %+v", wake)
		}
	})

	t.Run("priced usage clamps remaining budget", func(t *testing.T) {
		fixture := newServiceFixture(t)
		ctx := context.Background()
		created := createSingleOutcomeRoom(t, fixture, "Priced Room", 100)
		wake, err := fixture.service.Wake(ctx, WakeInput{
			WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
			Source: "manual", WakeKey: "manual:priced-budget",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.New(fixture.pool).UpsertTaskUsage(ctx, db.UpsertTaskUsageParams{
			TaskID: wake.Tasks[0].ID, Provider: "test", Model: "priced",
			InputTokens: 10, OutputTokens: 5, CostUsdTicks: pgtype.Int8{Int64: 150, Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
		usage, err := fixture.service.Usage(ctx, fixture.workspaceID, created.Room.ID)
		if err != nil {
			t.Fatal(err)
		}
		if usage.TurnsTotal != 1 || usage.CostTicks != 150 || usage.UncostedTurns != 0 {
			t.Fatalf("priced usage = %+v", usage)
		}
		clearActiveCycle(t, fixture, created.Room.ID, wake.Cycle.ID)
		preflight, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, created.Room.ID, "manual"))
		if err != nil {
			t.Fatal(err)
		}
		if preflight.Allowed || preflight.RefusalReason != "budget_exhausted" ||
			preflight.Budget.RemainingCostTicks == nil || *preflight.Budget.RemainingCostTicks != 0 ||
			preflight.Budget.UsedCostTicks != 150 || preflight.Budget.UncostedTurns != 0 {
			t.Fatalf("priced budget preflight = %+v", preflight)
		}
	})

	t.Run("uncosted latest task blocks further work", func(t *testing.T) {
		fixture := newServiceFixture(t)
		ctx := context.Background()
		created := createSingleOutcomeRoom(t, fixture, "Uncosted Room", 100)
		wake, err := fixture.service.Wake(ctx, WakeInput{
			WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
			Source: "manual", WakeKey: "manual:uncosted-budget",
		})
		if err != nil {
			t.Fatal(err)
		}
		usage, err := fixture.service.Usage(ctx, fixture.workspaceID, created.Room.ID)
		if err != nil {
			t.Fatal(err)
		}
		if usage.TurnsTotal != 1 || usage.CostTicks != 0 || usage.UncostedTurns != 1 {
			t.Fatalf("uncosted usage = %+v", usage)
		}
		clearActiveCycle(t, fixture, created.Room.ID, wake.Cycle.ID)
		preflight, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, created.Room.ID, "manual"))
		if err != nil {
			t.Fatal(err)
		}
		if preflight.Allowed || preflight.RefusalReason != "budget_exhausted" ||
			preflight.Budget.RemainingCostTicks == nil || *preflight.Budget.RemainingCostTicks != 100 ||
			preflight.Budget.UncostedTurns != 1 {
			t.Fatalf("uncosted budget preflight = %+v", preflight)
		}
	})
}

func TestRoomValueSignalsTrackOutcomeReuseReviewPromotionAndCost(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	accepted, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID,
		CycleID: fixture.detail.Cycles[0].ID, ActorUserID: fixture.userID,
		Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "value:accept",
	})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.MemoryRevision.ReviewStatus != "accepted" {
		t.Fatalf("accepted revision = %+v", accepted.MemoryRevision)
	}
	if promoted, err := fixture.service.Promote(ctx, recommendationPromotion(fixture, "value:promote")); err != nil || !promoted.Created {
		t.Fatalf("promote accepted recommendation = %+v, %v", promoted, err)
	}

	participantTask := latestTaskForKind(t, fixture.serviceFixture, fixture.detail.Room.ID, "participant")
	if err := db.New(fixture.pool).UpsertTaskUsage(ctx, db.UpsertTaskUsageParams{
		TaskID: participantTask.ID, Provider: "test", Model: "value",
		InputTokens: 10, OutputTokens: 5,
		CostUsdTicks: pgtype.Int8{Int64: 100, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET daily_turn_limit = 1 WHERE id = $1`, fixture.detail.Room.ID); err != nil {
		t.Fatal(err)
	}
	refused, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID,
		ActorUserID: fixture.userID, Source: "manual", WakeKey: "value:repeat-refused",
	})
	if err != nil {
		t.Fatal(err)
	}
	if refused.Cycle.Status != "refused" || refused.Cycle.RefusalReason.String != "budget_exhausted" {
		t.Fatalf("repeat refusal = %+v", refused.Cycle)
	}

	signals, err := fixture.service.ListValueSignals(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("value signals = %+v", signals)
	}
	signal := signals[0]
	if signal.RoomID != fixture.detail.Room.ID || signal.LastAcceptedRevisionID != accepted.MemoryRevision.ID ||
		signal.LastRunStatus != "refused" || signal.LastRunReason.String != "budget_exhausted" ||
		signal.RepeatRunCount != 1 || signal.AcceptedOutcomes != 1 || signal.ActiveWeeks != 1 ||
		signal.AcceptedOutcomesPerActiveWeek != 1 || signal.PromotionRate != 1 ||
		signal.FailedCycles != 0 || signal.RefusedCycles != 1 ||
		!signal.LastAcceptedAt.Valid || !signal.LastRunAt.Valid || signal.MedianReviewLatencySeconds < 0 {
		t.Fatalf("Room value signal = %+v", signal)
	}

	usage, err := fixture.service.Usage(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage.AcceptedSyntheses != 1 || usage.PromotedArtifacts != 1 ||
		usage.RepeatRunCount != 1 || usage.ActiveWeeks != 1 || usage.RefusedCycles != 1 ||
		usage.AcceptedOutcomesPerActiveWeek != 1 || usage.PromotionRate != 1 ||
		usage.CostTicks != 100 || usage.CostTicksPerAcceptedOutcome != 100 ||
		usage.MedianReviewLatencySeconds < 0 {
		t.Fatalf("Room usage value summary = %+v", usage)
	}
}

func TestSingleParticipantDirectSynthesisAndMalformedFallback(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	direct := createSingleOutcomeRoom(t, fixture, "Direct Synthesis Room", 0)
	citation, err := db.New(fixture.pool).AddRoomEntry(ctx, db.AddRoomEntryParams{
		WorkspaceID: fixture.workspaceID, RoomID: direct.Room.ID,
		EntryType: "message", AuthorType: "member", AuthorID: fixture.userID,
		Body: "A verified source supports the decision.", Mentions: []byte("[]"),
	})
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, direct.Room.ID, "manual"))
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.Allowed || preflight.ExpectedMaxTurns != 2 || preflight.SynthesisRequired {
		t.Fatalf("single participant preflight = %+v", preflight)
	}
	scheduled, err := fixture.service.Preflight(ctx, roomTestPreflightInput(fixture, direct.Room.ID, "schedule"))
	if err != nil {
		t.Fatal(err)
	}
	if scheduled.Source != "schedule" || !scheduled.Allowed ||
		scheduled.ExpectedMaxTurns != 2 || !scheduled.SynthesisRequired {
		t.Fatalf("single participant scheduled preflight = %+v", scheduled)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: direct.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:direct-synthesis",
	})
	if err != nil {
		t.Fatal(err)
	}
	output := testSynthesis(t, []string{roomUUIDString(citation.ID)}, "Use the verified decision.")
	completeRoomTask(t, fixture, wake.Tasks[0].ID, string(output))
	if changed, err := fixture.service.SyncTask(ctx, wake.Tasks[0].ID); err != nil || !changed {
		t.Fatalf("sync direct synthesis = %t, %v", changed, err)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, direct.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "awaiting_review" || len(detail.MemoryRevisions) != 1 ||
		countTurns(detail.Turns, "participant") != 1 || countTurns(detail.Turns, "synthesis") != 0 ||
		detail.MemoryRevisions[0].SynthesisTurnID != wake.Turns[0].ID {
		t.Fatalf("direct synthesis detail = cycle %+v revisions %+v turns %+v", detail.Cycles[0], detail.MemoryRevisions, detail.Turns)
	}

	fallback := createSingleOutcomeRoom(t, fixture, "Fallback Synthesis Room", 0)
	fallbackWake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: fallback.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:fallback-synthesis",
	})
	if err != nil {
		t.Fatal(err)
	}
	completeRoomTask(t, fixture, fallbackWake.Tasks[0].ID, "This is not structured synthesis JSON.")
	if changed, err := fixture.service.SyncTask(ctx, fallbackWake.Tasks[0].ID); err != nil || !changed {
		t.Fatalf("sync malformed direct output = %t, %v", changed, err)
	}
	fallbackDetail, err := fixture.service.Get(ctx, fixture.workspaceID, fallback.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fallbackDetail.Cycles[0].Phase != "synthesizing" || len(fallbackDetail.MemoryRevisions) != 0 ||
		countTurns(fallbackDetail.Turns, "participant") != 1 || countTurns(fallbackDetail.Turns, "synthesis") != 1 {
		t.Fatalf("fallback synthesis detail = cycle %+v revisions %+v turns %+v", fallbackDetail.Cycles[0], fallbackDetail.MemoryRevisions, fallbackDetail.Turns)
	}
	synthesisTask := latestTaskForKind(t, fixture, fallback.Room.ID, "synthesis")
	if synthesisTask.Status != "queued" {
		t.Fatalf("fallback synthesis task status = %q, want queued", synthesisTask.Status)
	}
}

func TestSynthesisCitationInfrastructureErrorRollsBack(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	created := createSingleOutcomeRoom(t, fixture, "Citation Failure Room", 0)
	citation, err := db.New(fixture.pool).AddRoomEntry(ctx, db.AddRoomEntryParams{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		EntryType: "message", AuthorType: "member", AuthorID: fixture.userID,
		Body: "Durable citation source.", Mentions: []byte("[]"),
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:citation-failure",
	})
	if err != nil {
		t.Fatal(err)
	}
	completeRoomTask(t, fixture, wake.Tasks[0].ID, string(testSynthesis(t, []string{roomUUIDString(citation.ID)}, "Cited recommendation.")))
	sentinel := errors.New("forced Room citation query failure")
	fixture.service.tx = citationFailingTxStarter{base: fixture.pool, failure: sentinel}
	changed, syncErr := fixture.service.SyncTask(ctx, wake.Tasks[0].ID)
	if changed || !errors.Is(syncErr, sentinel) {
		t.Errorf("citation query failure sync = changed %t error %v, want propagated sentinel", changed, syncErr)
	}

	var turnStatus, cycleStatus, cyclePhase string
	var resultEntries, revisions, synthesisTurns int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT
			(SELECT status FROM room_turn WHERE id = $1),
			(SELECT status FROM room_cycle WHERE id = $2),
			(SELECT phase FROM room_cycle WHERE id = $2),
			(SELECT count(*) FROM room_entry WHERE cycle_id = $2 AND entry_type = 'result'),
			(SELECT count(*) FROM room_memory_revision WHERE cycle_id = $2),
			(SELECT count(*) FROM room_turn WHERE cycle_id = $2 AND turn_kind = 'synthesis')
	`, wake.Turns[0].ID, wake.Cycle.ID).Scan(
		&turnStatus, &cycleStatus, &cyclePhase, &resultEntries, &revisions, &synthesisTurns,
	); err != nil {
		t.Fatal(err)
	}
	if turnStatus != "queued" || cycleStatus != "queued" || cyclePhase != "gathering" ||
		resultEntries != 0 || revisions != 0 || synthesisTurns != 0 {
		t.Fatalf("citation failure did not roll back: turn=%q cycle=%q/%q entries=%d revisions=%d synthesis_turns=%d",
			turnStatus, cycleStatus, cyclePhase, resultEntries, revisions, synthesisTurns)
	}
}

func TestMentionReplayIgnoresLaterSynthesisTurn(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	enableRoomOutcomes(t, fixture)
	setRoomOutcomeRuntimeReady(t, fixture)
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Mention Replay Room", FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := MessageInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Body: "Investigate this directly.", MentionAgents: []pgtype.UUID{fixture.workerID},
		IdempotencyKey: "message:replay-after-synthesis",
	}
	first, err := fixture.service.PostMessage(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tasks) != 1 || first.Turns[0].AgentID != fixture.workerID {
		t.Fatalf("initial mentioned wake = %+v", first.WakeResult)
	}
	completeRoomTask(t, fixture, first.Tasks[0].ID, "Unstructured participant response.")
	if changed, err := fixture.service.SyncTask(ctx, first.Tasks[0].ID); err != nil || !changed {
		t.Fatalf("sync mentioned participant = %t, %v", changed, err)
	}
	before := roomPersistenceCounts(t, fixture, created.Room.ID)
	replay, err := fixture.service.PostMessage(ctx, input)
	if err != nil {
		t.Fatalf("message replay after synthesis turn: %v", err)
	}
	after := roomPersistenceCounts(t, fixture, created.Room.ID)
	if replay.Entry.ID != first.Entry.ID || replay.Cycle.ID != first.Cycle.ID || before != after {
		t.Fatalf("message replay changed identity/state: first=%+v replay=%+v before=%+v after=%+v", first, replay, before, after)
	}
	if countTurns(replay.Turns, "participant") != 1 || countTurns(replay.Turns, "synthesis") != 1 {
		t.Fatalf("replayed turns = %+v", replay.Turns)
	}
}

func TestInitialSynthesisHonorsCostBudgetWithoutLosingResults(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	enableRoomOutcomes(t, fixture)
	setRoomOutcomeRuntimeReady(t, fixture)
	setRoomCostBoundRuntimeReady(t, fixture)
	maxCost := int64(100)
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Initial Synthesis Budget Room", FacilitatorSquadID: fixture.squadID,
		MaxCostTicks: &maxCost,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:initial-synthesis-budget",
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, task := range wake.Tasks {
		if err := db.New(fixture.pool).UpsertTaskUsage(ctx, db.UpsertTaskUsageParams{
			TaskID: task.ID, Provider: "test", Model: "participant",
			InputTokens: 10, OutputTokens: 5, CostUsdTicks: pgtype.Int8{Int64: 60, Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
		completeRoomTask(t, fixture, task.ID, []string{"First result.", "Second result."}[index])
		if changed, err := fixture.service.SyncTask(ctx, task.ID); err != nil || !changed {
			t.Fatalf("sync participant %d = %t, %v", index, changed, err)
		}
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	var synthesisFailure synthesisError
	if err := json.Unmarshal(detail.Cycles[0].SynthesisError, &synthesisFailure); err != nil {
		t.Fatalf("decode synthesis budget error: %v; raw=%s", err, detail.Cycles[0].SynthesisError)
	}
	if detail.Cycles[0].Status != "running" || detail.Cycles[0].Phase != "awaiting_review" ||
		synthesisFailure.Code != "budget_exhausted" || !synthesisFailure.Retryable ||
		countTurns(detail.Turns, "participant") != 2 || countTurns(detail.Turns, "synthesis") != 0 ||
		len(detail.Entries) != 2 || len(detail.MemoryRevisions) != 0 || !detail.Room.ActiveCycleID.Valid {
		t.Fatalf("budget-blocked initial synthesis = cycle %+v error %+v entries=%d turns=%+v room=%+v",
			detail.Cycles[0], synthesisFailure, len(detail.Entries), detail.Turns, detail.Room)
	}
	var taskCount int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT count(*) FROM agent_task_queue task
		JOIN room_turn turn ON turn.id = task.room_turn_id
		WHERE turn.cycle_id = $1
	`, wake.Cycle.ID).Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 2 {
		t.Fatalf("tasks after budget-blocked initial synthesis = %d, want 2", taskCount)
	}
}

func TestRetrySynthesisHonorsBudgetThenSucceedsWithSameKey(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	if _, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "reject", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:reject-before-budgeted-retry",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET max_cost_ticks = 100 WHERE id = $1`, fixture.detail.Room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `
		INSERT INTO task_usage (task_id, provider, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_usd_ticks)
		SELECT task.id, 'test', 'budgeted-retry', 10, 5, 0, 0, 40
		FROM agent_task_queue task
		JOIN room_turn turn ON turn.id = task.room_turn_id
		WHERE turn.cycle_id = $1
	`, fixture.detail.Cycles[0].ID); err != nil {
		t.Fatal(err)
	}
	input := RetrySynthesisInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, IdempotencyKey: "synthesis:budgeted-retry",
	}
	before := roomPersistenceCounts(t, fixture.serviceFixture, fixture.detail.Room.ID)
	if _, err := fixture.service.RetrySynthesis(ctx, input); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("over-budget retry error = %v, want canonical budget error", err)
	}
	afterRefusal := roomPersistenceCounts(t, fixture.serviceFixture, fixture.detail.Room.ID)
	if before != afterRefusal {
		t.Fatalf("over-budget retry changed persistence: before=%+v after=%+v", before, afterRefusal)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "awaiting_review" || countTurns(detail.Turns, "synthesis") != 1 {
		t.Fatalf("over-budget retry state = cycle %+v turns %+v", detail.Cycles[0], detail.Turns)
	}

	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET max_cost_ticks = 1000 WHERE id = $1`, fixture.detail.Room.ID); err != nil {
		t.Fatal(err)
	}
	retry, err := fixture.service.RetrySynthesis(ctx, input)
	if err != nil {
		t.Fatalf("retry after budget increase: %v", err)
	}
	completeRoomTask(t, fixture.serviceFixture, retry.Task.ID, string(testSynthesis(t, fixture.participantIDs, "Valid retried recommendation.")))
	if changed, err := fixture.service.SyncTask(ctx, retry.Task.ID); err != nil || !changed {
		t.Fatalf("sync valid retry = %t, %v", changed, err)
	}
	detail, err = fixture.service.Get(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Phase != "awaiting_review" || len(detail.MemoryRevisions) != 2 ||
		detail.MemoryRevisions[0].ReviewStatus != "pending" || detail.MemoryRevisions[0].Version != 2 ||
		countTurns(detail.Turns, "participant") != 2 || countTurns(detail.Turns, "synthesis") != 2 {
		t.Fatalf("valid retry outcome = cycle %+v revisions %+v turns %+v", detail.Cycles[0], detail.MemoryRevisions, detail.Turns)
	}
}

func TestAllUnsuccessfulParticipantsFailWithoutStaleSynthesis(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	enableRoomOutcomes(t, fixture)
	setRoomOutcomeRuntimeReady(t, fixture)
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Failed Participant Room", FacilitatorSquadID: fixture.squadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.New(fixture.pool).AddRoomEntry(ctx, db.AddRoomEntryParams{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID,
		EntryType: "message", AuthorType: "member", AuthorID: fixture.userID,
		Body: "This stale transcript must not be synthesized.", Mentions: []byte("[]"),
	}); err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:all-participants-unsuccessful",
	})
	if err != nil {
		t.Fatal(err)
	}
	setRoomTaskTerminal(t, fixture, wake.Tasks[0].ID, "failed")
	if changed, err := fixture.service.SyncTask(ctx, wake.Tasks[0].ID); err != nil || !changed {
		t.Fatalf("sync failed participant = %t, %v", changed, err)
	}
	setRoomTaskTerminal(t, fixture, wake.Tasks[1].ID, "cancelled")
	if changed, err := fixture.service.SyncTask(ctx, wake.Tasks[1].ID); err != nil || !changed {
		t.Fatalf("sync cancelled participant = %t, %v", changed, err)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Cycles[0].Status != "failed" || detail.Cycles[0].Phase != "failed" ||
		countTurns(detail.Turns, "synthesis") != 0 || len(detail.MemoryRevisions) != 0 ||
		len(detail.Entries) != 1 || detail.Room.ActiveCycleID.Valid {
		t.Fatalf("all-unsuccessful outcome status=%q phase=%q synthesis_turns=%d entries=%d revisions=%d active=%t",
			detail.Cycles[0].Status, detail.Cycles[0].Phase, countTurns(detail.Turns, "synthesis"),
			len(detail.Entries), len(detail.MemoryRevisions), detail.Room.ActiveCycleID.Valid)
	}
}

func TestUsageIncludesEveryTaskAttempt(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	created, err := fixture.service.Create(ctx, CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Attempt Usage Room", FacilitatorAgentID: fixture.leaderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:attempt-usage",
	})
	if err != nil {
		t.Fatal(err)
	}
	var retryTaskID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, status, priority, context, attempt, max_attempts,
			force_fresh_session, squad_id, originator_user_id, accountable_user_id,
			originator_source, trigger_evidence_kind, trigger_evidence_ref_id, room_turn_id
		)
		SELECT agent_id, runtime_id, 'queued', priority, context, 2, max_attempts,
		       force_fresh_session, squad_id, originator_user_id, accountable_user_id,
		       originator_source, trigger_evidence_kind, trigger_evidence_ref_id, room_turn_id
		FROM agent_task_queue WHERE id = $1
		RETURNING id
	`, wake.Tasks[0].ID).Scan(&retryTaskID); err != nil {
		t.Fatal(err)
	}
	queries := db.New(fixture.pool)
	for _, usage := range []struct {
		task pgtype.UUID
		cost int64
	}{
		{task: wake.Tasks[0].ID, cost: 40},
		{task: retryTaskID, cost: 60},
	} {
		if err := queries.UpsertTaskUsage(ctx, db.UpsertTaskUsageParams{
			TaskID: usage.task, Provider: "test", Model: "attempt",
			InputTokens: 10, OutputTokens: 5, CostUsdTicks: pgtype.Int8{Int64: usage.cost, Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := fixture.service.Usage(ctx, fixture.workspaceID, created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if usage.TurnsTotal != 1 || usage.CostTicks != 100 || usage.UncostedTurns != 0 {
		t.Fatalf("multi-attempt usage = %+v, want one turn and cost across both attempts", usage)
	}
}

func TestOldPromotionKeyReplaysAfterNewRevisionAccepted(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	firstReview, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:first-accepted-revision",
	})
	if err != nil {
		t.Fatal(err)
	}
	oldInput := recommendationPromotion(fixture, "promotion:stable-old-key")
	firstPromotion, err := fixture.service.Promote(ctx, oldInput)
	if err != nil {
		t.Fatal(err)
	}

	secondWake, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:second-outcome",
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, task := range secondWake.Tasks {
		completeRoomTask(t, fixture.serviceFixture, task.ID, []string{"New first evidence.", "New second evidence."}[index])
		if changed, err := fixture.service.SyncTask(ctx, task.ID); err != nil || !changed {
			t.Fatalf("sync second-cycle participant %d = %t, %v", index, changed, err)
		}
	}
	participantIDs := resultEntryIDsForCycle(t, fixture.serviceFixture, secondWake.Cycle.ID)
	synthesisTask := latestTaskForCycleKind(t, fixture.serviceFixture, secondWake.Cycle.ID, "synthesis")
	completeRoomTask(t, fixture.serviceFixture, synthesisTask.ID, string(testSynthesis(t, participantIDs, "New accepted recommendation.")))
	if changed, err := fixture.service.SyncTask(ctx, synthesisTask.ID); err != nil || !changed {
		t.Fatalf("sync second synthesis = %t, %v", changed, err)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.MemoryRevisions) != 2 || detail.MemoryRevisions[0].Version != 2 {
		t.Fatalf("second pending revision = %+v", detail.MemoryRevisions)
	}
	if _, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: secondWake.Cycle.ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: firstReview.Room.MemoryVersion,
		IdempotencyKey: "review:second-accepted-revision",
	}); err != nil {
		t.Fatal(err)
	}
	replay, err := fixture.service.Promote(ctx, oldInput)
	if err != nil {
		t.Fatalf("old promotion replay after new accepted revision: %v", err)
	}
	if replay.Created || replay.Artifact.ID != firstPromotion.Artifact.ID {
		t.Fatalf("old promotion replay = %+v, want artifact %v", replay, firstPromotion.Artifact.ID)
	}
}

func TestCorrectedRevisionRequiresExplicitAcceptance(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	original := fixture.detail.MemoryRevisions[0]
	if original.CreatorType != "agent" || original.CreatorID != fixture.detail.Room.FacilitatorAgentID {
		t.Fatalf("synthesized revision creator = %q/%v", original.CreatorType, original.CreatorID)
	}
	corrected, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "correct", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		Correction:     testSynthesis(t, fixture.participantIDs, "Corrected and still pending."),
		IdempotencyKey: "review:correct-before-explicit-accept",
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := fixture.service.Get(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if corrected.MemoryRevision.ReviewStatus != "pending" || detail.Room.AcceptedMemoryRevisionID.Valid ||
		detail.Room.MemoryVersion != fixture.detail.Room.MemoryVersion || detail.Cycles[0].Phase != "awaiting_review" {
		t.Fatalf("corrected revision before accept = result %+v detail %+v", corrected, detail)
	}
	if corrected.MemoryRevision.CreatorType != "member" || corrected.MemoryRevision.CreatorID != fixture.userID {
		t.Fatalf("corrected revision creator = %q/%v", corrected.MemoryRevision.CreatorType, corrected.MemoryRevision.CreatorID)
	}
	accepted, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "review:accept-corrected-revision",
	})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.MemoryRevision.ID != corrected.MemoryRevision.ID || accepted.Room.AcceptedMemoryRevisionID != corrected.MemoryRevision.ID ||
		accepted.Room.MemoryVersion != fixture.detail.Room.MemoryVersion+1 {
		t.Fatalf("explicit corrected acceptance = %+v", accepted)
	}
	final, err := fixture.service.Get(ctx, fixture.workspaceID, fixture.detail.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Cycles[0].Status != "completed" || final.Cycles[0].Phase != "completed" || final.Room.ActiveCycleID.Valid {
		t.Fatalf("corrected acceptance final state = room %+v cycle %+v", final.Room, final.Cycles[0])
	}
}

type persistenceCounts struct {
	entries   int
	cycles    int
	turns     int
	tasks     int
	revisions int
	artifacts int
}

func roomPersistenceCounts(t *testing.T, fixture serviceFixture, roomID pgtype.UUID) persistenceCounts {
	t.Helper()
	var counts persistenceCounts
	if err := fixture.pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM room_entry WHERE room_id = $1),
			(SELECT count(*) FROM room_cycle WHERE room_id = $1),
			(SELECT count(*) FROM room_turn WHERE room_id = $1),
			(SELECT count(*) FROM agent_task_queue task JOIN room_turn turn ON turn.id = task.room_turn_id WHERE turn.room_id = $1),
			(SELECT count(*) FROM room_memory_revision WHERE room_id = $1),
			(SELECT count(*) FROM room_artifact WHERE room_id = $1)
	`, roomID).Scan(&counts.entries, &counts.cycles, &counts.turns, &counts.tasks, &counts.revisions, &counts.artifacts); err != nil {
		t.Fatal(err)
	}
	return counts
}

func setRoomTaskTerminal(t *testing.T, fixture serviceFixture, taskID pgtype.UUID, status string) {
	t.Helper()
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = $2, started_at = now(), completed_at = now(),
		    error = CASE WHEN $2 = 'failed' THEN 'forced participant failure' ELSE error END
		WHERE id = $1
	`, taskID, status); err != nil {
		t.Fatal(err)
	}
}

func resultEntryIDsForCycle(t *testing.T, fixture serviceFixture, cycleID pgtype.UUID) []string {
	t.Helper()
	rows, err := fixture.pool.Query(context.Background(), `
		SELECT entry.id
		FROM room_entry entry
		JOIN room_turn turn ON turn.id = entry.turn_id
		WHERE entry.cycle_id = $1 AND entry.entry_type = 'result' AND turn.turn_kind = 'participant'
		ORDER BY entry.ordinal, entry.id
	`, cycleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ids := make([]string, 0, 2)
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, roomUUIDString(id))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func latestTaskForCycleKind(t *testing.T, fixture serviceFixture, cycleID pgtype.UUID, kind string) db.AgentTaskQueue {
	t.Helper()
	var taskID pgtype.UUID
	if err := fixture.pool.QueryRow(context.Background(), `
		SELECT task.id
		FROM agent_task_queue task
		JOIN room_turn turn ON turn.id = task.room_turn_id
		WHERE turn.cycle_id = $1 AND turn.turn_kind = $2
		ORDER BY turn.attempt DESC, task.attempt DESC, task.created_at DESC, task.id DESC
		LIMIT 1
	`, cycleID, kind).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	task, err := db.New(fixture.pool).GetAgentTask(context.Background(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func enableRoomOutcomes(t *testing.T, fixture serviceFixture) {
	t.Helper()
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE workspace SET settings = '{"room_outcomes_v2":true}'::jsonb WHERE id = $1`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
}

func setRoomOutcomeRuntimeReady(t *testing.T, fixture serviceFixture) {
	t.Helper()
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_runtime
		SET metadata = '{"capabilities":["room-tasks-v1","room-outcomes-v2"]}'::jsonb
		WHERE workspace_id = $1
	`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
}

func setRoomCostBoundRuntimeReady(t *testing.T, fixture serviceFixture) {
	t.Helper()
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE agent_runtime
		SET metadata = '{"capabilities":["room-tasks-v1","room-outcomes-v2","room-cost-limits-v1"]}'::jsonb
		WHERE workspace_id = $1
	`, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
}

func createSingleOutcomeRoom(t *testing.T, fixture serviceFixture, title string, maxCostTicks int64) Detail {
	t.Helper()
	enableRoomOutcomes(t, fixture)
	setRoomOutcomeRuntimeReady(t, fixture)
	input := CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: title, Objective: "Produce a cited outcome.", TemplateID: "decision",
		FacilitatorAgentID: fixture.leaderID,
	}
	if maxCostTicks > 0 {
		setRoomCostBoundRuntimeReady(t, fixture)
		input.MaxCostTicks = &maxCostTicks
	}
	created, err := fixture.service.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Room.CapabilityVersion != 2 {
		t.Fatalf("Room capability version = %d, want 2", created.Room.CapabilityVersion)
	}
	return created
}

func clearActiveCycle(t *testing.T, fixture serviceFixture, roomID, cycleID pgtype.UUID) {
	t.Helper()
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE room SET active_cycle_id = NULL WHERE id = $1`, roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE room_cycle SET status = 'cancelled', phase = 'cancelled', completed_at = now() WHERE id = $1
	`, cycleID); err != nil {
		t.Fatal(err)
	}
}

func roomUUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return id.String()
}

func roomTestPreflightInput(
	fixture serviceFixture,
	roomID pgtype.UUID,
	source string,
) PreflightInput {
	return PreflightInput{
		WorkspaceID: fixture.workspaceID,
		RoomID:      roomID,
		ActorUserID: fixture.userID,
		Source:      source,
	}
}
