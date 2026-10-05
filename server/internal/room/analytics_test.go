package room

import (
	"context"
	"testing"

	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRoomCreateAndBudgetRefusalAnalyticsAreIdempotent(t *testing.T) {
	fixture := newServiceFixture(t)
	roomMetrics := obsmetrics.NewBusinessMetrics()
	fixture.service.SetAnalytics(nil, roomMetrics)
	limit := int32(1)
	created, err := fixture.service.Create(context.Background(), CreateInput{
		WorkspaceID: fixture.workspaceID, ActorUserID: fixture.userID,
		Title: "Budgeted room", TemplateID: "decision", FacilitatorSquadID: fixture.squadID,
		DailyTurnLimit: &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	wake := WakeInput{
		WorkspaceID: fixture.workspaceID, RoomID: created.Room.ID, ActorUserID: fixture.userID,
		Source: "manual", WakeKey: "manual:budget-analytics",
	}
	first, err := fixture.service.Wake(context.Background(), wake)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.Wake(context.Background(), wake)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cycle.Status != "refused" || second.Cycle.ID != first.Cycle.ID {
		t.Fatalf("budget wake replay = first %+v second %+v", first.Cycle, second.Cycle)
	}
	assertRoomMetric(t, roomMetrics, "created", "other", "none", "none", 1)
	assertRoomMetric(t, roomMetrics, "budget_refused", "manual", "budget_exhausted", "none", 1)
}

func TestRoomReviewRetryAndFailureAnalyticsAreIdempotent(t *testing.T) {
	fixture := setupPendingOutcome(t)
	roomMetrics := obsmetrics.NewBusinessMetrics()
	fixture.service.SetAnalytics(nil, roomMetrics)
	ctx := context.Background()
	review := ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "reject", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "analytics:reject",
	}
	if _, err := fixture.service.Review(ctx, review); err != nil {
		t.Fatal(err)
	}
	if replay, err := fixture.service.Review(ctx, review); err != nil || !replay.Replayed {
		t.Fatalf("review replay = %+v, %v", replay, err)
	}
	retry := RetrySynthesisInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, IdempotencyKey: "analytics:retry",
	}
	retried, err := fixture.service.RetrySynthesis(ctx, retry)
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := fixture.service.RetrySynthesis(ctx, retry); err != nil || !replay.Replayed {
		t.Fatalf("retry replay = %+v, %v", replay, err)
	}
	completeRoomTask(t, fixture.serviceFixture, retried.Task.ID, "not structured synthesis")
	if changed, err := fixture.service.SyncTask(ctx, retried.Task.ID); err != nil || !changed {
		t.Fatalf("sync malformed retry = %t, %v", changed, err)
	}
	assertRoomMetric(t, roomMetrics, "synthesis_rejected", "manual", "review_rejected", "none", 1)
	assertRoomMetric(t, roomMetrics, "synthesis_retried", "manual", "none", "none", 1)
	assertRoomMetric(t, roomMetrics, "cycle_failed", "manual", "malformed_synthesis", "none", 1)
}

func TestRoomAcceptanceAndPromotionAnalyticsAreIdempotent(t *testing.T) {
	fixture := setupPendingOutcome(t)
	roomMetrics := obsmetrics.NewBusinessMetrics()
	fixture.service.SetAnalytics(nil, roomMetrics)
	ctx := context.Background()
	review := ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "analytics:accept",
	}
	if _, err := fixture.service.Review(ctx, review); err != nil {
		t.Fatal(err)
	}
	if replay, err := fixture.service.Review(ctx, review); err != nil || !replay.Replayed {
		t.Fatalf("accept replay = %+v, %v", replay, err)
	}
	promotion := recommendationPromotion(fixture, "analytics:promotion")
	if result, err := fixture.service.Promote(ctx, promotion); err != nil || !result.Created {
		t.Fatalf("promotion = %+v, %v", result, err)
	}
	if replay, err := fixture.service.Promote(ctx, promotion); err != nil || replay.Created {
		t.Fatalf("promotion replay = %+v, %v", replay, err)
	}
	assertRoomMetric(t, roomMetrics, "synthesis_accepted", "manual", "none", "none", 1)
	assertRoomMetric(t, roomMetrics, "first_cycle_completed", "manual", "none", "none", 1)
	assertRoomMetric(t, roomMetrics, "artifact_promoted", "other", "none", "decision", 1)
}

func assertRoomMetric(t *testing.T, metrics *obsmetrics.BusinessMetrics, event, source, reason, kind string, want float64) {
	t.Helper()
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(metrics.Collectors()...)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]string{"event_kind": event, "source": source, "reason": reason, "kind": kind}
	for _, family := range families {
		if family.GetName() != "multica_room_outcome_total" {
			continue
		}
		for _, metric := range family.Metric {
			matched := len(metric.Label) == len(wanted)
			for _, label := range metric.Label {
				if wanted[label.GetName()] != label.GetValue() {
					matched = false
					break
				}
			}
			if matched {
				if got := metric.GetCounter().GetValue(); got != want {
					t.Fatalf("Room metric %v = %v, want %v", wanted, got, want)
				}
				return
			}
		}
	}
	t.Fatalf("Room metric %v not found", wanted)
}
