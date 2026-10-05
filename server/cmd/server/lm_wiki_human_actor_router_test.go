package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestLMWikiTwinReviewRoutesRequireHumanActor(t *testing.T) {
	taskToken := createReviewRouteTaskToken(t)
	cloudToken := "mcn_lm_wiki_review_test"
	fleet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pat/verify" {
			t.Fatalf("Fleet verify path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"valid":true,"owner_id":%q,"instance_id":"i-test","instance_record_id":"00000000-0000-0000-0000-000000000001"}`, testUserID)
	}))
	defer fleet.Close()
	t.Setenv("MULTICA_CLOUD_URL", fleet.URL)

	hub := realtime.NewHub()
	go hub.Run()
	bus := events.New()
	registerListeners(bus, hub)
	cloudRouter := httptest.NewServer(NewRouter(testPool, hub, bus, analytics.NoopClient{}, nil))
	defer cloudRouter.Close()

	const artifactID = "00000000-0000-0000-0000-000000000001"
	paths := []string{
		"/api/lm-wiki/revisions/" + artifactID + "/accept",
		"/api/lm-wiki/revisions/" + artifactID + "/reject",
		"/api/twins/proposals/" + artifactID + "/accept",
		"/api/twins/proposals/" + artifactID + "/reject",
	}
	machines := []struct {
		name    string
		baseURL string
		token   string
	}{
		{name: "task token", baseURL: testServer.URL, token: taskToken},
		{name: "cloud node PAT", baseURL: cloudRouter.URL, token: cloudToken},
	}
	for _, machine := range machines {
		for _, path := range paths {
			t.Run(machine.name+" "+path, func(t *testing.T) {
				response := reviewRouteRequest(t, machine.baseURL, machine.token, path)
				if response.StatusCode != http.StatusForbidden {
					t.Fatalf("machine review mutation status = %d, want 403", response.StatusCode)
				}
			})
		}
	}

	adminID, adminToken := createLMWikiHTTPUser(t, "review-admin")
	if _, err := testPool.Exec(context.Background(), `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'admin')`, testWorkspaceID, adminID); err != nil {
		t.Fatalf("create review admin membership: %v", err)
	}
	for _, human := range []struct {
		name  string
		token string
	}{{name: "owner", token: testToken}, {name: "admin", token: adminToken}} {
		for _, path := range paths {
			t.Run(human.name+" "+path, func(t *testing.T) {
				response := reviewRouteRequest(t, testServer.URL, human.token, path)
				if response.StatusCode != http.StatusNotFound {
					t.Fatalf("human review mutation status = %d, want downstream 404", response.StatusCode)
				}
			})
		}
	}
}

func createReviewRouteTaskToken(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	var agentID, runtimeID, taskID pgtype.UUID
	if err := testPool.QueryRow(ctx, `SELECT id, runtime_id FROM agent WHERE workspace_id = $1 ORDER BY created_at LIMIT 1`, testWorkspaceID).Scan(&agentID, &runtimeID); err != nil {
		t.Fatalf("load review route agent: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, originator_user_id, accountable_user_id)
VALUES ($1, $2, 'queued', 0, $3, $3)
RETURNING id
`, agentID, runtimeID, testUserID).Scan(&taskID); err != nil {
		t.Fatalf("create review route task: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})

	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatalf("generate review route task token: %v", err)
	}
	queries := db.New(testPool)
	if _, err := queries.CreateTaskToken(ctx, db.CreateTaskTokenParams{
		TokenHash:   auth.HashToken(token),
		TaskID:      taskID,
		AgentID:     agentID,
		WorkspaceID: parseRouterUUID(t, testWorkspaceID),
		UserID:      parseRouterUUID(t, testUserID),
		ExpiresAt:   pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("persist review route task token: %v", err)
	}
	return token
}

func reviewRouteRequest(t *testing.T, baseURL, token, path string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, baseURL+path, nil)
	if err != nil {
		t.Fatalf("create review route request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", testWorkspaceID)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("execute review route request: %v", err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}
