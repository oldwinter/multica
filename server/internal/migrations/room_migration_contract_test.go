package migrations

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRoomMigrationContract(t *testing.T) {
	dir := realMigrationsDir(t)
	tablesPath := filepath.Join(dir, "285_room_tables.up.sql")
	tables, err := os.ReadFile(tablesPath)
	if err != nil {
		t.Fatalf("read Room table migration: %v", err)
	}

	sql := string(tables)
	for _, table := range []string{
		"room",
		"room_participant",
		"room_entry",
		"room_cycle",
		"room_turn",
		"room_artifact",
	} {
		if !strings.Contains(sql, "CREATE TABLE "+table+" (") {
			t.Errorf("Room table migration does not create %s", table)
		}
	}
	if !strings.Contains(sql, "ALTER TABLE agent_task_queue ADD COLUMN room_turn_id UUID NULL") {
		t.Error("Room table migration does not add the nullable task queue Room turn link")
	}
	for _, forbidden := range []string{"FOREIGN KEY", "REFERENCES", "ON DELETE", "ON UPDATE"} {
		if strings.Contains(strings.ToUpper(sql), forbidden) {
			t.Errorf("Room table migration contains forbidden relationship clause %q", forbidden)
		}
	}
}

func TestRoomConcurrentIndexMigrationsContainOneStatement(t *testing.T) {
	dir := realMigrationsDir(t)
	indexFiles, err := filepath.Glob(filepath.Join(dir, "*_room_*index.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	indexFiles = append(indexFiles, filepath.Join(dir, "302_agent_task_room_turn_index.up.sql"))

	wantFiles := map[string]bool{
		"286_room_id_index.up.sql":                             true,
		"287_room_participant_id_index.up.sql":                 true,
		"288_room_entry_id_index.up.sql":                       true,
		"289_room_cycle_id_index.up.sql":                       true,
		"290_room_turn_id_index.up.sql":                        true,
		"291_room_artifact_id_index.up.sql":                    true,
		"293_room_workspace_index.up.sql":                      true,
		"294_room_participant_identity_index.up.sql":           true,
		"295_room_entry_ordinal_index.up.sql":                  true,
		"296_room_cycle_sequence_index.up.sql":                 true,
		"297_room_cycle_wake_index.up.sql":                     true,
		"298_room_active_cycle_index.up.sql":                   true,
		"299_room_turn_participant_index.up.sql":               true,
		"300_room_artifact_kind_index.up.sql":                  true,
		"301_room_due_index.up.sql":                            true,
		"302_agent_task_room_turn_index.up.sql":                true,
		"303_room_artifact_room_index.up.sql":                  true,
		"304_room_turn_room_index.up.sql":                      true,
		"305_room_participant_room_index.up.sql":               true,
		"307_agent_task_room_turn_lookup_index.up.sql":         true,
		"308_room_entry_turn_result_index.up.sql":              true,
		"404_room_memory_revision_id_index.up.sql":             true,
		"406_room_memory_revision_version_index.up.sql":        true,
		"407_room_cycle_phase_index.up.sql":                    true,
		"408_room_artifact_memory_revision_index.up.sql":       true,
		"410_room_recommendation_review_id_index.up.sql":       true,
		"412_room_recommendation_review_identity_index.up.sql": true,
		"445_room_turn_kind_attempt_index.up.sql":              true,
		"447_room_synthesis_retry_key_index.up.sql":            true,
		"450_room_memory_review_key_index.up.sql":              true,
	}

	seen := make(map[string]bool, len(indexFiles))
	createIndex := regexp.MustCompile(`(?is)^CREATE\s+(?:UNIQUE\s+)?INDEX\s+CONCURRENTLY\b.*;\s*$`)
	for _, path := range indexFiles {
		name := filepath.Base(path)
		if !wantFiles[name] {
			continue
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("read %s: %v", name, readErr)
			continue
		}
		seen[name] = true
		if !createIndex.Match(bytesWithoutSQLComments(body)) {
			t.Errorf("%s must contain one CREATE INDEX CONCURRENTLY statement", name)
		}
	}

	for name := range wantFiles {
		if !seen[name] {
			t.Errorf("missing Room concurrent index migration %s", name)
		}
	}
}

func TestRoomOutcomeConcurrentIndexMigrationsAreCrashReplaySafe(t *testing.T) {
	dir := realMigrationsDir(t)
	createFiles := []string{
		"404_room_memory_revision_id_index.up.sql",
		"406_room_memory_revision_version_index.up.sql",
		"407_room_cycle_phase_index.up.sql",
		"408_room_artifact_memory_revision_index.up.sql",
		"410_room_recommendation_review_id_index.up.sql",
		"412_room_recommendation_review_identity_index.up.sql",
		"444_room_turn_identity_index_drop.down.sql",
		"445_room_turn_kind_attempt_index.up.sql",
		"447_room_synthesis_retry_key_index.up.sql",
		"450_room_memory_review_key_index.up.sql",
	}
	dropFiles := []string{
		"404_room_memory_revision_id_index.down.sql",
		"406_room_memory_revision_version_index.down.sql",
		"407_room_cycle_phase_index.down.sql",
		"408_room_artifact_memory_revision_index.down.sql",
		"410_room_recommendation_review_id_index.down.sql",
		"412_room_recommendation_review_identity_index.down.sql",
		"444_room_turn_identity_index_drop.up.sql",
		"445_room_turn_kind_attempt_index.down.sql",
		"447_room_synthesis_retry_key_index.down.sql",
		"450_room_memory_review_key_index.down.sql",
	}

	createIndex := regexp.MustCompile(`(?is)^CREATE\s+(?:UNIQUE\s+)?INDEX\s+CONCURRENTLY\s+IF\s+NOT\s+EXISTS\b.*;\s*$`)
	dropIndex := regexp.MustCompile(`(?is)^DROP\s+INDEX\s+CONCURRENTLY\s+IF\s+EXISTS\b.*;\s*$`)
	for _, name := range createFiles {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if !createIndex.Match(bytesWithoutSQLComments(body)) {
			t.Errorf("%s must use CREATE INDEX CONCURRENTLY IF NOT EXISTS", name)
		}
	}
	for _, name := range dropFiles {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if !dropIndex.Match(bytesWithoutSQLComments(body)) {
			t.Errorf("%s must use DROP INDEX CONCURRENTLY IF EXISTS", name)
		}
	}
}

func TestRoomCapabilityRolloutDoesNotRewriteExistingRooms(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(realMigrationsDir(t), "448_room_capability_rollout.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToUpper(string(bytesWithoutSQLComments(body)))
	if strings.Contains(sql, "UPDATE ROOM") {
		t.Fatal("capability rollout must not rewrite existing or concurrently created Rooms")
	}
	if !strings.Contains(sql, "ALTER TABLE ROOM ALTER COLUMN CAPABILITY_VERSION SET DEFAULT 1") {
		t.Fatal("capability rollout must keep capability_version defaulted to 1")
	}
}

func TestRoomRecommendationTargetTaxonomyMigration(t *testing.T) {
	dir := realMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "513_room_recommendation_target_taxonomy.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	validate, err := os.ReadFile(filepath.Join(dir, "514_room_recommendation_target_taxonomy_validate.up.sql"))
	if err != nil {
		t.Fatal(err)
	}

	taxonomy := string(up)
	for _, kind := range []string{
		"knowledge",
		"preference",
		"constraint",
		"executable_procedure",
		"implementation_defect",
		"decision",
		"unsupported",
	} {
		if !strings.Contains(taxonomy, "'"+kind+"'") {
			t.Errorf("Room recommendation target migration omits %q", kind)
		}
	}
	if !strings.Contains(strings.ToUpper(taxonomy), "NOT VALID") {
		t.Fatal("Room recommendation target constraint must avoid an inline table scan")
	}
	for _, forbidden := range []string{"FOREIGN KEY", "REFERENCES", "ON DELETE", "ON UPDATE"} {
		if strings.Contains(strings.ToUpper(taxonomy), forbidden) {
			t.Errorf("Room recommendation target migration contains forbidden relationship clause %q", forbidden)
		}
	}
	if !strings.Contains(strings.ToUpper(string(validate)), "VALIDATE CONSTRAINT ROOM_ARTIFACT_KIND_CHECK") {
		t.Fatal("Room recommendation target validation migration is missing")
	}
}

func bytesWithoutSQLComments(input []byte) []byte {
	lines := strings.Split(string(input), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			kept = append(kept, line)
		}
	}
	return []byte(strings.TrimSpace(strings.Join(kept, "\n")))
}
