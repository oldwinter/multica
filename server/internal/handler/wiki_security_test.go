package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAuthorizedWikiPageReadRejectsWrongWorkspaceAndOwnerBeforeReturningContent(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	queries := db.New(testPool)
	ownerID := util.MustParseUUID(testUserID)
	workspaceID := util.MustParseUUID(testWorkspaceID)
	foreignWorkspaceID := util.MustParseUUID(dbfx.Workspace(t, "Wiki authz foreign", "wiki-authz-"+uuid.NewString()[:8]))
	foreignOwnerID := util.MustParseUUID(uuid.NewString())
	pathSuffix := uuid.NewString()

	personalID := util.MustParseUUID(dbfx.Insert(t, "wiki_page", testutil.Cols{
		"workspace_id": nil, "scope": "user", "owner_user_id": testUserID,
		"path": "security/personal-" + pathSuffix + ".md", "title": "Personal sentinel",
		"content": "PERSONAL_SECRET_SENTINEL", "created_by": testUserID,
	}))
	sharedID := util.MustParseUUID(dbfx.Insert(t, "wiki_page", testutil.Cols{
		"workspace_id": uuidToString(foreignWorkspaceID), "scope": "workspace",
		"path": "security/shared-" + pathSuffix + ".md", "title": "Shared sentinel",
		"content": "SHARED_SECRET_SENTINEL", "created_by": testUserID,
	}))

	personalRead, err := queries.GetWikiPageForActor(ctx, db.GetWikiPageForActorParams{
		PageID: personalID, WorkspaceID: pgtype.UUID{}, OwnerUserID: ownerID,
	})
	if err != nil || personalRead.Content != "PERSONAL_SECRET_SENTINEL" {
		t.Fatalf("authorized personal read = content %q, err %v", personalRead.Content, err)
	}
	if _, err := queries.GetWikiPageForActor(ctx, db.GetWikiPageForActorParams{
		PageID: personalID, WorkspaceID: workspaceID, OwnerUserID: foreignOwnerID,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("wrong owner read error = %v, want pgx.ErrNoRows", err)
	}

	sharedRead, err := queries.GetWikiPageForActor(ctx, db.GetWikiPageForActorParams{
		PageID: sharedID, WorkspaceID: foreignWorkspaceID, OwnerUserID: pgtype.UUID{},
	})
	if err != nil || sharedRead.Content != "SHARED_SECRET_SENTINEL" {
		t.Fatalf("authorized shared read = content %q, err %v", sharedRead.Content, err)
	}
	if _, err := queries.GetWikiPageForActor(ctx, db.GetWikiPageForActorParams{
		PageID: sharedID, WorkspaceID: workspaceID, OwnerUserID: ownerID,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("wrong workspace read error = %v, want pgx.ErrNoRows", err)
	}
}

func TestWikiProposalRejectsUnsupportedEvidenceAndBlankRationale(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	queries := db.New(testPool)
	workspaceID := util.MustParseUUID(testWorkspaceID)
	ownerID := util.MustParseUUID(testUserID)
	pathSuffix := uuid.NewString()
	pageID := util.MustParseUUID(dbfx.Insert(t, "wiki_page", testutil.Cols{
		"workspace_id": testWorkspaceID, "scope": "workspace",
		"path": "security/proposal-" + pathSuffix + ".md", "title": "Proposal evidence",
		"content": "Original", "created_by": testUserID,
	}))
	page, err := queries.GetWikiPageForActor(ctx, db.GetWikiPageForActorParams{
		PageID: pageID, WorkspaceID: workspaceID, OwnerUserID: ownerID,
	})
	if err != nil {
		t.Fatalf("load shared Wiki page: %v", err)
	}
	agentID := createHandlerTestAgent(t, "Wiki evidence agent", nil)
	taskID := createHandlerTestTaskForAgent(t, agentID)
	dbfx.Cleanup(t, `DELETE FROM wiki_page_edit_proposal WHERE page_id = $1`, page.ID)

	for _, tc := range []struct {
		name         string
		rationale    string
		evidenceRefs []string
	}{
		{name: "unsupported evidence", rationale: "Apply verified findings", evidenceRefs: []string{"issue:" + uuid.NewString()}},
		{name: "blank rationale", rationale: " \t\n ", evidenceRefs: []string{"task:" + taskID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.JSONRequest(http.MethodPost, "/api/wiki/pages/"+uuidToString(page.ID)+"/proposals", map[string]any{
				"base_revision_number": page.CurrentRevisionNumber,
				"proposed_content":     "Proposed " + tc.name,
				"rationale":            tc.rationale,
				"evidence_refs":        tc.evidenceRefs,
				"agent_id":             agentID,
				"idempotency_key":      "wiki-security-" + tc.name + "-" + uuid.NewString(),
			})
			testutil.WithHeaders(req,
				"X-Workspace-ID", testWorkspaceID,
				"X-User-ID", testUserID,
				"X-Actor-Source", "task_token",
				"X-Agent-ID", agentID,
				"X-Task-ID", taskID,
			)
			req = testutil.WithURLParams(req, "id", uuidToString(page.ID))

			testutil.Call(t, testHandler.CreateWikiPageEditProposal, req).Want(http.StatusBadRequest)
		})
	}
}

func TestWikiProposalEvidenceRefsAreBoundToAuthenticatedRunAndWorkspace(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	queries := db.New(testPool)
	workspaceID := util.MustParseUUID(testWorkspaceID)
	ownerID := util.MustParseUUID(testUserID)
	pageID := util.MustParseUUID(dbfx.Insert(t, "wiki_page", testutil.Cols{
		"workspace_id": testWorkspaceID, "scope": "workspace",
		"path": "security/evidence-matrix-" + uuid.NewString() + ".md", "title": "Evidence matrix",
		"content": "Original", "created_by": testUserID,
	}))
	page, err := queries.GetWikiPageForActor(ctx, db.GetWikiPageForActorParams{
		PageID: pageID, WorkspaceID: workspaceID, OwnerUserID: ownerID,
	})
	if err != nil {
		t.Fatalf("load shared Wiki page: %v", err)
	}
	agentID := createHandlerTestAgent(t, "Wiki evidence matrix agent", nil)
	taskID := createHandlerTestTaskForAgent(t, agentID)
	otherAgentID := createHandlerTestAgent(t, "Wiki evidence other agent", nil)
	otherAgentTaskID := createHandlerTestTaskForAgent(t, otherAgentID)
	roomID := dbfx.Insert(t, "room", testutil.Cols{
		"workspace_id": testWorkspaceID, "title": "Evidence room",
		"created_by_user_id": testUserID, "facilitator_agent_id": agentID,
	})
	roomArtifactID := dbfx.Insert(t, "room_artifact", testutil.Cols{
		"workspace_id": testWorkspaceID, "room_id": roomID, "kind": "decision",
		"idempotency_key": "wiki-evidence-" + uuid.NewString(), "title": "Verified decision",
		"body": "Use the verified result", "source_digest": "sha256:" + strings.Repeat("a", 64),
		"created_by_user_id": testUserID,
	})

	foreignWorkspaceID := dbfx.Workspace(t, "Wiki evidence foreign", "wiki-evidence-"+uuid.NewString()[:8])
	foreignFixture := testutil.New(testPool, foreignWorkspaceID, testUserID)
	foreignAgentID := foreignFixture.Agent(t, "Foreign evidence agent", "")
	foreignTaskID := foreignFixture.Task(t, foreignAgentID, testutil.Cols{
		"status": "cancelled", "completed_at": testutil.Raw("now()"),
	})
	foreignRoomID := foreignFixture.Insert(t, "room", testutil.Cols{
		"workspace_id": foreignWorkspaceID, "title": "Foreign evidence room",
		"created_by_user_id": testUserID, "facilitator_agent_id": foreignAgentID,
	})
	foreignRoomArtifactID := foreignFixture.Insert(t, "room_artifact", testutil.Cols{
		"workspace_id": foreignWorkspaceID, "room_id": foreignRoomID, "kind": "decision",
		"idempotency_key": "foreign-wiki-evidence-" + uuid.NewString(), "title": "Foreign decision",
		"body": "Must not cross tenants", "source_digest": "sha256:" + strings.Repeat("b", 64),
		"created_by_user_id": testUserID,
	})
	dbfx.Cleanup(t, `DELETE FROM wiki_page_edit_proposal WHERE page_id = $1`, page.ID)

	for _, tc := range []struct {
		name         string
		evidenceRefs []string
		headerTaskID string
		wantStatus   int
	}{
		{name: "current task", evidenceRefs: []string{"task:" + taskID}, headerTaskID: taskID, wantStatus: http.StatusCreated},
		{name: "same workspace room artifact", evidenceRefs: []string{"room:" + roomArtifactID}, headerTaskID: taskID, wantStatus: http.StatusCreated},
		{name: "empty", evidenceRefs: []string{}, headerTaskID: taskID, wantStatus: http.StatusBadRequest},
		{name: "malformed", evidenceRefs: []string{"task:not-a-uuid"}, headerTaskID: taskID, wantStatus: http.StatusBadRequest},
		{name: "unknown kind", evidenceRefs: []string{"issue:" + uuid.NewString()}, headerTaskID: taskID, wantStatus: http.StatusBadRequest},
		{name: "fabricated task", evidenceRefs: []string{"task:" + uuid.NewString()}, headerTaskID: uuid.NewString(), wantStatus: http.StatusBadRequest},
		{name: "fabricated room artifact", evidenceRefs: []string{"room:" + uuid.NewString()}, headerTaskID: taskID, wantStatus: http.StatusBadRequest},
		{name: "task belongs to another agent", evidenceRefs: []string{"task:" + otherAgentTaskID}, headerTaskID: otherAgentTaskID, wantStatus: http.StatusBadRequest},
		{name: "cross workspace task", evidenceRefs: []string{"task:" + foreignTaskID}, headerTaskID: foreignTaskID, wantStatus: http.StatusBadRequest},
		{name: "cross workspace room artifact", evidenceRefs: []string{"room:" + foreignRoomArtifactID}, headerTaskID: taskID, wantStatus: http.StatusBadRequest},
		{name: "room with fabricated actor task", evidenceRefs: []string{"room:" + roomArtifactID}, headerTaskID: uuid.NewString(), wantStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "fabricated task" {
				tc.evidenceRefs[0] = "task:" + tc.headerTaskID
			}
			req := testutil.JSONRequest(http.MethodPost, "/api/wiki/pages/"+uuidToString(page.ID)+"/proposals", map[string]any{
				"base_revision_number": page.CurrentRevisionNumber,
				"proposed_content":     "Proposed " + tc.name,
				"rationale":            "Evidence matrix: " + tc.name,
				"evidence_refs":        tc.evidenceRefs,
				"agent_id":             agentID,
				"idempotency_key":      "wiki-evidence-" + uuid.NewString(),
			})
			testutil.WithHeaders(req,
				"X-Workspace-ID", testWorkspaceID,
				"X-User-ID", testUserID,
				"X-Actor-Source", "task_token",
				"X-Agent-ID", agentID,
				"X-Task-ID", tc.headerTaskID,
			)
			req = testutil.WithURLParams(req, "id", uuidToString(page.ID))

			testutil.Call(t, testHandler.CreateWikiPageEditProposal, req).Want(tc.wantStatus)
		})
	}
}

func TestDeleteWikiPagePreservesStableRevisionAndLMCitation(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	revisionID := uuid.NewString()
	digest := "sha256:" + strings.Repeat("c", 64)
	pageID := dbfx.Insert(t, "wiki_page", testutil.Cols{
		"workspace_id": testWorkspaceID, "scope": "workspace",
		"path": "security/immutable-" + uuid.NewString() + ".md", "title": "Immutable evidence",
		"content": "IMMUTABLE_WIKI_EVIDENCE", "content_digest": digest,
		"current_revision_id": revisionID, "created_by": testUserID,
	})
	dbfx.Insert(t, "wiki_page_revision", testutil.Cols{
		"id": revisionID, "workspace_id": testWorkspaceID, "page_id": pageID,
		"revision_number": 1, "path": "security/immutable.md", "title": "Immutable evidence",
		"content": "IMMUTABLE_WIKI_EVIDENCE", "content_digest": digest,
		"actor_type": "member", "actor_id": testUserID, "source_kind": "human",
	})
	citationID := dbfx.Insert(t, "lm_wiki_citation", testutil.Cols{
		"workspace_id": testWorkspaceID, "revision_id": uuid.NewString(), "ordinal": 0,
		"citation_key": "wiki:" + revisionID, "source_type": "wiki_page_revision", "source_id": revisionID,
		"locator": "/api/wiki/revisions/" + revisionID, "label": "Immutable evidence",
		"safe_metadata": testutil.Raw("'{}'::jsonb"), "source_digest": digest,
	})

	deleteReq := testutil.JSONRequest(http.MethodDelete, "/api/wiki/pages/"+pageID, nil)
	testutil.WithHeaders(deleteReq, "X-Workspace-ID", testWorkspaceID, "X-User-ID", testUserID)
	deleteReq = testutil.WithURLParams(deleteReq, "id", pageID)
	testutil.Call(t, testHandler.DeleteWikiPage, deleteReq).Want(http.StatusNoContent)

	var revisionCount, citationCount int
	dbfx.QueryRow(t, `SELECT count(*) FROM wiki_page_revision WHERE id = $1`, revisionID).Scan(&revisionCount)
	dbfx.QueryRow(t, `SELECT count(*) FROM lm_wiki_citation WHERE id = $1`, citationID).Scan(&citationCount)
	if revisionCount != 1 || citationCount != 1 {
		t.Fatalf("after page delete revision count=%d citation count=%d, want both 1", revisionCount, citationCount)
	}

	pageReq := testutil.JSONRequest(http.MethodGet, "/api/wiki/pages/"+pageID, nil)
	testutil.WithHeaders(pageReq, "X-Workspace-ID", testWorkspaceID, "X-User-ID", testUserID)
	pageReq = testutil.WithURLParams(pageReq, "id", pageID)
	testutil.Call(t, testHandler.GetWikiPage, pageReq).Want(http.StatusNotFound)

	nestedReq := testutil.JSONRequest(http.MethodGet, "/api/wiki/pages/"+pageID+"/revisions/"+revisionID, nil)
	testutil.WithHeaders(nestedReq, "X-Workspace-ID", testWorkspaceID, "X-User-ID", testUserID)
	nestedReq = testutil.WithURLParams(nestedReq, "id", pageID, "revisionId", revisionID)
	testutil.Call(t, testHandler.GetWikiPageRevision, nestedReq).Want(http.StatusNotFound)

	stableReq := testutil.JSONRequest(http.MethodGet, "/api/wiki/revisions/"+revisionID, nil)
	testutil.WithHeaders(stableReq, "X-Workspace-ID", testWorkspaceID, "X-User-ID", testUserID)
	stableReq = testutil.WithURLParams(stableReq, "revisionId", revisionID)
	stable := testutil.Decode[WikiPageRevisionResponse](t, testHandler.GetStableWikiPageRevision, stableReq, http.StatusOK)
	if stable.ID != revisionID || stable.Content != "IMMUTABLE_WIKI_EVIDENCE" || stable.ContentDigest != digest || stable.SourceKind != "human" {
		t.Fatalf("stable revision response = %+v", stable)
	}

	foreignWorkspaceID := dbfx.Workspace(t, "Wiki stable foreign", "wiki-stable-"+uuid.NewString()[:8])
	foreignReq := testutil.JSONRequest(http.MethodGet, "/api/wiki/revisions/"+revisionID, nil)
	testutil.WithHeaders(foreignReq, "X-Workspace-ID", foreignWorkspaceID, "X-User-ID", testUserID)
	foreignReq = testutil.WithURLParams(foreignReq, "revisionId", revisionID)
	testutil.Call(t, testHandler.GetStableWikiPageRevision, foreignReq).Want(http.StatusNotFound)
}

func TestStablePersonalWikiRevisionRejectsMachineActor(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	revisionID := dbfx.Insert(t, "wiki_page_revision", testutil.Cols{
		"owner_user_id": testUserID, "page_id": uuid.NewString(), "revision_number": 1,
		"path": "private.md", "title": "Private", "content": "PRIVATE_STABLE_EVIDENCE",
		"content_digest": "sha256:" + strings.Repeat("d", 64),
		"actor_type":     "member", "actor_id": testUserID, "source_kind": "human",
	})
	req := testutil.JSONRequest(http.MethodGet, "/api/personal-wiki/revisions/"+revisionID, nil)
	testutil.WithHeaders(req,
		"X-User-ID", testUserID,
		"X-Actor-Source", "task_token",
		"X-Agent-ID", uuid.NewString(),
		"X-Task-ID", uuid.NewString(),
	)
	req = testutil.WithURLParams(req, "revisionId", revisionID)
	testutil.Call(t, testHandler.GetStablePersonalWikiPageRevision, req).Want(http.StatusForbidden)
}

func TestPersonalWikiLifecycleDoesNotRequireWorkspaceMembership(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	email := "personal-wiki-" + uuid.NewString() + "@multica.test"
	userID := dbfx.User(t, "Personal Wiki owner", email)
	dbfx.Cleanup(t, `DELETE FROM wiki_page WHERE owner_user_id = $1`, userID)
	dbfx.Cleanup(t, `DELETE FROM wiki_page_revision WHERE owner_user_id = $1`, userID)
	var membershipCount int
	dbfx.QueryRow(t, `SELECT count(*) FROM member WHERE user_id = $1`, userID).Scan(&membershipCount)
	if membershipCount != 0 {
		t.Fatalf("personal Wiki owner membership count = %d, want 0", membershipCount)
	}

	createReq := testutil.JSONRequest(http.MethodPost, "/api/personal-wiki/pages", map[string]any{
		"path": "private/notes.md", "title": "Private notes", "content": "first private draft",
	})
	testutil.WithHeaders(createReq, "X-User-ID", userID)
	created := testutil.Decode[WikiPageResponse](t, testHandler.CreatePersonalWikiPage, createReq, http.StatusCreated)
	if created.Scope != "user" || created.WorkspaceID != nil || created.OwnerUserID == nil || *created.OwnerUserID != userID {
		t.Fatalf("created personal page = %+v", created)
	}

	listReq := testutil.JSONRequest(http.MethodGet, "/api/personal-wiki/pages", nil)
	testutil.WithHeaders(listReq, "X-User-ID", userID)
	pages := testutil.Decode[[]WikiPageSummaryResponse](t, testHandler.ListPersonalWikiPages, listReq, http.StatusOK)
	if len(pages) != 1 || pages[0].ID != created.ID {
		t.Fatalf("personal pages = %+v", pages)
	}

	searchReq := testutil.JSONRequest(http.MethodGet, "/api/personal-wiki/search?q=private+draft", nil)
	testutil.WithHeaders(searchReq, "X-User-ID", userID)
	search := testutil.Decode[[]WikiPageSummaryResponse](t, testHandler.SearchPersonalWikiPages, searchReq, http.StatusOK)
	if len(search) != 1 || search[0].ID != created.ID {
		t.Fatalf("personal search = %+v", search)
	}

	updateReq := testutil.JSONRequest(http.MethodPut, "/api/personal-wiki/pages/"+created.ID, map[string]any{
		"expected_revision_number": 1, "content": "second private draft",
	})
	testutil.WithHeaders(updateReq, "X-User-ID", userID)
	updateReq = testutil.WithURLParams(updateReq, "id", created.ID)
	updated := testutil.Decode[WikiPageResponse](t, testHandler.UpdatePersonalWikiPage, updateReq, http.StatusOK)
	if updated.CurrentRevisionNumber != 2 || updated.Content != "second private draft" {
		t.Fatalf("updated personal page = %+v", updated)
	}

	revisionsReq := testutil.JSONRequest(http.MethodGet, "/api/personal-wiki/pages/"+created.ID+"/revisions", nil)
	testutil.WithHeaders(revisionsReq, "X-User-ID", userID)
	revisionsReq = testutil.WithURLParams(revisionsReq, "id", created.ID)
	revisions := testutil.Decode[[]WikiPageRevisionResponse](t, testHandler.ListPersonalWikiPageRevisions, revisionsReq, http.StatusOK)
	if len(revisions) != 2 || revisions[1].RevisionNumber != 1 {
		t.Fatalf("personal revisions = %+v", revisions)
	}

	restoreReq := testutil.JSONRequest(http.MethodPost, "/api/personal-wiki/pages/"+created.ID+"/revisions/"+revisions[1].ID+"/restore", map[string]any{
		"expected_revision_number": 2,
	})
	testutil.WithHeaders(restoreReq, "X-User-ID", userID)
	restoreReq = testutil.WithURLParams(restoreReq, "id", created.ID, "revisionId", revisions[1].ID)
	restored := testutil.Decode[WikiPageResponse](t, testHandler.RestorePersonalWikiPageRevision, restoreReq, http.StatusOK)
	if restored.CurrentRevisionNumber != 3 || restored.Content != "first private draft" {
		t.Fatalf("restored personal page = %+v", restored)
	}

	deleteReq := testutil.JSONRequest(http.MethodDelete, "/api/personal-wiki/pages/"+created.ID, nil)
	testutil.WithHeaders(deleteReq, "X-User-ID", userID)
	deleteReq = testutil.WithURLParams(deleteReq, "id", created.ID)
	testutil.Call(t, testHandler.DeletePersonalWikiPage, deleteReq).Want(http.StatusNoContent)

	stableReq := testutil.JSONRequest(http.MethodGet, "/api/personal-wiki/revisions/"+restored.CurrentRevisionID, nil)
	testutil.WithHeaders(stableReq, "X-User-ID", userID)
	stableReq = testutil.WithURLParams(stableReq, "revisionId", restored.CurrentRevisionID)
	stable := testutil.Decode[WikiPageRevisionResponse](t, testHandler.GetStablePersonalWikiPageRevision, stableReq, http.StatusOK)
	if stable.Content != "first private draft" || stable.RevisionNumber != 3 {
		t.Fatalf("stable personal revision = %+v", stable)
	}

	sharedPageID := dbfx.Insert(t, "wiki_page", testutil.Cols{
		"workspace_id": testWorkspaceID, "scope": "workspace", "path": "security/shared-via-personal-" + uuid.NewString() + ".md",
		"title": "Shared", "content": "SHARED_CONTENT", "created_by": testUserID,
	})
	sharedReq := testutil.JSONRequest(http.MethodGet, "/api/personal-wiki/pages/"+sharedPageID, nil)
	testutil.WithHeaders(sharedReq, "X-User-ID", userID, "X-Workspace-ID", testWorkspaceID)
	sharedReq = testutil.WithURLParams(sharedReq, "id", sharedPageID)
	testutil.Call(t, testHandler.GetPersonalWikiPage, sharedReq).Want(http.StatusNotFound)

	for _, source := range []string{"task_token", "cloud_pat"} {
		t.Run(source+" rejected", func(t *testing.T) {
			req := testutil.JSONRequest(http.MethodGet, "/api/personal-wiki/pages", nil)
			testutil.WithHeaders(req, "X-User-ID", userID, "X-Actor-Source", source)
			testutil.Call(t, testHandler.ListPersonalWikiPages, req).Want(http.StatusForbidden)
		})
	}
}
