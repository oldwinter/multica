package daemon

import (
	"strings"
	"testing"
)

func TestBuildRoomPromptCarriesPersistentContext(t *testing.T) {
	out := buildPromptBody(Task{
		RoomID:           "room-1",
		RoomCycleID:      "cycle-2",
		RoomTurnID:       "turn-3",
		RoomTitle:        "Architecture council",
		RoomInstructions: "Challenge assumptions.",
		RoomMemory:       []byte(`{"summary":"Prefer explicit contracts"}`),
		RoomTranscript:   []byte(`[{"body":"Review the retry boundary."}]`),
	}, "codex")
	for _, want := range []string{
		"persistent Multica Room",
		"Architecture council",
		"Challenge assumptions.",
		"Prefer explicit contracts",
		"Review the retry boundary.",
		"appended to the Room transcript",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Room prompt missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "multica issue get") {
		t.Fatalf("Room prompt contains Issue instructions:\n%s", out)
	}
}
