package skillevolution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWorkspaceCleanupDelegatesToTransactionalQueries(t *testing.T) {
	executor := &cleanupDBTX{}
	workspaceID := attributionTestUUID()

	err := NewWorkspaceCleanup().DeleteWorkspace(context.Background(), db.New(executor), workspaceID)

	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if executor.calls != 1 || !strings.Contains(executor.sql, "DELETE FROM skill_evolution_") {
		t.Fatalf("calls/sql = %d/%q, want one skill evolution cleanup", executor.calls, executor.sql)
	}
	if len(executor.args) != 1 || executor.args[0] != workspaceID {
		t.Fatalf("args = %+v, want workspace id", executor.args)
	}
}

func TestWorkspaceCleanupRejectsInvalidDependencies(t *testing.T) {
	cleanup := NewWorkspaceCleanup()
	if !errors.Is(cleanup.DeleteWorkspace(context.Background(), nil, attributionTestUUID()), ErrPersistenceInvalidInput) {
		t.Fatal("nil queries must fail closed")
	}
	if !errors.Is(cleanup.DeleteWorkspace(context.Background(), db.New(&cleanupDBTX{}), pgtype.UUID{}), ErrPersistenceInvalidInput) {
		t.Fatal("invalid workspace must fail closed")
	}
}

type cleanupDBTX struct {
	calls int
	sql   string
	args  []any
}

func (f *cleanupDBTX) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.calls++
	f.sql = sql
	f.args = append([]any(nil), args...)
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (*cleanupDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (*cleanupDBTX) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("unexpected QueryRow")
}
