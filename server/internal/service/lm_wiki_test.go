package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type coordinatedLMWikiSnapshotStarter struct {
	pool         *pgxpool.Pool
	issuesRead   chan struct{}
	mutationDone chan struct{}
	once         *sync.Once
}

func (s coordinatedLMWikiSnapshotStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return coordinatedLMWikiSnapshotTx{Tx: tx, issuesRead: s.issuesRead, mutationDone: s.mutationDone, once: s.once}, nil
}

func (s coordinatedLMWikiSnapshotStarter) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := s.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return coordinatedLMWikiSnapshotTx{Tx: tx, issuesRead: s.issuesRead, mutationDone: s.mutationDone, once: s.once}, nil
}

type coordinatedLMWikiSnapshotTx struct {
	pgx.Tx
	issuesRead   chan struct{}
	mutationDone chan struct{}
	once         *sync.Once
}

func (tx coordinatedLMWikiSnapshotTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if err == nil && strings.Contains(sql, "FROM issue") {
		tx.once.Do(func() { close(tx.issuesRead) })
		select {
		case <-tx.mutationDone:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return rows, err
}

func TestLMWikiRefreshRejectsInvalidTrigger(t *testing.T) {
	service := &WikiService{}
	_, err := service.Refresh(context.Background(), pgtype.UUID{}, "webhook", pgtype.UUID{}, "")
	if !errors.Is(err, ErrLMWikiInvalidReview) {
		t.Fatalf("invalid trigger error = %v", err)
	}
}

func TestLMWikiReviewRejectsOversizedReason(t *testing.T) {
	service := &WikiService{}
	_, err := service.Review(context.Background(), pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, "rejected", strings.Repeat("x", LMWikiReviewReasonLimit+1))
	if !errors.Is(err, ErrLMWikiInvalidReview) {
		t.Fatalf("oversized reason error = %v", err)
	}
}

func TestLMWikiRefreshRetryableErrorClassifiesOnlyLifecycleRaces(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "serialization failure", err: &pgconn.PgError{Code: "40001"}, want: true},
		{name: "revision number race", err: fmt.Errorf("create revision: %w", &pgconn.PgError{Code: "23505", ConstraintName: "lm_wiki_revision_workspace_number_uidx"}), want: true},
		{name: "unrelated unique violation", err: &pgconn.PgError{Code: "23505", ConstraintName: "other_unique_index"}, want: false},
		{name: "non PostgreSQL error", err: errors.New("connection closed"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetryableLMWikiRefreshError(test.err); got != test.want {
				t.Fatalf("isRetryableLMWikiRefreshError() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLMWikiRefreshPersistsCoherentSnapshotDuringConcurrentSourceMutation(t *testing.T) {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database unavailable: %v", err)
	}

	var workspaceID, projectID, issueID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, issue_prefix)
		VALUES ('LM Wiki snapshot test', 'lm-wiki-snapshot-' || gen_random_uuid()::text, 'LWS')
		RETURNING id
	`).Scan(&workspaceID); err != nil {
		t.Fatalf("create snapshot workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM lm_wiki_review WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM lm_wiki_citation WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM lm_wiki_revision WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM issue WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM project WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (workspace_id, title, status, priority)
		VALUES ($1, 'project-before', 'in_progress', 'medium')
		RETURNING id
	`, workspaceID).Scan(&projectID); err != nil {
		t.Fatalf("create snapshot project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, project_id)
		VALUES ($1, 'issue-before', 'todo', 'high', 'member', gen_random_uuid(), 1, $2)
		RETURNING id
	`, workspaceID, projectID).Scan(&issueID); err != nil {
		t.Fatalf("create snapshot issue: %v", err)
	}

	issuesRead := make(chan struct{})
	mutationDone := make(chan struct{})
	mutationErr := make(chan error, 1)
	go func() {
		<-issuesRead
		tx, beginErr := pool.Begin(ctx)
		if beginErr == nil {
			_, beginErr = tx.Exec(ctx, `UPDATE issue SET title = 'issue-after', updated_at = now() WHERE id = $1`, issueID)
		}
		if beginErr == nil {
			_, beginErr = tx.Exec(ctx, `UPDATE project SET title = 'project-after', updated_at = now() WHERE id = $1`, projectID)
		}
		if beginErr == nil {
			beginErr = tx.Commit(ctx)
		} else if tx != nil {
			_ = tx.Rollback(ctx)
		}
		mutationErr <- beginErr
		close(mutationDone)
	}()

	queries := db.New(pool)
	service := NewWikiService(queries, coordinatedLMWikiSnapshotStarter{
		pool: pool, issuesRead: issuesRead, mutationDone: mutationDone, once: &sync.Once{},
	})
	result, err := service.Refresh(ctx, workspaceID, "manual", pgtype.UUID{}, "")
	if err != nil {
		t.Fatalf("refresh coherent snapshot: %v", err)
	}
	if err := <-mutationErr; err != nil {
		t.Fatalf("mutate sources: %v", err)
	}
	var content LMWikiContent
	if err := json.Unmarshal(result.Revision.Content, &content); err != nil {
		t.Fatalf("decode persisted snapshot: %v", err)
	}
	if len(content.Issues) != 1 || len(content.Projects) != 1 {
		t.Fatalf("persisted source counts: issues=%d projects=%d", len(content.Issues), len(content.Projects))
	}
	if content.Issues[0].Title != "issue-before" || content.Projects[0].Title != "project-before" {
		t.Fatalf("persisted snapshot mixed source versions: issue=%q project=%q", content.Issues[0].Title, content.Projects[0].Title)
	}
}
