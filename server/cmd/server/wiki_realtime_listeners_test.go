package main

import (
	"encoding/json"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestWikiRealtimePersonalEventsAreUserDirected(t *testing.T) {
	bus := events.New()
	broadcaster := &fakeBroadcaster{}
	registerListeners(bus, broadcaster)

	bus.Publish(events.Event{
		Type:        protocol.EventWikiPageUpdated,
		WorkspaceID: "ws-should-not-receive",
		ActorType:   "member",
		ActorID:     "user-1",
		Payload: protocol.WikiEventPayload{
			PageID:      "page-1",
			Scope:       "user",
			RecipientID: "user-1",
		},
	})

	if len(broadcaster.workspaceCalls) != 0 {
		t.Fatalf("personal Wiki event reached workspace fanout: %+v", broadcaster.workspaceCalls)
	}
	if len(broadcaster.userCalls) != 1 || broadcaster.userCalls[0].userID != "user-1" {
		t.Fatalf("personal Wiki event user calls = %+v, want user-1 once", broadcaster.userCalls)
	}

	var frame struct {
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(broadcaster.userCalls[0].msg, &frame); err != nil {
		t.Fatalf("unmarshal personal Wiki frame: %v", err)
	}
	if _, ok := frame.Payload["recipient_id"]; ok {
		t.Fatal("personal Wiki frame exposed recipient_id")
	}
}

func TestWikiRealtimePersonalEventWithoutRecipientFailsClosed(t *testing.T) {
	bus := events.New()
	broadcaster := &fakeBroadcaster{}
	registerListeners(bus, broadcaster)

	bus.Publish(events.Event{
		Type:        protocol.EventWikiPageCreated,
		WorkspaceID: "ws-should-not-receive",
		Payload: protocol.WikiEventPayload{
			PageID: "page-1",
			Scope:  "user",
		},
	})

	if len(broadcaster.workspaceCalls) != 0 || len(broadcaster.userCalls) != 0 {
		t.Fatalf("recipient-less personal Wiki event was delivered: workspace=%+v user=%+v", broadcaster.workspaceCalls, broadcaster.userCalls)
	}
}

func TestWikiRealtimeWorkspaceEventsUseWorkspaceFanout(t *testing.T) {
	bus := events.New()
	broadcaster := &fakeBroadcaster{}
	registerListeners(bus, broadcaster)

	bus.Publish(events.Event{
		Type:        protocol.EventWikiRevisionCreated,
		WorkspaceID: "ws-1",
		Payload: protocol.WikiEventPayload{
			PageID:         "page-1",
			Scope:          "workspace",
			RevisionID:     "revision-1",
			RevisionNumber: 1,
		},
	})

	if len(broadcaster.userCalls) != 0 {
		t.Fatalf("workspace Wiki event reached user fanout: %+v", broadcaster.userCalls)
	}
	if len(broadcaster.workspaceCalls) != 1 || broadcaster.workspaceCalls[0].workspaceID != "ws-1" {
		t.Fatalf("workspace Wiki event calls = %+v, want ws-1 once", broadcaster.workspaceCalls)
	}
}
