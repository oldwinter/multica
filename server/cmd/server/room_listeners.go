package main

import (
	"context"
	"log/slog"

	"github.com/multica-ai/multica/server/internal/events"
	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/util"
)

func registerRoomListeners(bus *events.Bus, runtime roomdomain.Runtime) {
	bus.Subscribe(roomdomain.EventTaskLifecycle, func(event events.Event) {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return
		}
		roomTurnID, ok := payload["room_turn_id"].(string)
		if !ok || roomTurnID == "" {
			return
		}
		if _, err := util.ParseUUID(roomTurnID); err != nil {
			return
		}
		taskID, ok := payload["task_id"].(string)
		if !ok || taskID == "" {
			return
		}
		parsed, err := util.ParseUUID(taskID)
		if err != nil {
			return
		}
		if _, err := runtime.SyncTask(context.Background(), parsed); err != nil {
			slog.Error("room listener: failed to synchronize task", "task_id", taskID, "error", err)
		}
	})
}
