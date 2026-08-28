package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestSkillEvolutionMigrationsUpgradeExisting514LedgerAndReplay(t *testing.T) {
	pool, schema := newRoomMigrationTestSchema(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, `
CREATE TABLE skill_evolution_task_attribution (
    id UUID NOT NULL, workspace_id UUID NOT NULL
);
CREATE TABLE skill_evolution_proposal (
    id UUID NOT NULL, rationale_digest TEXT NULL
);
CREATE TABLE wiki_page_edit_proposal (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL
)`); err != nil {
		t.Fatalf("create migration prerequisites: %v", err)
	}

	opts := runOptions{
		Direction:             "up",
		SchemaMigrationsTable: schema + ".schema_migrations",
		AdvisoryLockKey:       roomMigrationTestLockKey(),
		Hooks:                 hooksForDirection("up"),
	}
	if err := runMigrations(ctx, pool, opts); err != nil {
		t.Fatalf("initialize migration ledger: %v", err)
	}
	const existingVersion = "514_room_recommendation_target_taxonomy_validate"
	if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, existingVersion); err != nil {
		t.Fatalf("seed migration 514 ledger: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO wiki_page_edit_proposal (workspace_id, agent_id, idempotency_key) VALUES (gen_random_uuid(), gen_random_uuid(), 'agent-existing')`); err != nil {
		t.Fatalf("seed existing agent proposal: %v", err)
	}

	through521 := []string{
		"515_skill_evolution_task_dispatch_snapshot",
		"516_skill_evolution_task_dispatch_snapshot_id_index",
		"517_skill_evolution_task_dispatch_snapshot_identity_index",
		"518_skill_evolution_task_dispatch_snapshot_primary_key",
		"519_skill_evolution_proposal_rationale",
		"520_wiki_page_proposal_source",
		"521_wiki_page_room_proposal_idempotency_index",
	}
	opts.Files = realMigrationFiles(t, through521, "up")
	if err := runMigrations(ctx, pool, opts); err != nil {
		t.Fatalf("upgrade existing 514 ledger through 521: %v", err)
	}
	assertSkillEvolutionConstraintValidated(t, ctx, pool, "wiki_page_edit_proposal_source", false)

	const validateVersion = "522_wiki_page_proposal_source_validate"
	opts.Files = realMigrationFiles(t, []string{validateVersion}, "up")
	if err := runMigrations(ctx, pool, opts); err != nil {
		t.Fatalf("validate Wiki proposal source constraint: %v", err)
	}
	assertSkillEvolutionConstraintValidated(t, ctx, pool, "wiki_page_edit_proposal_source", true)

	for _, version := range append(append([]string{existingVersion}, through521...), validateVersion) {
		assertRoomMigrationRecorded(t, ctx, pool, version, true)
	}

	// Simulate DDL commit followed by a crash before either ledger write. Both
	// the PK attach and validation migrations must accept their exact end state.
	for _, version := range []string{"518_skill_evolution_task_dispatch_snapshot_primary_key", validateVersion} {
		if _, err := pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
			t.Fatalf("remove %s ledger row: %v", version, err)
		}
		opts.Files = realMigrationFiles(t, []string{version}, "up")
		if err := runMigrations(ctx, pool, opts); err != nil {
			t.Fatalf("replay %s after DDL success: %v", version, err)
		}
		assertRoomMigrationRecorded(t, ctx, pool, version, true)
	}

	var exactPrimaryKey bool
	if err := pool.QueryRow(ctx, `
SELECT c.contype = 'p'
       AND c.conkey = ARRAY[a.attnum]::SMALLINT[]
       AND c.convalidated
       AND NOT c.condeferrable
       AND NOT c.condeferred
       AND i.indnatts = 1 AND i.indnkeyatts = 1 AND i.indkey[0] = a.attnum
       AND i.indisunique AND i.indisprimary AND i.indisvalid AND i.indisready AND i.indislive
       AND i.indpred IS NULL AND i.indexprs IS NULL
       AND am.amname = 'btree'
FROM pg_constraint c
JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attname = 'id' AND NOT a.attisdropped
JOIN pg_index i ON i.indexrelid = c.conindid
JOIN pg_class index_relation ON index_relation.oid = c.conindid
JOIN pg_am am ON am.oid = index_relation.relam
WHERE c.conrelid = 'skill_evolution_task_dispatch_snapshot'::regclass
  AND c.conname = 'skill_evolution_task_dispatch_snapshot_pkey'
`).Scan(&exactPrimaryKey); err != nil || !exactPrimaryKey {
		t.Fatalf("dispatch snapshot primary key exact/error = %t/%v", exactPrimaryKey, err)
	}
}

func TestSkillEvolutionPrimaryKeyMigrationRejectsWrongCandidateIndex(t *testing.T) {
	pool, schema := newRoomMigrationTestSchema(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
CREATE TABLE skill_evolution_task_dispatch_snapshot (id UUID NOT NULL, other_id UUID NOT NULL);
CREATE UNIQUE INDEX skill_evolution_task_dispatch_snapshot_id_uidx
    ON skill_evolution_task_dispatch_snapshot (other_id)`); err != nil {
		t.Fatalf("create wrong candidate index: %v", err)
	}
	version := "518_skill_evolution_task_dispatch_snapshot_primary_key"
	err := runMigrations(ctx, pool, runOptions{
		Direction: "up", Files: realMigrationFiles(t, []string{version}, "up"),
		SchemaMigrationsTable: schema + ".schema_migrations", AdvisoryLockKey: roomMigrationTestLockKey(),
	})
	if err == nil || !strings.Contains(err.Error(), "missing or incompatible") {
		t.Fatalf("wrong candidate index error = %v, want incompatible diagnostic", err)
	}
}

func TestWikiProposalSourceRollbackPreservesRoomRows(t *testing.T) {
	pool, schema := newRoomMigrationTestSchema(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
CREATE TABLE wiki_page_edit_proposal (
    id UUID NOT NULL DEFAULT gen_random_uuid(), workspace_id UUID NOT NULL,
    agent_id UUID NOT NULL, idempotency_key TEXT NOT NULL
)`); err != nil {
		t.Fatalf("create Wiki proposal prerequisite: %v", err)
	}
	versions := []string{
		"520_wiki_page_proposal_source",
		"521_wiki_page_room_proposal_idempotency_index",
		"522_wiki_page_proposal_source_validate",
	}
	opts := runOptions{
		Direction: "up", Files: realMigrationFiles(t, versions, "up"),
		SchemaMigrationsTable: schema + ".schema_migrations", AdvisoryLockKey: roomMigrationTestLockKey(), Hooks: hooksForDirection("up"),
	}
	if err := runMigrations(ctx, pool, opts); err != nil {
		t.Fatalf("apply Wiki proposal source migrations: %v", err)
	}
	roomID := "00000000-0000-0000-0000-000000000522"
	if _, err := pool.Exec(ctx, `INSERT INTO wiki_page_edit_proposal (
workspace_id, agent_id, idempotency_key, source_kind, source_ref_id
) VALUES (gen_random_uuid(), NULL, 'room-existing', 'room', $1)`, roomID); err != nil {
		t.Fatalf("seed Room proposal: %v", err)
	}

	err := runMigrations(ctx, pool, runOptions{
		Direction: "down", Files: realMigrationFiles(t, []string{"520_wiki_page_proposal_source"}, "down"),
		SchemaMigrationsTable: schema + ".schema_migrations", AdvisoryLockKey: roomMigrationTestLockKey(),
	})
	if err == nil || !strings.Contains(err.Error(), "back up and explicitly remove Room proposals") {
		t.Fatalf("Room rollback preflight error = %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM wiki_page_edit_proposal WHERE source_kind = 'room' AND source_ref_id = $1`, roomID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("Room proposal after failed rollback count/error = %d/%v", count, err)
	}
	assertRoomMigrationRecorded(t, ctx, pool, "520_wiki_page_proposal_source", true)
}

func assertSkillEvolutionConstraintValidated(t *testing.T, ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, name string, want bool) {
	t.Helper()
	var got bool
	if err := pool.QueryRow(ctx, `SELECT convalidated FROM pg_constraint WHERE conname = $1`, name).Scan(&got); err != nil {
		t.Fatalf("read constraint %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("constraint %s validated = %t, want %t", name, got, want)
	}
}
