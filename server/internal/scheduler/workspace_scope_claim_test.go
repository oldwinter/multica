package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkspaceScopeClaimAfterCommittedDeleteDoesNotInsertExecution(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	workspaceID := createSchedulerClaimWorkspace(t, pool)
	job := newTestJobSpec(uniqueJobName(t, "workspace_gone"))
	t.Cleanup(func() { cleanupExecutions(t, pool, job.Name) })

	deleteTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin workspace delete: %v", err)
	}
	defer deleteTx.Rollback(ctx)
	if _, err := deleteTx.Exec(ctx, `SELECT id FROM workspace WHERE id = $1 FOR UPDATE`, workspaceID); err != nil {
		t.Fatalf("lock workspace for delete: %v", err)
	}
	if err := db.New(deleteTx).DeleteWorkspaceSkillEvolutionData(ctx, pgUUID(workspaceID)); err != nil {
		t.Fatalf("cleanup skill evolution data: %v", err)
	}
	if _, err := deleteTx.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, workspaceID); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if err := deleteTx.Commit(ctx); err != nil {
		t.Fatalf("commit workspace delete: %v", err)
	}

	now, err := dbNow(ctx, pool)
	if err != nil {
		t.Fatalf("dbNow: %v", err)
	}
	scope := Scope{Kind: ScopeKindWorkspace, ID: workspaceID.String()}
	claimed, err := tryClaim(ctx, pool, job, scope, FloorPlan(now, job.Cadence), now, "runner-after-delete")
	if err != nil {
		t.Fatalf("claim deleted workspace: %v", err)
	}
	if !claimed.Conflicted || claimed.Won || claimed.Stole {
		t.Fatalf("claim deleted workspace = %+v, want content-free conflict/no-op", claimed)
	}
	assertSchedulerExecutionCount(t, pool, job.Name, 0)
}

func TestWorkspaceScopeClaimLockSerializesDeleteAndCleanup(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	workspaceID := createSchedulerClaimWorkspace(t, pool)
	job := newTestJobSpec("skill_evolution")
	cleanupExecutions(t, pool, job.Name)
	t.Cleanup(func() {
		cleanupExecutions(t, pool, job.Name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})
	now, err := dbNow(ctx, pool)
	if err != nil {
		t.Fatalf("dbNow: %v", err)
	}
	scope := Scope{Kind: ScopeKindWorkspace, ID: workspaceID.String()}
	planTime := FloorPlan(now, job.Cadence)

	claimTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin workspace claim: %v", err)
	}
	defer claimTx.Rollback(ctx)
	exists, err := lockWorkspaceScopeForClaim(ctx, claimTx, workspaceID)
	if err != nil || !exists {
		t.Fatalf("lock workspace scope = (%v, %v), want true", exists, err)
	}

	deletePID := make(chan int32, 1)
	deleteDone := make(chan error, 1)
	go deleteWorkspaceAfterSchedulerCleanup(ctx, pool, workspaceID, deletePID, deleteDone)
	pid := <-deletePID
	waitForSchedulerLockWait(t, pool, pid, deleteDone)

	claimed, err := tryClaimOnDB(ctx, claimTx, job, scope, planTime, now, "runner-before-delete")
	if err != nil {
		t.Fatalf("insert claimed execution: %v", err)
	}
	if !claimed.Won {
		t.Fatalf("claim = %+v, want fresh win", claimed)
	}
	if err := claimTx.Commit(ctx); err != nil {
		t.Fatalf("commit claim: %v", err)
	}
	if err := <-deleteDone; err != nil {
		t.Fatalf("delete after claim: %v", err)
	}

	assertSchedulerExecutionCount(t, pool, job.Name, 0)
	var workspaceCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM workspace WHERE id = $1`, workspaceID).Scan(&workspaceCount); err != nil {
		t.Fatalf("count workspace: %v", err)
	}
	if workspaceCount != 0 {
		t.Fatalf("workspace rows = %d, want 0", workspaceCount)
	}
}

func TestNonWorkspaceScopeDoesNotParseOpaqueIDAsUUID(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()
	job := newTestJobSpec(uniqueJobName(t, "opaque_scope"))
	t.Cleanup(func() { cleanupExecutions(t, pool, job.Name) })
	now, err := dbNow(ctx, pool)
	if err != nil {
		t.Fatalf("dbNow: %v", err)
	}

	claimed, err := tryClaim(ctx, pool, job, Scope{Kind: "opaque", ID: "not-a-uuid"}, FloorPlan(now, job.Cadence), now, "opaque-runner")
	if err != nil {
		t.Fatalf("claim opaque scope: %v", err)
	}
	if !claimed.Won {
		t.Fatalf("opaque scope claim = %+v, want fresh win", claimed)
	}

	_, err = tryClaim(ctx, pool, job, Scope{Kind: ScopeKindWorkspace, ID: "not-a-uuid"}, FloorPlan(now.Add(job.Cadence), job.Cadence), now, "invalid-workspace-runner")
	if err == nil {
		t.Fatal("invalid workspace scope ID was accepted")
	}
	assertSchedulerExecutionCount(t, pool, job.Name, 1)
}

func createSchedulerClaimWorkspace(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	workspaceID := uuid.New()
	slug := "scheduler-claim-" + workspaceID.String()[:12]
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO workspace (id, name, slug)
		VALUES ($1, 'Scheduler claim race', $2)
	`, workspaceID, slug); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID) })
	return workspaceID
}

func deleteWorkspaceAfterSchedulerCleanup(
	ctx context.Context,
	pool *pgxpool.Pool,
	workspaceID uuid.UUID,
	pid chan<- int32,
	done chan<- error,
) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		pid <- 0
		done <- fmt.Errorf("begin delete: %w", err)
		return
	}
	defer tx.Rollback(ctx)
	var backendPID int32
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backendPID); err != nil {
		pid <- 0
		done <- fmt.Errorf("read delete pid: %w", err)
		return
	}
	pid <- backendPID
	if _, err := tx.Exec(ctx, `SELECT id FROM workspace WHERE id = $1 FOR UPDATE`, workspaceID); err != nil {
		done <- fmt.Errorf("lock workspace for delete: %w", err)
		return
	}
	if err := db.New(tx).DeleteWorkspaceSkillEvolutionData(ctx, pgUUID(workspaceID)); err != nil {
		done <- fmt.Errorf("cleanup skill evolution data: %w", err)
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, workspaceID); err != nil {
		done <- fmt.Errorf("delete workspace: %w", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		done <- fmt.Errorf("commit delete: %w", err)
		return
	}
	done <- nil
}

func waitForSchedulerLockWait(t *testing.T, pool *pgxpool.Pool, pid int32, done <-chan error) {
	t.Helper()
	if pid == 0 {
		t.Fatalf("delete setup failed before reporting backend pid: %v", <-done)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("workspace delete completed before claim released its lock: %v", err)
		default:
		}
		var waiting bool
		if err := pool.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE pid = $1 AND state = 'active' AND wait_event_type = 'Lock'
			)
		`, pid).Scan(&waiting); err != nil {
			t.Fatalf("observe workspace delete lock wait: %v", err)
		}
		if waiting {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("workspace delete backend %d never waited on scheduler claim lock", pid)
}

func assertSchedulerExecutionCount(t *testing.T, pool *pgxpool.Pool, jobName string, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sys_cron_executions WHERE job_name = $1`, jobName).Scan(&count); err != nil {
		t.Fatalf("count scheduler executions: %v", err)
	}
	if count != want {
		t.Fatalf("scheduler execution rows = %d, want %d", count, want)
	}
}

func pgUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}
