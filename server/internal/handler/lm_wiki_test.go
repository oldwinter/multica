package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type failLMWikiCitationTx struct{ pgx.Tx }

func (tx failLMWikiCitationTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO lm_wiki_citation") {
		return pgconn.CommandTag{}, errors.New("injected citation failure")
	}
	return tx.Tx.Exec(ctx, sql, arguments...)
}

type failLMWikiCitationStarter struct{}

func (failLMWikiCitationStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := testPool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return failLMWikiCitationTx{Tx: tx}, nil
}

func (failLMWikiCitationStarter) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := testPool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return failLMWikiCitationTx{Tx: tx}, nil
}

func resetLMWikiFixture(t *testing.T) {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	for _, query := range []string{
		`DELETE FROM twin_proposal_review WHERE workspace_id = $1`,
		`DELETE FROM twin_version WHERE workspace_id = $1`,
		`DELETE FROM twin_proposal WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_review WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_citation WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_revision WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_source_wiki_page WHERE workspace_id = $1`,
		`DELETE FROM lm_wiki_source_policy WHERE workspace_id = $1`,
		`DELETE FROM wiki_page_edit_proposal WHERE workspace_id = $1 AND page_id IN (SELECT id FROM wiki_page WHERE path LIKE 'lm-wiki-http/%')`,
		`DELETE FROM wiki_page_revision WHERE workspace_id = $1 AND page_id IN (SELECT id FROM wiki_page WHERE path LIKE 'lm-wiki-http/%')`,
		`DELETE FROM wiki_page WHERE workspace_id = $1 AND path LIKE 'lm-wiki-http/%'`,
		`DELETE FROM issue WHERE workspace_id = $1 AND title LIKE 'LM Wiki HTTP %'`,
	} {
		if _, err := testPool.Exec(ctx, query, testWorkspaceID); err != nil {
			t.Fatalf("reset LM Wiki fixture: %v", err)
		}
	}
}

func callLMWiki(t *testing.T, method, path string, body any) *testutil.Response {
	t.Helper()
	r := newRequest(method, path, body)
	if id := chiRevisionID(path); id != "" {
		r = withURLParam(r, "revisionId", id)
	}
	switch {
	case method == http.MethodGet && path == "/api/lm-wiki/source-policy":
		return testutil.Call(t, testHandler.GetLMWikiSourcePolicy, r)
	case method == http.MethodPut && path == "/api/lm-wiki/source-policy":
		return testutil.Call(t, testHandler.UpdateLMWikiSourcePolicy, r)
	case method == http.MethodPost && path == "/api/lm-wiki/refresh":
		return testutil.Call(t, testHandler.RefreshLMWiki, r)
	case method == http.MethodGet && path == "/api/lm-wiki":
		return testutil.Call(t, testHandler.GetLMWiki, r)
	case method == http.MethodPost && len(path) > len("/api/lm-wiki/revisions/") && path[len(path)-7:] == "/accept":
		return testutil.Call(t, testHandler.AcceptLMWikiRevision, r)
	case method == http.MethodPost && len(path) > len("/api/lm-wiki/revisions/") && path[len(path)-7:] == "/reject":
		return testutil.Call(t, testHandler.RejectLMWikiRevision, r)
	default:
		t.Fatalf("unsupported LM Wiki test request: %s %s", method, path)
	}
	return nil
}

func TestLMWikiSourcePolicyPinsExactSharedWikiRevision(t *testing.T) {
	resetLMWikiFixture(t)
	t.Cleanup(func() { resetLMWikiFixture(t) })
	ctx := context.Background()
	workspaceID := parseUUID(testWorkspaceID)
	userID := parseUUID(testUserID)

	created, err := testHandler.Queries.CreateWikiPage(ctx, db.CreateWikiPageParams{
		WorkspaceID: workspaceID, Scope: "workspace", ProjectID: pgtype.UUID{}, OwnerUserID: pgtype.UUID{},
		Path: "lm-wiki-http/pinned.md", Title: "Pinned knowledge", Content: "immutable version one", CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create selected Wiki page: %v", err)
	}
	page := wikiPageFromCreate(created)

	policy := callLMWiki(t, http.MethodPut, "/api/lm-wiki/source-policy", map[string]any{
		"source_classes":            []string{"wiki_page"},
		"wiki_pages":                []map[string]any{{"page_id": uuidToString(page.ID), "revision_number": page.CurrentRevisionNumber}},
		"remote_generation_enabled": true,
	}).Want(http.StatusOK)
	var saved service.LMWikiSourcePolicyState
	policy.JSON(&saved)
	if len(saved.SourceClasses) != 1 || saved.SourceClasses[0] != "wiki_page" || len(saved.WikiPages) != 1 ||
		!saved.RemoteGenerationEnabled || saved.PolicyVersion != 1 || !strings.HasPrefix(saved.PolicyDigest, "sha256:") || len(saved.Exclusions) != 2 {
		t.Fatalf("saved source policy=%+v", saved)
	}

	refreshed := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil).Want(http.StatusCreated)
	var first struct {
		Created  bool `json:"created"`
		Revision struct {
			ID                      string          `json:"id"`
			SchemaVersion           int32           `json:"schema_version"`
			SourcePolicyVersion     int64           `json:"source_policy_version"`
			SourcePolicyDigest      string          `json:"source_policy_digest"`
			RemoteGenerationEnabled bool            `json:"remote_generation_enabled"`
			Content                 json.RawMessage `json:"content"`
		} `json:"revision"`
	}
	refreshed.JSON(&first)
	var content struct {
		SchemaVersion int                        `json:"schema_version"`
		EgressPolicy  service.LMWikiEgressPolicy `json:"egress_policy"`
		WikiPages     []struct {
			RevisionID     string `json:"revision_id"`
			RevisionNumber int64  `json:"revision_number"`
			Content        string `json:"content"`
		} `json:"wiki_pages"`
	}
	if err := json.Unmarshal(first.Revision.Content, &content); err != nil {
		t.Fatalf("decode canonical v2 content: %v", err)
	}
	if first.Revision.SchemaVersion != 2 || content.SchemaVersion != 2 || len(content.WikiPages) != 1 {
		t.Fatalf("v2 snapshot=%+v page_count=%d", first.Revision, len(content.WikiPages))
	}
	if content.EgressPolicy.PolicyVersion != saved.PolicyVersion || content.EgressPolicy.PolicyDigest != saved.PolicyDigest || !content.EgressPolicy.RemoteGenerationEnabled {
		t.Fatalf("revision egress policy=%+v, source policy=%+v", content.EgressPolicy, saved)
	}
	if first.Revision.SourcePolicyVersion != saved.PolicyVersion || first.Revision.SourcePolicyDigest != saved.PolicyDigest || !first.Revision.RemoteGenerationEnabled {
		t.Fatalf("revision policy proof=%+v, source policy=%+v", first.Revision, saved)
	}
	if content.WikiPages[0].RevisionID != uuidToString(page.CurrentRevisionID) || content.WikiPages[0].Content != "immutable version one" {
		t.Fatalf("selected Wiki revision=%+v", content.WikiPages[0])
	}
	citations, err := testHandler.Queries.ListLMWikiCitations(ctx, db.ListLMWikiCitationsParams{WorkspaceID: workspaceID, RevisionID: parseUUID(first.Revision.ID)})
	if err != nil || len(citations) != 1 || citations[0].SourceType != "wiki_page_revision" || citations[0].SourceID != page.CurrentRevisionID {
		t.Fatalf("Wiki citations=%+v err=%v", citations, err)
	}

	if _, err := testHandler.Queries.UpdateWikiPage(ctx, db.UpdateWikiPageParams{
		PageID: page.ID, ExpectedRevisionNumber: page.CurrentRevisionNumber,
		NewContent: pgtype.Text{String: "mutable version two", Valid: true}, ActorID: userID,
	}); err != nil {
		t.Fatalf("update mutable Wiki page: %v", err)
	}
	unchanged := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil).Want(http.StatusOK)
	var second struct {
		Created  bool `json:"created"`
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	unchanged.JSON(&second)
	if second.Created || second.Revision.ID != first.Revision.ID {
		t.Fatalf("mutable edit changed pinned snapshot: first=%s second=%+v", first.Revision.ID, second)
	}
}

func TestLMWikiReviewRejectsRevisionAfterSourcePolicyChanges(t *testing.T) {
	resetLMWikiFixture(t)

	callLMWiki(t, http.MethodPut, "/api/lm-wiki/source-policy", map[string]any{
		"source_classes":            []string{"issue"},
		"wiki_pages":                []any{},
		"remote_generation_enabled": true,
	}).Want(http.StatusOK)
	created := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil).Want(http.StatusCreated)
	var refresh struct {
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	created.JSON(&refresh)

	callLMWiki(t, http.MethodPut, "/api/lm-wiki/source-policy", map[string]any{
		"source_classes":            []string{"issue"},
		"wiki_pages":                []any{},
		"remote_generation_enabled": false,
	}).Want(http.StatusOK)
	callLMWiki(t, http.MethodPost, "/api/lm-wiki/revisions/"+refresh.Revision.ID+"/accept", nil).Want(http.StatusConflict)

	_, err := testHandler.Queries.GetLMWikiReview(context.Background(), db.GetLMWikiReviewParams{
		WorkspaceID: parseUUID(testWorkspaceID), RevisionID: parseUUID(refresh.Revision.ID),
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale policy review error = %v, want pgx.ErrNoRows", err)
	}
}

func TestLMWikiSourcePolicyRejectsPersonalWikiAndMachineWrites(t *testing.T) {
	resetLMWikiFixture(t)
	ctx := context.Background()
	userID := parseUUID(testUserID)
	created, err := testHandler.Queries.CreateWikiPage(ctx, db.CreateWikiPageParams{
		Scope: "user", OwnerUserID: userID, Path: "lm-wiki-private.md", Title: "Private", Content: "never shared", CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create personal Wiki page: %v", err)
	}
	page := wikiPageFromCreate(created)
	t.Cleanup(func() { _ = testHandler.Queries.DeleteWikiPage(context.Background(), page.ID) })

	callLMWiki(t, http.MethodPut, "/api/lm-wiki/source-policy", map[string]any{
		"source_classes": []string{"wiki_page"},
		"wiki_pages":     []map[string]any{{"page_id": uuidToString(page.ID), "revision_number": 1}},
	}).Want(http.StatusBadRequest)

	r := newRequest(http.MethodPut, "/api/lm-wiki/source-policy", map[string]any{"source_classes": []string{}, "wiki_pages": []any{}})
	r.Header.Set("X-Actor-Source", "cloud_pat")
	testutil.Call(t, testHandler.UpdateLMWikiSourcePolicy, r).Want(http.StatusForbidden)
}

func chiRevisionID(path string) string {
	const prefix = "/api/lm-wiki/revisions/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return ""
	}
	rest := path[len(prefix):]
	for i, value := range rest {
		if value == '/' {
			return rest[:i]
		}
	}
	return rest
}

func TestLMWikiHTTPRefreshCreatesThenReturnsNoChange(t *testing.T) {
	resetLMWikiFixture(t)
	dbfx.Issue(t, "LM Wiki HTTP safe issue", testutil.Cols{"description": "public description", "priority": "high"})

	created := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil).Want(http.StatusCreated)
	var first struct {
		Created  bool `json:"created"`
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	created.JSON(&first)
	if !first.Created || first.Revision.ID == "" {
		t.Fatalf("first refresh = %+v", first)
	}

	unchanged := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil).Want(http.StatusOK)
	var second struct {
		Created  bool `json:"created"`
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	unchanged.JSON(&second)
	if second.Created || second.Revision.ID != first.Revision.ID {
		t.Fatalf("unchanged refresh = %+v, first = %+v", second, first)
	}
}

func TestLMWikiReviewSameDecisionIsIdempotentAndOppositeConflicts(t *testing.T) {
	resetLMWikiFixture(t)
	created := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil).Want(http.StatusCreated)
	var refresh struct {
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	created.JSON(&refresh)
	path := "/api/lm-wiki/revisions/" + refresh.Revision.ID + "/accept"
	callLMWiki(t, http.MethodPost, path, nil).Want(http.StatusOK)
	callLMWiki(t, http.MethodPost, path, nil).Want(http.StatusOK)
	opposite := callLMWiki(t, http.MethodPost, "/api/lm-wiki/revisions/"+refresh.Revision.ID+"/reject", map[string]any{"reason": "changed mind"}).Want(http.StatusConflict)
	if !json.Valid(opposite.Body.Bytes()) {
		t.Fatalf("opposite decision body is not JSON: %s", opposite.Body.String())
	}
	proposals, err := testHandler.Queries.ListTwinProposals(context.Background(), db.ListTwinProposalsParams{WorkspaceID: parseUUID(testWorkspaceID), ResultLimit: 10})
	if err != nil || len(proposals) != 0 {
		t.Fatalf("LM review must not synchronously create Twin proposals: %#v, %v", proposals, err)
	}
}

func TestLMWikiRollbackCitationFailureLeavesNoFragment(t *testing.T) {
	resetLMWikiFixture(t)
	original := testHandler.WikiService
	testHandler.WikiService = service.NewWikiService(testHandler.Queries, failLMWikiCitationStarter{})
	t.Cleanup(func() { testHandler.WikiService = original })

	callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil).Want(http.StatusInternalServerError)
	var revisions, citations int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*), (SELECT count(*) FROM lm_wiki_citation WHERE workspace_id = $1)
		FROM lm_wiki_revision WHERE workspace_id = $1
	`, testWorkspaceID).Scan(&revisions, &citations); err != nil {
		t.Fatalf("count LM Wiki fragments: %v", err)
	}
	if revisions != 0 || citations != 0 {
		t.Fatalf("rollback fragments: revisions = %d, citations = %d", revisions, citations)
	}
}
