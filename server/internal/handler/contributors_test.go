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

func TestHandlerAndTaskServiceDoNotImportSkillEvolutionLeaf(t *testing.T) {
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
