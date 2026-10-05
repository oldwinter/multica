package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestDeleteProjectRemovesOnlyMatchingTwinBinding(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	projectID := dbfx.Project(t, "Twin binding project teardown")
	controlWorkspaceID := twinBindingControlWorkspace(t)
	targetID := seedTwinBindingForScope(t, testWorkspaceID, "project", projectID)
	foreignWorkspaceID := seedTwinBindingForScope(t, controlWorkspaceID, "project", projectID)
	otherScopeID := seedTwinBindingForScope(t, testWorkspaceID, "issue", projectID)

	req := withURLParam(newRequest(http.MethodDelete, "/api/projects/"+projectID, nil), "id", projectID)
	testutil.Call(t, testHandler.DeleteProject, req).Want(http.StatusNoContent)

	assertTwinBindingCount(t, targetID, 0)
	assertTwinBindingCount(t, foreignWorkspaceID, 1)
	assertTwinBindingCount(t, otherScopeID, 1)
}

func TestDeleteIssueRemovesOnlyMatchingTwinBinding(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	issueID := dbfx.Issue(t, "Twin binding issue teardown")
	controlWorkspaceID := twinBindingControlWorkspace(t)
	targetID := seedTwinBindingForScope(t, testWorkspaceID, "issue", issueID)
	foreignWorkspaceID := seedTwinBindingForScope(t, controlWorkspaceID, "issue", issueID)
	otherScopeID := seedTwinBindingForScope(t, testWorkspaceID, "project", issueID)

	req := withURLParam(newRequest(http.MethodDelete, "/api/issues/"+issueID, nil), "id", issueID)
	testutil.Call(t, testHandler.DeleteIssue, req).Want(http.StatusNoContent)

	assertTwinBindingCount(t, targetID, 0)
	assertTwinBindingCount(t, foreignWorkspaceID, 1)
	assertTwinBindingCount(t, otherScopeID, 1)
}

func TestBatchDeleteIssuesRemovesMatchingTwinBindings(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	firstIssueID := dbfx.Issue(t, "Twin binding batch teardown one")
	secondIssueID := dbfx.Issue(t, "Twin binding batch teardown two")
	firstBindingID := seedTwinBindingForScope(t, testWorkspaceID, "issue", firstIssueID)
	secondBindingID := seedTwinBindingForScope(t, testWorkspaceID, "issue", secondIssueID)

	req := newRequest(http.MethodPost, "/api/issues/batch-delete", map[string]any{
		"issue_ids": []string{firstIssueID, secondIssueID},
	})
	var response struct {
		Deleted int `json:"deleted"`
	}
	testutil.Call(t, testHandler.BatchDeleteIssues, req).
		Want(http.StatusOK).
		JSON(&response)
	if response.Deleted != 2 {
		t.Fatalf("BatchDeleteIssues deleted = %d, want 2", response.Deleted)
	}

	assertTwinBindingCount(t, firstBindingID, 0)
	assertTwinBindingCount(t, secondBindingID, 0)
}

func twinBindingControlWorkspace(t *testing.T) string {
	t.Helper()
	return dbfx.Workspace(t, "Twin binding teardown control", "twin-binding-control-"+uuid.NewString())
}

func seedTwinBindingForScope(t *testing.T, workspaceID, scopeType, scopeID string) string {
	t.Helper()
	return dbfx.Insert(t, "twin_binding", testutil.Cols{
		"workspace_id":    workspaceID,
		"scope_type":      scopeType,
		"scope_id":        scopeID,
		"state":           "enabled",
		"twin_version_id": uuid.NewString(),
	})
}

func assertTwinBindingCount(t *testing.T, bindingID string, want int) {
	t.Helper()
	var got int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM twin_binding WHERE id = $1`, bindingID).Scan(&got); err != nil {
		t.Fatalf("count Twin binding %s: %v", bindingID, err)
	}
	if got != want {
		t.Fatalf("Twin binding %s count = %d, want %d", bindingID, got, want)
	}
}
