package analytics

import "strings"

var (
	roomSources = map[string]string{
		"agent": "agent", "manual": "manual", "mention": "mention",
		"message": "message", "schedule": "schedule",
	}
	roomTemplates = map[string]string{
		"decision": "decision", "incident": "incident", "planning": "planning",
		"research": "research", "risk": "risk",
	}
	roomReasons = map[string]string{
		"budget_exhausted": "budget_exhausted", "daily_turn_limit": "daily_turn_limit",
		"facilitator_unavailable": "facilitator_unavailable", "malformed_synthesis": "malformed_synthesis",
		"participant_failed": "participant_failed", "review_rejected": "review_rejected",
		"room_paused": "room_paused", "synthesis_failed": "synthesis_failed",
	}
	roomArtifactKinds = map[string]string{
		"decision": "decision", "issue": "issue", "wiki": "wiki",
	}
)

func RoomCreated(actorID, workspaceID, roomID, templateID string) Event {
	return roomEvent(EventRoomCreated, actorID, workspaceID, roomID, "", map[string]any{
		"template_id": normalizeRoomValue(templateID, roomTemplates, "none"),
	})
}

func RoomFirstCycleCompleted(actorID, workspaceID, roomID, cycleID, source string) Event {
	return roomEvent(EventRoomFirstCycleCompleted, actorID, workspaceID, roomID, cycleID, map[string]any{
		"source": normalizeRoomValue(source, roomSources, "other"),
	})
}

func RoomSynthesisAccepted(actorID, workspaceID, roomID, cycleID, source string) Event {
	return roomEvent(EventRoomSynthesisAccepted, actorID, workspaceID, roomID, cycleID, map[string]any{
		"source": normalizeRoomValue(source, roomSources, "other"),
	})
}

func RoomSynthesisRejected(actorID, workspaceID, roomID, cycleID, source string) Event {
	return roomEvent(EventRoomSynthesisRejected, actorID, workspaceID, roomID, cycleID, map[string]any{
		"source": normalizeRoomValue(source, roomSources, "other"),
		"reason": "review_rejected",
	})
}

func RoomSynthesisRetried(actorID, workspaceID, roomID, cycleID, source string) Event {
	return roomEvent(EventRoomSynthesisRetried, actorID, workspaceID, roomID, cycleID, map[string]any{
		"source": normalizeRoomValue(source, roomSources, "other"),
		"retry":  true,
	})
}

func RoomArtifactPromoted(actorID, workspaceID, roomID, cycleID, kind string) Event {
	return roomEvent(EventRoomArtifactPromoted, actorID, workspaceID, roomID, cycleID, map[string]any{
		"kind": normalizeRoomValue(kind, roomArtifactKinds, "other"),
	})
}

func RoomBudgetRefused(actorID, workspaceID, roomID, cycleID, source, reason string) Event {
	return roomEvent(EventRoomBudgetRefused, actorID, workspaceID, roomID, cycleID, map[string]any{
		"source": normalizeRoomValue(source, roomSources, "other"),
		"reason": normalizeRoomValue(reason, roomReasons, "other"),
	})
}

func RoomCycleFailed(actorID, workspaceID, roomID, cycleID, source, reason string) Event {
	return roomEvent(EventRoomCycleFailed, actorID, workspaceID, roomID, cycleID, map[string]any{
		"source": normalizeRoomValue(source, roomSources, "other"),
		"reason": normalizeRoomValue(reason, roomReasons, "other"),
	})
}

func roomEvent(name, actorID, workspaceID, roomID, cycleID string, properties map[string]any) Event {
	if actorID == "" {
		actorID = "workspace:" + workspaceID
	}
	properties["room_id"] = roomID
	if cycleID != "" {
		properties["cycle_id"] = cycleID
	}
	return Event{
		Name: name, DistinctID: actorID, WorkspaceID: workspaceID,
		Properties: withCoreProperties(properties, CoreProperties{
			UserID: nonAgentUserID(actorID), WorkspaceID: workspaceID,
		}),
	}
}

func normalizeRoomValue(value string, allowed map[string]string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if normalized, ok := allowed[value]; ok {
		return normalized
	}
	return fallback
}
