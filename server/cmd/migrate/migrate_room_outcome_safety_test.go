package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRoomPrimaryKeyMigrationsReplayAfterDDLSuccessBeforeLedger(t *testing.T) {
	pool, schema := newRoomMigrationTestSchema(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, fixture := range []struct {
		version   string
		tableName string
		indexName string
		pkName    string
	}{
		{
			version:   "379_room_memory_revision_primary_key",
			tableName: "room_memory_revision",
			indexName: "room_memory_revision_id_uidx",
			pkName:    "room_memory_revision_pkey",
		},
		{
			version:   "385_room_recommendation_review_primary_key",
			tableName: "room_recommendation_review",
			indexName: "room_recommendation_review_id_uidx",
			pkName:    "room_recommendation_review_pkey",
		},
	} {
		t.Run(fixture.version, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "CREATE TABLE "+pgx.Identifier{fixture.tableName}.Sanitize()+" (id UUID NOT NULL)"); err != nil {
				t.Fatalf("create %s: %v", fixture.tableName, err)
			}
			if _, err := pool.Exec(ctx,
				"CREATE UNIQUE INDEX CONCURRENTLY "+pgx.Identifier{fixture.indexName}.Sanitize()+
					" ON "+pgx.Identifier{fixture.tableName}.Sanitize()+" (id)",
			); err != nil {
				t.Fatalf("create %s: %v", fixture.indexName, err)
			}

			path := filepath.Join("..", "..", "migrations", fixture.version+".up.sql")
			runRoomMigrationFile(t, ctx, pool, schema, "up", path, nil)

			// Simulate the crash window after ALTER TABLE committed but before
			// schema_migrations was updated. The retry must validate and accept
			// the exact existing primary key instead of executing ALTER again.
			if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", fixture.version); err != nil {
				t.Fatalf("remove %s ledger row: %v", fixture.version, err)
			}
			runRoomMigrationFile(t, ctx, pool, schema, "up", path, nil)

			var exact bool
			if err := pool.QueryRow(ctx, `
				SELECT c.contype = 'p'
				       AND c.conkey = ARRAY[a.attnum]::SMALLINT[]
				       AND c.convalidated
				       AND NOT c.condeferrable
				       AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive
				FROM pg_constraint c
				JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attname = 'id' AND NOT a.attisdropped
				JOIN pg_index i ON i.indexrelid = c.conindid
				WHERE c.conrelid = $1::regclass AND c.conname = $2
			`, fixture.tableName, fixture.pkName).Scan(&exact); err != nil {
				t.Fatalf("inspect replayed primary key: %v", err)
			}
			if !exact {
				t.Fatalf("%s is not the expected primary key", fixture.pkName)
			}
		})
	}
}

func TestRoomPrimaryKeyMigrationRejectsSameNameWithWrongShape(t *testing.T) {
	pool, schema := newRoomMigrationTestSchema(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, `
		CREATE TABLE room_memory_revision (
			id UUID NOT NULL,
			legacy_id UUID NOT NULL,
			CONSTRAINT room_memory_revision_pkey PRIMARY KEY (legacy_id)
		)
	`); err != nil {
		t.Fatalf("create drifted Room revision table: %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE UNIQUE INDEX CONCURRENTLY room_memory_revision_id_uidx ON room_memory_revision (id)`); err != nil {
		t.Fatalf("create candidate Room revision index: %v", err)
	}

	path := filepath.Join("..", "..", "migrations", "379_room_memory_revision_primary_key.up.sql")
	err := runMigrations(ctx, pool, runOptions{
		Direction:             "up",
		Files:                 []string{path},
		SchemaMigrationsTable: schema + ".schema_migrations",
		AdvisoryLockKey:       roomMigrationTestLockKey(),
	})
	if err == nil || !strings.Contains(err.Error(), "exists with an incompatible definition") {
		t.Fatalf("wrong-shape constraint error = %v, want incompatible-definition diagnostic", err)
	}
}

func TestRoomTurnIdentityMigrationReplayAndRollbackPreflight(t *testing.T) {
	pool, schema := newRoomMigrationTestSchema(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, `
		CREATE TABLE room_turn (
			id UUID NOT NULL,
			cycle_id UUID NOT NULL,
			agent_id UUID NOT NULL
		)
	`); err != nil {
		t.Fatalf("create room_turn: %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE UNIQUE INDEX CONCURRENTLY room_turn_participant_uidx ON room_turn (cycle_id, agent_id)`); err != nil {
		t.Fatalf("create v1 Room turn identity index: %v", err)
	}

	const version = "418_room_turn_identity_index_drop"
	upPath := filepath.Join("..", "..", "migrations", version+".up.sql")
	downPath := filepath.Join("..", "..", "migrations", version+".down.sql")
	runRoomMigrationFile(t, ctx, pool, schema, "up", upPath, hooksForDirection("up"))

	// Recreate the DDL-success/ledger-missing crash window for the up path.
	if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
		t.Fatalf("remove up ledger row: %v", err)
	}
	runRoomMigrationFile(t, ctx, pool, schema, "up", upPath, hooksForDirection("up"))

	const (
		cycleID = "00000000-0000-0000-0000-000000000101"
		agentID = "00000000-0000-0000-0000-000000000201"
	)
	if _, err := pool.Exec(ctx, `
		INSERT INTO room_turn (id, cycle_id, agent_id) VALUES
		('00000000-0000-0000-0000-000000000301', $1, $2),
		('00000000-0000-0000-0000-000000000302', $1, $2)
	`, cycleID, agentID); err != nil {
		t.Fatalf("seed v2 Room turn attempts: %v", err)
	}

	err := runMigrations(ctx, pool, runOptions{
		Direction:             "down",
		Files:                 []string{downPath},
		SchemaMigrationsTable: schema + ".schema_migrations",
		AdvisoryLockKey:       roomMigrationTestLockKey(),
		Hooks:                 hooksForDirection("down"),
	})
	if err == nil {
		t.Fatal("rollback unexpectedly accepted duplicate v2 Room turns")
	}
	for _, want := range []string{
		"cannot roll back 418_room_turn_identity_index_drop",
		"duplicate (cycle_id, agent_id) group(s)",
		cycleID,
		agentID,
		"returns no rows and rerun migrate down",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("rollback diagnostic %q does not contain %q", err, want)
		}
	}
	assertRoomMigrationRecorded(t, ctx, pool, version, true)

	if _, err := pool.Exec(ctx, `DELETE FROM room_turn WHERE id = '00000000-0000-0000-0000-000000000302'`); err != nil {
		t.Fatalf("reconcile duplicate Room attempt: %v", err)
	}
	runRoomMigrationFile(t, ctx, pool, schema, "down", downPath, hooksForDirection("down"))
	assertRoomMigrationRecorded(t, ctx, pool, version, false)
	assertRoomTurnIdentityIndex(t, ctx, pool)

	// Recreate the down-direction crash window. The restored valid index is
	// preserved, IF NOT EXISTS is a no-op, and the ledger is removed on retry.
	if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		t.Fatalf("restore down ledger row: %v", err)
	}
	runRoomMigrationFile(t, ctx, pool, schema, "down", downPath, hooksForDirection("down"))
	assertRoomMigrationRecorded(t, ctx, pool, version, false)
	assertRoomTurnIdentityIndex(t, ctx, pool)
}

func newRoomMigrationTestSchema(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	adminPool := openTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schema := fmt.Sprintf("migrate_room_outcome_%d_%d", time.Now().UnixNano(), rand.Uint32())
	schemaIdent := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schemaIdent); err != nil {
		t.Fatalf("create Room migration schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, err := adminPool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+schemaIdent+" CASCADE"); err != nil {
			t.Logf("drop schema %s: %v", schema, err)
		}
	})

	return openTestPoolWithSearchPath(t, schema), schema
}

func runRoomMigrationFile(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	schema string,
	direction string,
	path string,
	hooks map[string]preMigrationHook,
) {
	t.Helper()
	if err := runMigrations(ctx, pool, runOptions{
		Direction:             direction,
		Files:                 []string{path},
		SchemaMigrationsTable: schema + ".schema_migrations",
		AdvisoryLockKey:       roomMigrationTestLockKey(),
		Hooks:                 hooks,
	}); err != nil {
		t.Fatalf("run Room migration %s %s: %v", direction, filepath.Base(path), err)
	}
}

func roomMigrationTestLockKey() int64 {
	return int64(rand.Uint64()&0x7fffffffffffffff) | 1
}

func assertRoomMigrationRecorded(t *testing.T, ctx context.Context, pool *pgxpool.Pool, version string, want bool) {
	t.Helper()
	var got bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&got); err != nil {
		t.Fatalf("inspect Room migration ledger: %v", err)
	}
	if got != want {
		t.Fatalf("migration %s recorded = %t, want %t", version, got, want)
	}
}

func assertRoomTurnIdentityIndex(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var valid, unique bool
	var definition string
	if err := pool.QueryRow(ctx, `
		SELECT i.indisvalid, i.indisunique, pg_get_indexdef(i.indexrelid)
		FROM pg_index i
		WHERE i.indexrelid = to_regclass('room_turn_participant_uidx')
	`).Scan(&valid, &unique, &definition); err != nil {
		t.Fatalf("inspect restored Room turn identity index: %v", err)
	}
	if !valid || !unique || !strings.Contains(definition, "(cycle_id, agent_id)") {
		t.Fatalf("restored Room turn identity index valid=%t unique=%t definition=%q", valid, unique, definition)
	}
}
