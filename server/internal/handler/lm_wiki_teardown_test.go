package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeleteWorkspaceRemovesWikiTwinData(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	targetID := createLMWikiTestWorkspace(t, ctx, "wiki-delete")
	controlID := createLMWikiTestWorkspace(t, ctx, "wiki-control")
	dbfx.Member(t, uuidToString(targetID), testUserID, "owner")
	queries := db.New(testPool)
	personal, err := queries.CreateWikiPage(ctx, db.CreateWikiPageParams{
		Scope: "user", OwnerUserID: parseUUID(testUserID), Path: "teardown-personal.md",
		Title: "Personal", Content: "preserve", CreatedBy: parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create preserved personal Wiki page: %v", err)
	}
	personalPage := wikiPageFromCreate(personal)
	t.Cleanup(func() { _ = queries.DeleteWikiPage(context.Background(), personalPage.ID) })
	for _, workspaceID := range []pgtype.UUID{targetID, controlID} {
		pageRow, err := queries.CreateWikiPage(ctx, db.CreateWikiPageParams{
			WorkspaceID: workspaceID, Scope: "workspace", Path: "teardown/" + uuidToString(workspaceID) + ".md",
			Title: "Shared", Content: "shared", CreatedBy: parseUUID(testUserID),
		})
		if err != nil {
			t.Fatalf("create teardown shared Wiki page: %v", err)
		}
		page := wikiPageFromCreate(pageRow)
		if _, err := queries.CreateWikiPageEditProposal(ctx, db.CreateWikiPageEditProposalParams{
			WorkspaceID: workspaceID, PageID: page.ID, AgentID: parseUUID(testUserID),
			IdempotencyKey: "teardown-proposal", BaseRevisionNumber: page.CurrentRevisionNumber,
			ProposedPath: page.Path, ProposedTitle: page.Title, ProposedContent: "proposed",
			Rationale: "teardown", EvidenceRefs: []byte(`[]`),
		}); err != nil {
			t.Fatalf("create teardown Wiki proposal: %v", err)
		}
		if _, err := queries.UpsertLMWikiSourcePolicy(ctx, db.UpsertLMWikiSourcePolicyParams{
			WorkspaceID: workspaceID, SourceClasses: []byte(`["wiki_page"]`), UpdatedByID: parseUUID(testUserID),
		}); err != nil {
			t.Fatalf("create teardown source policy: %v", err)
		}
		selection, err := json.Marshal([]map[string]any{{
			"page_id": uuidToString(page.ID), "revision_id": uuidToString(page.CurrentRevisionID),
			"revision_number": page.CurrentRevisionNumber,
		}})
		if err != nil {
			t.Fatalf("marshal teardown selection: %v", err)
		}
		if err := queries.CreateLMWikiSourceWikiPages(ctx, db.CreateLMWikiSourceWikiPagesParams{
			WorkspaceID: workspaceID, SelectedByID: parseUUID(testUserID), Selections: selection,
		}); err != nil {
			t.Fatalf("create teardown Wiki source selection: %v", err)
		}
		revision, err := queries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
			WorkspaceID:  workspaceID,
			SourceDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			Content:      json.RawMessage(`{"schema_version":2,"issues":[],"projects":[],"project_resources":[],"autopilot_runs":[],"wiki_pages":[]}`),
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
		version, err := queries.CreateTwinVersion(ctx, db.CreateTwinVersionParams{
			WorkspaceID:   workspaceID,
			ProposalID:    proposal.ID,
			SignedOffByID: parseUUID(testUserID),
		})
		if err != nil {
			t.Fatalf("create teardown Twin version: %v", err)
		}
		if err := queries.RecordTwinActivationPreview(ctx, db.RecordTwinActivationPreviewParams{
			WorkspaceID: workspaceID, TwinVersionID: version.ID, PolicyState: "preview",
		}); err != nil {
			t.Fatalf("create teardown Twin activation preview checkpoint: %v", err)
		}
		taskID, agentID, runtimeID := randomTwinExecutionIDs(t, ctx)
		dispatchedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
		if _, err := queries.UpsertTwinBinding(ctx, db.UpsertTwinBindingParams{
			WorkspaceID: workspaceID, ScopeType: "workspace", ScopeID: workspaceID,
			State: "enabled", TwinVersionID: version.ID,
		}); err != nil {
			t.Fatalf("create teardown Twin binding: %v", err)
		}
		if _, err := queries.CreateTwinTaskAttributionForClaim(ctx, db.CreateTwinTaskAttributionForClaimParams{
			WorkspaceID: workspaceID, TaskID: taskID, AgentID: agentID,
			RuntimeID: runtimeID, TaskDispatchedAt: dispatchedAt,
			TwinVersionID: version.ID, Briefing: "bounded teardown briefing",
			BriefingDigest:  "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			AssertionIds:    json.RawMessage(`["assertion:teardown"]`),
			CitationKeys:    json.RawMessage(`["issue:teardown"]`),
			PolicyScopeType: "workspace", PolicyScopeID: workspaceID,
			PolicyState: "enabled", CompilerVersion: "teardown-test-v1",
		}); err != nil {
			t.Fatalf("create teardown Twin attribution: %v", err)
		}
		if _, err := queries.UpsertTwinRunFeedback(ctx, db.UpsertTwinRunFeedbackParams{
			WorkspaceID: workspaceID, TaskID: taskID, Rating: "helped",
		}); err != nil {
			t.Fatalf("create teardown Twin feedback: %v", err)
		}
		depositionProposal, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
			WorkspaceID: workspaceID, Kind: "evolution",
			SourceWikiRevisionID: revision.ID, BaseTwinVersionID: version.ID,
			Content:       json.RawMessage(`{"schema_version":1,"assertions":[]}`),
			ContentDigest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			RequestedByID: parseUUID(testUserID),
		})
		if err != nil {
			t.Fatalf("create teardown deposition proposal: %v", err)
		}
		if _, err := queries.LinkTwinDeposition(ctx, db.LinkTwinDepositionParams{
			WorkspaceID: workspaceID, TaskID: taskID, BaseTwinVersionID: version.ID,
			ProposalID:     depositionProposal.ID,
			EvidenceDigest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		}); err != nil {
			t.Fatalf("create teardown Twin deposition: %v", err)
		}
		dbfx.Insert(t, "sys_cron_executions", testutil.Cols{
			"job_name": "lm_wiki_daily_reconcile", "scope_kind": "workspace",
			"scope_id": uuidToString(workspaceID), "plan_time": testutil.Raw("now()"), "status": "SUCCESS",
		})
	}

	req := withURLParam(newRequest(http.MethodDelete, "/api/workspaces/"+uuidToString(targetID), nil), "id", uuidToString(targetID))
	testutil.Call(t, testHandler.DeleteWorkspace, req).Want(http.StatusNoContent)

	for _, table := range []string{
		"twin_activation_preview_checkpoint", "twin_binding", "twin_task_attribution", "twin_run_feedback", "twin_deposition",
		"twin_proposal_review", "twin_version", "twin_proposal",
		"lm_wiki_review", "lm_wiki_citation", "lm_wiki_revision",
		"lm_wiki_source_wiki_page", "lm_wiki_source_policy",
		"wiki_page_edit_proposal", "wiki_page_revision", "wiki_page",
	} {
		var targetCount, controlCount int
		query := fmt.Sprintf("SELECT count(*) FILTER (WHERE workspace_id = $1), count(*) FILTER (WHERE workspace_id = $2) FROM %s", table)
		if err := testPool.QueryRow(ctx, query, targetID, controlID).Scan(&targetCount, &controlCount); err != nil {
			t.Fatalf("count %s after teardown: %v", table, err)
		}
		controlWant := 1
		if table == "twin_proposal" {
			// The control workspace has both the signed initial proposal and the
			// pending deposition proposal created above.
			controlWant = 2
		}
		if targetCount != 0 || controlCount != controlWant {
			t.Fatalf("%s counts target=%d control=%d, want 0 and %d", table, targetCount, controlCount, controlWant)
		}
	}
	var personalCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM wiki_page WHERE id = $1 AND workspace_id IS NULL AND owner_user_id = $2`, personalPage.ID, testUserID).Scan(&personalCount); err != nil {
		t.Fatalf("count preserved personal Wiki page: %v", err)
	}
	if personalCount != 1 {
		t.Fatalf("personal Wiki page count=%d, want 1", personalCount)
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

func randomTwinExecutionIDs(t *testing.T, ctx context.Context) (pgtype.UUID, pgtype.UUID, pgtype.UUID) {
	t.Helper()
	var taskID, agentID, runtimeID pgtype.UUID
	if err := testPool.QueryRow(ctx, `SELECT gen_random_uuid(), gen_random_uuid(), gen_random_uuid()`).Scan(&taskID, &agentID, &runtimeID); err != nil {
		t.Fatalf("create Twin execution teardown ids: %v", err)
	}
	return taskID, agentID, runtimeID
}
