package migrations

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func roomPersistencePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func createPersistenceRoom(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ('Room persistence', 'room-persistence-' || gen_random_uuid()::text, '', 'RMP')
		RETURNING id
	`).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	var roomID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO room (
			workspace_id, title, created_by_user_id, facilitator_agent_id, daily_turn_limit
		) VALUES ($1, 'Persistence Room', gen_random_uuid(), gen_random_uuid(), 20)
		RETURNING id
	`, workspaceID).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM room_artifact WHERE workspace_id = $1`, workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_turn WHERE workspace_id = $1`, workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_cycle WHERE workspace_id = $1`, workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_entry WHERE workspace_id = $1`, workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room_participant WHERE workspace_id = $1`, workspaceID)
		pool.Exec(context.Background(), `DELETE FROM room WHERE workspace_id = $1`, workspaceID)
		pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})
	return workspaceID, roomID
}

func TestRoomActiveCycleConcurrency(t *testing.T) {
	pool := roomPersistencePool(t)
	workspaceID, roomID := createPersistenceRoom(t, pool)
	ctx := context.Background()

	const attempts = 16
	var accepted atomic.Int32
	var wg sync.WaitGroup
	wg.Add(attempts)
	for index := 0; index < attempts; index++ {
		go func() {
			defer wg.Done()
			var id string
			err := pool.QueryRow(ctx, `
				WITH allocated AS (
					UPDATE room
					SET last_cycle_sequence = last_cycle_sequence + 1
					WHERE id = $1 AND workspace_id = $2
					RETURNING last_cycle_sequence
				)
				INSERT INTO room_cycle (
					workspace_id, room_id, sequence, source, wake_key, status
				)
				SELECT $2, $1, last_cycle_sequence, 'manual', gen_random_uuid()::text, 'queued'
				FROM allocated
				RETURNING id
			`, roomID, workspaceID).Scan(&id)
			if err == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()

	if accepted.Load() != 1 {
		t.Fatalf("accepted active cycles = %d, want 1", accepted.Load())
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM room_cycle WHERE room_id = $1`, roomID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("persisted cycles = %d, want 1", count)
	}
}

func TestRoomWakeAndArtifactIdentity(t *testing.T) {
	pool := roomPersistencePool(t)
	workspaceID, roomID := createPersistenceRoom(t, pool)
	ctx := context.Background()

	var cycleID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO room_cycle (workspace_id, room_id, sequence, source, wake_key, status)
		VALUES ($1, $2, 1, 'manual', 'manual:request-1', 'completed')
		RETURNING id
	`, workspaceID, roomID).Scan(&cycleID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO room_cycle (workspace_id, room_id, sequence, source, wake_key, status)
		VALUES ($1, $2, 2, 'manual', 'manual:request-1', 'completed')
	`, workspaceID, roomID)
	if err == nil {
		t.Fatal("duplicate Room wake key was accepted")
	}

	for attempt := 0; attempt < 2; attempt++ {
		_, insertErr := pool.Exec(ctx, `
			INSERT INTO room_artifact (
				workspace_id, room_id, cycle_id, kind, idempotency_key,
				title, body, source_digest, created_by_user_id
			) VALUES (
				$1, $2, $3, 'decision', 'decision:request-1',
				'Decide', 'Ship it', 'sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', gen_random_uuid()
			)
			ON CONFLICT (room_id, kind, idempotency_key) DO NOTHING
		`, workspaceID, roomID, cycleID)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM room_artifact WHERE room_id = $1`, roomID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("artifacts = %d, want 1", count)
	}
}

func TestRoomEntryOrdinalAndWorkspaceScope(t *testing.T) {
	pool := roomPersistencePool(t)
	workspaceID, roomID := createPersistenceRoom(t, pool)
	ctx := context.Background()

	var otherWorkspaceID string
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&otherWorkspaceID); err != nil {
		t.Fatal(err)
	}
	var title string
	err := pool.QueryRow(ctx, `SELECT title FROM room WHERE id = $1 AND workspace_id = $2`, roomID, otherWorkspaceID).Scan(&title)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace Room read error = %v, want no rows", err)
	}

	const entryCount = 12
	var wg sync.WaitGroup
	wg.Add(entryCount)
	for index := 0; index < entryCount; index++ {
		go func() {
			defer wg.Done()
			_, insertErr := pool.Exec(ctx, `
				WITH allocated AS (
					UPDATE room
					SET last_entry_ordinal = last_entry_ordinal + 1
					WHERE id = $1 AND workspace_id = $2
					RETURNING last_entry_ordinal
				)
				INSERT INTO room_entry (
					workspace_id, room_id, ordinal, author_type, author_id, body
				)
				SELECT $2, $1, last_entry_ordinal, 'system', NULL, 'entry'
				FROM allocated
			`, roomID, workspaceID)
			if insertErr != nil {
				t.Errorf("insert concurrent entry: %v", insertErr)
			}
		}()
	}
	wg.Wait()

	var count int
	var distinct int
	var minimum int64
	var maximum int64
	if err := pool.QueryRow(ctx, `
		SELECT count(*), count(DISTINCT ordinal), min(ordinal), max(ordinal)
		FROM room_entry WHERE room_id = $1
	`, roomID).Scan(&count, &distinct, &minimum, &maximum); err != nil {
		t.Fatal(err)
	}
	if count != entryCount || distinct != entryCount || minimum != 1 || maximum != entryCount {
		t.Fatalf("entry ordinal stats = count %d distinct %d min %d max %d", count, distinct, minimum, maximum)
	}
}
