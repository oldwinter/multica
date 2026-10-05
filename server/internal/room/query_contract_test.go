package room

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func insertRoomQueryFixture(t *testing.T, fixture serviceFixture, label string, capabilityVersion int32) pgtype.UUID {
	t.Helper()
	var roomID pgtype.UUID
	if err := fixture.pool.QueryRow(context.Background(), `
		INSERT INTO room (
			workspace_id, title, created_by_user_id, facilitator_agent_id, capability_version
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, fixture.workspaceID, label, fixture.userID, fixture.leaderID, capabilityVersion).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	return roomID
}

func TestGetRoomUsageSummaryCountsEveryTaskAttempt(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	queries := db.New(fixture.pool)
	roomID := insertRoomQueryFixture(t, fixture, "Attempt usage room", 2)

	var cycleID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO room_cycle (
			workspace_id, room_id, sequence, source, wake_key, status, phase
		) VALUES ($1, $2, 1, 'manual', 'manual:attempt-usage', 'failed', 'failed')
		RETURNING id
	`, fixture.workspaceID, roomID).Scan(&cycleID); err != nil {
		t.Fatal(err)
	}
	var turnID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO room_turn (
			workspace_id, room_id, cycle_id, agent_id, status, turn_kind, attempt
		) VALUES ($1, $2, $3, $4, 'failed', 'participant', 1)
		RETURNING id
	`, fixture.workspaceID, roomID, cycleID, fixture.leaderID).Scan(&turnID); err != nil {
		t.Fatal(err)
	}
	var runtimeID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `SELECT runtime_id FROM agent WHERE id = $1`, fixture.leaderID).Scan(&runtimeID); err != nil {
		t.Fatal(err)
	}

	taskIDs := make([]pgtype.UUID, 3)
	for index, status := range []string{"completed", "failed", "cancelled"} {
		if err := fixture.pool.QueryRow(ctx, `
			INSERT INTO agent_task_queue (
				agent_id, runtime_id, status, context, room_turn_id, attempt, completed_at
			) VALUES ($1, $2, $3, '{}'::jsonb, $4, $5, now())
			RETURNING id
		`, fixture.leaderID, runtimeID, status, turnID, index+1).Scan(&taskIDs[index]); err != nil {
			t.Fatal(err)
		}
	}
	for index, cost := range []int64{11, 17} {
		if _, err := fixture.pool.Exec(ctx, `
			INSERT INTO task_usage (task_id, provider, model, cost_usd_ticks)
			VALUES ($1, 'room-attempt-test', $2, $3)
		`, taskIDs[index], "attempt-model-"+string(rune('1'+index)), cost); err != nil {
			t.Fatal(err)
		}
	}

	usage, err := queries.GetRoomUsageSummary(ctx, db.GetRoomUsageSummaryParams{
		WorkspaceID: fixture.workspaceID,
		RoomID:      roomID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if usage.TurnsTotal != 1 || usage.Failures != 1 {
		t.Fatalf("turn counts = total %d failures %d, want 1/1", usage.TurnsTotal, usage.Failures)
	}
	if usage.CostTicks != 28 {
		t.Fatalf("cost ticks = %d, want 28 from both costed attempts", usage.CostTicks)
	}
	if usage.UncostedTurns != 1 {
		t.Fatalf("uncosted turns = %d, want 1 from the third attempt without usage", usage.UncostedTurns)
	}
}

func TestCompleteRoomCycleSynchronizesLegacyTerminalPhase(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	queries := db.New(fixture.pool)
	roomID := insertRoomQueryFixture(t, fixture, "Legacy terminal phase room", 1)

	for sequence, status := range []string{"completed", "failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			var cycleID pgtype.UUID
			if err := fixture.pool.QueryRow(ctx, `
				INSERT INTO room_cycle (
					workspace_id, room_id, sequence, source, wake_key, status, phase
				) VALUES ($1, $2, $3, 'manual', $4, 'running', 'gathering')
				RETURNING id
			`, fixture.workspaceID, roomID, sequence+1, "manual:terminal:"+status).Scan(&cycleID); err != nil {
				t.Fatal(err)
			}
			cycle, err := queries.CompleteRoomCycle(ctx, db.CompleteRoomCycleParams{
				Status:      status,
				StartedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				ID:          cycleID,
				WorkspaceID: fixture.workspaceID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if cycle.Status != status || cycle.Phase != status {
				t.Fatalf("cycle terminal state = %s/%s, want %s/%s", cycle.Status, cycle.Phase, status, status)
			}
		})
	}
}

func TestFailRoomOutcomeCycleReturnsFailedCycleForTransactionalCleanup(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	queries := db.New(fixture.pool)
	roomID := insertRoomQueryFixture(t, fixture, "Zero-success outcome room", 2)
	var cycleID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO room_cycle (
			workspace_id, room_id, sequence, source, wake_key, status, phase
		) VALUES ($1, $2, 1, 'manual', 'manual:zero-success', 'running', 'gathering')
		RETURNING id
	`, fixture.workspaceID, roomID).Scan(&cycleID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET active_cycle_id = $1 WHERE id = $2`, cycleID, roomID); err != nil {
		t.Fatal(err)
	}

	cycle, err := queries.FailRoomOutcomeCycle(ctx, db.FailRoomOutcomeCycleParams{
		SynthesisError: []byte(`{"code":"no_successful_participant_turns","message":"No participant completed","retryable":false}`),
		ID:             cycleID,
		WorkspaceID:    fixture.workspaceID,
		RoomID:         roomID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cycle.Status != "failed" || cycle.Phase != "failed" || !cycle.CompletedAt.Valid {
		t.Fatalf("failed outcome cycle = status %q phase %q completed %v", cycle.Status, cycle.Phase, cycle.CompletedAt.Valid)
	}
	if _, err := queries.ClearRoomActiveCycle(ctx, db.ClearRoomActiveCycleParams{
		ID:               roomID,
		WorkspaceID:      fixture.workspaceID,
		CompletedCycleID: cycleID,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRoomDetailQueriesAreRecentAndBounded(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	queries := db.New(fixture.pool)
	roomID := insertRoomQueryFixture(t, fixture, "Bounded detail room", 2)

	if _, err := fixture.pool.Exec(ctx, `
		INSERT INTO room_memory_revision (
			workspace_id, room_id, cycle_id, synthesis_turn_id, version,
			schema_version, synthesis, digest, review_status,
			reviewed_by_user_id, reviewed_at, creator_type, creator_id, created_at
		)
		SELECT $1, $2, gen_random_uuid(), gen_random_uuid(), version,
		       1, '{}'::jsonb, 'sha256:' || repeat('0', 64), 'rejected',
		       $3, now() + make_interval(secs => version::double precision), 'member', $3,
		       now() + make_interval(secs => version::double precision)
		FROM generate_series(1, 105) AS version
	`, fixture.workspaceID, roomID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `
		INSERT INTO room_recommendation_review (
			workspace_id, room_id, memory_revision_id, recommendation_key,
			status, idempotency_key, request_digest, reviewed_by_user_id, reviewed_at
		)
		SELECT $1, $2, id, 'sha256:' || repeat('1', 64),
		       'rejected', 'bounded-review-' || version::text,
		       'sha256:' || repeat('2', 64), $3, created_at
		FROM room_memory_revision
		WHERE workspace_id = $1 AND room_id = $2
	`, fixture.workspaceID, roomID, fixture.userID); err != nil {
		t.Fatal(err)
	}
	var acceptedRevisionID, activeRevisionID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		SELECT id
		FROM room_memory_revision
		WHERE workspace_id = $1 AND room_id = $2 AND version = 1
	`, fixture.workspaceID, roomID).Scan(&acceptedRevisionID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(ctx, `
		SELECT id
		FROM room_memory_revision
		WHERE workspace_id = $1 AND room_id = $2 AND version = 2
	`, fixture.workspaceID, roomID).Scan(&activeRevisionID); err != nil {
		t.Fatal(err)
	}
	var activeCycleID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO room_cycle (
			workspace_id, room_id, sequence, source, wake_key, status, phase, memory_revision_id
		) VALUES ($1, $2, 1, 'manual', 'manual:bounded-detail', 'running', 'awaiting_review', $3)
		RETURNING id
	`, fixture.workspaceID, roomID, activeRevisionID).Scan(&activeCycleID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `
		UPDATE room
		SET accepted_memory_revision_id = $1, active_cycle_id = $2
		WHERE id = $3 AND workspace_id = $4
	`, acceptedRevisionID, activeCycleID, roomID, fixture.workspaceID); err != nil {
		t.Fatal(err)
	}

	revisions, err := queries.ListRoomMemoryRevisions(ctx, db.ListRoomMemoryRevisionsParams{
		WorkspaceID: fixture.workspaceID,
		RoomID:      roomID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 102 || revisions[0].Version != 105 || revisions[101].Version != 1 {
		t.Fatalf("revision window = len %d first %d last %d, want 102/105/1",
			len(revisions), revisions[0].Version, revisions[len(revisions)-1].Version)
	}
	if revisions[100].ID != activeRevisionID || revisions[101].ID != acceptedRevisionID {
		t.Fatalf("detail window omitted protected revisions: tail = %v/%v, want active %v accepted %v",
			revisions[100].ID, revisions[101].ID, activeRevisionID, acceptedRevisionID)
	}

	reviews, err := queries.ListRoomRecommendationReviews(ctx, db.ListRoomRecommendationReviewsParams{
		WorkspaceID: fixture.workspaceID,
		RoomID:      roomID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 102 {
		t.Fatalf("recommendation review window = %d, want 102", len(reviews))
	}
	recentRevisionIDs := make(map[pgtype.UUID]struct{}, len(revisions))
	for _, revision := range revisions {
		recentRevisionIDs[revision.ID] = struct{}{}
	}
	for _, review := range reviews {
		if _, ok := recentRevisionIDs[review.MemoryRevisionID]; !ok {
			t.Fatalf("review for old revision %v escaped the recent revision window", review.MemoryRevisionID)
		}
	}
}
