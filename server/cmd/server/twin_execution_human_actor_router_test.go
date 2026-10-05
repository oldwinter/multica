package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
)

func TestTwinProposalCreateRouteRejectsMachineCredentialsBeforeGeneration(t *testing.T) {
	taskToken := createReviewRouteTaskToken(t)
	cloudToken := "mcn_twin_execution_test"
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

	for _, machine := range []struct {
		name    string
		baseURL string
		token   string
	}{
		{name: "task token", baseURL: testServer.URL, token: taskToken},
		{name: "cloud node PAT", baseURL: cloudRouter.URL, token: cloudToken},
	} {
		t.Run(machine.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, machine.baseURL+"/api/twins/proposals", nil)
			if err != nil {
				t.Fatalf("create Twin proposal request: %v", err)
			}
			request.Header.Set("Authorization", "Bearer "+machine.token)
			request.Header.Set("X-Workspace-ID", testWorkspaceID)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("execute Twin proposal request: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("machine proposal mutation status = %d, want 403", response.StatusCode)
			}
		})
	}
}
