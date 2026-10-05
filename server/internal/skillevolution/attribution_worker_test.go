package skillevolution

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/handler"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestAttributionWorkerPersistsClaimBeforeAttributingCompletion(t *testing.T) {
	store := newFakeAttributionWorkerStore()
	worker, err := newAttributionWorker(store, 4, time.Second)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	workspaceID, taskID, agentID, runtimeID := attributionTestUUID(), attributionTestUUID(), attributionTestUUID(), attributionTestUUID()
	skillID, revisionID := attributionTestUUID(), attributionTestUUID()
	dispatchedAt := time.Date(2026, time.August, 28, 13, 0, 0, 0, time.UTC)
	executedHash := attributionTestDigest('b')
	store.revisions = []attributionRevision{{
		ID: revisionID, WorkspaceID: workspaceID, SkillID: skillID,
		Source: skillbundle.SourceWorkspace, BundleHash: executedHash, OwnershipClass: OwnershipWorkspace,
	}}
	if !worker.OfferTaskDispatch(handler.TaskDispatchEvent{
		WorkspaceID:  attributionUUIDString(workspaceID),
		TaskID:       attributionUUIDString(taskID),
		AgentID:      attributionUUIDString(agentID),
		RuntimeID:    attributionUUIDString(runtimeID),
		DispatchedAt: dispatchedAt,
		Skills: []handler.TaskDispatchSkill{{
			Source: skillbundle.SourceWorkspace, SkillID: attributionUUIDString(skillID),
		}},
	}) {
		t.Fatal("dispatch was not accepted")
	}
	if !worker.OfferTaskCompletion(handler.TaskCompletionEvent{
		WorkspaceID:      attributionUUIDString(workspaceID),
		TaskID:           attributionUUIDString(taskID),
		AgentID:          attributionUUIDString(agentID),
		RuntimeID:        attributionUUIDString(runtimeID),
		DispatchedAt:     dispatchedAt,
		CapabilityProven: true,
		SkillExecutionManifest: executionManifestJSON(t, skillbundle.ExecutionRecord{
			Source: skillbundle.SourceWorkspace, SkillID: attributionUUIDString(skillID),
			BundleHash: string(executedHash), RevisionID: attributionUUIDString(revisionID),
		}),
	}) {
		t.Fatal("completion was not accepted")
	}
	worker.Close()
	worker.Close()

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.snapshotWrites != 1 || store.snapshotReads != 1 || store.recordCalls != 1 || len(store.rows) != 1 {
		t.Fatalf("writes/reads/records/rows = %d/%d/%d/%d, want 1/1/1/1", store.snapshotWrites, store.snapshotReads, store.recordCalls, len(store.rows))
	}
	row := store.rows[0]
	if row.DispatchSnapshotID != store.snapshot.Snapshot.ID || !row.TaskDispatchedAt.Equal(dispatchedAt) || row.BundleHash != executedHash {
		t.Fatalf("attribution row = %+v, want snapshot-bound execution hash", row)
	}
}

func TestAttributionWorkerCopiesInputsAndAcceptsNonWorkspaceManifest(t *testing.T) {
	store := newFakeAttributionWorkerStore()
	worker, err := newAttributionWorker(store, 4, time.Second)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	workspaceID, taskID, agentID, runtimeID := attributionTestUUID(), attributionTestUUID(), attributionTestUUID(), attributionTestUUID()
	dispatchedAt := time.Date(2026, time.August, 28, 14, 0, 0, 0, time.UTC)
	hash := attributionTestDigest('c')
	claim := handler.TaskDispatchEvent{
		WorkspaceID: attributionUUIDString(workspaceID), TaskID: attributionUUIDString(taskID),
		AgentID: attributionUUIDString(agentID), RuntimeID: attributionUUIDString(runtimeID), DispatchedAt: dispatchedAt,
		Skills: []handler.TaskDispatchSkill{{Source: skillbundle.SourceBuiltin, SkillID: "plan"}},
	}
	manifest := executionManifestJSON(t, skillbundle.ExecutionRecord{Source: skillbundle.SourceBuiltin, SkillID: "plan", BundleHash: string(hash)})
	completion := handler.TaskCompletionEvent{
		WorkspaceID: claim.WorkspaceID, TaskID: claim.TaskID, AgentID: claim.AgentID, RuntimeID: claim.RuntimeID,
		DispatchedAt: dispatchedAt, CapabilityProven: true, SkillExecutionManifest: manifest,
	}
	if !worker.OfferTaskDispatch(claim) || !worker.OfferTaskCompletion(completion) {
		t.Fatal("worker unexpectedly rejected test events")
	}
	claim.Skills[0].SkillID = "mutated"
	completion.SkillExecutionManifest[0] = 'x'
	worker.Close()

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.snapshotWrites != 1 || store.snapshotReads != 1 || store.recordCalls != 0 || len(store.rows) != 0 {
		t.Fatalf("writes/reads/records/rows = %d/%d/%d/%d, want 1/1/0/0", store.snapshotWrites, store.snapshotReads, store.recordCalls, len(store.rows))
	}
	if len(store.snapshot.Skills) != 1 || store.snapshot.Skills[0].SkillID != "plan" {
		t.Fatalf("snapshot skills = %+v, want copied claim", store.snapshot.Skills)
	}
}

func TestAttributionWorkerNotificationsRemainNonblockingWhenFull(t *testing.T) {
	store := newFakeAttributionWorkerStore()
	store.waitForContext = true
	worker, err := newAttributionWorker(store, 1, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	claim := handler.TaskDispatchEvent{
		WorkspaceID:  attributionUUIDString(attributionTestUUID()),
		TaskID:       attributionUUIDString(attributionTestUUID()),
		AgentID:      attributionUUIDString(attributionTestUUID()),
		RuntimeID:    attributionUUIDString(attributionTestUUID()),
		DispatchedAt: time.Now().UTC(),
		Skills:       []handler.TaskDispatchSkill{{Source: skillbundle.SourceBuiltin, SkillID: "plan"}},
	}

	started := time.Now()
	accepted := 0
	for range 1000 {
		if worker.OfferTaskDispatch(claim) {
			accepted++
		}
	}
	elapsed := time.Since(started)
	worker.Close()
	if elapsed > 100*time.Millisecond {
		t.Fatalf("notifications took %s, want nonblocking enqueue/drop", elapsed)
	}
	if accepted == 1000 {
		t.Fatal("bounded queue accepted every event while its worker was blocked")
	}
	if worker.OfferTaskDispatch(claim) {
		t.Fatal("closed worker accepted a dispatch")
	}
}

func TestAttributionWorkerIgnoresCompletionWithoutAllowlistedManifestCapability(t *testing.T) {
	store := newFakeAttributionWorkerStore()
	worker, err := newAttributionWorker(store, 4, time.Second)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	event := handler.TaskCompletionEvent{
		WorkspaceID:  attributionUUIDString(attributionTestUUID()),
		TaskID:       attributionUUIDString(attributionTestUUID()),
		AgentID:      attributionUUIDString(attributionTestUUID()),
		RuntimeID:    attributionUUIDString(attributionTestUUID()),
		DispatchedAt: time.Now().UTC(),
	}
	if !worker.OfferTaskCompletion(event) {
		t.Fatal("worker unexpectedly rejected completion")
	}
	event.CapabilityProven = true
	event.SkillExecutionManifest = json.RawMessage(`{"version":1,"skills":[]}`)
	if !worker.OfferTaskCompletion(event) {
		t.Fatal("worker unexpectedly rejected malformed completion notification")
	}
	worker.Close()

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.snapshotReads != 0 || store.recordCalls != 0 {
		t.Fatalf("snapshot reads/records = %d/%d, want no attribution without valid allowlisted manifest", store.snapshotReads, store.recordCalls)
	}
}

func TestNewAttributionWorkerRejectsInvalidConfiguration(t *testing.T) {
	store := newFakeAttributionWorkerStore()
	for _, tc := range []struct {
		name     string
		store    attributionWorkerStore
		capacity int
		timeout  time.Duration
	}{
		{name: "nil store", capacity: 1, timeout: time.Second},
		{name: "zero capacity", store: store, timeout: time.Second},
		{name: "zero timeout", store: store, capacity: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if worker, err := newAttributionWorker(tc.store, tc.capacity, tc.timeout); err == nil || worker != nil {
				t.Fatalf("worker/error = %v/%v, want invalid configuration", worker, err)
			}
		})
	}
}

type fakeAttributionWorkerStore struct {
	mu             sync.Mutex
	snapshot       TaskDispatchSnapshot
	snapshotWrites int
	snapshotReads  int
	revisions      []attributionRevision
	rows           []TaskAttributionInput
	recordCalls    int
	waitForContext bool
}

func newFakeAttributionWorkerStore() *fakeAttributionWorkerStore {
	return &fakeAttributionWorkerStore{}
}

func (f *fakeAttributionWorkerStore) RecordTaskDispatchSnapshot(ctx context.Context, input TaskDispatchSnapshotInput) (TaskDispatchSnapshot, error) {
	if f.waitForContext {
		<-ctx.Done()
		return TaskDispatchSnapshot{}, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshotWrites++
	f.snapshot = TaskDispatchSnapshot{
		Snapshot: db.SkillEvolutionTaskDispatchSnapshot{
			ID: attributionTestUUID(), WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, AgentID: input.AgentID, RuntimeID: input.RuntimeID,
			TaskDispatchedAt: pgtype.Timestamptz{Time: input.TaskDispatchedAt, Valid: true},
		},
		Skills: append([]DispatchSkillIdentity(nil), input.Skills...),
	}
	return f.snapshot, nil
}

func (f *fakeAttributionWorkerStore) GetTaskDispatchSnapshot(_ context.Context, workspaceID, taskID, agentID, runtimeID pgtype.UUID, dispatchedAt time.Time) (TaskDispatchSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshotReads++
	if f.snapshot.Snapshot.WorkspaceID != workspaceID || f.snapshot.Snapshot.TaskID != taskID || f.snapshot.Snapshot.AgentID != agentID ||
		f.snapshot.Snapshot.RuntimeID != runtimeID || !f.snapshot.Snapshot.TaskDispatchedAt.Time.Equal(dispatchedAt) {
		return TaskDispatchSnapshot{}, ErrPersistenceNotFound
	}
	return f.snapshot, nil
}

func (f *fakeAttributionWorkerStore) resolveAttributionRevisions(_ context.Context, match attributionRevisionMatch) ([]attributionRevision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]attributionRevision, 0, len(f.revisions))
	for _, revision := range f.revisions {
		if revision.WorkspaceID == match.WorkspaceID && revision.SkillID == match.SkillID &&
			revision.Source == match.Source && revision.BundleHash == match.BundleHash &&
			(!match.RevisionID.Valid || revision.ID == match.RevisionID) {
			result = append(result, revision)
		}
	}
	return result, nil
}

func (f *fakeAttributionWorkerStore) recordAttributionBatch(_ context.Context, inputs []TaskAttributionInput) (AttributionBatchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCalls++
	f.rows = append(f.rows, inputs...)
	return AttributionBatchResult{Inserted: true}, nil
}

func executionManifestJSON(t *testing.T, records ...skillbundle.ExecutionRecord) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(skillbundle.ExecutionManifest{Version: skillbundle.ExecutionManifestVersion, Skills: records})
	if err != nil {
		t.Fatalf("marshal execution manifest: %v", err)
	}
	return raw
}
