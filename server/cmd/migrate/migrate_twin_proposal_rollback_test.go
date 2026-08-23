package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestTwinProposalIdentityRollbackRejectsReplacementHistory(t *testing.T) {
	pool := openTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := fmt.Sprintf("migrate_twin_rollback_%d_%d", time.Now().UnixNano(), rand.Uint32())
	schemaName := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE"); err != nil {
			t.Logf("drop schema %s: %v", schema, err)
		}
	})

	tableName := pgx.Identifier{schema, "twin_proposal"}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE TABLE "+tableName+` (
		workspace_id UUID NOT NULL,
		kind TEXT NOT NULL,
		source_wiki_revision_id UUID NOT NULL,
		base_twin_version_id UUID
	)`); err != nil {
		t.Fatalf("create Twin proposal table: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+tableName+` (
		workspace_id, kind, source_wiki_revision_id, base_twin_version_id
	) VALUES
		('00000000-0000-0000-0000-000000000001', 'deposition', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000003'),
		('00000000-0000-0000-0000-000000000001', 'deposition', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000003')`); err != nil {
		t.Fatalf("seed replacement history: %v", err)
	}

	err := blockTwinProposalIdentityRollbackHook(tableName)(ctx, pool)
	if err == nil || !strings.Contains(err.Error(), "replacement history cannot be represented") {
		t.Fatalf("rollback guard error = %v", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM "+tableName+" WHERE ctid = (SELECT max(ctid) FROM "+tableName+")"); err != nil {
		t.Fatalf("remove duplicate history: %v", err)
	}
	if err := blockTwinProposalIdentityRollbackHook(tableName)(ctx, pool); err != nil {
		t.Fatalf("rollback guard rejected compatible history: %v", err)
	}
}
