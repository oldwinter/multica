package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type taskRunReviewDBAccess struct {
	tasks map[string]TaskRunReviewTask
}

func (a taskRunReviewDBAccess) ValidateWorkspaceMember(context.Context, string, string) error {
	return nil
}

func (a taskRunReviewDBAccess) LoadAuthorizedTask(_ context.Context, workspaceID, _ string, taskID string) (TaskRunReviewTask, error) {
	task, ok := a.tasks[taskID]
	if !ok || task.WorkspaceID != workspaceID {
		return TaskRunReviewTask{}, ErrTaskRunReviewNotFound
	}
	return task, nil
}

func (a taskRunReviewDBAccess) ValidateTargetSkill(context.Context, string, string) error {
	return nil
}

type taskRunReviewDBScope struct {
	workspaceID  string
	reviewerID   string
	secondUserID string
	taskID       string
	secondTaskID string
	tasks        map[string]TaskRunReviewTask
}

func TestTaskRunReviewIdempotencyLivePostgres(t *testing.T) {
	pool := taskRunReviewDBPool(t)
	primary := seedTaskRunReviewDBScope(t, pool, "primary")
	other := seedTaskRunReviewDBScope(t, pool, "other")
	tasks := make(map[string]TaskRunReviewTask, len(primary.tasks)+len(other.tasks))
	for id, task := range primary.tasks {
		tasks[id] = task
	}
	for id, task := range other.tasks {
		tasks[id] = task
	}
	svc := NewTaskRunReviewService(NewDBTaskRunReviewRepository(db.New(pool)), taskRunReviewDBAccess{tasks: tasks})
	input := CreateTaskRunReviewInput{
		TaskID: primary.taskID, IdempotencyKey: "task-review:concurrent",
		Outcome: TaskRunReviewOutcomeNeedsCorrection, Target: TaskRunReviewTargetProductDefect,
		Correction: "Bound the retry.", Reason: "The task retried forever.",
	}

	const writers = 12
	start := make(chan struct{})
	results := make(chan TaskRunReviewEvidence, writers)
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := svc.CreateTaskRunReview(context.Background(), primary.workspaceID, primary.reviewerID, input)
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent create: %v", err)
		}
	}
	var first TaskRunReviewEvidence
	for result := range results {
		if first.ID == "" {
			first = result
			continue
		}
		if result.ID != first.ID || result.Digest != first.Digest || !result.CreatedAt.Equal(first.CreatedAt) {
			t.Fatalf("concurrent replay diverged: first=%#v result=%#v", first, result)
		}
	}
	if count := taskRunReviewDBCount(t, pool, primary.workspaceID, input.IdempotencyKey); count != 1 {
		t.Fatalf("concurrent writes persisted %d rows, want 1", count)
	}

	replayed, err := svc.CreateTaskRunReview(context.Background(), primary.workspaceID, primary.reviewerID, input)
	if err != nil || replayed.ID != first.ID || replayed.Digest != first.Digest {
		t.Fatalf("same replay = (%#v, %v), want original", replayed, err)
	}
	conflict := input
	conflict.Reason = "Different canonical payload."
	if _, err := svc.CreateTaskRunReview(context.Background(), primary.workspaceID, primary.reviewerID, conflict); !errors.Is(err, ErrTaskRunReviewSourceChanged) {
		t.Fatalf("conflicting replay error = %v, want source changed", err)
	}
	preserved, err := svc.LoadTaskRunReviewEvidence(context.Background(), primary.workspaceID, primary.reviewerID, first.ID)
	if err != nil {
		t.Fatalf("load original after conflicting replay: %v", err)
	}
	if preserved.Reason != input.Reason || preserved.Correction != input.Correction || preserved.Digest != first.Digest {
		t.Fatalf("conflicting replay overwrote original: %#v", preserved)
	}

	isolated := []struct {
		workspaceID string
		reviewerID  string
		taskID      string
	}{
		{primary.workspaceID, primary.reviewerID, primary.secondTaskID},
		{primary.workspaceID, primary.secondUserID, primary.taskID},
		{other.workspaceID, other.reviewerID, other.taskID},
	}
	seen := map[string]bool{first.ID: true}
	for _, scope := range isolated {
		scopedInput := input
		scopedInput.TaskID = scope.taskID
		created, err := svc.CreateTaskRunReview(context.Background(), scope.workspaceID, scope.reviewerID, scopedInput)
		if err != nil {
			t.Fatalf("isolated create %+v: %v", scope, err)
		}
		if seen[created.ID] {
			t.Fatalf("idempotency identity leaked across scope %+v: %s", scope, created.ID)
		}
		seen[created.ID] = true
	}
	if count := taskRunReviewDBCount(t, pool, primary.workspaceID, input.IdempotencyKey); count != 3 {
		t.Fatalf("primary scoped rows = %d, want 3", count)
	}
	if count := taskRunReviewDBCount(t, pool, other.workspaceID, input.IdempotencyKey); count != 1 {
		t.Fatalf("other workspace rows = %d, want 1", count)
	}

	workspaceUUID, err := util.ParseUUID(primary.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.New(pool).DeleteWorkspaceSkillEvolutionData(context.Background(), workspaceUUID); err != nil {
		t.Fatalf("cleanup task reviews: %v", err)
	}
	if count := taskRunReviewDBCount(t, pool, primary.workspaceID, input.IdempotencyKey); count != 0 {
		t.Fatalf("cleanup left %d primary task reviews", count)
	}
	if count := taskRunReviewDBCount(t, pool, other.workspaceID, input.IdempotencyKey); count != 1 {
		t.Fatalf("cleanup removed other workspace rows: %d", count)
	}
}

func taskRunReviewDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("database unreachable: %v", err)
	}
	var schemaReady bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'task_run_review'
			  AND column_name = 'idempotency_key'
		)
	`).Scan(&schemaReady); err != nil {
		pool.Close()
		t.Fatalf("check task review schema: %v", err)
	}
	if !schemaReady {
		pool.Close()
		t.Skip("task run review idempotency migrations are not applied")
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedTaskRunReviewDBScope(t *testing.T, pool *pgxpool.Pool, label string) taskRunReviewDBScope {
	t.Helper()
	suffix := fmt.Sprintf("%s-%d", label, time.Now().UnixNano())
	root := testutil.New(pool, "", "")
	reviewerID := root.User(t, "Task review "+label, suffix+"@multica.test")
	secondUserID := root.User(t, "Task review second "+label, "second-"+suffix+"@multica.test")
	workspaceID := root.Workspace(t, "Task review "+label, "task-review-"+suffix)
	fx := testutil.New(pool, workspaceID, reviewerID)
	fx.Member(t, workspaceID, reviewerID, "owner")
	fx.Member(t, workspaceID, secondUserID, "member")
	runtimeID := fx.Runtime(t, "Task review runtime "+label)
	agentID := fx.Agent(t, "Task review agent "+label, runtimeID)
	taskID := fx.Task(t, agentID, testutil.Cols{"status": "completed", "completed_at": testutil.Raw("now()")})
	secondTaskID := fx.Task(t, agentID, testutil.Cols{"status": "failed", "completed_at": testutil.Raw("now()")})
	tasks := map[string]TaskRunReviewTask{
		taskID:       {ID: taskID, WorkspaceID: workspaceID, AgentID: agentID, Status: "completed"},
		secondTaskID: {ID: secondTaskID, WorkspaceID: workspaceID, AgentID: agentID, Status: "failed"},
	}
	return taskRunReviewDBScope{
		workspaceID: workspaceID, reviewerID: reviewerID, secondUserID: secondUserID,
		taskID: taskID, secondTaskID: secondTaskID, tasks: tasks,
	}
}

func taskRunReviewDBCount(t *testing.T, pool *pgxpool.Pool, workspaceID, idempotencyKey string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM task_run_review
		WHERE workspace_id = $1 AND idempotency_key = $2
	`, workspaceID, idempotencyKey).Scan(&count); err != nil {
		t.Fatalf("count task reviews: %v", err)
	}
	return count
}
