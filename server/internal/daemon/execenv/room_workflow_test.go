package execenv

import (
	"strings"
	"testing"
)

func TestRoomWorkflowUsesDedicatedRoomPath(t *testing.T) {
	costLimit := int64(17)
	content := buildMetaSkillContentSlim("codex", TaskContextForEnv{
		RoomID:             "room-1",
		RoomCycleID:        "cycle-1",
		RoomTurnID:         "turn-1",
		RoomTitle:          "Architecture council",
		RoomInstructions:   "Challenge assumptions.",
		RoomCostLimitTicks: &costLimit,
		AutopilotRunID:     "autopilot-run-sentinel",
		AutopilotID:        "autopilot-sentinel",
		AgentName:          "Reviewer",
		AgentID:            "agent-1",
	})

	if count := strings.Count(content, "### Workflow"); count != 1 {
		t.Fatalf("Workflow section count = %d, want 1", count)
	}
	for _, sentinel := range []string{"autopilot-run-sentinel", "autopilot-sentinel"} {
		if strings.Contains(content, sentinel) {
			t.Errorf("Room workflow contains Autopilot sentinel %q", sentinel)
		}
	}
	if !strings.Contains(content, "17 ticks") {
		t.Fatalf("Room workflow omits cost limit:\n%s", content)
	}
}
