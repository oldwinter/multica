package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/events"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/scheduler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRoomMaintenanceSchedulerDispatchesOnce(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)
	bus := events.New()
	taskService := service.NewTaskService(queries, testPool, nil, bus)
	issueService := service.NewIssueService(queries, testPool, bus, nil, taskService)
	roomService := roomdomain.NewService(
		queries,
		testPool,
		func(q *db.Queries) roomdomain.AgentRuntimeLookup {
			return service.RuntimeLookup{Queries: q, Source: obsmetrics.RuntimeLookupSourceRoom}
		},
		taskService,
		service.NewRoomArtifactTargets(issueService),
		bus,
	)
	var facilitatorID string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text FROM agent WHERE workspace_id = $1 ORDER BY created_at LIMIT 1
	`, testWorkspaceID).Scan(&facilitatorID); err != nil {
		t.Fatalf("load Room scheduler facilitator: %v", err)
	}
	interval := int32(5)
	created, err := roomService.Create(ctx, roomdomain.CreateInput{
		WorkspaceID: parseUUID(testWorkspaceID), ActorUserID: parseUUID(testUserID),
		Title: "Scheduler Room", FacilitatorAgentID: parseUUID(facilitatorID),
		ScheduleIntervalMinutes: &interval,
	})
	if err != nil {
		t.Fatal(err)
	}
	roomID := util.UUIDToString(created.Room.ID)
	job := scheduler.RoomMaintenanceJob(roomService)
	job.Name = scheduler.JobNameRoomMaintenance + "_test_" + uuid.NewString()
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = testPool.Exec(bg, `DELETE FROM sys_cron_executions WHERE job_name = $1`, job.Name)
		_, _ = testPool.Exec(bg, `DELETE FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE room_id = $1)`, created.Room.ID)
		_, _ = testPool.Exec(bg, `DELETE FROM room_artifact WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(bg, `DELETE FROM room_turn WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(bg, `DELETE FROM room_cycle WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(bg, `DELETE FROM room_entry WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(bg, `DELETE FROM room_participant WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(bg, `DELETE FROM room WHERE id = $1`, created.Room.ID)
	})
	if _, err := testPool.Exec(ctx, `UPDATE room SET next_wake_at = now() - interval '2 minutes' WHERE id = $1`, created.Room.ID); err != nil {
		t.Fatal(err)
	}

	manager := scheduler.NewManager(testPool, scheduler.Options{RunnerID: "room-maintenance-test"})
	if err := manager.Register(job); err != nil {
		t.Fatal(err)
	}
	if err := manager.RunOnce(ctx); err != nil {
		t.Fatalf("first Room maintenance tick: %v", err)
	}
	if err := manager.RunOnce(ctx); err != nil {
		t.Fatalf("second Room maintenance tick: %v", err)
	}

	var auditRows, cycles, tasks int
	var auditStatus string
	var nextWake time.Time
	if err := testPool.QueryRow(ctx, `
		SELECT count(*), COALESCE(max(status), '')
		FROM sys_cron_executions WHERE job_name = $1 AND scope_kind = 'global' AND scope_id = 'global'
		`, job.Name).Scan(&auditRows, &auditStatus); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM room_cycle WHERE room_id = $1),
		       (SELECT count(*) FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE room_id = $1)),
		       (SELECT next_wake_at FROM room WHERE id = $1)
	`, created.Room.ID).Scan(&cycles, &tasks, &nextWake); err != nil {
		t.Fatal(err)
	}
	if auditRows != 1 || auditStatus != "SUCCESS" || cycles != 1 || tasks != 1 || !nextWake.After(time.Now().UTC()) {
		t.Fatalf("Room %s maintenance = audit %d/%s cycles %d tasks %d next %s", roomID, auditRows, auditStatus, cycles, tasks, nextWake)
	}
}
