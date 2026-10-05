package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type lmWikiHTTPRefresh struct {
	Created  bool `json:"created"`
	Revision struct {
		ID             string          `json:"id"`
		RevisionNumber int64           `json:"revision_number"`
		Content        json.RawMessage `json:"content"`
	} `json:"revision"`
}

type lmWikiHTTPSources struct {
	ProjectID      string
	IssueID        string
	ResourceID     string
	AutopilotRunID string
}

func resetLMWikiHTTPFixture(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	for _, query := range []string{
		`DELETE FROM twin_proposal_review WHERE workspace_id = $1`,
		`DELETE FROM twin_version WHERE workspace_id = $1`,
		`DELETE FROM twin_proposal WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_review WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_citation WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_revision WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_source_wiki_page WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_source_policy WHERE workspace_id = $1`,
		`DELETE FROM autopilot_run WHERE autopilot_id IN (SELECT id FROM autopilot WHERE workspace_id = $1 AND title LIKE 'LM Wiki HTTP %')`,
		`DELETE FROM autopilot WHERE workspace_id = $1 AND title LIKE 'LM Wiki HTTP %'`,
		`DELETE FROM project_resource WHERE workspace_id = $1 AND label LIKE 'LM Wiki HTTP %'`,
		`DELETE FROM issue WHERE workspace_id = $1 AND title LIKE 'LM Wiki HTTP %'`,
		`DELETE FROM project WHERE workspace_id = $1 AND title LIKE 'LM Wiki HTTP %'`,
	} {
		if _, err := testPool.Exec(ctx, query, testWorkspaceID); err != nil {
			t.Fatalf("reset LM Wiki HTTP fixture: %v", err)
		}
	}
}

func lmWikiHTTPRequest(t *testing.T, token, workspaceID, method, path string, body io.Reader) *http.Response {
	t.Helper()
	response, err := lmWikiHTTPDo(token, workspaceID, method, path, body)
	if err != nil {
		t.Fatalf("perform LM Wiki request: %v", err)
	}
	return response
}

func lmWikiHTTPDo(token, workspaceID, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, testServer.URL+path, body)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if workspaceID != "" {
		req.Header.Set("X-Workspace-ID", workspaceID)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func decodeLMWikiHTTP(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode LM Wiki response: %v", err)
	}
}

func createLMWikiHTTPUser(t *testing.T, label string) (string, string) {
	t.Helper()
	email := fmt.Sprintf("lm-wiki-http-%s-%s@multica.test", label, strings.ReplaceAll(t.Name(), "/", "-"))
	var id string
	if err := testPool.QueryRow(context.Background(), `INSERT INTO "user" (name, email) VALUES ('LM Wiki HTTP actor', $1) RETURNING id`, email).Scan(&id); err != nil {
		t.Fatalf("create LM Wiki user: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, id) })
	token, err := generateTestJWT(id, email, "LM Wiki HTTP actor")
	if err != nil {
		t.Fatalf("generate LM Wiki token: %v", err)
	}
	return id, token
}

func createLMWikiHTTPWorkspace(t *testing.T) string {
	t.Helper()
	var id string
	slug := "lm-wiki-http-" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-")
	if err := testPool.QueryRow(context.Background(), `INSERT INTO workspace (name, slug) VALUES ('LM Wiki HTTP other', $1) RETURNING id`, slug).Scan(&id); err != nil {
		t.Fatalf("create second LM Wiki workspace: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, id, testUserID); err != nil {
		t.Fatalf("add second LM Wiki workspace owner: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, id) })
	return id
}

func seedLMWikiHTTPSources(t *testing.T) lmWikiHTTPSources {
	t.Helper()
	ctx := context.Background()
	var projectID, issueID, resourceID, autopilotID, autopilotRunID string
	if err := testPool.QueryRow(ctx, `INSERT INTO project (workspace_id, title, description, status, priority) VALUES ($1, 'LM Wiki HTTP project', 'public project', 'in_progress', 'medium') RETURNING id`, testWorkspaceID).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `INSERT INTO issue (workspace_id, title, description, status, priority, creator_type, creator_id, number, project_id, metadata) VALUES ($1, 'LM Wiki HTTP safe issue', 'public description', 'todo', 'high', 'member', $2, 9101, $3, '{"private-chat":"do-not-leak"}') RETURNING id`, testWorkspaceID, testUserID, projectID).Scan(&issueID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `INSERT INTO project_resource (project_id, workspace_id, resource_type, resource_ref, label, position) VALUES ($1, $2, 'github_repo', '{"url":"https://user:top-secret@git.example.com/acme/wiki.git?token=secret"}', 'LM Wiki HTTP repository', 1) RETURNING id`, projectID, testWorkspaceID).Scan(&resourceID); err != nil {
		t.Fatal(err)
	}
	var agentID string
	if err := testPool.QueryRow(ctx, `SELECT id FROM agent WHERE workspace_id = $1 ORDER BY created_at LIMIT 1`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `INSERT INTO autopilot (workspace_id, title, description, assignee_id, status, execution_mode, created_by_type, created_by_id) VALUES ($1, 'LM Wiki HTTP autopilot', '', $2, 'active', 'run_only', 'member', $3) RETURNING id`, testWorkspaceID, agentID, testUserID).Scan(&autopilotID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `INSERT INTO autopilot_run (autopilot_id, source, status, issue_id, completed_at, trigger_payload, result) VALUES ($1, 'manual', 'completed', $2, now(), '{"trigger_payload":"private"}', '{"result-secret":"private"}') RETURNING id`, autopilotID, issueID).Scan(&autopilotRunID); err != nil {
		t.Fatal(err)
	}
	return lmWikiHTTPSources{
		ProjectID:      projectID,
		IssueID:        issueID,
		ResourceID:     resourceID,
		AutopilotRunID: autopilotRunID,
	}
}
