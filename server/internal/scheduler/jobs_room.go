package scheduler

import (
	"context"
	"fmt"
	"time"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
)

const JobNameRoomMaintenance = "room_maintenance"

func RoomMaintenanceJob(maintainer roomdomain.Maintenance) JobSpec {
	return JobSpec{
		Name:              JobNameRoomMaintenance,
		Cadence:           time.Minute,
		CatchUpMode:       CatchUpLatestOnly,
		CatchUpWindow:     5 * time.Minute,
		RunTimeout:        45 * time.Second,
		StaleTimeout:      2 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		AllowStaleReentry: true,
		MaxAttempts:       3,
		RetryBackoff:      []time.Duration{time.Minute, 5 * time.Minute},
		Scopes:            StaticScopes(ScopeGlobal),
		Handler: func(ctx context.Context, input HandlerInput) (HandlerResult, error) {
			due, err := maintainer.DispatchDue(ctx, input.PlanTime, 100)
			if err != nil {
				return HandlerResult{}, fmt.Errorf("dispatch due Rooms: %w", err)
			}
			reconciled, err := maintainer.Reconcile(ctx, 100)
			if err != nil {
				return HandlerResult{}, fmt.Errorf("reconcile Room tasks: %w", err)
			}
			return HandlerResult{
				RowsAffected: int64(due.RoomsAdvanced + reconciled),
				Result: map[string]any{
					"rooms_advanced":   due.RoomsAdvanced,
					"cycles_queued":    due.CyclesQueued,
					"cycles_refused":   due.CyclesRefused,
					"tasks_queued":     due.TasksQueued,
					"tasks_reconciled": reconciled,
				},
			}, nil
		},
	}
}
