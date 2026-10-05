package room

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestUpdateBudgetProtectsActiveCommitmentAndSupportsClearing(t *testing.T) {
	fixture := newServiceFixture(t)
	ctx := context.Background()
	created := createSingleOutcomeRoom(t, fixture, "Budget management Room", 100)
	if _, err := fixture.service.Wake(ctx, WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:budget-management",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.UpdateBudget(ctx, UpdateBudgetInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		DailyTurnLimit: pgtype.Int4{Int32: 20, Valid: true},
		MaxCostTicks:   pgtype.Int8{Int64: 99, Valid: true},
	}); !errors.Is(err, ErrBudgetBelowCommitted) {
		t.Fatalf("lower active ceiling error = %v, want %v", err, ErrBudgetBelowCommitted)
	}
	updated, err := fixture.service.UpdateBudget(ctx, UpdateBudgetInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		DailyTurnLimit: pgtype.Int4{Int32: 20, Valid: true},
		MaxCostTicks:   pgtype.Int8{Int64: 100, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.DailyTurnLimit.Valid || updated.DailyTurnLimit.Int32 != 20 ||
		!updated.MaxCostTicks.Valid || updated.MaxCostTicks.Int64 != 100 {
		t.Fatalf("updated Room budget = %+v", updated)
	}
	cleared, err := fixture.service.UpdateBudget(ctx, UpdateBudgetInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.DailyTurnLimit.Valid || cleared.MaxCostTicks.Valid {
		t.Fatalf("cleared Room budget = %+v", cleared)
	}
}
