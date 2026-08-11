package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const lmWikiReviewBodyLimit = 32 * 1024

type lmWikiRevisionResponse struct {
	ID             string                `json:"id"`
	RevisionNumber int64                 `json:"revision_number"`
	SchemaVersion  int32                 `json:"schema_version"`
	SourceDigest   string                `json:"source_digest"`
	Content        json.RawMessage       `json:"content"`
	TriggerKind    string                `json:"trigger_kind"`
	RequestedByID  *string               `json:"requested_by_id"`
	CreatedAt      time.Time             `json:"created_at"`
	Review         *lmWikiReviewResponse `json:"review"`
}

type lmWikiReviewResponse struct {
	ID         string    `json:"id"`
	Decision   string    `json:"decision"`
	ReviewerID string    `json:"reviewer_id"`
	Reason     *string   `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

type lmWikiCitationResponse struct {
	ID              string          `json:"id"`
	Ordinal         int32           `json:"ordinal"`
	CitationKey     string          `json:"citation_key"`
	SourceType      string          `json:"source_type"`
	SourceID        string          `json:"source_id"`
	SourceUpdatedAt *time.Time      `json:"source_updated_at"`
	Locator         string          `json:"locator"`
	Label           string          `json:"label"`
	SafeMetadata    json.RawMessage `json:"safe_metadata"`
	SourceDigest    string          `json:"source_digest"`
}

type lmWikiDetailResponse struct {
	Revision  lmWikiRevisionResponse   `json:"revision"`
	Citations []lmWikiCitationResponse `json:"citations"`
}

func (h *Handler) GetLMWiki(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	overview, err := h.WikiService.Overview(r.Context(), workspaceUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LM Wiki")
		return
	}
	revisions := make([]lmWikiRevisionResponse, len(overview.Revisions))
	for index, detail := range overview.Revisions {
		revisions[index] = mapLMWikiRevision(detail.Revision, detail.Review)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"latest_revision":   mapLMWikiOverviewHead(overview.Latest, overview.Revisions),
		"accepted_revision": mapLMWikiOverviewHead(overview.Accepted, overview.Revisions),
		"pending_revision":  mapLMWikiOverviewHead(overview.Pending, overview.Revisions),
		"revisions":         revisions,
		"can_manage":        roleAllowed(member.Role, "owner", "admin"),
	})
}

func (h *Handler) GetLMWikiRevision(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, revisionUUID, ok := h.lmWikiReadIDs(w, r)
	if !ok {
		return
	}
	detail, err := h.WikiService.Detail(r.Context(), workspaceUUID, revisionUUID)
	if err != nil {
		h.writeLMWikiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapLMWikiDetail(detail))
}

func (h *Handler) RefreshLMWiki(w http.ResponseWriter, r *http.Request) {
	if !requireEmptyLMWikiBody(w, r) {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	result, err := h.WikiService.Refresh(r.Context(), workspaceUUID, "manual", member.UserID, "")
	if err != nil {
		h.writeLMWikiError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"created": result.Created, "revision": mapLMWikiRevision(result.Revision, nil)})
}

func (h *Handler) AcceptLMWikiRevision(w http.ResponseWriter, r *http.Request) {
	if !requireEmptyLMWikiBody(w, r) {
		return
	}
	h.reviewLMWikiRevision(w, r, "accepted", "")
}

func (h *Handler) RejectLMWikiRevision(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Reason string `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, lmWikiReviewBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		writeLMWikiReviewBodyError(w, err)
		return
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeLMWikiReviewBodyError(w, err)
		return
	}
	h.reviewLMWikiRevision(w, r, "rejected", request.Reason)
}

func writeLMWikiReviewBodyError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid request body")
}

func (h *Handler) reviewLMWikiRevision(w http.ResponseWriter, r *http.Request, decision, reason string) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin")
	if !ok {
		return
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	revisionUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "revisionId"), "revision id")
	if !ok {
		return
	}
	detail, err := h.WikiService.Review(r.Context(), workspaceUUID, revisionUUID, member.UserID, decision, reason)
	if err != nil {
		h.writeLMWikiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapLMWikiDetail(detail))
}

func (h *Handler) lmWikiReadIDs(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	revisionUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "revisionId"), "revision id")
	return workspaceUUID, revisionUUID, ok
}

func (h *Handler) writeLMWikiError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLMWikiNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "LM Wiki revision not found", "code": "wiki_revision_not_found"})
	case errors.Is(err, service.ErrLMWikiInvalidReview):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid LM Wiki review", "code": "wiki_review_invalid"})
	case errors.Is(err, service.ErrLMWikiAlreadyDecided):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "LM Wiki revision already decided", "code": "wiki_revision_already_decided"})
	case errors.Is(err, service.ErrLMWikiStale):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "LM Wiki revision is stale", "code": "wiki_revision_stale"})
	default:
		writeError(w, http.StatusInternalServerError, "LM Wiki operation failed")
	}
}

func mapLMWikiDetail(detail service.LMWikiRevisionDetail) lmWikiDetailResponse {
	citations := make([]lmWikiCitationResponse, len(detail.Citations))
	for index, citation := range detail.Citations {
		citations[index] = lmWikiCitationResponse{ID: uuidToString(citation.ID), Ordinal: citation.Ordinal, CitationKey: citation.CitationKey, SourceType: citation.SourceType, SourceID: uuidToString(citation.SourceID), SourceUpdatedAt: optionalTime(citation.SourceUpdatedAt), Locator: citation.Locator, Label: citation.Label, SafeMetadata: json.RawMessage(citation.SafeMetadata), SourceDigest: citation.SourceDigest}
	}
	return lmWikiDetailResponse{Revision: mapLMWikiRevision(detail.Revision, detail.Review), Citations: citations}
}

func mapLMWikiRevision(revision db.LmWikiRevision, review *db.LmWikiReview) lmWikiRevisionResponse {
	response := lmWikiRevisionResponse{ID: uuidToString(revision.ID), RevisionNumber: revision.RevisionNumber, SchemaVersion: revision.SchemaVersion, SourceDigest: revision.SourceDigest, Content: json.RawMessage(revision.Content), TriggerKind: revision.TriggerKind, RequestedByID: optionalUUID(revision.RequestedByID), CreatedAt: revision.CreatedAt.Time}
	if review != nil {
		response.Review = &lmWikiReviewResponse{ID: uuidToString(review.ID), Decision: review.Decision, ReviewerID: uuidToString(review.ReviewerID), Reason: optionalText(review.Reason), CreatedAt: review.CreatedAt.Time}
	}
	return response
}

func mapLMWikiOverviewHead(revision *db.LmWikiRevision, details []service.LMWikiRevisionDetail) any {
	if revision == nil {
		return nil
	}
	for _, detail := range details {
		if detail.Revision.ID == revision.ID {
			return mapLMWikiRevision(*revision, detail.Review)
		}
	}
	return mapLMWikiRevision(*revision, nil)
}

func requireEmptyLMWikiBody(w http.ResponseWriter, r *http.Request) bool {
	value, err := io.ReadAll(io.LimitReader(r.Body, 1025))
	if err != nil || len(bytes.TrimSpace(value)) != 0 {
		writeError(w, http.StatusBadRequest, "request body must be empty")
		return false
	}
	return true
}

func optionalUUID(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	result := uuidToString(value)
	return &result
}

func optionalText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
