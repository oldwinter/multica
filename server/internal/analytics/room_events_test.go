package analytics

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRoomEventsExposeOnlyBoundedMetadata(t *testing.T) {
	const secret = "private transcript memory body summary title"
	events := []Event{
		RoomCreated("user", "workspace", "room", secret),
		RoomFirstCycleCompleted("user", "workspace", "room", "cycle", secret),
		RoomSynthesisAccepted("user", "workspace", "room", "cycle", secret),
		RoomSynthesisRejected("user", "workspace", "room", "cycle", secret),
		RoomSynthesisRetried("user", "workspace", "room", "cycle", secret),
		RoomArtifactPromoted("user", "workspace", "room", "cycle", secret),
		RoomBudgetRefused("user", "workspace", "room", "cycle", secret, secret),
		RoomCycleFailed("user", "workspace", "room", "cycle", secret, secret),
	}
	allowed := map[string]bool{
		"cycle_id": true, "is_demo": true, "kind": true, "reason": true,
		"retry": true, "room_id": true, "source": true, "template_id": true,
		"user_id": true,
	}
	for _, event := range events {
		encoded, err := json.Marshal(event.Properties)
		if err != nil {
			t.Fatalf("marshal %s properties: %v", event.Name, err)
		}
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("%s leaked unbounded metadata: %s", event.Name, encoded)
		}
		for key := range event.Properties {
			if !allowed[key] {
				t.Errorf("%s has unapproved property %q", event.Name, key)
			}
			for _, forbidden := range []string{"transcript", "memory", "body", "summary", "title"} {
				if strings.Contains(key, forbidden) {
					t.Errorf("%s has private property %q", event.Name, key)
				}
			}
		}
	}
}

func TestRoomEventUsesWorkspaceIdentityForSystemActor(t *testing.T) {
	event := RoomCycleFailed("", "workspace", "room", "cycle", "schedule", "synthesis_failed")
	if event.DistinctID != "workspace:workspace" {
		t.Fatalf("distinct id = %q", event.DistinctID)
	}
	if _, ok := event.Properties["user_id"]; ok {
		t.Fatalf("system event unexpectedly has user_id: %+v", event.Properties)
	}
}
