package room

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *Service) DispatchDue(ctx context.Context, now time.Time, limit int32) (DueResult, error) {
	if limit <= 0 || limit > 1000 {
		return DueResult{}, ErrInvalidInput
	}
	now = now.UTC()
	rooms, err := s.queries.ListDueRooms(ctx, db.ListDueRoomsParams{
		DueAt: pgtype.Timestamptz{Time: now, Valid: true}, LimitCount: limit,
	})
	if err != nil {
		return DueResult{}, fmt.Errorf("list due Rooms: %w", err)
	}
	result := DueResult{}
	var firstErr error
	for _, roomRow := range rooms {
		plannedAt := roomRow.NextWakeAt.Time.UTC()
		wake, wakeErr := s.Wake(ctx, WakeInput{
			WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
			Source: "schedule", WakeKey: "schedule:" + plannedAt.Format(time.RFC3339Nano),
			PlannedAt: pgtype.Timestamptz{Time: plannedAt, Valid: true},
		})
		if wakeErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("dispatch due Room %s: %w", util.UUIDToString(roomRow.ID), wakeErr)
			}
			continue
		}
		if wake.Cycle.Status == "refused" {
			result.CyclesRefused++
		} else {
			result.CyclesQueued++
			result.TasksQueued += len(wake.Tasks)
		}
		nextWake := nextRoomWake(plannedAt, now, roomRow.ScheduleIntervalMinutes.Int32)
		if advanced, advanceErr := s.queries.AdvanceRoomSchedule(ctx, db.AdvanceRoomScheduleParams{
			NextWakeAt: pgtype.Timestamptz{Time: nextWake, Valid: true},
			ID:         roomRow.ID, WorkspaceID: roomRow.WorkspaceID, ExpectedNextWakeAt: roomRow.NextWakeAt,
		}); advanceErr == nil {
			result.RoomsAdvanced++
			s.publish("room:updated", advanced, pgtype.UUID{}, map[string]any{"room": advanced})
		} else if !errors.Is(advanceErr, pgx.ErrNoRows) {
			if firstErr == nil {
				firstErr = fmt.Errorf("advance due Room %s: %w", util.UUIDToString(roomRow.ID), advanceErr)
			}
		}
	}
	return result, firstErr
}

func nextRoomWake(plannedAt, now time.Time, intervalMinutes int32) time.Time {
	interval := time.Duration(intervalMinutes) * time.Minute
	next := plannedAt.Add(interval)
	if next.After(now) {
		return next
	}
	missed := now.Sub(next)/interval + 1
	return next.Add(missed * interval)
}
