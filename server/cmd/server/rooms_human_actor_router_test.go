package main

import (
	"net/http"
	"testing"
)

func TestRoomOutcomeMutationRoutesRequireHumanActor(t *testing.T) {
	taskToken := createReviewRouteTaskToken(t)
	const resourceID = "00000000-0000-0000-0000-000000000001"
	paths := []string{
		"/api/rooms/" + resourceID + "/cycles/" + resourceID + "/synthesis/retry",
		"/api/rooms/" + resourceID + "/cycles/" + resourceID + "/review",
		"/api/rooms/" + resourceID + "/cycles/" + resourceID + "/cancel",
		"/api/rooms/" + resourceID + "/memory-revisions/" + resourceID + "/recommendations/next-step/review",
		"/api/rooms/" + resourceID + "/promotions",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			response := reviewRouteRequest(t, testServer.URL, taskToken, path)
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("machine Room mutation status = %d, want 403", response.StatusCode)
			}
		})
	}
}
