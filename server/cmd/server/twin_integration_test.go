package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinHTTPThroughRouterAuthorizationAndLifecycle(t *testing.T) {
	// Given
	productionGenerator := testHandler.TwinService.ProposalGenerator
	testHandler.TwinService.ProposalGenerator = service.InventoryTwinProposalGenerator{}
	t.Cleanup(func() { testHandler.TwinService.ProposalGenerator = productionGenerator })

	ctx := context.Background()
	queries := db.New(testPool)
	var workspaceID, revisionID pgtype.UUID
	if err := testPool.QueryRow(ctx, `INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('Twin Router', 'twin-router-' || gen_random_uuid()::text, '', 'TWR') RETURNING id`).Scan(&workspaceID); err != nil {
		t.Fatalf("create Twin router workspace: %v", err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, workspaceID, testUserID); err != nil {
		t.Fatalf("create Twin router owner: %v", err)
	}
	memberID, memberToken := createTwinRouterUser(t, workspaceID, "member")
	content := json.RawMessage(`{"schema_version":1,"issues":[],"projects":[],"project_resources":[],"autopilot_runs":[]}`)
	revision, err := queries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{WorkspaceID: workspaceID, SourceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Content: content, TriggerKind: "manual"})
	if err != nil {
		t.Fatalf("create Twin router Wiki: %v", err)
	}
	revisionID = revision.ID
	if _, err := queries.CreateLMWikiReview(ctx, db.CreateLMWikiReviewParams{WorkspaceID: workspaceID, RevisionID: revisionID, Decision: "accepted", ReviewerID: parseRouterUUID(t, testUserID)}); err != nil {
		t.Fatalf("accept Twin router Wiki: %v", err)
	}
	adminID, adminToken := createTwinRouterUser(t, workspaceID, "admin")
	outsiderID, outsiderToken := createTwinRouterUser(t, workspaceID, "")
	t.Cleanup(func() {
		_ = queries.DeleteWorkspaceWikiTwinData(context.Background(), workspaceID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM member WHERE workspace_id = $1`, workspaceID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, memberID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, adminID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, outsiderID)
	})

	// When
	unauthenticated := twinRouterRequest(t, "", workspaceID.String(), http.MethodGet, "/api/twins", nil)
	memberOverview := twinRouterRequest(t, memberToken, workspaceID.String(), http.MethodGet, "/api/twins", nil)
	outsiderOverview := twinRouterRequest(t, outsiderToken, workspaceID.String(), http.MethodGet, "/api/twins", nil)
	memberWrite := twinRouterRequest(t, memberToken, workspaceID.String(), http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": revisionID.String()})
	created := twinRouterRequest(t, testToken, workspaceID.String(), http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": revisionID.String()})
	var createdBody struct {
		Created  bool `json:"created"`
		Proposal struct {
			ID string `json:"id"`
		} `json:"proposal"`
	}
	decodeTwinRouterResponse(t, created, &createdBody)
	adminRetry := twinRouterRequest(t, adminToken, workspaceID.String(), http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": revisionID.String()})
	accepted := twinRouterRequest(t, testToken, workspaceID.String(), http.MethodPost, "/api/twins/proposals/"+createdBody.Proposal.ID+"/accept", nil)
	var acceptedBody struct {
		Version struct {
			ID string `json:"id"`
		} `json:"version"`
	}
	decodeTwinRouterResponse(t, accepted, &acceptedBody)
	detail := twinRouterRequest(t, memberToken, workspaceID.String(), http.MethodGet, "/api/twins/proposals/"+createdBody.Proposal.ID, nil)
	versionDetail := twinRouterRequest(t, memberToken, workspaceID.String(), http.MethodGet, "/api/twins/versions/"+acceptedBody.Version.ID, nil)

	// Then
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated Twin GET = %d", unauthenticated.StatusCode)
	}
	if memberOverview.StatusCode != http.StatusOK {
		t.Fatalf("member Twin GET = %d", memberOverview.StatusCode)
	}
	if outsiderOverview.StatusCode != http.StatusNotFound {
		t.Fatalf("outsider Twin GET = %d", outsiderOverview.StatusCode)
	}
	if memberWrite.StatusCode != http.StatusForbidden {
		t.Fatalf("member Twin POST = %d", memberWrite.StatusCode)
	}
	if created.StatusCode != http.StatusCreated || !createdBody.Created || createdBody.Proposal.ID == "" {
		t.Fatalf("owner Twin POST = %d %#v", created.StatusCode, createdBody)
	}
	if adminRetry.StatusCode != http.StatusOK {
		t.Fatalf("admin Twin retry = %d", adminRetry.StatusCode)
	}
	if accepted.StatusCode != http.StatusCreated {
		t.Fatalf("owner Twin accept = %d", accepted.StatusCode)
	}
	if detail.StatusCode != http.StatusOK {
		t.Fatalf("member Twin detail = %d", detail.StatusCode)
	}
	if versionDetail.StatusCode != http.StatusOK {
		t.Fatalf("member Twin version detail = %d", versionDetail.StatusCode)
	}
}

func createTwinRouterUser(t *testing.T, workspaceID pgtype.UUID, role string) (string, string) {
	t.Helper()
	ctx := context.Background()
	var userID string
	email := "twin-router-" + role + "-" + workspaceID.String() + "@example.com"
	if err := testPool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Twin Router Member', $1) RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatalf("create Twin router user: %v", err)
	}
	if role != "" {
		if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, $3)`, workspaceID, userID, role); err != nil {
			t.Fatalf("create Twin router membership: %v", err)
		}
	}
	token, err := generateTestJWT(userID, email, "Twin Router Member")
	if err != nil {
		t.Fatalf("create Twin router token: %v", err)
	}
	return userID, token
}

func twinRouterRequest(t *testing.T, token, workspaceID, method, path string, body any) *http.Response {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode Twin router request: %v", err)
		}
	}
	request, err := http.NewRequest(method, testServer.URL+path, &payload)
	if err != nil {
		t.Fatalf("create Twin router request: %v", err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("X-Workspace-ID", workspaceID)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("execute Twin router request: %v", err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func decodeTwinRouterResponse(t *testing.T, response *http.Response, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode Twin router response: %v", err)
	}
}

func parseRouterUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil {
		t.Fatalf("parse router UUID: %v", err)
	}
	return parsed
}
