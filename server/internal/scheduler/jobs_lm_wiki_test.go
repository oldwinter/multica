package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type wikiRefreshCall struct {
	workspaceID pgtype.UUID
	trigger     string
	requestedBy pgtype.UUID
	planKey     string
}

type recordingWikiRefresher struct {
	mu            sync.Mutex
	calls         []wikiRefreshCall
	failuresLeft  int
	createdResult bool
}

func (r *recordingWikiRefresher) Refresh(
	_ context.Context,
	workspaceID pgtype.UUID,
	trigger string,
	requestedBy pgtype.UUID,
	planKey string,
) (service.LMWikiRefreshResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, wikiRefreshCall{workspaceID, trigger, requestedBy, planKey})
	if r.failuresLeft > 0 {
		r.failuresLeft--
		return service.LMWikiRefreshResult{}, errors.New("transient wiki refresh failure")
	}
	return service.LMWikiRefreshResult{Created: r.createdResult}, nil
}

func (r *recordingWikiRefresher) snapshotCalls() []wikiRefreshCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]wikiRefreshCall(nil), r.calls...)
}

func TestLMWikiDailySpec(t *testing.T) {
	job := LMWikiDailyReconcileJob(db.New(nil), &recordingWikiRefresher{})
	if job.Name != JobNameLMWikiDailyReconcile || JobNameLMWikiDailyReconcile != "lm_wiki_daily_reconcile" {
		t.Fatalf("job name = %q", job.Name)
	}
	if job.Cadence != 24*time.Hour || job.CatchUpMode != CatchUpLatestOnly || job.CatchUpWindow != 48*time.Hour {
		t.Fatalf("planning spec = cadence %s mode %s window %s", job.Cadence, job.CatchUpMode, job.CatchUpWindow)
	}
	if job.RunTimeout != 2*time.Minute || job.StaleTimeout != 5*time.Minute || job.HeartbeatInterval != 30*time.Second {
		t.Fatalf("lease timing = run %s stale %s heartbeat %s", job.RunTimeout, job.StaleTimeout, job.HeartbeatInterval)
	}
	if !job.AllowStaleReentry || job.MaxAttempts != 3 {
		t.Fatalf("retry policy = stale_reentry %t max_attempts %d", job.AllowStaleReentry, job.MaxAttempts)
	}
	wantBackoff := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}
	if fmt.Sprint(job.RetryBackoff) != fmt.Sprint(wantBackoff) {
		t.Fatalf("retry backoff = %v, want %v", job.RetryBackoff, wantBackoff)
	}
	mgr := NewManager(nil, Options{RunnerID: "spec-validator"})
	if err := mgr.Register(job); err != nil {
		t.Fatalf("register exact spec: %v", err)
	}
}

func TestLMWikiDailyUsesSharedService(t *testing.T) {
	pool := integrationPool(t)
	workspaceID := seedLMWikiSchedulerWorkspace(t, pool)
	queries := db.New(pool)
	job := LMWikiDailyReconcileJob(queries, service.NewWikiService(queries, pool))

	scopes, err := job.Scopes(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("enumerate DB workspaces: %v", err)
	}
	if !containsWorkspaceScope(scopes, workspaceID) {
		t.Fatalf("DB scope enumeration omitted workspace %s", util.UUIDToString(workspaceID))
	}

	planTime := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	in := HandlerInput{Job: &job, Scope: Scope{Kind: ScopeKindWorkspace, ID: util.UUIDToString(workspaceID)}, PlanTime: planTime}
	first, err := job.Handler(context.Background(), in)
	if err != nil {
		t.Fatalf("first scheduled refresh: %v", err)
	}
	second, err := job.Handler(context.Background(), in)
	if err != nil {
		t.Fatalf("unchanged scheduled refresh: %v", err)
	}
	if first.RowsAffected != 1 || second.RowsAffected != 0 {
		t.Fatalf("rows affected = first %d second %d", first.RowsAffected, second.RowsAffected)
	}

	var revisions, pending int
	var scheduled, systemActor bool
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE review.id IS NULL),
		       BOOL_AND(revision.trigger_kind = 'scheduled'),
		       BOOL_AND(revision.requested_by_id IS NULL)
		FROM lm_wiki_revision revision
		LEFT JOIN lm_wiki_review review
		  ON review.workspace_id = revision.workspace_id AND review.revision_id = revision.id
		WHERE revision.workspace_id = $1
	`, workspaceID).Scan(&revisions, &pending, &scheduled, &systemActor); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if revisions != 1 || pending != 1 || !scheduled || !systemActor {
		t.Fatalf("scheduled refresh revisions=%d pending=%d scheduled=%t system_actor=%t", revisions, pending, scheduled, systemActor)
	}
}

func TestLMWikiDailyDeleted(t *testing.T) {
	pool := integrationPool(t)
	workspaceID := seedLMWikiSchedulerWorkspace(t, pool)
	if _, err := pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID); err != nil {
		t.Fatalf("delete workspace before handler: %v", err)
	}
	queries := db.New(pool)
	job := LMWikiDailyReconcileJob(queries, service.NewWikiService(queries, pool))
	result, err := job.Handler(context.Background(), HandlerInput{
		Job: &job, Scope: Scope{Kind: ScopeKindWorkspace, ID: util.UUIDToString(workspaceID)}, PlanTime: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("deleted workspace should be successful no-op: %v", err)
	}
	if result.RowsAffected != 0 || result.Result["skipped_reason"] != "workspace_deleted" {
		t.Fatalf("deleted result = %+v", result)
	}
}

func TestLMWikiDailyConcurrent(t *testing.T) {
	pool := integrationPool(t)
	workspaceID := seedLMWikiSchedulerWorkspace(t, pool)
	queries := db.New(pool)
	job := LMWikiDailyReconcileJob(queries, service.NewWikiService(queries, pool))
	job.Scopes = StaticScopes(Scope{Kind: ScopeKindWorkspace, ID: util.UUIDToString(workspaceID)})

	mgrA := NewManager(pool, Options{RunnerID: "wiki-runner-A"})
	mgrB := NewManager(pool, Options{RunnerID: "wiki-runner-B"})
	if err := mgrA.Register(job); err != nil {
		t.Fatalf("register A: %v", err)
	}
	if err := mgrB.Register(job); err != nil {
		t.Fatalf("register B: %v", err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, mgr := range []*Manager{mgrA, mgrB} {
		go func(mgr *Manager) {
			<-start
			errs <- mgr.RunOnce(context.Background())
		}(mgr)
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent manager: %v", err)
		}
	}

	var executions, revisions int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM sys_cron_executions WHERE job_name = $1 AND scope_kind = $2 AND scope_id = $3`, job.Name, ScopeKindWorkspace, util.UUIDToString(workspaceID)).Scan(&executions); err != nil {
		t.Fatalf("count executions: %v", err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM lm_wiki_revision WHERE workspace_id = $1`, workspaceID).Scan(&revisions); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if executions != 1 || revisions != 1 {
		t.Fatalf("two managers produced executions=%d revisions=%d, want 1/1", executions, revisions)
	}
}

func TestLMWikiDailyRetry(t *testing.T) {
	pool := integrationPool(t)
	workspaceID := util.UUIDToString(seedLMWikiSchedulerWorkspace(t, pool))
	refresher := &recordingWikiRefresher{failuresLeft: 1, createdResult: true}
	job := LMWikiDailyReconcileJob(db.New(pool), refresher)
	job.Scopes = StaticScopes(Scope{Kind: ScopeKindWorkspace, ID: workspaceID})
	t.Cleanup(func() { cleanupWorkspaceExecution(t, pool, workspaceID) })

	mgr := NewManager(pool, Options{RunnerID: "wiki-retry-runner"})
	if err := mgr.Register(job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("first tick: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE sys_cron_executions SET next_retry_at = now() - INTERVAL '1 minute' WHERE job_name = $1 AND scope_id = $2`, job.Name, workspaceID); err != nil {
		t.Fatalf("make retry due: %v", err)
	}
	if err := mgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("retry tick: %v", err)
	}
	assertWikiExecution(t, pool, workspaceID, "SUCCESS", 2)
	calls := refresher.snapshotCalls()
	if len(calls) != 2 {
		t.Fatalf("refresh calls = %d, want 2", len(calls))
	}
	for _, call := range calls {
		if call.trigger != "scheduled" || call.requestedBy.Valid {
			t.Fatalf("refresh attribution = trigger %q actor_valid %t", call.trigger, call.requestedBy.Valid)
		}
		planTime, err := time.Parse(time.RFC3339, call.planKey)
		if err != nil || planTime.Location() != time.UTC {
			t.Fatalf("plan key %q is not UTC RFC3339: %v", call.planKey, err)
		}
	}
}

func TestLMWikiDailyStale(t *testing.T) {
	pool := integrationPool(t)
	workspaceID := util.UUIDToString(seedLMWikiSchedulerWorkspace(t, pool))
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		t.Fatalf("parse workspace UUID: %v", err)
	}
	refresher := &recordingWikiRefresher{createdResult: true}
	job := LMWikiDailyReconcileJob(db.New(pool), refresher)
	job.Scopes = StaticScopes(Scope{Kind: ScopeKindWorkspace, ID: workspaceID})
	t.Cleanup(func() { cleanupWorkspaceExecution(t, pool, workspaceID) })

	now, err := dbNow(context.Background(), pool)
	if err != nil {
		t.Fatalf("db now: %v", err)
	}
	planTime := FloorPlan(now, job.Cadence)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO sys_cron_executions (
			job_name, scope_kind, scope_id, plan_time, status, attempt, max_attempts,
			runner_id, lease_token, heartbeat_at, stale_after, started_at, updated_at
		) VALUES ($1, $2, $3, $4, 'RUNNING', 1, $5, 'dead-runner', gen_random_uuid(),
		          now() - INTERVAL '10 minutes', now() - INTERVAL '5 minutes',
		          now() - INTERVAL '10 minutes', now() - INTERVAL '10 minutes')
	`, job.Name, ScopeKindWorkspace, util.UUIDToString(workspaceUUID), planTime, job.MaxAttempts); err != nil {
		t.Fatalf("seed stale execution: %v", err)
	}

	mgr := NewManager(pool, Options{RunnerID: "wiki-stale-reentry"})
	if err := mgr.Register(job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := mgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("stale recovery tick: %v", err)
	}
	assertWikiExecution(t, pool, workspaceID, "SUCCESS", 2)
	if calls := refresher.snapshotCalls(); len(calls) != 1 {
		t.Fatalf("stale recovery refresh calls = %d, want 1", len(calls))
	}
}

func seedLMWikiSchedulerWorkspace(t *testing.T, pool *pgxpool.Pool) pgtype.UUID {
	t.Helper()
	ctx := context.Background()
	var workspaceID pgtype.UUID
	slug := "wiki-scheduler-" + uuid.NewString()
	if err := pool.QueryRow(ctx, `INSERT INTO workspace (name, slug) VALUES ('Wiki scheduler test', $1) RETURNING id`, slug).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM sys_cron_executions WHERE job_name = $1 AND scope_id = $2`, JobNameLMWikiDailyReconcile, util.UUIDToString(workspaceID))
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})
	return workspaceID
}

func containsWorkspaceScope(scopes []Scope, workspaceID pgtype.UUID) bool {
	want := util.UUIDToString(workspaceID)
	for _, scope := range scopes {
		if scope.Kind == ScopeKindWorkspace && scope.ID == want {
			return true
		}
	}
	return false
}

func cleanupWorkspaceExecution(t *testing.T, pool *pgxpool.Pool, workspaceID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `DELETE FROM sys_cron_executions WHERE job_name = $1 AND scope_id = $2`, JobNameLMWikiDailyReconcile, workspaceID); err != nil {
		t.Fatalf("clean execution: %v", err)
	}
}

func assertWikiExecution(t *testing.T, pool *pgxpool.Pool, workspaceID, wantStatus string, wantAttempt int) {
	t.Helper()
	var status string
	var attempt int
	if err := pool.QueryRow(context.Background(), `SELECT status, attempt FROM sys_cron_executions WHERE job_name = $1 AND scope_id = $2`, JobNameLMWikiDailyReconcile, workspaceID).Scan(&status, &attempt); err != nil {
		t.Fatalf("read execution: %v", err)
	}
	if status != wantStatus || attempt != wantAttempt {
		t.Fatalf("execution status=%s attempt=%d, want %s/%d", status, attempt, wantStatus, wantAttempt)
	}
}
