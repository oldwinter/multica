package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type recordingRoomRuntime struct {
	mu      sync.Mutex
	taskIDs []pgtype.UUID
	err     error
}

func (runtime *recordingRoomRuntime) SyncTask(_ context.Context, taskID pgtype.UUID) (bool, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.taskIDs = append(runtime.taskIDs, taskID)
	return true, runtime.err
}

func (*recordingRoomRuntime) Reconcile(context.Context, int32) (int, error) {
	return 0, nil
}

func TestRoomListenersSynchronizeTaskLifecycleEvents(t *testing.T) {
	bus := events.New()
	runtime := &recordingRoomRuntime{}
	registerRoomListeners(bus, runtime)
	taskID := "11111111-2222-4333-8444-555555555555"
	roomTurnID := "66666666-7777-4888-8999-aaaaaaaaaaaa"

	for _, taskEvent := range []string{
		protocol.EventTaskRunning,
		protocol.EventTaskCompleted,
		protocol.EventTaskFailed,
		protocol.EventTaskCancelled,
	} {
		bus.Publish(events.Event{Type: roomdomain.EventTaskLifecycle, Payload: map[string]any{"task_id": taskID, "room_turn_id": roomTurnID, "task_event": taskEvent}})
	}
	bus.Publish(events.Event{Type: roomdomain.EventTaskLifecycle, Payload: map[string]any{"task_id": "not-a-uuid", "room_turn_id": roomTurnID}})
	bus.Publish(events.Event{Type: roomdomain.EventTaskLifecycle, Payload: "wrong-shape"})

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if len(runtime.taskIDs) != 4 {
		t.Fatalf("Room synchronization calls = %d, want 4", len(runtime.taskIDs))
	}
	want, err := util.ParseUUID(taskID)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range runtime.taskIDs {
		if got != want {
			t.Fatalf("synchronized task = %v, want %v", got, want)
		}
	}
}

func TestRoomListenersIgnoreTaskLifecycleEventsWithoutRoomTurn(t *testing.T) {
	bus := events.New()
	runtime := &recordingRoomRuntime{}
	registerRoomListeners(bus, runtime)

	bus.Publish(events.Event{
		Type:    roomdomain.EventTaskLifecycle,
		Payload: map[string]any{"task_id": "11111111-2222-4333-8444-555555555555"},
	})
	bus.Publish(events.Event{
		Type: roomdomain.EventTaskLifecycle,
		Payload: map[string]any{
			"task_id":      "11111111-2222-4333-8444-555555555555",
			"room_turn_id": "not-a-uuid",
		},
	})

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if len(runtime.taskIDs) != 0 {
		t.Fatalf("Room synchronization calls = %d, want 0", len(runtime.taskIDs))
	}
}

func TestRoomListenerDoesNotInterruptOtherSubscribersOnSyncFailure(t *testing.T) {
	bus := events.New()
	runtime := &recordingRoomRuntime{err: errors.New("projection unavailable")}
	registerRoomListeners(bus, runtime)
	called := false
	bus.SubscribeAll(func(events.Event) { called = true })

	bus.Publish(events.Event{
		Type: roomdomain.EventTaskLifecycle,
		Payload: map[string]any{
			"task_id":      "11111111-2222-4333-8444-555555555555",
			"room_turn_id": "66666666-7777-4888-8999-aaaaaaaaaaaa",
		},
	})
	if !called {
		t.Fatal("Room listener failure prevented later event subscribers")
	}
}

func TestRoomTaskLifecycleEventSynchronizesWithoutWorkspaceFanout(t *testing.T) {
	bus := events.New()
	runtime := &recordingRoomRuntime{}
	broadcaster := &fakeBroadcaster{}
	registerRoomListeners(bus, runtime)
	registerListeners(bus, broadcaster)
	bus.Publish(events.Event{
		Type: roomdomain.EventTaskLifecycle,
		Payload: map[string]any{
			"task_id":      "11111111-2222-4333-8444-555555555555",
			"room_turn_id": "66666666-7777-4888-8999-aaaaaaaaaaaa",
		},
	})
	runtime.mu.Lock()
	syncCalls := len(runtime.taskIDs)
	runtime.mu.Unlock()
	if syncCalls != 1 {
		t.Fatalf("Room synchronization calls = %d, want 1", syncCalls)
	}
	if len(broadcaster.workspaceCalls) != 0 || len(broadcaster.scopeCalls) != 0 || broadcaster.broadcastCalled != 0 {
		t.Fatalf("internal Room task event reached realtime broadcaster: %+v", broadcaster)
	}
}
