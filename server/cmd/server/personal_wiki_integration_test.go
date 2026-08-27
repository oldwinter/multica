package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestPersonalWikiRoutesDoNotRequireWorkspaceMembership(t *testing.T) {
	email := "personal-wiki-router-" + uuid.NewString() + "@multica.test"
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	userID := fx.User(t, "Personal Wiki router user", email)
	userFixture := testutil.New(testPool, testWorkspaceID, userID)
	userFixture.Cleanup(t, `DELETE FROM wiki_page WHERE owner_user_id = $1`, userID)
	userFixture.Cleanup(t, `DELETE FROM wiki_page_revision WHERE owner_user_id = $1`, userID)
	token, err := generateTestJWT(userID, email, "Personal Wiki router user")
	if err != nil {
		t.Fatalf("generate personal Wiki JWT: %v", err)
	}

	var membershipCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM member WHERE user_id = $1`, userID).Scan(&membershipCount); err != nil {
		t.Fatalf("count personal Wiki memberships: %v", err)
	}
	if membershipCount != 0 {
		t.Fatalf("personal Wiki route user membership count = %d, want 0", membershipCount)
	}

	createResponse := personalWikiRouteRequest(t, testServer.URL, token, http.MethodPost, "/api/personal-wiki/pages", map[string]any{
		"path": "router/private.md", "title": "Router private", "content": "private route content",
	})
	if createResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create personal Wiki route status = %d", createResponse.StatusCode)
	}
	var created struct {
		ID                    string  `json:"id"`
		Scope                 string  `json:"scope"`
		WorkspaceID           *string `json:"workspace_id"`
		CurrentRevisionNumber int64   `json:"current_revision_number"`
	}
	decodePersonalWikiRoute(t, createResponse, &created)
	if created.Scope != "user" || created.WorkspaceID != nil || created.CurrentRevisionNumber != 1 {
		t.Fatalf("created personal Wiki route response = %+v", created)
	}

	listResponse := personalWikiRouteRequest(t, testServer.URL, token, http.MethodGet, "/api/personal-wiki/pages", nil)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("list personal Wiki route status = %d", listResponse.StatusCode)
	}
	var listed []struct {
		ID string `json:"id"`
	}
	decodePersonalWikiRoute(t, listResponse, &listed)
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed personal Wiki routes = %+v", listed)
	}

	getResponse := personalWikiRouteRequest(t, testServer.URL, token, http.MethodGet, "/api/personal-wiki/pages/"+created.ID, nil)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get personal Wiki route status = %d", getResponse.StatusCode)
	}
	getResponse.Body.Close()

	updateResponse := personalWikiRouteRequest(t, testServer.URL, token, http.MethodPut, "/api/personal-wiki/pages/"+created.ID, map[string]any{
		"expected_revision_number": 1, "content": "updated private route content",
	})
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("update personal Wiki route status = %d", updateResponse.StatusCode)
	}
	var updated struct {
		CurrentRevisionNumber int64  `json:"current_revision_number"`
		Content               string `json:"content"`
	}
	decodePersonalWikiRoute(t, updateResponse, &updated)
	if updated.CurrentRevisionNumber != 2 || updated.Content != "updated private route content" {
		t.Fatalf("updated personal Wiki route response = %+v", updated)
	}

	sharedPageID := fx.Insert(t, "wiki_page", testutil.Cols{
		"workspace_id": testWorkspaceID, "scope": "workspace",
		"path": "router/shared-" + uuid.NewString() + ".md", "title": "Shared",
		"content": "shared route content", "created_by": testUserID,
	})
	sharedResponse := personalWikiRouteRequest(t, testServer.URL, token, http.MethodGet, "/api/personal-wiki/pages/"+sharedPageID, nil)
	if sharedResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("shared page through personal alias status = %d, want 404", sharedResponse.StatusCode)
	}
	sharedResponse.Body.Close()

	taskResponse := personalWikiRouteRequest(t, testServer.URL, createReviewRouteTaskToken(t), http.MethodGet, "/api/personal-wiki/pages", nil)
	if taskResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("task token personal Wiki route status = %d, want 403", taskResponse.StatusCode)
	}
	taskResponse.Body.Close()
}

func TestPersonalWikiRoutesRejectCloudMachineCredential(t *testing.T) {
	fleet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"valid":true,"owner_id":%q,"instance_id":"personal-wiki-test","instance_record_id":"00000000-0000-0000-0000-000000000001"}`, testUserID)
	}))
	defer fleet.Close()
	t.Setenv("MULTICA_CLOUD_URL", fleet.URL)

	hub := realtime.NewHub()
	go hub.Run()
	bus := events.New()
	registerListeners(bus, hub)
	server := httptest.NewServer(NewRouter(testPool, hub, bus, analytics.NoopClient{}, nil))
	defer server.Close()

	response := personalWikiRouteRequest(t, server.URL, "mcn_personal_wiki_test", http.MethodGet, "/api/personal-wiki/pages", nil)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cloud PAT personal Wiki route status = %d, want 403", response.StatusCode)
	}
	response.Body.Close()
}

func personalWikiRouteRequest(t *testing.T, baseURL, token, method, path string, body any) *http.Response {
	t.Helper()
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal personal Wiki request: %v", err)
		}
	}
	request, err := http.NewRequest(method, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create personal Wiki request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("execute personal Wiki request: %v", err)
	}
	return response
}

func decodePersonalWikiRoute(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode personal Wiki response: %v", err)
	}
}
