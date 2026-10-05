package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	migrationfiles "github.com/multica-ai/multica/server/internal/migrations"
)

func TestLegacyMigrationVersionAliasesReferenceCurrentFiles(t *testing.T) {
	files, err := migrationfiles.Files("up")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	currentVersions := make(map[string]bool, len(files))
	for _, file := range files {
		currentVersions[migrationfiles.ExtractVersion(file)] = true
	}

	if got, want := len(legacyMigrationVersionAliases), 52; got != want {
		t.Fatalf("legacy alias groups = %d, want %d", got, want)
	}
	seenAliases := make(map[string]string, len(legacyMigrationVersionAliases)*2)
	for current, aliases := range legacyMigrationVersionAliases {
		if !currentVersions[current] {
			t.Errorf("alias target %q has no current up migration", current)
		}
		if len(aliases) != 2 {
			t.Errorf("alias target %q has %d legacy identities, want 2", current, len(aliases))
		}
		for _, alias := range aliases {
			if alias == current {
				t.Errorf("alias target %q aliases itself", current)
			}
			if previous, duplicate := seenAliases[alias]; duplicate {
				t.Errorf("legacy identity %q maps to both %q and %q", alias, previous, current)
			}
			seenAliases[alias] = current
		}
	}
}

func TestRunMigrationsReconcilesLegacyVersionOnUp(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	const current = "900_current_identity"
	const legacy = "800_legacy_identity"

	initializeMigrationLedger(t, ctx, f)
	insertMigrationVersion(t, ctx, f, legacy)

	path := filepath.Join(t.TempDir(), current+".up.sql")
	body := "CREATE TABLE " + pgx.Identifier{f.schema, "must_not_run"}.Sanitize() + " (id BIGINT);\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	opts := runOptions{
		Direction:             "up",
		Files:                 []string{path},
		SchemaMigrationsTable: f.tableFQN,
		AdvisoryLockKey:       f.lockKey,
		VersionAliases:        map[string][]string{current: {legacy}},
	}
	if err := runMigrations(ctx, f.pool, opts); err != nil {
		t.Fatalf("reconcile legacy migration: %v", err)
	}
	if f.tableExists(t, "must_not_run") {
		t.Fatal("aliased migration SQL executed instead of being reconciled")
	}
	if got, want := f.appliedVersions(t), []string{legacy, current}; !equalStrings(got, want) {
		t.Fatalf("schema_migrations = %v, want %v", got, want)
	}
	if err := runMigrations(ctx, f.pool, opts); err != nil {
		t.Fatalf("repeat reconciled migration: %v", err)
	}
}

func TestRunMigrationsRollbackClearsCurrentAndLegacyVersions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	const current = "900_current_identity"
	const legacy = "800_legacy_identity"

	initializeMigrationLedger(t, ctx, f)
	insertMigrationVersion(t, ctx, f, legacy)
	if _, err := f.pool.Exec(ctx, "CREATE TABLE "+pgx.Identifier{f.schema, "legacy_object"}.Sanitize()+" (id BIGINT)"); err != nil {
		t.Fatalf("create legacy migration object: %v", err)
	}

	path := filepath.Join(t.TempDir(), current+".down.sql")
	body := "DROP TABLE " + pgx.Identifier{f.schema, "legacy_object"}.Sanitize() + ";\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write rollback migration: %v", err)
	}

	if err := runMigrations(ctx, f.pool, runOptions{
		Direction:             "down",
		Files:                 []string{path},
		SchemaMigrationsTable: f.tableFQN,
		AdvisoryLockKey:       f.lockKey,
		VersionAliases:        map[string][]string{current: {legacy}},
	}); err != nil {
		t.Fatalf("roll back legacy migration: %v", err)
	}
	if f.tableExists(t, "legacy_object") {
		t.Fatal("legacy migration object still exists after rollback")
	}
	if got := f.appliedVersions(t); len(got) != 0 {
		t.Fatalf("schema_migrations after rollback = %v, want empty", got)
	}
}

func initializeMigrationLedger(t *testing.T, ctx context.Context, f *fixture) {
	t.Helper()
	if err := runMigrations(ctx, f.pool, runOptions{
		Direction:             "up",
		SchemaMigrationsTable: f.tableFQN,
		AdvisoryLockKey:       f.lockKey,
	}); err != nil {
		t.Fatalf("initialize migration ledger: %v", err)
	}
}

func insertMigrationVersion(t *testing.T, ctx context.Context, f *fixture, version string) {
	t.Helper()
	table := pgx.Identifier{f.schema, "schema_migrations"}.Sanitize()
	if _, err := f.pool.Exec(ctx, "INSERT INTO "+table+" (version) VALUES ($1)", version); err != nil {
		t.Fatalf("insert migration version %q: %v", version, err)
	}
}
