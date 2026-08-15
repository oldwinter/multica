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
		"286_room_id_index.up.sql":                      true,
		"287_room_participant_id_index.up.sql":          true,
		"288_room_entry_id_index.up.sql":                true,
		"289_room_cycle_id_index.up.sql":                true,
		"290_room_turn_id_index.up.sql":                 true,
		"291_room_artifact_id_index.up.sql":             true,
		"293_room_workspace_index.up.sql":               true,
		"294_room_participant_identity_index.up.sql":    true,
		"295_room_entry_ordinal_index.up.sql":           true,
		"296_room_cycle_sequence_index.up.sql":          true,
		"297_room_cycle_wake_index.up.sql":              true,
		"298_room_active_cycle_index.up.sql":            true,
		"299_room_turn_participant_index.up.sql":        true,
		"300_room_artifact_kind_index.up.sql":           true,
		"301_room_due_index.up.sql":                     true,
		"302_agent_task_room_turn_index.up.sql":         true,
		"303_room_artifact_room_index.up.sql":           true,
		"304_room_turn_room_index.up.sql":               true,
		"305_room_participant_room_index.up.sql":        true,
		"307_agent_task_room_turn_lookup_index.up.sql":  true,
		"308_room_entry_turn_result_index.up.sql":       true,
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
