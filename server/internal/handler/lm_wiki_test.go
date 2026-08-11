package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/service"
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
		`DELETE FROM issue WHERE workspace_id = $1 AND title LIKE 'LM Wiki HTTP %'`,
	} {
		if _, err := testPool.Exec(ctx, query, testWorkspaceID); err != nil {
			t.Fatalf("reset LM Wiki fixture: %v", err)
		}
	}
}

func callLMWiki(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := newRequest(method, path, body)
	if id := chiRevisionID(path); id != "" {
		r = withURLParam(r, "revisionId", id)
	}
	switch {
	case method == http.MethodPost && path == "/api/lm-wiki/refresh":
		testHandler.RefreshLMWiki(w, r)
	case method == http.MethodGet && path == "/api/lm-wiki":
		testHandler.GetLMWiki(w, r)
	case method == http.MethodPost && len(path) > len("/api/lm-wiki/revisions/") && path[len(path)-7:] == "/accept":
		testHandler.AcceptLMWikiRevision(w, r)
	case method == http.MethodPost && len(path) > len("/api/lm-wiki/revisions/") && path[len(path)-7:] == "/reject":
		testHandler.RejectLMWikiRevision(w, r)
	default:
		t.Fatalf("unsupported LM Wiki test request: %s %s", method, path)
	}
	return w
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
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, `
		INSERT INTO issue (workspace_id, title, description, status, priority, creator_type, creator_id, number)
		VALUES ($1, 'LM Wiki HTTP safe issue', 'public description', 'todo', 'high', 'member', $2, 9001)
	`, testWorkspaceID, testUserID); err != nil {
		t.Fatalf("seed issue: %v", err)
	}

	created := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("first refresh status = %d, body = %s", created.Code, created.Body.String())
	}
	var first struct {
		Created  bool `json:"created"`
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first refresh: %v", err)
	}
	if !first.Created || first.Revision.ID == "" {
		t.Fatalf("first refresh = %+v", first)
	}

	unchanged := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil)
	if unchanged.Code != http.StatusOK {
		t.Fatalf("unchanged refresh status = %d, body = %s", unchanged.Code, unchanged.Body.String())
	}
	var second struct {
		Created  bool `json:"created"`
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	if err := json.Unmarshal(unchanged.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode unchanged refresh: %v", err)
	}
	if second.Created || second.Revision.ID != first.Revision.ID {
		t.Fatalf("unchanged refresh = %+v, first = %+v", second, first)
	}
}

func TestLMWikiReviewSameDecisionIsIdempotentAndOppositeConflicts(t *testing.T) {
	resetLMWikiFixture(t)
	created := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil)
	var refresh struct {
		Revision struct {
			ID string `json:"id"`
		} `json:"revision"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &refresh); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	path := "/api/lm-wiki/revisions/" + refresh.Revision.ID + "/accept"
	first := callLMWiki(t, http.MethodPost, path, nil)
	retry := callLMWiki(t, http.MethodPost, path, nil)
	opposite := callLMWiki(t, http.MethodPost, "/api/lm-wiki/revisions/"+refresh.Revision.ID+"/reject", map[string]any{"reason": "changed mind"})
	if first.Code != http.StatusOK || retry.Code != http.StatusOK {
		t.Fatalf("accept statuses = %d, %d", first.Code, retry.Code)
	}
	if opposite.Code != http.StatusConflict || !json.Valid(opposite.Body.Bytes()) {
		t.Fatalf("opposite decision status = %d, body = %s", opposite.Code, opposite.Body.String())
	}
	proposals, err := testHandler.Queries.ListTwinProposals(context.Background(), db.ListTwinProposalsParams{WorkspaceID: parseUUID(testWorkspaceID), ResultLimit: 10})
	if err != nil || len(proposals) != 1 || proposals[0].Kind != "initial" {
		t.Fatalf("automatic Twin proposals = %#v, %v", proposals, err)
	}
}

func TestLMWikiRollbackCitationFailureLeavesNoFragment(t *testing.T) {
	resetLMWikiFixture(t)
	original := testHandler.WikiService
	testHandler.WikiService = service.NewWikiService(testHandler.Queries, failLMWikiCitationStarter{})
	t.Cleanup(func() { testHandler.WikiService = original })

	response := callLMWiki(t, http.MethodPost, "/api/lm-wiki/refresh", nil)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed refresh status = %d, body = %s", response.Code, response.Body.String())
	}
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
