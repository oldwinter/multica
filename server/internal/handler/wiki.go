package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type WikiPageSummaryResponse struct {
	ID          string  `json:"id"`
	WorkspaceID *string `json:"workspace_id"`
	Scope       string  `json:"scope"`
	ProjectID   *string `json:"project_id"`
	OwnerUserID *string `json:"owner_user_id"`
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	CreatedBy   *string `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type WikiPageResponse struct {
	WikiPageSummaryResponse
	Content string `json:"content"`
}

type CreateWikiPageRequest struct {
	Scope     string  `json:"scope"`
	ProjectID *string `json:"project_id"`
	Path      string  `json:"path"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
}

type UpdateWikiPageRequest struct {
	Path    *string `json:"path"`
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

func wikiPageSummaryFromRow(
	id, workspaceID pgtype.UUID,
	scope string,
	projectID, ownerUserID, createdBy pgtype.UUID,
	path, title string,
	createdAt, updatedAt pgtype.Timestamptz,
) WikiPageSummaryResponse {
	return WikiPageSummaryResponse{
		ID:          uuidToString(id),
		WorkspaceID: uuidPtrString(workspaceID),
		Scope:       scope,
		ProjectID:   uuidPtrString(projectID),
		OwnerUserID: uuidPtrString(ownerUserID),
		Path:        path,
		Title:       title,
		CreatedBy:   uuidPtrString(createdBy),
		CreatedAt:   timestampToString(createdAt),
		UpdatedAt:   timestampToString(updatedAt),
	}
}

func uuidPtrString(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuidToString(u)
	return &s
}

func wikiPageToResponse(page db.WikiPage) WikiPageResponse {
	return WikiPageResponse{
		WikiPageSummaryResponse: wikiPageSummaryFromRow(
			page.ID, page.WorkspaceID, page.Scope,
			page.ProjectID, page.OwnerUserID, page.CreatedBy,
			page.Path, page.Title, page.CreatedAt, page.UpdatedAt,
		),
		Content: page.Content,
	}
}

func validateWikiPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(p))
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return false
	}
	if strings.Contains(cleaned, "\\") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(cleaned), ".md") {
		return false
	}
	return true
}

func normalizeWikiPath(p string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
}

func (h *Handler) ListWikiPages(w http.ResponseWriter, r *http.Request) {
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		scope = "workspace"
	}

	switch scope {
	case "workspace":
		workspaceID := h.resolveWorkspaceID(r)
		workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
		if !ok {
			return
		}
		pages, err := h.Queries.ListWikiPagesByWorkspaceScope(r.Context(), workspaceUUID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
			return
		}
		resp := make([]WikiPageSummaryResponse, len(pages))
		for i, p := range pages {
			resp[i] = wikiPageSummaryFromRow(
				p.ID, p.WorkspaceID, p.Scope, p.ProjectID, p.OwnerUserID, p.CreatedBy,
				p.Path, p.Title, p.CreatedAt, p.UpdatedAt,
			)
		}
		writeJSON(w, http.StatusOK, resp)

	case "project":
		workspaceID := h.resolveWorkspaceID(r)
		workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
		if !ok {
			return
		}
		projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
		projectUUID, ok := parseUUIDOrBadRequest(w, projectID, "project_id")
		if !ok {
			return
		}
		if !h.projectInWorkspace(w, r, workspaceUUID, projectUUID) {
			return
		}
		pages, err := h.Queries.ListWikiPagesByProject(r.Context(), db.ListWikiPagesByProjectParams{
			WorkspaceID: workspaceUUID,
			ProjectID:   projectUUID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
			return
		}
		resp := make([]WikiPageSummaryResponse, len(pages))
		for i, p := range pages {
			resp[i] = wikiPageSummaryFromRow(
				p.ID, p.WorkspaceID, p.Scope, p.ProjectID, p.OwnerUserID, p.CreatedBy,
				p.Path, p.Title, p.CreatedAt, p.UpdatedAt,
			)
		}
		writeJSON(w, http.StatusOK, resp)

	case "user":
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		// Personal wiki is cross-workspace and private to the signed-in user.
		ownerUUID := parseUUID(userID)
		pages, err := h.Queries.ListWikiPagesByOwner(r.Context(), ownerUUID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
			return
		}
		resp := make([]WikiPageSummaryResponse, len(pages))
		for i, p := range pages {
			resp[i] = wikiPageSummaryFromRow(
				p.ID, p.WorkspaceID, p.Scope, p.ProjectID, p.OwnerUserID, p.CreatedBy,
				p.Path, p.Title, p.CreatedAt, p.UpdatedAt,
			)
		}
		writeJSON(w, http.StatusOK, resp)

	default:
		writeError(w, http.StatusBadRequest, "scope must be workspace, project, or user")
	}
}

func (h *Handler) projectInWorkspace(w http.ResponseWriter, r *http.Request, workspaceID, projectID pgtype.UUID) bool {
	_, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID:          projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return false
		}
		writeError(w, http.StatusInternalServerError, "failed to load project")
		return false
	}
	return true
}

// loadWikiPageForUser loads a page and enforces ACL.
// - workspace/project pages must belong to the request workspace.
// - user pages are cross-workspace and only readable by their owner.
func (h *Handler) loadWikiPageForUser(w http.ResponseWriter, r *http.Request, id string) (db.WikiPage, bool) {
	pageUUID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return db.WikiPage{}, false
	}
	page, err := h.Queries.GetWikiPage(r.Context(), pageUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "wiki page not found")
			return db.WikiPage{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load wiki page")
		return db.WikiPage{}, false
	}

	if page.Scope == "user" {
		userID, ok := requireUserID(w, r)
		if !ok {
			return db.WikiPage{}, false
		}
		if !page.OwnerUserID.Valid || uuidToString(page.OwnerUserID) != userID {
			writeError(w, http.StatusNotFound, "wiki page not found")
			return db.WikiPage{}, false
		}
		return page, true
	}

	workspaceID := h.resolveWorkspaceID(r)
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return db.WikiPage{}, false
	}
	if !page.WorkspaceID.Valid || uuidToString(page.WorkspaceID) != uuidToString(workspaceUUID) {
		writeError(w, http.StatusNotFound, "wiki page not found")
		return db.WikiPage{}, false
	}
	return page, true
}

func (h *Handler) GetWikiPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	page, ok := h.loadWikiPageForUser(w, r, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, wikiPageToResponse(page))
}

func (h *Handler) CreateWikiPage(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	userUUID := parseUUID(userID)

	var req CreateWikiPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "workspace"
	}
	path := normalizeWikiPath(req.Path)
	if !validateWikiPath(path) {
		writeError(w, http.StatusBadRequest, "path must be a relative .md path without '..'")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	var workspaceID pgtype.UUID
	var projectID pgtype.UUID
	var ownerUserID pgtype.UUID
	switch scope {
	case "workspace":
		wsHeader := h.resolveWorkspaceID(r)
		wsUUID, ok := parseUUIDOrBadRequest(w, wsHeader, "workspace_id")
		if !ok {
			return
		}
		workspaceID = wsUUID
	case "project":
		wsHeader := h.resolveWorkspaceID(r)
		wsUUID, ok := parseUUIDOrBadRequest(w, wsHeader, "workspace_id")
		if !ok {
			return
		}
		if req.ProjectID == nil || strings.TrimSpace(*req.ProjectID) == "" {
			writeError(w, http.StatusBadRequest, "project_id is required for project scope")
			return
		}
		projectUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(*req.ProjectID), "project_id")
		if !ok {
			return
		}
		if !h.projectInWorkspace(w, r, wsUUID, projectUUID) {
			return
		}
		workspaceID = wsUUID
		projectID = projectUUID
	case "user":
		// Cross-workspace personal library: no workspace_id.
		ownerUserID = userUUID
	default:
		writeError(w, http.StatusBadRequest, "scope must be workspace, project, or user")
		return
	}

	page, err := h.Queries.CreateWikiPage(r.Context(), db.CreateWikiPageParams{
		WorkspaceID: workspaceID,
		Scope:       scope,
		ProjectID:   projectID,
		OwnerUserID: ownerUserID,
		Path:        path,
		Title:       title,
		Content:     req.Content,
		CreatedBy:   userUUID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "a wiki page already exists at this path")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create wiki page")
		return
	}
	writeJSON(w, http.StatusCreated, wikiPageToResponse(page))
}

func (h *Handler) UpdateWikiPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	page, ok := h.loadWikiPageForUser(w, r, id)
	if !ok {
		return
	}

	var req UpdateWikiPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateWikiPageParams{ID: page.ID}
	if req.Path != nil {
		path := normalizeWikiPath(*req.Path)
		if !validateWikiPath(path) {
			writeError(w, http.StatusBadRequest, "path must be a relative .md path without '..'")
			return
		}
		params.Path = pgtype.Text{String: path, Valid: true}
	}
	if req.Title != nil {
		params.Title = pgtype.Text{String: strings.TrimSpace(*req.Title), Valid: true}
	}
	if req.Content != nil {
		params.Content = pgtype.Text{String: *req.Content, Valid: true}
	}

	updated, err := h.Queries.UpdateWikiPage(r.Context(), params)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "a wiki page already exists at this path")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update wiki page")
		return
	}
	writeJSON(w, http.StatusOK, wikiPageToResponse(updated))
}

func (h *Handler) DeleteWikiPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	page, ok := h.loadWikiPageForUser(w, r, id)
	if !ok {
		return
	}
	if err := h.Queries.DeleteWikiPage(r.Context(), page.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete wiki page")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
