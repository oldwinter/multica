package skillevolution

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWorkspaceCleanupOwnsEveryEvolutionAndTaskReviewTable(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	query := mustReadOwnershipSource(t, filepath.Join(serverRoot, "pkg/db/queries/skill_evolution.sql"))
	for _, table := range []string{
		"skill_evolution_evaluation",
		"skill_evolution_evidence",
		"skill_evolution_loop",
		"skill_evolution_proposal",
		"skill_evolution_release",
		"skill_evolution_review",
		"skill_evolution_revision",
		"skill_evolution_revision_file",
		"skill_evolution_task_attribution",
		"skill_evolution_task_dispatch_snapshot",
		"task_run_review",
	} {
		if !strings.Contains(query, "DELETE FROM "+table) {
			t.Errorf("DeleteWorkspaceSkillEvolutionData does not own %s", table)
		}
	}

	module := mustReadOwnershipSource(t, filepath.Join(filepath.Dir(currentFile), "module.go"))
	if !strings.Contains(module, "RegisterWorkspaceCleanupContributor(NewWorkspaceCleanup())") {
		t.Error("production module does not register the workspace cleanup owner")
	}
}

func mustReadOwnershipSource(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
