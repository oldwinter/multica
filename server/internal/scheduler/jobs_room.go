package scheduler

import (
	"context"
	"errors"
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
			due, dispatchErr := maintainer.DispatchDue(ctx, input.PlanTime, 100)
			reconciled, reconcileErr := maintainer.Reconcile(ctx, 100)
			result := HandlerResult{
				RowsAffected: int64(due.RoomsAdvanced + reconciled),
				Result: map[string]any{
					"rooms_advanced":   due.RoomsAdvanced,
					"cycles_queued":    due.CyclesQueued,
					"cycles_refused":   due.CyclesRefused,
					"tasks_queued":     due.TasksQueued,
					"tasks_reconciled": reconciled,
				},
			}
			if dispatchErr != nil || reconcileErr != nil {
				return result, errors.Join(
					wrapRoomMaintenanceError("dispatch due Rooms", dispatchErr),
					wrapRoomMaintenanceError("reconcile Room tasks", reconcileErr),
				)
			}
			return result, nil
		},
	}
}

func wrapRoomMaintenanceError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
