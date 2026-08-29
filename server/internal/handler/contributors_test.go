package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type taskDispatchContributorStub struct {
	offer func(TaskDispatchEvent) bool
}

func (s taskDispatchContributorStub) OfferTaskDispatch(event TaskDispatchEvent) bool {
	return s.offer(event)
}

type taskCompletionContributorStub struct {
	offer func(TaskCompletionEvent) bool
}

func (s taskCompletionContributorStub) OfferTaskCompletion(event TaskCompletionEvent) bool {
	return s.offer(event)
}

type workspaceCleanupContributorStub struct {
	deleteWorkspace func(context.Context, *db.Queries, pgtype.UUID) error
}

func (s workspaceCleanupContributorStub) DeleteWorkspace(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) error {
	return s.deleteWorkspace(ctx, queries, workspaceID)
}

func TestTaskContributorsAreBestEffortAndIsolated(t *testing.T) {
	h := &Handler{}
	dispatch := TaskDispatchEvent{
		WorkspaceID: "workspace-1",
		TaskID:      "task-1",
		RuntimeID:   "runtime-1",
		AgentID:     "agent-1",
		Skills: []TaskDispatchSkill{{
			Source: "workspace", SkillID: "skill-1",
		}},
	}
	completion := TaskCompletionEvent{
		WorkspaceID:            "workspace-1",
		TaskID:                 "task-1",
		RuntimeID:              "runtime-1",
		AgentID:                "agent-1",
		SkillExecutionManifest: json.RawMessage(`{"version":1}`),
	}

	h.RegisterTaskDispatchContributor(nil)
	h.RegisterTaskCompletionContributor(nil)
	h.RegisterTaskDispatchContributor(taskDispatchContributorStub{offer: func(got TaskDispatchEvent) bool {
		got.Skills[0].SkillID = "corrupted"
		panic("claim failure")
	}})
	h.RegisterTaskCompletionContributor(taskCompletionContributorStub{offer: func(got TaskCompletionEvent) bool {
		got.SkillExecutionManifest[0] = 'x'
		panic("completion failure")
	}})
	var gotDispatch TaskDispatchEvent
	var gotCompletion TaskCompletionEvent
	var finalDispatchOffered bool
	var finalCompletionOffered bool
	h.RegisterTaskDispatchContributor(taskDispatchContributorStub{offer: func(got TaskDispatchEvent) bool {
		gotDispatch = got
		return false
	}})
	h.RegisterTaskCompletionContributor(taskCompletionContributorStub{offer: func(got TaskCompletionEvent) bool {
		gotCompletion = got
		return false
	}})
	h.RegisterTaskDispatchContributor(taskDispatchContributorStub{offer: func(TaskDispatchEvent) bool {
		finalDispatchOffered = true
		return true
	}})
	h.RegisterTaskCompletionContributor(taskCompletionContributorStub{offer: func(TaskCompletionEvent) bool {
		finalCompletionOffered = true
		return true
	}})

	h.offerTaskDispatch(dispatch)
	h.offerTaskCompletion(completion)

	if gotDispatch.TaskID != dispatch.TaskID || len(gotDispatch.Skills) != 1 || gotDispatch.Skills[0].SkillID != "skill-1" {
		t.Fatalf("second contributor observed mutated dispatch: %+v", gotDispatch)
	}
	if !bytes.Equal(gotCompletion.SkillExecutionManifest, completion.SkillExecutionManifest) {
		t.Fatalf("second contributor observed mutated completion manifest: %q", gotCompletion.SkillExecutionManifest)
	}
	if !finalDispatchOffered || !finalCompletionOffered {
		t.Fatal("queue refusal stopped a later task contributor")
	}
	if dispatch.Skills[0].SkillID != "skill-1" || completion.SkillExecutionManifest[0] != '{' {
		t.Fatal("notification mutated caller-owned task data")
	}
}

func TestTaskDispatchEventFromResponseOnlyUsesDeliveredSkillRefs(t *testing.T) {
	dispatchedAt := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	resp := AgentTaskResponse{
		WorkspaceID: "workspace-1",
		Agent: &TaskAgentData{
			ID:        "agent-1",
			Skills:    []service.AgentSkillData{{ID: "full", Source: "workspace", Hash: "sha256:full"}},
			SkillRefs: []service.AgentSkillRefData{{ID: "ref", Source: "plugin", Hash: "sha256:ref"}},
		},
	}

	event, ok := taskDispatchEventFromResponse("task-1", "runtime-1", pgtype.Timestamptz{Time: dispatchedAt, Valid: true}, resp)
	if !ok {
		t.Fatal("skill-ref dispatch was not offered")
	}
	if event.WorkspaceID != "workspace-1" || event.TaskID != "task-1" || event.RuntimeID != "runtime-1" || event.AgentID != "agent-1" || !event.DispatchedAt.Equal(dispatchedAt) {
		t.Fatalf("dispatch identity = %+v", event)
	}
	if len(event.Skills) != 1 || event.Skills[0] != (TaskDispatchSkill{Source: "plugin", SkillID: "ref"}) {
		t.Fatalf("dispatch skills = %+v, want delivered refs", event.Skills)
	}

	legacy, ok := taskDispatchEventFromResponse("task-2", "runtime-2", pgtype.Timestamptz{}, AgentTaskResponse{
		WorkspaceID: "workspace-2",
		Agent:       &TaskAgentData{Skills: []service.AgentSkillData{{ID: "legacy"}}},
	})
	if ok || len(legacy.Skills) != 0 || !legacy.DispatchedAt.IsZero() {
		t.Fatalf("legacy full-bundle dispatch offered: %+v, ok=%v", legacy, ok)
	}
}

func TestTaskResponsePreservesDispatchGenerationPrecision(t *testing.T) {
	dispatchedAt := time.Date(2026, time.August, 29, 12, 34, 56, 123456000, time.UTC)
	response := taskToResponse(db.AgentTaskQueue{
		ID:           parseUUID("00000000-0000-0000-0000-000000000111"),
		AgentID:      parseUUID("00000000-0000-0000-0000-000000000112"),
		RuntimeID:    parseUUID("00000000-0000-0000-0000-000000000113"),
		IssueID:      parseUUID("00000000-0000-0000-0000-000000000114"),
		DispatchedAt: pgtype.Timestamptz{Time: dispatchedAt, Valid: true},
	}, "workspace-1")
	if response.DispatchedAt == nil || *response.DispatchedAt != "2026-08-29T12:34:56.123456Z" {
		t.Fatalf("dispatch generation = %v, want full PostgreSQL precision", response.DispatchedAt)
	}
}

func TestTaskCompletionEventUsesAuthoritativeCommittedResult(t *testing.T) {
	dispatchedAt := time.Date(2026, time.August, 29, 12, 34, 56, 123456000, time.UTC)
	task := committedCompletionTask(t, dispatchedAt, `{
  "task_dispatched_at":"2026-08-29T12:34:56.123456Z",
  "skill_execution_manifest":{"version":1,"skills":[{"source":"workspace","skill_id":"winner","bundle_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}],"future_field":true}
}`)

	event, ok := taskCompletionEventFromCommittedTask("workspace-1", task, true)
	if !ok {
		t.Fatal("committed completion did not produce an attribution event")
	}
	if event.TaskID != uuidToString(task.ID) || event.DispatchedAt != dispatchedAt {
		t.Fatalf("event identity = %+v", event)
	}
	if strings.Contains(string(event.SkillExecutionManifest), "future_field") || !strings.Contains(string(event.SkillExecutionManifest), `"skill_id":"winner"`) {
		t.Fatalf("event did not use normalized authoritative manifest: %s", event.SkillExecutionManifest)
	}
	// A concurrent/replayed request can carry a different manifest, but the
	// extractor has no request payload input: only the CAS winner in task.Result
	// can reach contributors.
	if strings.Contains(string(event.SkillExecutionManifest), "loser") {
		t.Fatalf("event contains replay manifest: %s", event.SkillExecutionManifest)
	}
}

func TestTaskCompletionEventRejectsForgedTerminalReplay(t *testing.T) {
	dispatchedAt := time.Date(2026, time.August, 29, 13, 0, 0, 0, time.UTC)
	task := committedCompletionTask(t, dispatchedAt, `{"task_dispatched_at":"2026-08-29T13:00:00Z","output":"completed without manifest"}`)
	if _, ok := taskCompletionEventFromCommittedTask("workspace-1", task, true); ok {
		t.Fatal("a replay cannot add a manifest absent from the committed result")
	}

	task.Result = []byte(`{"task_dispatched_at":"2026-08-29T13:00:00Z","skill_execution_manifest":{"version":1,"skills":[{"source":"workspace","skill_id":"skill-1","bundle_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}}`)
	for _, status := range []string{"failed", "cancelled"} {
		task.Status = status
		if _, ok := taskCompletionEventFromCommittedTask("workspace-1", task, true); ok {
			t.Fatalf("terminal status %q produced attribution", status)
		}
	}
}

func TestTaskCompletionEventRejectsMissingOrStaleDispatchGeneration(t *testing.T) {
	dispatchedAt := time.Date(2026, time.August, 29, 13, 0, 0, 500000000, time.UTC)
	manifest := `"skill_execution_manifest":{"version":1,"skills":[{"source":"workspace","skill_id":"skill-1","bundle_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`
	for _, tc := range []struct {
		name   string
		result string
	}{
		{name: "old daemon omitted generation", result: `{` + manifest + `}`},
		{name: "old claim after reclaim", result: `{"task_dispatched_at":"2026-08-29T13:00:00Z",` + manifest + `}`},
		{name: "invalid generation", result: `{"task_dispatched_at":"not-a-time",` + manifest + `}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := committedCompletionTask(t, dispatchedAt, tc.result)
			if _, ok := taskCompletionEventFromCommittedTask("workspace-1", task, true); ok {
				t.Fatal("missing or stale claim generation produced attribution")
			}
		})
	}
	task := committedCompletionTask(t, dispatchedAt, `{"task_dispatched_at":"2026-08-29T13:00:00.5Z",`+manifest+`}`)
	if _, ok := taskCompletionEventFromCommittedTask("workspace-1", task, false); ok {
		t.Fatal("request without skill-execution-manifest-v1 capability produced attribution")
	}
}

func committedCompletionTask(t *testing.T, dispatchedAt time.Time, result string) *db.AgentTaskQueue {
	t.Helper()
	return &db.AgentTaskQueue{
		ID:           parseUUID("00000000-0000-0000-0000-000000000101"),
		RuntimeID:    parseUUID("00000000-0000-0000-0000-000000000102"),
		AgentID:      parseUUID("00000000-0000-0000-0000-000000000103"),
		Status:       "completed",
		DispatchedAt: pgtype.Timestamptz{Time: dispatchedAt, Valid: true},
		Result:       []byte(result),
	}
}

func TestWorkspaceCleanupContributorsFailClosed(t *testing.T) {
	h := &Handler{}
	wantErr := errors.New("cleanup failed")
	var calls []string
	h.RegisterWorkspaceCleanupContributor(nil)
	h.RegisterWorkspaceCleanupContributor(workspaceCleanupContributorStub{deleteWorkspace: func(context.Context, *db.Queries, pgtype.UUID) error {
		calls = append(calls, "first")
		return nil
	}})
	h.RegisterWorkspaceCleanupContributor(workspaceCleanupContributorStub{deleteWorkspace: func(context.Context, *db.Queries, pgtype.UUID) error {
		calls = append(calls, "second")
		return wantErr
	}})
	h.RegisterWorkspaceCleanupContributor(workspaceCleanupContributorStub{deleteWorkspace: func(context.Context, *db.Queries, pgtype.UUID) error {
		calls = append(calls, "third")
		return nil
	}})

	err := h.deleteWorkspaceContributorData(context.Background(), nil, pgtype.UUID{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("cleanup error = %v, want %v", err, wantErr)
	}
	if strings.Join(calls, ",") != "first,second" {
		t.Fatalf("cleanup calls = %v", calls)
	}
}

func TestWorkspaceCleanupContributorPanicFailsClosed(t *testing.T) {
	h := &Handler{}
	h.RegisterWorkspaceCleanupContributor(workspaceCleanupContributorStub{deleteWorkspace: func(context.Context, *db.Queries, pgtype.UUID) error {
		panic("cleanup panic")
	}})

	err := h.deleteWorkspaceContributorData(context.Background(), nil, pgtype.UUID{})
	if err == nil || !strings.Contains(err.Error(), "cleanup panic") {
		t.Fatalf("cleanup panic error = %v", err)
	}
}

func TestNeutralPackagesAvoidDirectSkillEvolutionImports(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	internalDir := filepath.Dir(filepath.Dir(currentFile))
	for _, dir := range []string{filepath.Join(internalDir, "handler"), filepath.Join(internalDir, "service")} {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if bytes.Contains(body, []byte("internal/skillevolution")) {
				t.Errorf("neutral source package imports skill-evolution leaf: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}
}
