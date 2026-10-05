package handler

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestPublishTwinRealtimePreservesWorkspaceAndHumanActor(t *testing.T) {
	bus := events.New()
	var captured events.Event
	bus.Subscribe(protocol.EventTwinProposalChanged, func(event events.Event) {
		captured = event
	})
	h := &Handler{Bus: bus}

	h.publishTwinProposalChanged("workspace-1", "member-1", "proposal-1", "accepted", "version-1")

	if captured.Type != protocol.EventTwinProposalChanged || captured.WorkspaceID != "workspace-1" || captured.ActorType != "member" || captured.ActorID != "member-1" {
		t.Fatalf("captured event = %#v", captured)
	}
	payload, ok := captured.Payload.(protocol.TwinProposalChangedPayload)
	if !ok || payload.ProposalID != "proposal-1" || payload.State != "accepted" || payload.VersionID != "version-1" {
		t.Fatalf("captured payload = %#v", captured.Payload)
	}
}

func TestPublishTwinRealtimeIsNilBusSafe(t *testing.T) {
	(&Handler{}).publishTwinBindingDeleted("workspace-1", "member-1", "binding-1")
}
