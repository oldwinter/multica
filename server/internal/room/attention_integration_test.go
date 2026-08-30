package room

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRoomAttentionLifecycleDedupeRecipientsPrivacyAndCleanup(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	roomRow := fixture.detail.Room
	cycle := fixture.detail.Cycles[0]
	revision := fixture.detail.MemoryRevisions[0]

	var participantID pgtype.UUID
	if err := fixture.pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Room Reviewer', 'room-reviewer-' || gen_random_uuid()::text || '@example.com')
		RETURNING id
	`).Scan(&participantID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fixture.pool.Exec(context.Background(), `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, fixture.workspaceID, participantID)
		fixture.pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, participantID)
	})
	if _, err := fixture.pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')
	`, fixture.workspaceID, participantID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `
		INSERT INTO room_participant (workspace_id, room_id, participant_type, participant_id, role)
		VALUES ($1, $3, 'member', $2, 'participant')
	`, fixture.workspaceID, participantID, roomRow.ID); err != nil {
		t.Fatal(err)
	}

	tx, err := fixture.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	queries := db.New(tx)
	input := roomAttentionInput{
		RoomID: roomRow.ID, CycleID: cycle.ID, MemoryRevisionID: revision.ID,
		Phase: cycle.Phase,
	}
	for range 2 {
		if _, err := fixture.service.upsertRoomAttention(
			ctx, queries, roomRow, RoomInboxOutcomeReviewRequired, input,
		); err != nil {
			tx.Rollback(ctx)
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	assertRoomAttentionRows(t, fixture, roomRow.ID, RoomInboxOutcomeReviewRequired, false, 2)
	var distinctRecipients int
	if err := fixture.pool.QueryRow(ctx, `
		SELECT count(DISTINCT recipient_id)
		FROM inbox_item
		WHERE workspace_id = $1 AND room_id = $2 AND type = $3 AND archived = false
	`, fixture.workspaceID, roomRow.ID, RoomInboxOutcomeReviewRequired).Scan(&distinctRecipients); err != nil {
		t.Fatal(err)
	}
	if distinctRecipients != 2 {
		t.Fatalf("outcome attention recipients = %d, want creator and explicit human participant", distinctRecipients)
	}

	accepted, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
		ActorUserID: fixture.userID, Action: "accept",
		ExpectedMemoryVersion: roomRow.MemoryVersion, IdempotencyKey: "attention:accept",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRoomAttentionRows(t, fixture, roomRow.ID, RoomInboxOutcomeReviewRequired, true, 2)
	assertRoomAttentionRows(t, fixture, roomRow.ID, RoomInboxRecommendationReviewRequired, false, 2)

	if _, err := fixture.service.ReviewRecommendation(ctx, RecommendationReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: roomRow.ID,
		MemoryRevisionID:  accepted.MemoryRevision.ID,
		RecommendationKey: fixture.recommendation.Key,
		ActorUserID:       fixture.userID, Action: "reject", IdempotencyKey: "attention:reject-recommendation",
	}); err != nil {
		t.Fatal(err)
	}
	assertRoomAttentionRows(t, fixture, roomRow.ID, RoomInboxRecommendationReviewRequired, true, 2)
}

func assertRoomAttentionRows(
	t *testing.T,
	fixture pendingOutcomeFixture,
	roomID pgtype.UUID,
	kind string,
	archived bool,
	want int,
) {
	t.Helper()
	var count int
	var unsafeRows int
	err := fixture.pool.QueryRow(context.Background(), `
		SELECT count(*)::int,
		       count(*) FILTER (WHERE body IS NOT NULL
		         OR details ?| ARRAY['objective', 'transcript', 'summary', 'artifact', 'content'])::int
		FROM inbox_item
		WHERE workspace_id = $1 AND room_id = $2 AND type = $3 AND archived = $4
	`, fixture.workspaceID, roomID, kind, archived).Scan(&count, &unsafeRows)
	if err != nil && err != pgx.ErrNoRows {
		t.Fatal(err)
	}
	if count != want || unsafeRows != 0 {
		t.Fatalf("Room attention %s archived=%t rows=%d unsafe=%d, want rows=%d unsafe=0", kind, archived, count, unsafeRows, want)
	}
}
