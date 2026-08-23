package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestValidateWikiPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		ok   bool
	}{
		{"index.md", true},
		{"concepts/agents.md", true},
		{"Concepts/Agents.md", true},
		{"", false},
		{"/abs.md", false},
		{"../escape.md", false},
		{"foo/../../bar.md", false},
		{"no-extension", false},
		{"readme.txt", false},
		{"./nested/page.md", true},
	}
	for _, tc := range cases {
		path := tc.path
		if path == "./nested/page.md" {
			path = normalizeWikiPath(path)
		}
		if got := validateWikiPath(path); got != tc.ok {
			t.Fatalf("validateWikiPath(%q) = %v, want %v", tc.path, got, tc.ok)
		}
	}
}

func TestNormalizeWikiPath(t *testing.T) {
	t.Parallel()
	if got := normalizeWikiPath(" ./a/b/../c.md "); got != "a/c.md" {
		t.Fatalf("normalize = %q", got)
	}
}

func TestUuidPtrStringAndWikiSummary(t *testing.T) {
	t.Parallel()
	if uuidPtrString(pgtype.UUID{}) != nil {
		t.Fatal("invalid uuid should be nil pointer")
	}
	id := util.MustParseUUID("11111111-1111-1111-1111-111111111111")
	ptr := uuidPtrString(id)
	if ptr == nil || *ptr != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("got %v", ptr)
	}

	resp := wikiPageSummaryFromRow(
		id, pgtype.UUID{}, "user",
		pgtype.UUID{}, id, id,
		"notes.md", "Notes",
		pgtype.Timestamptz{Time: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), Valid: true},
		pgtype.Timestamptz{Time: time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC), Valid: true},
		1, id, "sha256:digest", "human", "member", id,
	)
	if resp.WorkspaceID != nil {
		t.Fatalf("personal page should have null workspace_id, got %v", resp.WorkspaceID)
	}
	if resp.OwnerUserID == nil || *resp.OwnerUserID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("owner = %v", resp.OwnerUserID)
	}
	if resp.CreatedAt != "2026-08-05T00:00:00Z" {
		t.Fatalf("created_at = %q", resp.CreatedAt)
	}

	full := wikiPageToResponse(db.WikiPage{
		ID: id, Scope: "user", OwnerUserID: id, Path: "notes.md", Title: "Notes", Content: "# hi",
		UpdatedAt: pgtype.Timestamptz{Time: time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC), Valid: true},
	})
	if full.Content != "# hi" || full.WorkspaceID != nil {
		t.Fatalf("response = %+v", full)
	}
}

func TestDecodeWikiProposalUsesFrozenAgentContract(t *testing.T) {
	t.Parallel()
	body := `{"base_revision_number":3,"proposed_path":"playbook/review.md","proposed_title":"Review","proposed_content":"# Review","rationale":"evidence","evidence_refs":["issue:1"],"agent_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","idempotency_key":"run-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/pages/page/proposals", strings.NewReader(body))
	w := httptest.NewRecorder()
	var got createWikiProposalRequest

	if !decodeWikiJSON(w, req, &got) {
		t.Fatalf("frozen proposal body was rejected: status=%d body=%s", w.Code, w.Body.String())
	}
	if got.ProposedPath == nil || *got.ProposedPath != "playbook/review.md" || got.AgentID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("decoded proposal = %+v", got)
	}
}

func TestRequireWikiHumanRejectsCloudMachineCredential(t *testing.T) {
	t.Parallel()
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPut, "/api/wiki/pages/page", nil)
	req.Header.Set("X-User-ID", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	req.Header.Set("X-Actor-Source", "cloud_pat")
	w := httptest.NewRecorder()

	if _, ok := h.requireWikiHuman(w, req, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); ok {
		t.Fatal("cloud machine credential must not pass the human Wiki guard")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

// wikiMockDB implements the sqlc QueryRow/Exec surface used by wiki handlers.
type wikiMockDB struct {
	db.DBTX
	// list mode: rows returned for Query
	listRows []db.WikiPage
	listErr  error
	// single-row QueryRow
	page    db.WikiPage
	pageErr error
	// create/update return
	writePage db.WikiPage
	writeErr  error
	// project lookup
	projectExists bool
	projectErr    error
	// delete
	deleteErr error
	// last SQL for assertions
	lastSQL string
}

func (m *wikiMockDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	m.lastSQL = sql
	if m.listErr != nil {
		return nil, m.listErr
	}
	return &wikiMockRows{rows: m.listRows, i: -1}, nil
}

func (m *wikiMockDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	m.lastSQL = sql
	// Project lookup scans a single id.
	if strings.Contains(sql, "FROM project") {
		return &wikiMockRow{kind: "project", projectOK: m.projectExists, err: m.projectErr}
	}
	// Create/update RETURNING uses QueryRow with many fields.
	if strings.Contains(sql, "INSERT INTO wiki_page") || strings.Contains(sql, "UPDATE wiki_page") {
		return &wikiMockRow{kind: "page", page: m.writePage, err: m.writeErr}
	}
	return &wikiMockRow{kind: "page", page: m.page, err: m.pageErr}
}

func (m *wikiMockDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	m.lastSQL = sql
	if m.deleteErr != nil {
		return pgconn.CommandTag{}, m.deleteErr
	}
	return pgconn.NewCommandTag("DELETE 1"), nil
}

type wikiMockRows struct {
	rows []db.WikiPage
	i    int
	err  error
}

func (r *wikiMockRows) Close()                                       {}
func (r *wikiMockRows) Err() error                                   { return r.err }
func (r *wikiMockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *wikiMockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *wikiMockRows) RawValues() [][]byte                          { return nil }
func (r *wikiMockRows) Values() ([]any, error)                       { return nil, nil }
func (r *wikiMockRows) Conn() *pgx.Conn                              { return nil }

func (r *wikiMockRows) Next() bool {
	r.i++
	return r.i < len(r.rows)
}

func (r *wikiMockRows) Scan(dest ...interface{}) error {
	p := r.rows[r.i]
	// List queries scan summary fields without content.
	*(dest[0].(*pgtype.UUID)) = p.ID
	*(dest[1].(*pgtype.UUID)) = p.WorkspaceID
	*(dest[2].(*string)) = p.Scope
	*(dest[3].(*pgtype.UUID)) = p.ProjectID
	*(dest[4].(*pgtype.UUID)) = p.OwnerUserID
	*(dest[5].(*string)) = p.Path
	*(dest[6].(*string)) = p.Title
	*(dest[7].(*pgtype.UUID)) = p.CreatedBy
	*(dest[8].(*pgtype.Timestamptz)) = p.CreatedAt
	*(dest[9].(*pgtype.Timestamptz)) = p.UpdatedAt
	*(dest[10].(*int64)) = p.CurrentRevisionNumber
	*(dest[11].(*pgtype.UUID)) = p.CurrentRevisionID
	*(dest[12].(*string)) = p.ContentDigest
	*(dest[13].(*string)) = p.LastSourceKind
	*(dest[14].(*string)) = p.LastActorType
	*(dest[15].(*pgtype.UUID)) = p.LastActorID
	return nil
}

type wikiMockRow struct {
	kind      string
	page      db.WikiPage
	err       error
	projectOK bool
}

func (r *wikiMockRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if r.kind == "project" {
		if !r.projectOK {
			return pgx.ErrNoRows
		}
		// GetProjectInWorkspace scans the full project row.
		*(dest[0].(*pgtype.UUID)) = util.MustParseUUID("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
		*(dest[1].(*pgtype.UUID)) = util.MustParseUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
		*(dest[2].(*string)) = "Project"
		*(dest[3].(*pgtype.Text)) = pgtype.Text{}
		*(dest[4].(*pgtype.Text)) = pgtype.Text{}
		*(dest[5].(*string)) = "planned"
		*(dest[6].(*pgtype.Text)) = pgtype.Text{}
		*(dest[7].(*pgtype.UUID)) = pgtype.UUID{}
		*(dest[8].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
		*(dest[9].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
		*(dest[10].(*string)) = "none"
		*(dest[11].(*pgtype.Date)) = pgtype.Date{}
		*(dest[12].(*pgtype.Date)) = pgtype.Date{}
		return nil
	}
	p := r.page
	*(dest[0].(*pgtype.UUID)) = p.ID
	*(dest[1].(*pgtype.UUID)) = p.WorkspaceID
	*(dest[2].(*string)) = p.Scope
	*(dest[3].(*pgtype.UUID)) = p.ProjectID
	*(dest[4].(*pgtype.UUID)) = p.OwnerUserID
	*(dest[5].(*string)) = p.Path
	*(dest[6].(*string)) = p.Title
	*(dest[7].(*string)) = p.Content
	*(dest[8].(*pgtype.UUID)) = p.CreatedBy
	*(dest[9].(*pgtype.Timestamptz)) = p.CreatedAt
	*(dest[10].(*pgtype.Timestamptz)) = p.UpdatedAt
	*(dest[11].(*int64)) = p.CurrentRevisionNumber
	*(dest[12].(*pgtype.UUID)) = p.CurrentRevisionID
	*(dest[13].(*string)) = p.ContentDigest
	*(dest[14].(*string)) = p.LastSourceKind
	*(dest[15].(*string)) = p.LastActorType
	*(dest[16].(*pgtype.UUID)) = p.LastActorID
	return nil
}

func TestWikiHandlersWithMockDB(t *testing.T) {
	const (
		userID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		wsID        = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		otherUserID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		pageID      = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		projectID   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	userUUID := util.MustParseUUID(userID)
	wsUUID := util.MustParseUUID(wsID)
	pageUUID := util.MustParseUUID(pageID)
	projectUUID := util.MustParseUUID(projectID)

	newHandler := func(mock *wikiMockDB) *Handler {
		return &Handler{Queries: db.New(mock)}
	}

	authReq := func(method, path, body string) *http.Request {
		var r *http.Request
		if body != "" {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		r.Header.Set("X-Workspace-ID", wsID)
		r.Header.Set("X-User-ID", userID)
		return r
	}

	_ = userUUID

	t.Run("list_workspace_ok", func(t *testing.T) {
		mock := &wikiMockDB{listRows: []db.WikiPage{{
			ID: pageUUID, WorkspaceID: wsUUID, Scope: "workspace", Path: "index.md", Title: "Home",
		}}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=workspace", ""))
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var pages []WikiPageSummaryResponse
		if err := json.Unmarshal(w.Body.Bytes(), &pages); err != nil || len(pages) != 1 {
			t.Fatalf("pages=%v err=%v body=%s", pages, err, w.Body.String())
		}
	})

	t.Run("list_workspace_db_error", func(t *testing.T) {
		h := newHandler(&wikiMockDB{listErr: errors.New("db")})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=workspace", ""))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("list_project_requires_project_id", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=project", ""))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("list_project_not_found", func(t *testing.T) {
		h := newHandler(&wikiMockDB{projectExists: false})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=project&project_id="+projectID, ""))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("list_project_ok", func(t *testing.T) {
		mock := &wikiMockDB{
			projectExists: true,
			listRows: []db.WikiPage{{
				ID: pageUUID, WorkspaceID: wsUUID, Scope: "project", ProjectID: projectUUID,
				Path: "proj.md", Title: "P",
			}},
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=project&project_id="+projectID, ""))
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("list_user_cross_workspace", func(t *testing.T) {
		mock := &wikiMockDB{listRows: []db.WikiPage{{
			ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md", Title: "Me",
		}}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=user", ""))
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var pages []WikiPageSummaryResponse
		_ = json.Unmarshal(w.Body.Bytes(), &pages)
		if len(pages) != 1 || pages[0].WorkspaceID != nil {
			t.Fatalf("expected personal page without workspace, got %+v", pages)
		}
	})

	t.Run("list_invalid_scope", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=galaxy", ""))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("get_personal_page_as_owner", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md", Content: "x",
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := authReq(http.MethodGet, "/api/wiki/pages/"+pageID, "")
		req = withChiID(req, pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("get_personal_page_forbidden_other_owner", func(t *testing.T) {
		other := util.MustParseUUID(otherUserID)
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, Scope: "user", OwnerUserID: other, Path: "secret.md", Content: "nope",
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := authReq(http.MethodGet, "/api/wiki/pages/"+pageID, "")
		req = withChiID(req, pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("task_token_cannot_read_personal_page", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "secret.md", Content: "private",
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodGet, "/api/wiki/pages/"+pageID, ""), pageID)
		req.Header.Set("X-Actor-Source", "task_token")
		req.Header.Set("X-Agent-ID", "ffffffff-ffff-ffff-ffff-ffffffffffff")
		h.GetWikiPage(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("task_token_cannot_directly_update_shared_page", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, WorkspaceID: wsUUID, Scope: "workspace", Path: "shared.md", CurrentRevisionNumber: 1,
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":1,"content":"forbidden"}`), pageID)
		req.Header.Set("X-Actor-Source", "task_token")
		req.Header.Set("X-Agent-ID", "ffffffff-ffff-ffff-ffff-ffffffffffff")
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("task_token_cannot_delete_shared_page", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, WorkspaceID: wsUUID, Scope: "workspace", Path: "shared.md", CurrentRevisionNumber: 1,
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodDelete, "/api/wiki/pages/"+pageID, ""), pageID)
		req.Header.Set("X-Actor-Source", "task_token")
		req.Header.Set("X-Agent-ID", "ffffffff-ffff-ffff-ffff-ffffffffffff")
		h.DeleteWikiPage(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("get_workspace_page_wrong_workspace", func(t *testing.T) {
		otherWS := util.MustParseUUID("99999999-9999-9999-9999-999999999999")
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, Scope: "workspace", WorkspaceID: otherWS, Path: "x.md", Content: "x",
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := authReq(http.MethodGet, "/api/wiki/pages/"+pageID, "")
		req = withChiID(req, pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("get_missing_page", func(t *testing.T) {
		h := newHandler(&wikiMockDB{pageErr: pgx.ErrNoRows})
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodGet, "/api/wiki/pages/"+pageID, ""), pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_workspace_page", func(t *testing.T) {
		mock := &wikiMockDB{writePage: db.WikiPage{
			ID: pageUUID, WorkspaceID: wsUUID, Scope: "workspace", Path: "new.md", Title: "New", Content: "# n",
			CreatedBy: userUUID,
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		body := `{"scope":"workspace","path":"new.md","title":"New","content":"# n"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("create_personal_page_null_workspace", func(t *testing.T) {
		mock := &wikiMockDB{writePage: db.WikiPage{
			ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "journal.md", Title: "journal", Content: "hi",
			CreatedBy: userUUID,
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		body := `{"scope":"user","path":"journal.md","content":"hi"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var resp WikiPageResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.WorkspaceID != nil {
			t.Fatalf("personal create must not set workspace_id: %+v", resp)
		}
	})

	t.Run("create_invalid_path", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		body := `{"scope":"workspace","path":"../x.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_invalid_json", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", "{"))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_project_missing_project_id", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		body := `{"scope":"project","path":"a.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("create_conflict", func(t *testing.T) {
		mock := &wikiMockDB{writeErr: &pgconn.PgError{Code: "23505"}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		body := `{"scope":"workspace","path":"dup.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		// isUniqueViolation may or may not recognize pgconn.PgError depending on helper.
		if w.Code != http.StatusConflict && w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("create_invalid_scope", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		body := `{"scope":"galaxy","path":"a.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("update_ok", func(t *testing.T) {
		mock := &wikiMockDB{
			page: db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md", Content: "old"},
			writePage: db.WikiPage{
				ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md", Content: "new", Title: "Me",
			},
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":1,"content":"new"}`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("update_invalid_path", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md"}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":1,"path":"/abs.md"}`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("update_invalid_json", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md"}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("delete_ok", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md"}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodDelete, "/api/wiki/pages/"+pageID, ""), pageID)
		h.DeleteWikiPage(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("delete_db_error", func(t *testing.T) {
		mock := &wikiMockDB{
			page:      db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md"},
			deleteErr: errors.New("nope"),
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodDelete, "/api/wiki/pages/"+pageID, ""), pageID)
		h.DeleteWikiPage(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("validate_rejects_backslash", func(t *testing.T) {
		if validateWikiPath(`foo\bar.md`) {
			t.Fatal("backslash path should be rejected")
		}
	})

	t.Run("list_default_scope_workspace", func(t *testing.T) {
		h := newHandler(&wikiMockDB{listRows: nil})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages", ""))
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("list_workspace_invalid_workspace_header", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages?scope=workspace", nil)
		req.Header.Set("X-Workspace-ID", "not-uuid")
		req.Header.Set("X-User-ID", userID)
		h.ListWikiPages(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("list_project_db_error", func(t *testing.T) {
		h := newHandler(&wikiMockDB{projectExists: true, listErr: errors.New("db")})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=project&project_id="+projectID, ""))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("list_user_db_error", func(t *testing.T) {
		h := newHandler(&wikiMockDB{listErr: errors.New("db")})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=user", ""))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("list_user_unauthenticated", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages?scope=user", nil)
		req.Header.Set("X-Workspace-ID", wsID)
		h.ListWikiPages(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("project_lookup_internal_error", func(t *testing.T) {
		h := newHandler(&wikiMockDB{projectErr: errors.New("db")})
		w := httptest.NewRecorder()
		h.ListWikiPages(w, authReq(http.MethodGet, "/api/wiki/pages?scope=project&project_id="+projectID, ""))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("get_load_internal_error", func(t *testing.T) {
		h := newHandler(&wikiMockDB{pageErr: errors.New("db")})
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodGet, "/api/wiki/pages/"+pageID, ""), pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("get_invalid_page_id", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodGet, "/api/wiki/pages/not-uuid", ""), "not-uuid")
		h.GetWikiPage(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("get_workspace_page_ok", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, Scope: "workspace", WorkspaceID: wsUUID, Path: "x.md", Content: "body",
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodGet, "/api/wiki/pages/"+pageID, ""), pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("get_personal_unauthenticated", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "me.md",
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages/"+pageID, nil)
		req.Header.Set("X-Workspace-ID", wsID)
		req = withChiID(req, pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_defaults_title_from_path", func(t *testing.T) {
		mock := &wikiMockDB{writePage: db.WikiPage{
			ID: pageUUID, WorkspaceID: wsUUID, Scope: "workspace", Path: "deep/topic.md", Title: "topic", Content: "",
			CreatedBy: userUUID,
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		body := `{"scope":"workspace","path":"deep/topic.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("create_default_scope_workspace", func(t *testing.T) {
		mock := &wikiMockDB{writePage: db.WikiPage{
			ID: pageUUID, WorkspaceID: wsUUID, Scope: "workspace", Path: "a.md", Title: "a",
			CreatedBy: userUUID,
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		body := `{"path":"a.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("create_project_ok", func(t *testing.T) {
		mock := &wikiMockDB{
			projectExists: true,
			writePage: db.WikiPage{
				ID: pageUUID, WorkspaceID: wsUUID, Scope: "project", ProjectID: projectUUID,
				Path: "p.md", Title: "P", CreatedBy: userUUID,
			},
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		body := `{"scope":"project","project_id":"` + projectID + `","path":"p.md","title":"P"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("create_project_not_found", func(t *testing.T) {
		h := newHandler(&wikiMockDB{projectExists: false})
		w := httptest.NewRecorder()
		body := `{"scope":"project","project_id":"` + projectID + `","path":"p.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_generic_db_error", func(t *testing.T) {
		h := newHandler(&wikiMockDB{writeErr: errors.New("db")})
		w := httptest.NewRecorder()
		body := `{"scope":"workspace","path":"e.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_unauthenticated", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/wiki/pages", strings.NewReader(`{"path":"a.md"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Workspace-ID", wsID)
		h.CreateWikiPage(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("update_path_and_title", func(t *testing.T) {
		mock := &wikiMockDB{
			page: db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "old.md"},
			writePage: db.WikiPage{
				ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "new.md", Title: "New", Content: "c",
			},
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":1,"path":"new.md","title":"New","content":"c"}`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("update_conflict", func(t *testing.T) {
		mock := &wikiMockDB{
			page:     db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "old.md"},
			writeErr: &pgconn.PgError{Code: "23505"},
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":1,"path":"dup.md"}`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("stale_update_returns_structured_revision_conflict", func(t *testing.T) {
		mock := &wikiMockDB{
			page: db.WikiPage{
				ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "old.md", CurrentRevisionNumber: 3,
			},
			writeErr: pgx.ErrNoRows,
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":2,"content":"stale"}`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var response struct {
			Code                  string `json:"code"`
			CurrentRevisionNumber int64  `json:"current_revision_number"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode conflict: %v", err)
		}
		if response.Code != "wiki_revision_conflict" || response.CurrentRevisionNumber != 3 {
			t.Fatalf("conflict=%+v body=%s", response, w.Body.String())
		}
	})

	t.Run("update_db_error", func(t *testing.T) {
		mock := &wikiMockDB{
			page:     db.WikiPage{ID: pageUUID, Scope: "user", OwnerUserID: userUUID, Path: "old.md"},
			writeErr: errors.New("db"),
		}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":1,"title":"x"}`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("update_load_fail", func(t *testing.T) {
		h := newHandler(&wikiMockDB{pageErr: pgx.ErrNoRows})
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodPut, "/api/wiki/pages/"+pageID, `{"expected_revision_number":1,"title":"x"}`), pageID)
		h.UpdateWikiPage(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("delete_load_fail", func(t *testing.T) {
		h := newHandler(&wikiMockDB{pageErr: pgx.ErrNoRows})
		w := httptest.NewRecorder()
		req := withChiID(authReq(http.MethodDelete, "/api/wiki/pages/"+pageID, ""), pageID)
		h.DeleteWikiPage(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_workspace_invalid_header", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/wiki/pages", strings.NewReader(`{"scope":"workspace","path":"a.md"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Workspace-ID", "bad")
		req.Header.Set("X-User-ID", userID)
		h.CreateWikiPage(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_project_invalid_workspace_header", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/wiki/pages", strings.NewReader(`{"scope":"project","project_id":"`+projectID+`","path":"a.md"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Workspace-ID", "bad")
		req.Header.Set("X-User-ID", userID)
		h.CreateWikiPage(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_project_invalid_project_id", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		body := `{"scope":"project","project_id":"not-a-uuid","path":"a.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("create_unique_violation_is_conflict", func(t *testing.T) {
		h := newHandler(&wikiMockDB{writeErr: &pgconn.PgError{Code: "23505"}})
		w := httptest.NewRecorder()
		body := `{"scope":"user","path":"dup.md"}`
		h.CreateWikiPage(w, authReq(http.MethodPost, "/api/wiki/pages", body))
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("list_project_invalid_workspace", func(t *testing.T) {
		h := newHandler(&wikiMockDB{})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages?scope=project&project_id="+projectID, nil)
		req.Header.Set("X-Workspace-ID", "bad")
		req.Header.Set("X-User-ID", userID)
		h.ListWikiPages(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", w.Code)
		}
	})

	t.Run("get_workspace_page_missing_workspace_header", func(t *testing.T) {
		mock := &wikiMockDB{page: db.WikiPage{
			ID: pageUUID, Scope: "workspace", WorkspaceID: wsUUID, Path: "x.md", Content: "body",
		}}
		h := newHandler(mock)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages/"+pageID, nil)
		req.Header.Set("X-User-ID", userID)
		// No workspace identity means the tenant-bound query cannot authorize the
		// row. Keep existence private with the same 404 as any other denied read.
		req = withChiID(req, pageID)
		h.GetWikiPage(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
}

func withChiID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
