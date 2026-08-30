package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWikiEvidenceEgressCompatibilityMigration(t *testing.T) {
	dir := realMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "455_wiki_evidence_egress_compatibility.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join(dir, "455_wiki_evidence_egress_compatibility.down.sql"))
	if err != nil {
		t.Fatal(err)
	}

	upSQL := strings.ToUpper(string(bytesWithoutSQLComments(up)))
	for _, column := range []string{
		"SOURCE_POLICY_VERSION BIGINT NOT NULL DEFAULT 0",
		"SOURCE_POLICY_DIGEST TEXT NOT NULL",
	} {
		if !strings.Contains(upSQL, "ADD COLUMN IF NOT EXISTS "+column) {
			t.Errorf("compatibility migration does not add %s idempotently", column)
		}
	}
	if count := strings.Count(upSQL, "ADD COLUMN IF NOT EXISTS REMOTE_GENERATION_ENABLED BOOLEAN NOT NULL DEFAULT FALSE"); count != 2 {
		t.Errorf("compatibility migration adds remote_generation_enabled %d times, want once per Wiki policy and revision table", count)
	}
	for _, table := range []string{"LM_WIKI_SOURCE_POLICY", "LM_WIKI_REVISION"} {
		if !strings.Contains(upSQL, "ALTER TABLE "+table) {
			t.Errorf("compatibility migration does not alter %s", table)
		}
	}
	for _, constraint := range []string{
		"LM_WIKI_REVISION_SOURCE_POLICY_VERSION_CHECK",
		"LM_WIKI_REVISION_SOURCE_POLICY_DIGEST_CHECK",
	} {
		if !strings.Contains(upSQL, "ADD CONSTRAINT "+constraint) {
			t.Errorf("compatibility migration does not add %s", constraint)
		}
		if !strings.Contains(upSQL, "VALIDATE CONSTRAINT "+constraint) {
			t.Errorf("compatibility migration does not validate %s", constraint)
		}
	}

	downSQL := strings.ToUpper(string(bytesWithoutSQLComments(down)))
	if downSQL != "SELECT 1;" {
		t.Fatalf("compatibility rollback must preserve the version-454 schema, got %q", downSQL)
	}
}
