package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestLMWikiQueriesWorkspaceScope(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	workspaceID := createLMWikiTestWorkspace(t, ctx, "wiki-query")
	otherWorkspaceID := createLMWikiTestWorkspace(t, ctx, "wiki-other")
	queries := db.New(testPool)

	revision, err := queries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID:  workspaceID,
		SourceDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Content:      json.RawMessage(`{"schema_version":1}`),
		TriggerKind:  "manual",
	})
	if err != nil {
		t.Fatalf("create wiki revision: %v", err)
	}
	if revision.RevisionNumber != 1 {
		t.Fatalf("revision number = %d, want 1", revision.RevisionNumber)
	}
	citations := wikiCitationJSON(t, workspaceID, "issue:first")
	if err := queries.CreateLMWikiCitations(ctx, db.CreateLMWikiCitationsParams{
		WorkspaceID: workspaceID,
		RevisionID:  revision.ID,
		Citations:   citations,
	}); err != nil {
		t.Fatalf("create wiki citations: %v", err)
	}
	beforeReview, err := queries.ListLMWikiCitations(ctx, db.ListLMWikiCitationsParams{
		WorkspaceID: workspaceID,
		RevisionID:  revision.ID,
	})
	if err != nil {
		t.Fatalf("list wiki citations: %v", err)
	}
	if len(beforeReview) != 1 {
		t.Fatalf("citation count = %d, want 1", len(beforeReview))
	}

	second, err := queries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID:  workspaceID,
		SourceDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Content:      json.RawMessage(`{"schema_version":1,"issues":[]}`),
		TriggerKind:  "scheduled",
	})
	if err != nil {
		t.Fatalf("create second wiki revision: %v", err)
	}
	if second.RevisionNumber != 2 {
		t.Fatalf("second revision number = %d, want 2", second.RevisionNumber)
	}
	if _, err := queries.CreateLMWikiReview(ctx, db.CreateLMWikiReviewParams{
		WorkspaceID: workspaceID,
		RevisionID:  second.ID,
		Decision:    "accepted",
		ReviewerID:  parseUUID(testUserID),
	}); err != nil {
		t.Fatalf("accept wiki revision: %v", err)
	}
	afterReview, err := queries.ListLMWikiCitations(ctx, db.ListLMWikiCitationsParams{
		WorkspaceID: workspaceID,
		RevisionID:  revision.ID,
	})
	if err != nil {
		t.Fatalf("list wiki citations after review: %v", err)
	}
	beforeBytes, _ := json.Marshal(beforeReview)
	afterBytes, _ := json.Marshal(afterReview)
	if !bytes.Equal(beforeBytes, afterBytes) {
		t.Fatalf("citation snapshot changed after review: before=%s after=%s", beforeBytes, afterBytes)
	}

	_, err = queries.GetLMWikiRevision(ctx, db.GetLMWikiRevisionParams{
		WorkspaceID: otherWorkspaceID,
		ID:          revision.ID,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("wrong-workspace revision lookup error = %v, want pgx.ErrNoRows", err)
	}
}

func TestLMWikiConcurrentRevisionAndReview(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	workspaceID := createLMWikiTestWorkspace(t, ctx, "wiki-concurrent")
	queries := db.New(testPool)
	const workers = 8

	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			digest := fmt.Sprintf("sha256:%064x", i+1)
			tx, err := testPool.Begin(ctx)
			if err != nil {
				errCh <- err
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			txQueries := queries.WithTx(tx)
			if _, err = txQueries.LockWorkspaceForWikiArtifactCreate(ctx, workspaceID); err != nil {
				errCh <- err
				return
			}
			if err = txQueries.LockLMWikiLifecycle(ctx, workspaceID); err != nil {
				errCh <- err
				return
			}
			_, err = txQueries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
				WorkspaceID:  workspaceID,
				SourceDigest: digest,
				Content:      json.RawMessage(`{"schema_version":1}`),
				TriggerKind:  "manual",
			})
			if err == nil {
				err = tx.Commit(ctx)
			}
			errCh <- err
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent revision create: %v", err)
		}
	}
	revisions, err := queries.ListLMWikiRevisions(ctx, db.ListLMWikiRevisionsParams{
		WorkspaceID: workspaceID,
		ResultLimit: workers,
	})
	if err != nil {
		t.Fatalf("list concurrent revisions: %v", err)
	}
	if len(revisions) != workers || revisions[0].RevisionNumber != workers || revisions[workers-1].RevisionNumber != 1 {
		t.Fatalf("revision sequence = %#v, want unique 1..%d", revisions, workers)
	}

	successes := 0
	errCh = make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := queries.CreateLMWikiReview(ctx, db.CreateLMWikiReviewParams{
				WorkspaceID: workspaceID,
				RevisionID:  revisions[0].ID,
				Decision:    []string{"accepted", "rejected"}[i%2],
				ReviewerID:  parseUUID(testUserID),
			})
			errCh <- err
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, pgx.ErrNoRows):
		default:
			t.Fatalf("concurrent review create: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful terminal reviews = %d, want 1", successes)
	}
}

func createLMWikiTestWorkspace(t *testing.T, ctx context.Context, suffix string) pgtype.UUID {
	t.Helper()

	workspaceID := parseUUID(dbfx.Workspace(t, "Wiki Test "+suffix, "wiki-test-"+suffix+"-"+uuid.NewString(), testutil.Cols{
		"issue_prefix": "WIK",
	}))
	cleanupCtx := context.WithoutCancel(ctx)
	t.Cleanup(func() {
		_ = db.New(testPool).DeleteWorkspaceWikiTwinData(cleanupCtx, workspaceID)
	})
	return workspaceID
}

func wikiCitationJSON(t *testing.T, sourceID pgtype.UUID, key string) []byte {
	t.Helper()

	citations, err := json.Marshal([]map[string]any{{
		"ordinal":           0,
		"citation_key":      key,
		"source_type":       "issue",
		"source_id":         uuidToString(sourceID),
		"source_updated_at": nil,
		"locator":           "/issues/1",
		"label":             "Issue 1",
		"safe_metadata":     map[string]any{"status": "open"},
		"source_digest":     "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}})
	if err != nil {
		t.Fatalf("marshal wiki citation: %v", err)
	}
	return citations
}
