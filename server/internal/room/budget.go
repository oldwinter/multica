package room

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *Service) UpdateBudget(ctx context.Context, input UpdateBudgetInput) (db.Room, error) {
	if !input.WorkspaceID.Valid || !input.RoomID.Valid || !input.ActorUserID.Valid ||
		(input.DailyTurnLimit.Valid && input.DailyTurnLimit.Int32 <= 0) ||
		(input.MaxCostTicks.Valid && input.MaxCostTicks.Int64 <= 0) {
		return db.Room{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return db.Room{}, fmt.Errorf("begin Room budget update: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	roomRow, err := lockRoomForMember(ctx, queries, input.WorkspaceID, input.RoomID, input.ActorUserID)
	if err != nil {
		return db.Room{}, err
	}
	member, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID: input.ActorUserID, WorkspaceID: input.WorkspaceID,
	})
	if err != nil {
		return db.Room{}, fmt.Errorf("load Room budget actor: %w", err)
	}
	if member.Role != "owner" && member.Role != "admin" {
		return db.Room{}, ErrBudgetPermissionDenied
	}
	if err := s.validateBudgetCommitment(ctx, queries, roomRow, input); err != nil {
		return db.Room{}, err
	}
	updated, err := queries.UpdateRoomBudget(ctx, db.UpdateRoomBudgetParams{
		DailyTurnLimit: input.DailyTurnLimit, MaxCostTicks: input.MaxCostTicks,
		ID: input.RoomID, WorkspaceID: input.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Room{}, ErrNotFound
	}
	if err != nil {
		return db.Room{}, fmt.Errorf("update Room budget: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Room{}, fmt.Errorf("commit Room budget update: %w", err)
	}
	s.publish(EventRoomUpdated, updated, input.ActorUserID, roomEventPayload(updated))
	return updated, nil
}

func (s *Service) validateBudgetCommitment(ctx context.Context, queries *db.Queries, roomRow db.Room, input UpdateBudgetInput) error {
	now := s.now().UTC()
	usedToday, err := queries.CountRoomTurnsSince(ctx, db.CountRoomTurnsSinceParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
		SinceAt: pgtype.Timestamptz{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("count Room committed turns: %w", err)
	}
	usage, err := queries.GetRoomUsageSummary(ctx, db.GetRoomUsageSummaryParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
	})
	if err != nil {
		return fmt.Errorf("load Room committed cost: %w", err)
	}
	committedTurns := usedToday
	committedCost := usage.CostTicks
	uncostedOutsideActiveCycle := usage.UncostedTurns
	if roomRow.ActiveCycleID.Valid {
		cycle, cycleErr := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{
			ID: roomRow.ActiveCycleID, WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
		})
		if cycleErr != nil && !errors.Is(cycleErr, pgx.ErrNoRows) {
			return fmt.Errorf("load active Room commitment: %w", cycleErr)
		}
		if cycleErr == nil {
			createdTurns, countErr := queries.CountRoomTurnsByCycle(ctx, db.CountRoomTurnsByCycleParams{
				WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
			})
			if countErr != nil {
				return fmt.Errorf("count active Room turns: %w", countErr)
			}
			committedTurns += max(int64(0), int64(cycle.ExpectedMaxTurns)-createdTurns)
			if cycle.CostLimitTicks.Valid {
				cycleUsage, usageErr := queries.GetRoomCycleUsageSummary(ctx, db.GetRoomCycleUsageSummaryParams{
					WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
				})
				if usageErr != nil {
					return fmt.Errorf("load active Room cost: %w", usageErr)
				}
				uncostedOutsideActiveCycle = max(int64(0), uncostedOutsideActiveCycle-cycleUsage.UncostedTurns)
				committedCost += max(int64(0), cycle.CostLimitTicks.Int64-cycleUsage.CostTicks)
			}
		}
	}
	if input.DailyTurnLimit.Valid && int64(input.DailyTurnLimit.Int32) < committedTurns {
		return ErrBudgetBelowCommitted
	}
	if input.MaxCostTicks.Valid {
		if uncostedOutsideActiveCycle > 0 {
			return ErrBudgetHasUncostedUsage
		}
		if input.MaxCostTicks.Int64 < committedCost {
			return ErrBudgetBelowCommitted
		}
	}
	return nil
}
