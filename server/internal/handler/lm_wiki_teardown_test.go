package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeleteWorkspaceRemovesWikiTwinData(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	targetID := createLMWikiTestWorkspace(t, ctx, "wiki-delete")
	controlID := createLMWikiTestWorkspace(t, ctx, "wiki-control")
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, targetID, testUserID); err != nil {
		t.Fatalf("create target owner: %v", err)
	}
	queries := db.New(testPool)
	for _, workspaceID := range []pgtype.UUID{targetID, controlID} {
		revision, err := queries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
			WorkspaceID:  workspaceID,
			SourceDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			Content:      json.RawMessage(`{"schema_version":1}`),
			TriggerKind:  "scheduled",
		})
		if err != nil {
			t.Fatalf("create teardown revision: %v", err)
		}
		if err := queries.CreateLMWikiCitations(ctx, db.CreateLMWikiCitationsParams{
			WorkspaceID: workspaceID,
			RevisionID:  revision.ID,
			Citations:   wikiCitationJSON(t, workspaceID, "issue:teardown"),
		}); err != nil {
			t.Fatalf("create teardown citation: %v", err)
		}
		if _, err := queries.CreateLMWikiReview(ctx, db.CreateLMWikiReviewParams{
			WorkspaceID: workspaceID,
			RevisionID:  revision.ID,
			Decision:    "accepted",
			ReviewerID:  parseUUID(testUserID),
		}); err != nil {
			t.Fatalf("create teardown review: %v", err)
		}
		proposal, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
			WorkspaceID:          workspaceID,
			Kind:                 "initial",
			SourceWikiRevisionID: revision.ID,
			Content:              json.RawMessage(`{"schema_version":1,"assertions":[]}`),
			ContentDigest:        twinProposalDigest,
			RequestedByID:        parseUUID(testUserID),
		})
		if err != nil {
			t.Fatalf("create teardown Twin proposal: %v", err)
		}
		if _, err := queries.CreateTwinProposalReview(ctx, db.CreateTwinProposalReviewParams{
			WorkspaceID: workspaceID,
			ProposalID:  proposal.ID,
			Decision:    "accepted",
			ReviewerID:  parseUUID(testUserID),
		}); err != nil {
			t.Fatalf("create teardown Twin review: %v", err)
		}
		if _, err := queries.CreateTwinVersion(ctx, db.CreateTwinVersionParams{
			WorkspaceID:   workspaceID,
			ProposalID:    proposal.ID,
			SignedOffByID: parseUUID(testUserID),
		}); err != nil {
			t.Fatalf("create teardown Twin version: %v", err)
		}
		if _, err := testPool.Exec(ctx, `
INSERT INTO sys_cron_executions (job_name, scope_kind, scope_id, plan_time, status)
VALUES ('lm_wiki_daily_reconcile', 'workspace', $1::uuid::text, now(), 'SUCCESS')
`, workspaceID); err != nil {
			t.Fatalf("create teardown scheduler execution: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req := withURLParam(newRequest(http.MethodDelete, "/api/workspaces/"+uuidToString(targetID), nil), "id", uuidToString(targetID))
	testHandler.DeleteWorkspace(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete workspace status = %d, want 204: %s", w.Code, w.Body.String())
	}

	for _, table := range []string{
		"twin_proposal_review", "twin_version", "twin_proposal",
		"lm_wiki_review", "lm_wiki_citation", "lm_wiki_revision",
	} {
		var targetCount, controlCount int
		query := fmt.Sprintf("SELECT count(*) FILTER (WHERE workspace_id = $1), count(*) FILTER (WHERE workspace_id = $2) FROM %s", table)
		if err := testPool.QueryRow(ctx, query, targetID, controlID).Scan(&targetCount, &controlCount); err != nil {
			t.Fatalf("count %s after teardown: %v", table, err)
		}
		if targetCount != 0 || controlCount != 1 {
			t.Fatalf("%s counts target=%d control=%d, want 0 and 1", table, targetCount, controlCount)
		}
	}
	var targetExecutions, controlExecutions int
	if err := testPool.QueryRow(ctx, `
SELECT count(*) FILTER (WHERE scope_id = $1::uuid::text),
       count(*) FILTER (WHERE scope_id = $2::uuid::text)
FROM sys_cron_executions
WHERE job_name = 'lm_wiki_daily_reconcile'
`, targetID, controlID).Scan(&targetExecutions, &controlExecutions); err != nil {
		t.Fatalf("count scheduler executions after teardown: %v", err)
	}
	if targetExecutions != 0 || controlExecutions != 1 {
		t.Fatalf("scheduler counts target=%d control=%d, want 0 and 1", targetExecutions, controlExecutions)
	}
}
