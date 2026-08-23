package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/analytics"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	wikiRequestBodyLimit = 2 * 1024 * 1024
	wikiSearchLimit      = 100
)

func (h *Handler) wikiKnowledgeService() service.WikiKnowledge {
	if h.WikiKnowledge != nil {
		return h.WikiKnowledge
	}
	return service.NewWikiKnowledgeService(h.Bus)
}

type WikiPageSummaryResponse struct {
	ID                    string  `json:"id"`
	WorkspaceID           *string `json:"workspace_id"`
	Scope                 string  `json:"scope"`
	ProjectID             *string `json:"project_id"`
	OwnerUserID           *string `json:"owner_user_id"`
	Path                  string  `json:"path"`
	Title                 string  `json:"title"`
	CreatedBy             *string `json:"created_by"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
	CurrentRevisionNumber int64   `json:"current_revision_number"`
	CurrentRevisionID     string  `json:"current_revision_id"`
	ContentDigest         string  `json:"content_digest"`
	LastSourceKind        string  `json:"last_source_kind"`
	LastActorType         string  `json:"last_actor_type"`
	LastActorID           *string `json:"last_actor_id"`
}

type WikiPageResponse struct {
	WikiPageSummaryResponse
	Content string `json:"content"`
}

type WikiPageRevisionResponse struct {
	ID             string  `json:"id"`
	PageID         string  `json:"page_id"`
	RevisionNumber int64   `json:"revision_number"`
	Path           string  `json:"path"`
	Title          string  `json:"title"`
	Content        string  `json:"content"`
	ContentDigest  string  `json:"content_digest"`
	ActorType      string  `json:"actor_type"`
	ActorID        *string `json:"actor_id"`
	SourceKind     string  `json:"source_kind"`
	SourceRefID    *string `json:"source_ref_id"`
	CreatedAt      string  `json:"created_at"`
}

type WikiPageEditProposalResponse struct {
	ID                 string          `json:"id"`
	PageID             string          `json:"page_id"`
	BaseRevisionNumber int64           `json:"base_revision_number"`
	ProposedPath       string          `json:"proposed_path"`
	ProposedTitle      string          `json:"proposed_title"`
	ProposedContent    string          `json:"proposed_content"`
	ContentDigest      string          `json:"content_digest"`
	Rationale          string          `json:"rationale"`
	EvidenceRefs       json.RawMessage `json:"evidence_refs"`
	AgentID            string          `json:"agent_id"`
	IdempotencyKey     string          `json:"idempotency_key"`
	Status             string          `json:"status"`
	ReviewedByID       *string         `json:"reviewed_by_id"`
	ReviewReason       *string         `json:"review_reason"`
	ReviewedAt         *time.Time      `json:"reviewed_at"`
	AcceptedRevisionID *string         `json:"accepted_revision_id"`
	CreatedAt          string          `json:"created_at"`
}

type CreateWikiPageRequest struct {
	Scope     string  `json:"scope"`
	ProjectID *string `json:"project_id"`
	Path      string  `json:"path"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
}

type createPersonalWikiPageRequest struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type UpdateWikiPageRequest struct {
	ExpectedRevisionNumber *int64  `json:"expected_revision_number"`
	Path                   *string `json:"path"`
	Title                  *string `json:"title"`
	Content                *string `json:"content"`
}

type restoreWikiPageRequest struct {
	ExpectedRevisionNumber *int64 `json:"expected_revision_number"`
}

type createWikiProposalRequest struct {
	BaseRevisionNumber int64    `json:"base_revision_number"`
	ProposedPath       *string  `json:"proposed_path"`
	ProposedTitle      *string  `json:"proposed_title"`
	ProposedContent    *string  `json:"proposed_content"`
	Rationale          string   `json:"rationale"`
	EvidenceRefs       []string `json:"evidence_refs"`
	AgentID            string   `json:"agent_id"`
	IdempotencyKey     string   `json:"idempotency_key"`
}

type acceptWikiProposalRequest struct {
	ExpectedRevisionNumber *int64  `json:"expected_revision_number"`
	Path                   *string `json:"path"`
	Title                  *string `json:"title"`
	Content                *string `json:"content"`
}

func wikiPageSummaryFromRow(
	id, workspaceID pgtype.UUID,
	scope string,
	projectID, ownerUserID, createdBy pgtype.UUID,
	path, title string,
	createdAt, updatedAt pgtype.Timestamptz,
	currentRevisionNumber int64,
	currentRevisionID pgtype.UUID,
	contentDigest, lastSourceKind, lastActorType string,
	lastActorID pgtype.UUID,
) WikiPageSummaryResponse {
	return WikiPageSummaryResponse{
		ID: uuidToString(id), WorkspaceID: uuidPtrString(workspaceID), Scope: scope,
		ProjectID: uuidPtrString(projectID), OwnerUserID: uuidPtrString(ownerUserID),
		Path: path, Title: title, CreatedBy: uuidPtrString(createdBy),
		CreatedAt: timestampToString(createdAt), UpdatedAt: timestampToString(updatedAt),
		CurrentRevisionNumber: currentRevisionNumber, CurrentRevisionID: uuidToString(currentRevisionID),
		ContentDigest: contentDigest, LastSourceKind: lastSourceKind,
		LastActorType: lastActorType, LastActorID: uuidPtrString(lastActorID),
	}
}

func wikiPageSummary(page db.WikiPage) WikiPageSummaryResponse {
	return wikiPageSummaryFromRow(
		page.ID, page.WorkspaceID, page.Scope, page.ProjectID, page.OwnerUserID,
		page.CreatedBy, page.Path, page.Title, page.CreatedAt, page.UpdatedAt,
		page.CurrentRevisionNumber, page.CurrentRevisionID, page.ContentDigest,
		page.LastSourceKind, page.LastActorType, page.LastActorID,
	)
}

func uuidPtrString(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuidToString(u)
	return &s
}

func wikiPageToResponse(page db.WikiPage) WikiPageResponse {
	return WikiPageResponse{WikiPageSummaryResponse: wikiPageSummary(page), Content: page.Content}
}

func wikiRevisionToResponse(revision db.WikiPageRevision) WikiPageRevisionResponse {
	return WikiPageRevisionResponse{
		ID: uuidToString(revision.ID), PageID: uuidToString(revision.PageID),
		RevisionNumber: revision.RevisionNumber, Path: revision.Path, Title: revision.Title,
		Content: revision.Content, ContentDigest: revision.ContentDigest,
		ActorType: revision.ActorType, ActorID: uuidPtrString(revision.ActorID),
		SourceKind: revision.SourceKind, SourceRefID: uuidPtrString(revision.SourceRefID),
		CreatedAt: timestampToString(revision.CreatedAt),
	}
}

func wikiProposalToResponse(proposal db.WikiPageEditProposal) WikiPageEditProposalResponse {
	evidence := json.RawMessage(proposal.EvidenceRefs)
	if len(evidence) == 0 {
		evidence = json.RawMessage("[]")
	}
	return WikiPageEditProposalResponse{
		ID: uuidToString(proposal.ID), PageID: uuidToString(proposal.PageID),
		BaseRevisionNumber: proposal.BaseRevisionNumber,
		ProposedPath:       proposal.ProposedPath, ProposedTitle: proposal.ProposedTitle,
		ProposedContent: proposal.ProposedContent, ContentDigest: proposal.ContentDigest,
		Rationale: proposal.Rationale, EvidenceRefs: evidence,
		AgentID: uuidToString(proposal.AgentID), IdempotencyKey: proposal.IdempotencyKey,
		Status: proposal.Status, ReviewedByID: uuidPtrString(proposal.ReviewedByID),
		ReviewReason: optionalText(proposal.ReviewReason), ReviewedAt: optionalTime(proposal.ReviewedAt),
		AcceptedRevisionID: uuidPtrString(proposal.AcceptedRevisionID),
		CreatedAt:          timestampToString(proposal.CreatedAt),
	}
}

func wikiPageFromCreate(row db.CreateWikiPageRow) db.WikiPage {
	return wikiPageValues(row.ID, row.WorkspaceID, row.Scope, row.ProjectID, row.OwnerUserID, row.Path, row.Title, row.Content, row.CreatedBy, row.CreatedAt, row.UpdatedAt, row.CurrentRevisionNumber, row.CurrentRevisionID, row.ContentDigest, row.LastSourceKind, row.LastActorType, row.LastActorID)
}

func wikiPageFromUpdate(row db.UpdateWikiPageRow) db.WikiPage {
	return wikiPageValues(row.ID, row.WorkspaceID, row.Scope, row.ProjectID, row.OwnerUserID, row.Path, row.Title, row.Content, row.CreatedBy, row.CreatedAt, row.UpdatedAt, row.CurrentRevisionNumber, row.CurrentRevisionID, row.ContentDigest, row.LastSourceKind, row.LastActorType, row.LastActorID)
}

func wikiPageFromRestore(row db.RestoreWikiPageRevisionRow) db.WikiPage {
	return wikiPageValues(row.ID, row.WorkspaceID, row.Scope, row.ProjectID, row.OwnerUserID, row.Path, row.Title, row.Content, row.CreatedBy, row.CreatedAt, row.UpdatedAt, row.CurrentRevisionNumber, row.CurrentRevisionID, row.ContentDigest, row.LastSourceKind, row.LastActorType, row.LastActorID)
}

func wikiPageFromProposalAcceptance(row db.AcceptWikiPageEditProposalRow) db.WikiPage {
	return wikiPageValues(row.ID, row.WorkspaceID, row.Scope, row.ProjectID, row.OwnerUserID, row.Path, row.Title, row.Content, row.CreatedBy, row.CreatedAt, row.UpdatedAt, row.CurrentRevisionNumber, row.CurrentRevisionID, row.ContentDigest, row.LastSourceKind, row.LastActorType, row.LastActorID)
}

func wikiPageValues(
	id, workspaceID pgtype.UUID, scope string, projectID, ownerUserID pgtype.UUID,
	path, title, content string, createdBy pgtype.UUID, createdAt, updatedAt pgtype.Timestamptz,
	currentRevisionNumber int64, currentRevisionID pgtype.UUID,
	contentDigest, lastSourceKind, lastActorType string, lastActorID pgtype.UUID,
) db.WikiPage {
	return db.WikiPage{
		ID: id, WorkspaceID: workspaceID, Scope: scope, ProjectID: projectID,
		OwnerUserID: ownerUserID, Path: path, Title: title, Content: content,
		CreatedBy: createdBy, CreatedAt: createdAt, UpdatedAt: updatedAt,
		CurrentRevisionNumber: currentRevisionNumber, CurrentRevisionID: currentRevisionID,
		ContentDigest: contentDigest, LastSourceKind: lastSourceKind,
		LastActorType: lastActorType, LastActorID: lastActorID,
	}
}

func validateWikiPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(p))
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../") &&
		!strings.Contains(cleaned, "\\") && strings.HasSuffix(strings.ToLower(cleaned), ".md")
}

func normalizeWikiPath(p string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
}

func decodeWikiJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, wikiRequestBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func (h *Handler) requireWikiHuman(w http.ResponseWriter, r *http.Request, workspaceID string) (string, bool) {
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "this endpoint is only available to human actors")
		return "", false
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return "", false
	}
	actorType, _ := h.resolveActor(r, userID, workspaceID)
	if actorType != "member" {
		writeError(w, http.StatusForbidden, "agents must propose shared Wiki changes for human review")
		return "", false
	}
	return userID, true
}

func (h *Handler) wikiActor(r *http.Request, workspaceID string) (string, string) {
	userID := requestUserID(r)
	return h.resolveActor(r, userID, workspaceID)
}

func (h *Handler) ListWikiPages(w http.ResponseWriter, r *http.Request) {
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		scope = "workspace"
	}

	workspaceID := h.resolveWorkspaceID(r)
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	var pages []WikiPageSummaryResponse
	switch scope {
	case "workspace":
		rows, err := h.Queries.ListWikiPagesByWorkspaceScope(r.Context(), workspaceUUID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
			return
		}
		pages = make([]WikiPageSummaryResponse, len(rows))
		for i, page := range rows {
			pages[i] = wikiPageSummaryFromRow(page.ID, page.WorkspaceID, page.Scope, page.ProjectID, page.OwnerUserID, page.CreatedBy, page.Path, page.Title, page.CreatedAt, page.UpdatedAt, page.CurrentRevisionNumber, page.CurrentRevisionID, page.ContentDigest, page.LastSourceKind, page.LastActorType, page.LastActorID)
		}
	case "project":
		projectID, ok := parseUUIDOrBadRequest(w, r.URL.Query().Get("project_id"), "project_id")
		if !ok || !h.projectInWorkspace(w, r, workspaceUUID, projectID) {
			return
		}
		rows, err := h.Queries.ListWikiPagesByProject(r.Context(), db.ListWikiPagesByProjectParams{WorkspaceID: workspaceUUID, ProjectID: projectID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
			return
		}
		pages = make([]WikiPageSummaryResponse, len(rows))
		for i, page := range rows {
			pages[i] = wikiPageSummaryFromRow(page.ID, page.WorkspaceID, page.Scope, page.ProjectID, page.OwnerUserID, page.CreatedBy, page.Path, page.Title, page.CreatedAt, page.UpdatedAt, page.CurrentRevisionNumber, page.CurrentRevisionID, page.ContentDigest, page.LastSourceKind, page.LastActorType, page.LastActorID)
		}
	case "user":
		if isMachineCredentialActor(r) {
			writeError(w, http.StatusForbidden, "personal Wiki is unavailable to agents")
			return
		}
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		rows, err := h.Queries.ListWikiPagesByOwner(r.Context(), parseUUID(userID))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
			return
		}
		pages = make([]WikiPageSummaryResponse, len(rows))
		for i, page := range rows {
			pages[i] = wikiPageSummaryFromRow(page.ID, page.WorkspaceID, page.Scope, page.ProjectID, page.OwnerUserID, page.CreatedBy, page.Path, page.Title, page.CreatedAt, page.UpdatedAt, page.CurrentRevisionNumber, page.CurrentRevisionID, page.ContentDigest, page.LastSourceKind, page.LastActorType, page.LastActorID)
		}
	default:
		writeError(w, http.StatusBadRequest, "scope must be workspace, project, or user")
		return
	}
	writeJSON(w, http.StatusOK, pages)
}

func (h *Handler) ListPersonalWikiPages(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireWikiHuman(w, r, "")
	if !ok {
		return
	}
	rows, err := h.Queries.ListWikiPagesByOwner(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
		return
	}
	pages := make([]WikiPageSummaryResponse, len(rows))
	for i, page := range rows {
		pages[i] = wikiPageSummaryFromRow(page.ID, page.WorkspaceID, page.Scope, page.ProjectID, page.OwnerUserID, page.CreatedBy, page.Path, page.Title, page.CreatedAt, page.UpdatedAt, page.CurrentRevisionNumber, page.CurrentRevisionID, page.ContentDigest, page.LastSourceKind, page.LastActorType, page.LastActorID)
	}
	writeJSON(w, http.StatusOK, pages)
}

func (h *Handler) SearchWikiPages(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" || len([]rune(query)) > 500 {
		writeError(w, http.StatusBadRequest, "q must contain between 1 and 500 characters")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ownerID := parseUUID(userID)
	actorType, _ := h.wikiActor(r, workspaceID)
	machineActor := isMachineCredentialActor(r)
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		scope = "all"
	}
	input := service.WikiPageSearchInput{
		Scope: scope, WorkspaceID: workspaceUUID, OwnerUserID: ownerID,
		Query: query, Limit: wikiSearchLimit,
	}
	switch scope {
	case "all":
		input.AllowPersonal = actorType == "member" && !machineActor
	case "workspace":
	case "project":
		projectID, valid := parseUUIDOrBadRequest(w, r.URL.Query().Get("project_id"), "project_id")
		if !valid || !h.projectInWorkspace(w, r, workspaceUUID, projectID) {
			return
		}
		input.ProjectID = projectID
	case "user":
		if machineActor || actorType != "member" {
			writeError(w, http.StatusForbidden, "personal Wiki is unavailable to agents")
			return
		}
		input.AllowPersonal = true
	default:
		writeError(w, http.StatusBadRequest, "scope must be all, workspace, project, or user")
		return
	}
	rows, err := h.wikiKnowledgeService().SearchPages(r.Context(), h.Queries, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search wiki pages")
		return
	}
	response := make([]WikiPageSummaryResponse, len(rows))
	for i, page := range rows {
		response[i] = wikiPageSummary(page)
	}
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.WikiSearch(scope, len(response)))
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) SearchPersonalWikiPages(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" || len([]rune(query)) > 500 {
		writeError(w, http.StatusBadRequest, "q must contain between 1 and 500 characters")
		return
	}
	userID, ok := h.requireWikiHuman(w, r, "")
	if !ok {
		return
	}
	rows, err := h.wikiKnowledgeService().SearchPages(r.Context(), h.Queries, service.WikiPageSearchInput{
		Scope: "user", OwnerUserID: parseUUID(userID), Query: query,
		Limit: wikiSearchLimit, AllowPersonal: true,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search wiki pages")
		return
	}
	response := make([]WikiPageSummaryResponse, len(rows))
	for i, page := range rows {
		response[i] = wikiPageSummary(page)
	}
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.WikiSearch("user", len(response)))
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) projectInWorkspace(w http.ResponseWriter, r *http.Request, workspaceID, projectID pgtype.UUID) bool {
	_, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: projectID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "project not found")
		return false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project")
		return false
	}
	return true
}

func (h *Handler) loadWikiPageForUser(w http.ResponseWriter, r *http.Request, id string) (db.WikiPage, bool) {
	pageID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return db.WikiPage{}, false
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return db.WikiPage{}, false
	}
	workspaceID := strings.TrimSpace(h.resolveWorkspaceID(r))
	var workspaceUUID pgtype.UUID
	if workspaceID != "" {
		workspaceUUID, ok = parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
		if !ok {
			return db.WikiPage{}, false
		}
	}
	ownerID := pgtype.UUID{}
	if !isMachineCredentialActor(r) {
		ownerID = parseUUID(userID)
	}
	page, err := h.wikiKnowledgeService().GetPage(r.Context(), h.Queries, service.WikiPageAccess{
		PageID: pageID, WorkspaceID: workspaceUUID, OwnerUserID: ownerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki page not found")
		return db.WikiPage{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load wiki page")
		return db.WikiPage{}, false
	}

	if page.Scope == "user" {
		if isMachineCredentialActor(r) {
			writeError(w, http.StatusNotFound, "wiki page not found")
			return db.WikiPage{}, false
		}
		if !page.OwnerUserID.Valid || uuidToString(page.OwnerUserID) != userID {
			writeError(w, http.StatusNotFound, "wiki page not found")
			return db.WikiPage{}, false
		}
		return page, true
	}

	if !workspaceUUID.Valid || !page.WorkspaceID.Valid || page.WorkspaceID != workspaceUUID {
		writeError(w, http.StatusNotFound, "wiki page not found")
		return db.WikiPage{}, false
	}
	return page, true
}

func (h *Handler) loadPersonalWikiPageForUser(w http.ResponseWriter, r *http.Request, id string) (db.WikiPage, string, bool) {
	userID, ok := h.requireWikiHuman(w, r, "")
	if !ok {
		return db.WikiPage{}, "", false
	}
	pageID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return db.WikiPage{}, "", false
	}
	page, err := h.wikiKnowledgeService().GetPage(r.Context(), h.Queries, service.WikiPageAccess{
		PageID: pageID, OwnerUserID: parseUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki page not found")
		return db.WikiPage{}, "", false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load wiki page")
		return db.WikiPage{}, "", false
	}
	if page.Scope != "user" || page.WorkspaceID.Valid || !page.OwnerUserID.Valid || uuidToString(page.OwnerUserID) != userID {
		writeError(w, http.StatusNotFound, "wiki page not found")
		return db.WikiPage{}, "", false
	}
	return page, userID, true
}

func (h *Handler) GetWikiPage(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if ok {
		writeJSON(w, http.StatusOK, wikiPageToResponse(page))
	}
}

func (h *Handler) GetPersonalWikiPage(w http.ResponseWriter, r *http.Request) {
	page, _, ok := h.loadPersonalWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if ok {
		writeJSON(w, http.StatusOK, wikiPageToResponse(page))
	}
}

func (h *Handler) ListWikiPageRevisions(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.wikiKnowledgeService().ListRevisions(r.Context(), h.Queries, page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki revisions")
		return
	}
	response := make([]WikiPageRevisionResponse, len(rows))
	for i, row := range rows {
		response[i] = wikiRevisionToResponse(row)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListPersonalWikiPageRevisions(w http.ResponseWriter, r *http.Request) {
	page, _, ok := h.loadPersonalWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	rows, err := h.wikiKnowledgeService().ListRevisions(r.Context(), h.Queries, page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki revisions")
		return
	}
	response := make([]WikiPageRevisionResponse, len(rows))
	for i, row := range rows {
		response[i] = wikiRevisionToResponse(row)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetWikiPageRevision(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	revisionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "revisionId"), "revision_id")
	if !ok {
		return
	}
	revision, err := h.wikiKnowledgeService().GetPageRevision(r.Context(), h.Queries, page, revisionID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki revision not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load wiki revision")
		return
	}
	writeJSON(w, http.StatusOK, wikiRevisionToResponse(revision))
}

func (h *Handler) GetPersonalWikiPageRevision(w http.ResponseWriter, r *http.Request) {
	page, _, ok := h.loadPersonalWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	revisionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "revisionId"), "revision_id")
	if !ok {
		return
	}
	revision, err := h.wikiKnowledgeService().GetPageRevision(r.Context(), h.Queries, page, revisionID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki revision not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load wiki revision")
		return
	}
	writeJSON(w, http.StatusOK, wikiRevisionToResponse(revision))
}

func (h *Handler) GetStableWikiPageRevision(w http.ResponseWriter, r *http.Request) {
	h.getStableWikiPageRevision(w, r, false)
}

func (h *Handler) GetStablePersonalWikiPageRevision(w http.ResponseWriter, r *http.Request) {
	h.getStableWikiPageRevision(w, r, true)
}

func (h *Handler) getStableWikiPageRevision(w http.ResponseWriter, r *http.Request, personal bool) {
	revisionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "revisionId"), "revision_id")
	if !ok {
		return
	}

	var workspaceID, ownerID pgtype.UUID
	if personal {
		userID, ok := h.requireWikiHuman(w, r, "")
		if !ok {
			return
		}
		ownerID = parseUUID(userID)
	} else {
		if _, ok := requireUserID(w, r); !ok {
			return
		}
		workspaceID, ok = parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
		if !ok {
			return
		}
	}

	revision, err := h.wikiKnowledgeService().GetRevision(r.Context(), h.Queries, service.WikiRevisionAccess{
		RevisionID: revisionID, WorkspaceID: workspaceID, OwnerUserID: ownerID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki revision not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load wiki revision")
		return
	}
	writeJSON(w, http.StatusOK, wikiRevisionToResponse(revision))
}

func (h *Handler) CreateWikiPage(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	userID, ok := h.requireWikiHuman(w, r, workspaceID)
	if !ok {
		return
	}
	var request CreateWikiPageRequest
	if !decodeWikiJSON(w, r, &request) {
		return
	}
	scope := strings.TrimSpace(request.Scope)
	if scope == "" {
		scope = "workspace"
	}
	path := normalizeWikiPath(request.Path)
	if !validateWikiPath(path) {
		writeError(w, http.StatusBadRequest, "path must be a relative .md path without '..'")
		return
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	userUUID := parseUUID(userID)
	var workspaceUUID, projectID, ownerID pgtype.UUID
	switch scope {
	case "workspace":
		workspaceUUID, ok = parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	case "project":
		workspaceUUID, ok = parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
		if !ok {
			return
		}
		if request.ProjectID == nil {
			writeError(w, http.StatusBadRequest, "project_id is required for project scope")
			return
		}
		projectID, ok = parseUUIDOrBadRequest(w, *request.ProjectID, "project_id")
		if ok {
			ok = h.projectInWorkspace(w, r, workspaceUUID, projectID)
		}
	case "user":
		ownerID = userUUID
	default:
		writeError(w, http.StatusBadRequest, "scope must be workspace, project, or user")
		return
	}
	if !ok {
		return
	}
	page, err := h.wikiKnowledgeService().CreatePageAndPublish(r.Context(), h.Queries, service.WikiPageCreateInput{
		WorkspaceID: workspaceUUID, Scope: scope, ProjectID: projectID, OwnerUserID: ownerID,
		Path: path, Title: title, Content: request.Content,
		ActorType: "member", ActorID: userUUID, SourceKind: "human",
	})
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a wiki page already exists at this path")
		return
	}
	if errors.Is(err, service.ErrInvalidWikiPageCreate) {
		writeError(w, http.StatusBadRequest, "invalid wiki page")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create wiki page")
		return
	}
	writeJSON(w, http.StatusCreated, wikiPageToResponse(page))
}

func (h *Handler) CreatePersonalWikiPage(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireWikiHuman(w, r, "")
	if !ok {
		return
	}
	var request createPersonalWikiPageRequest
	if !decodeWikiJSON(w, r, &request) {
		return
	}
	path := normalizeWikiPath(request.Path)
	if !validateWikiPath(path) {
		writeError(w, http.StatusBadRequest, "path must be a relative .md path without '..'")
		return
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	userUUID := parseUUID(userID)
	page, err := h.wikiKnowledgeService().CreatePageAndPublish(r.Context(), h.Queries, service.WikiPageCreateInput{
		Scope: "user", OwnerUserID: userUUID, Path: path, Title: title,
		Content: request.Content, ActorType: "member", ActorID: userUUID, SourceKind: "human",
	})
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a wiki page already exists at this path")
		return
	}
	if errors.Is(err, service.ErrInvalidWikiPageCreate) {
		writeError(w, http.StatusBadRequest, "invalid wiki page")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create wiki page")
		return
	}
	writeJSON(w, http.StatusCreated, wikiPageToResponse(page))
}

func (h *Handler) UpdateWikiPage(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	workspaceID := uuidToString(page.WorkspaceID)
	if page.Scope == "user" {
		workspaceID = h.resolveWorkspaceID(r)
	}
	userID, ok := h.requireWikiHuman(w, r, workspaceID)
	if !ok {
		return
	}
	h.updateWikiPage(w, r, page, userID)
}

func (h *Handler) UpdatePersonalWikiPage(w http.ResponseWriter, r *http.Request) {
	page, userID, ok := h.loadPersonalWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	h.updateWikiPage(w, r, page, userID)
}

func (h *Handler) updateWikiPage(w http.ResponseWriter, r *http.Request, page db.WikiPage, userID string) {
	var request UpdateWikiPageRequest
	if !decodeWikiJSON(w, r, &request) {
		return
	}
	if request.ExpectedRevisionNumber == nil || *request.ExpectedRevisionNumber <= 0 {
		writeError(w, http.StatusBadRequest, "expected_revision_number is required")
		return
	}
	input := service.WikiPageUpdateInput{Page: page, ExpectedRevisionNumber: *request.ExpectedRevisionNumber, ActorID: parseUUID(userID)}
	if request.Path != nil {
		path := normalizeWikiPath(*request.Path)
		if !validateWikiPath(path) {
			writeError(w, http.StatusBadRequest, "path must be a relative .md path without '..'")
			return
		}
		input.Path = pgtype.Text{String: path, Valid: true}
	}
	if request.Title != nil {
		input.Title = pgtype.Text{String: strings.TrimSpace(*request.Title), Valid: true}
	}
	if request.Content != nil {
		input.Content = pgtype.Text{String: *request.Content, Valid: true}
	}
	updated, err := h.wikiKnowledgeService().UpdatePage(r.Context(), h.Queries, input)
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a wiki page already exists at this path")
		return
	}
	if errors.Is(err, service.ErrWikiRevisionConflict) {
		h.writeWikiRevisionConflict(w, r, page)
		return
	}
	if errors.Is(err, service.ErrInvalidWikiPageChange) {
		writeError(w, http.StatusBadRequest, "invalid wiki page update")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update wiki page")
		return
	}
	writeJSON(w, http.StatusOK, wikiPageToResponse(updated))
}

func (h *Handler) RestoreWikiPageRevision(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	workspaceID := uuidToString(page.WorkspaceID)
	if page.Scope == "user" {
		workspaceID = h.resolveWorkspaceID(r)
	}
	userID, ok := h.requireWikiHuman(w, r, workspaceID)
	if !ok {
		return
	}
	h.restoreWikiPageRevision(w, r, page, userID)
}

func (h *Handler) RestorePersonalWikiPageRevision(w http.ResponseWriter, r *http.Request) {
	page, userID, ok := h.loadPersonalWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	h.restoreWikiPageRevision(w, r, page, userID)
}

func (h *Handler) restoreWikiPageRevision(w http.ResponseWriter, r *http.Request, page db.WikiPage, userID string) {
	var request restoreWikiPageRequest
	if !decodeWikiJSON(w, r, &request) {
		return
	}
	if request.ExpectedRevisionNumber == nil || *request.ExpectedRevisionNumber <= 0 {
		writeError(w, http.StatusBadRequest, "expected_revision_number is required")
		return
	}
	revisionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "revisionId"), "revision_id")
	if !ok {
		return
	}
	restored, err := h.wikiKnowledgeService().RestorePage(r.Context(), h.Queries, service.WikiPageRestoreInput{
		Page: page, RevisionID: revisionID, ActorID: parseUUID(userID),
		ExpectedRevisionNumber: *request.ExpectedRevisionNumber,
	})
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a wiki page already exists at this path")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki revision not found")
		return
	}
	if errors.Is(err, service.ErrWikiRevisionConflict) {
		h.writeWikiRevisionConflict(w, r, page)
		return
	}
	if errors.Is(err, service.ErrInvalidWikiPageChange) {
		writeError(w, http.StatusBadRequest, "invalid wiki page restore")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore wiki revision")
		return
	}
	writeJSON(w, http.StatusOK, wikiPageToResponse(restored))
}

func (h *Handler) ListWikiPageEditProposals(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if page.Scope == "user" {
		writeJSON(w, http.StatusOK, []WikiPageEditProposalResponse{})
		return
	}
	rows, err := h.wikiKnowledgeService().ListProposals(r.Context(), h.Queries, page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki proposals")
		return
	}
	response := make([]WikiPageEditProposalResponse, len(rows))
	for i, row := range rows {
		response[i] = wikiProposalToResponse(row)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) CreateWikiPageEditProposal(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if page.Scope == "user" {
		writeError(w, http.StatusForbidden, "agents cannot propose personal Wiki changes")
		return
	}
	actorType, actorID := h.wikiActor(r, uuidToString(page.WorkspaceID))
	if actorType != "agent" || !isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "only an authenticated Agent run can create a Wiki proposal")
		return
	}
	var request createWikiProposalRequest
	if !decodeWikiJSON(w, r, &request) {
		return
	}
	if request.BaseRevisionNumber <= 0 || strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 200 {
		writeError(w, http.StatusBadRequest, "base_revision_number and idempotency_key are required")
		return
	}
	if request.AgentID == "" || request.AgentID != actorID {
		writeError(w, http.StatusForbidden, "proposal agent_id does not match the authenticated Agent")
		return
	}
	agentID, ok := parseUUIDOrBadRequest(w, request.AgentID, "agent_id")
	if !ok {
		return
	}
	path, title, content := page.Path, page.Title, page.Content
	if request.ProposedPath != nil {
		path = normalizeWikiPath(*request.ProposedPath)
		if !validateWikiPath(path) {
			writeError(w, http.StatusBadRequest, "path must be a relative .md path without '..'")
			return
		}
	}
	if request.ProposedTitle != nil {
		title = strings.TrimSpace(*request.ProposedTitle)
	}
	if request.ProposedContent != nil {
		content = *request.ProposedContent
	}
	rationale := strings.TrimSpace(request.Rationale)
	if rationale == "" || len([]rune(rationale)) > 8000 {
		writeError(w, http.StatusBadRequest, "rationale must contain between 1 and 8000 characters")
		return
	}
	taskID, taskErr := util.ParseUUID(strings.TrimSpace(r.Header.Get("X-Task-ID")))
	if taskErr != nil {
		taskID = pgtype.UUID{}
	}
	result, err := h.wikiKnowledgeService().CreateProposal(r.Context(), h.Queries, service.WikiProposalCreateInput{
		Page: page, AgentID: agentID, AuthenticatedTaskID: taskID,
		BaseRevisionNumber: request.BaseRevisionNumber, Path: path, Title: title,
		Content: content, Rationale: rationale, EvidenceRefs: request.EvidenceRefs,
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
	})
	if errors.Is(err, service.ErrInvalidWikiProposalEvidence) {
		writeError(w, http.StatusBadRequest, "evidence_refs must contain verified task or room references")
		return
	}
	if errors.Is(err, service.ErrInvalidWikiProposal) {
		writeError(w, http.StatusBadRequest, "invalid wiki proposal")
		return
	}
	if errors.Is(err, service.ErrWikiProposalIdempotencyConflict) {
		writeWikiProposalIdempotencyConflict(w)
		return
	}
	if errors.Is(err, service.ErrWikiProposalRevisionConflict) {
		h.writeWikiRevisionConflict(w, r, page)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create wiki proposal")
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, wikiProposalToResponse(result.Proposal))
}

func writeWikiProposalIdempotencyConflict(w http.ResponseWriter) {
	writeJSON(w, http.StatusConflict, map[string]any{"error": "idempotency key was already used for another Wiki proposal", "code": "wiki_proposal_idempotency_conflict"})
}

func (h *Handler) AcceptWikiPageEditProposal(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok || page.Scope == "user" {
		return
	}
	userID, ok := h.requireWikiHuman(w, r, uuidToString(page.WorkspaceID))
	if !ok {
		return
	}
	var request acceptWikiProposalRequest
	if !decodeWikiJSON(w, r, &request) {
		return
	}
	if request.ExpectedRevisionNumber == nil || *request.ExpectedRevisionNumber <= 0 {
		writeError(w, http.StatusBadRequest, "expected_revision_number is required")
		return
	}
	proposalID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposal_id")
	if !ok {
		return
	}
	proposal, err := h.wikiKnowledgeService().GetProposal(r.Context(), h.Queries, page, proposalID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki proposal not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load wiki proposal")
		return
	}
	input := service.WikiProposalAcceptInput{
		Page: page, Proposal: proposal, ExpectedRevisionNumber: *request.ExpectedRevisionNumber,
		ReviewerID: parseUUID(userID),
	}
	if request.Path != nil {
		path := normalizeWikiPath(*request.Path)
		if !validateWikiPath(path) {
			writeError(w, http.StatusBadRequest, "path must be a relative .md path without '..'")
			return
		}
		input.Path = pgtype.Text{String: path, Valid: true}
	}
	if request.Title != nil {
		input.Title = pgtype.Text{String: strings.TrimSpace(*request.Title), Valid: true}
	}
	if request.Content != nil {
		input.Content = pgtype.Text{String: *request.Content, Valid: true}
	}
	updated, _, err := h.wikiKnowledgeService().AcceptProposal(r.Context(), h.Queries, input)
	if errors.Is(err, service.ErrWikiProposalRevisionConflict) {
		h.writeWikiRevisionConflict(w, r, page)
		return
	}
	if errors.Is(err, service.ErrWikiProposalAlreadyReviewed) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Wiki proposal is no longer pending", "code": "wiki_proposal_already_reviewed"})
		return
	}
	if errors.Is(err, service.ErrInvalidWikiProposal) {
		writeError(w, http.StatusBadRequest, "invalid wiki proposal review")
		return
	}
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a wiki page already exists at this path")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to accept wiki proposal")
		return
	}
	edited := wikiProposalAcceptanceEdited(request, proposal)
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.WikiProposalReview("accepted", edited))
	writeJSON(w, http.StatusOK, wikiPageToResponse(updated))
}

func wikiProposalAcceptanceEdited(request acceptWikiProposalRequest, proposal db.WikiPageEditProposal) bool {
	return request.Path != nil && normalizeWikiPath(*request.Path) != proposal.ProposedPath ||
		request.Title != nil && strings.TrimSpace(*request.Title) != proposal.ProposedTitle ||
		request.Content != nil && *request.Content != proposal.ProposedContent
}

func (h *Handler) RejectWikiPageEditProposal(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok || page.Scope == "user" {
		return
	}
	userID, ok := h.requireWikiHuman(w, r, uuidToString(page.WorkspaceID))
	if !ok {
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if !decodeWikiJSON(w, r, &request) {
		return
	}
	if len([]rune(request.Reason)) > 2000 {
		writeError(w, http.StatusBadRequest, "reason is too long")
		return
	}
	proposalID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposal_id")
	if !ok {
		return
	}
	existing, err := h.wikiKnowledgeService().GetProposal(r.Context(), h.Queries, page, proposalID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "wiki proposal not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load wiki proposal")
		return
	}
	proposal, err := h.wikiKnowledgeService().RejectProposal(r.Context(), h.Queries, service.WikiProposalRejectInput{
		Page: page, Proposal: existing, ReviewerID: parseUUID(userID),
		ReviewReason: pgtype.Text{String: strings.TrimSpace(request.Reason), Valid: strings.TrimSpace(request.Reason) != ""},
	})
	if errors.Is(err, service.ErrWikiProposalAlreadyReviewed) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Wiki proposal is no longer pending", "code": "wiki_proposal_already_reviewed"})
		return
	}
	if errors.Is(err, service.ErrInvalidWikiProposal) {
		writeError(w, http.StatusBadRequest, "invalid wiki proposal review")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reject wiki proposal")
		return
	}
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.WikiProposalReview("rejected", false))
	writeJSON(w, http.StatusOK, wikiProposalToResponse(proposal))
}

func (h *Handler) DeleteWikiPage(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	workspaceID := uuidToString(page.WorkspaceID)
	if page.Scope == "user" {
		workspaceID = h.resolveWorkspaceID(r)
	}
	userID, ok := h.requireWikiHuman(w, r, workspaceID)
	if !ok {
		return
	}
	h.deleteWikiPage(w, r, page, userID)
}

func (h *Handler) DeletePersonalWikiPage(w http.ResponseWriter, r *http.Request) {
	page, userID, ok := h.loadPersonalWikiPageForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	h.deleteWikiPage(w, r, page, userID)
}

func (h *Handler) deleteWikiPage(w http.ResponseWriter, r *http.Request, page db.WikiPage, userID string) {
	if err := h.wikiKnowledgeService().DeletePage(r.Context(), h.Queries, page, parseUUID(userID)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete wiki page")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeWikiRevisionConflict(w http.ResponseWriter, r *http.Request, page db.WikiPage) {
	current := page.CurrentRevisionNumber
	access := service.WikiPageAccess{PageID: page.ID, WorkspaceID: page.WorkspaceID, OwnerUserID: page.OwnerUserID}
	if refreshed, err := h.wikiKnowledgeService().GetPage(r.Context(), h.Queries, access); err == nil {
		current = refreshed.CurrentRevisionNumber
	}
	writeJSON(w, http.StatusConflict, map[string]any{"error": "Wiki page changed since it was loaded", "code": "wiki_revision_conflict", "current_revision_number": current})
}

func optionalUUIDString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuidToString(value)
}
