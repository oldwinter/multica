package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

var ErrInvalidAttributionWorker = errors.New("invalid attribution worker configuration")

var (
	_ handler.TaskDispatchContributor   = (*AttributionWorker)(nil)
	_ handler.TaskCompletionContributor = (*AttributionWorker)(nil)
)

type attributionWorkerStore interface {
	attributionRepository
	RecordTaskDispatchSnapshot(context.Context, TaskDispatchSnapshotInput) (TaskDispatchSnapshot, error)
	GetTaskDispatchSnapshot(context.Context, pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, time.Time) (TaskDispatchSnapshot, error)
}

type attributionWorkerEvent struct {
	dispatch   *handler.TaskDispatchEvent
	completion *handler.TaskCompletionEvent
}

// AttributionWorker contains optional attribution I/O behind a bounded FIFO.
// Full or closed workers drop notifications so task claim and completion never
// wait on the evolution subsystem.
type AttributionWorker struct {
	store    attributionWorkerStore
	recorder *AttributionRecorder
	timeout  time.Duration
	queue    chan attributionWorkerEvent
	stop     chan struct{}
	done     chan struct{}
	closed   bool
	close    sync.Once
	offers   sync.RWMutex
}

func NewAttributionWorker(store *Store, capacity int, timeout time.Duration) (*AttributionWorker, error) {
	return newAttributionWorker(store, capacity, timeout)
}

func newAttributionWorker(store attributionWorkerStore, capacity int, timeout time.Duration) (*AttributionWorker, error) {
	if store == nil || capacity <= 0 || timeout <= 0 {
		return nil, ErrInvalidAttributionWorker
	}
	worker := &AttributionWorker{
		store:    store,
		recorder: newAttributionRecorder(store),
		timeout:  timeout,
		queue:    make(chan attributionWorkerEvent, capacity),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go worker.run()
	return worker, nil
}

func (w *AttributionWorker) OfferTaskDispatch(event handler.TaskDispatchEvent) bool {
	if w == nil || !w.offers.TryRLock() {
		return false
	}
	defer w.offers.RUnlock()
	if w.closed {
		return false
	}
	copy := cloneAttributionDispatch(event)
	select {
	case w.queue <- attributionWorkerEvent{dispatch: &copy}:
		return true
	default:
		return false
	}
}

func (w *AttributionWorker) OfferTaskCompletion(completion handler.TaskCompletionEvent) bool {
	if w == nil || !w.offers.TryRLock() {
		return false
	}
	defer w.offers.RUnlock()
	if w.closed {
		return false
	}
	copy := cloneAttributionCompletion(completion)
	select {
	case w.queue <- attributionWorkerEvent{completion: &copy}:
		return true
	default:
		return false
	}
}

func (w *AttributionWorker) Close() {
	if w == nil {
		return
	}
	w.close.Do(func() {
		w.offers.Lock()
		w.closed = true
		close(w.stop)
		w.offers.Unlock()
		<-w.done
	})
}

func (w *AttributionWorker) run() {
	defer close(w.done)
	for {
		select {
		case event := <-w.queue:
			w.process(event)
		case <-w.stop:
			for {
				select {
				case event := <-w.queue:
					w.process(event)
				default:
					return
				}
			}
		}
	}
}

func (w *AttributionWorker) process(event attributionWorkerEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	defer cancel()
	if event.dispatch != nil {
		w.processDispatch(ctx, *event.dispatch)
		return
	}
	if event.completion != nil {
		w.processCompletion(ctx, *event.completion)
	}
}

func (w *AttributionWorker) processDispatch(ctx context.Context, event handler.TaskDispatchEvent) {
	workspaceID, taskID, agentID, runtimeID, ok := parseExecutionIdentity(event.WorkspaceID, event.TaskID, event.AgentID, event.RuntimeID)
	if !ok || event.DispatchedAt.IsZero() || len(event.Skills) == 0 {
		return
	}
	skills := make([]DispatchSkillIdentity, len(event.Skills))
	for i, skill := range event.Skills {
		skills[i] = DispatchSkillIdentity{Source: skill.Source, SkillID: skill.SkillID}
	}
	_, _ = w.store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
		WorkspaceID:      workspaceID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeID:        runtimeID,
		TaskDispatchedAt: event.DispatchedAt,
		Skills:           skills,
	})
}

func (w *AttributionWorker) processCompletion(ctx context.Context, completion handler.TaskCompletionEvent) {
	if !completion.CapabilityProven || completion.DispatchedAt.IsZero() {
		return
	}
	workspaceID, taskID, agentID, runtimeID, ok := parseExecutionIdentity(completion.WorkspaceID, completion.TaskID, completion.AgentID, completion.RuntimeID)
	if !ok {
		return
	}
	manifest, err := skillbundle.NormalizeExecutionManifest(completion.SkillExecutionManifest)
	if err != nil {
		return
	}
	snapshot, err := w.store.GetTaskDispatchSnapshot(ctx, workspaceID, taskID, agentID, runtimeID, completion.DispatchedAt)
	if err != nil || !validUUID(snapshot.Snapshot.ID) || !snapshot.Snapshot.TaskDispatchedAt.Valid {
		return
	}
	w.recorder.Record(ctx, AttributionInput{
		WorkspaceID:        workspaceID,
		TaskID:             taskID,
		RuntimeID:          runtimeID,
		DispatchSnapshotID: snapshot.Snapshot.ID,
		TaskDispatchedAt:   snapshot.Snapshot.TaskDispatchedAt.Time,
		CapabilityProven:   true,
		DispatchedSkills:   append([]DispatchSkillIdentity(nil), snapshot.Skills...),
		Manifest:           manifest,
	})
}

func parseExecutionIdentity(workspace, task, agent, runtime string) (pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, bool) {
	workspaceID, err := parseUUID(workspace)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, false
	}
	taskID, err := parseUUID(task)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, false
	}
	agentID, err := parseUUID(agent)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, false
	}
	runtimeID, err := parseUUID(runtime)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, false
	}
	return workspaceID, taskID, agentID, runtimeID, true
}

func cloneAttributionDispatch(event handler.TaskDispatchEvent) handler.TaskDispatchEvent {
	event.Skills = append([]handler.TaskDispatchSkill(nil), event.Skills...)
	return event
}

func cloneAttributionCompletion(completion handler.TaskCompletionEvent) handler.TaskCompletionEvent {
	completion.SkillExecutionManifest = append(json.RawMessage(nil), completion.SkillExecutionManifest...)
	return completion
}
