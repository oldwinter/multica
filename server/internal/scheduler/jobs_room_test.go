package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
)

type recordingRoomMaintainer struct {
	dueResult      roomdomain.DueResult
	reconciled     int
	dispatchErr    error
	reconcileErr   error
	dispatchNow    time.Time
	dispatchLimit  int32
	reconcileLimit int32
}

func (maintainer *recordingRoomMaintainer) DispatchDue(_ context.Context, now time.Time, limit int32) (roomdomain.DueResult, error) {
	maintainer.dispatchNow = now
	maintainer.dispatchLimit = limit
	return maintainer.dueResult, maintainer.dispatchErr
}

func (maintainer *recordingRoomMaintainer) Reconcile(_ context.Context, limit int32) (int, error) {
	maintainer.reconcileLimit = limit
	return maintainer.reconciled, maintainer.reconcileErr
}

func TestRoomMaintenanceJobDispatchesDueAndReconciles(t *testing.T) {
	maintainer := &recordingRoomMaintainer{
		dueResult:  roomdomain.DueResult{RoomsAdvanced: 2, CyclesQueued: 1, CyclesRefused: 1, TasksQueued: 3},
		reconciled: 4,
	}
	job := RoomMaintenanceJob(maintainer)
	if job.Name != JobNameRoomMaintenance || job.Cadence != time.Minute || !job.AllowStaleReentry {
		t.Fatalf("Room maintenance spec = %+v", job)
	}
	if err := NewManager(nil, Options{RunnerID: "room-spec"}).Register(job); err != nil {
		t.Fatalf("register Room maintenance: %v", err)
	}
	planTime := time.Date(2026, 8, 13, 4, 41, 0, 0, time.UTC)
	result, err := job.Handler(context.Background(), HandlerInput{Job: &job, Scope: ScopeGlobal, PlanTime: planTime})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 6 || result.Result["tasks_queued"] != 3 || result.Result["tasks_reconciled"] != 4 {
		t.Fatalf("Room maintenance result = %+v", result)
	}
	if maintainer.dispatchNow != planTime || maintainer.dispatchLimit != 100 || maintainer.reconcileLimit != 100 {
		t.Fatalf("Room maintenance calls = now %s due %d reconcile %d", maintainer.dispatchNow, maintainer.dispatchLimit, maintainer.reconcileLimit)
	}
}

func TestRoomMaintenanceJobStopsBeforeReconcileOnDispatchFailure(t *testing.T) {
	maintainer := &recordingRoomMaintainer{dispatchErr: errors.New("database unavailable")}
	job := RoomMaintenanceJob(maintainer)
	if _, err := job.Handler(context.Background(), HandlerInput{Job: &job, Scope: ScopeGlobal, PlanTime: time.Now()}); err == nil {
		t.Fatal("Room maintenance dispatch failure was swallowed")
	}
	if maintainer.reconcileLimit != 0 {
		t.Fatalf("reconcile ran after dispatch failure with limit %d", maintainer.reconcileLimit)
	}
}
