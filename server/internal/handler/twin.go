package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) GetTwins(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, workspaceUUID, ok := h.twinReadWorkspace(w, r, workspaceID)
	if !ok {
		return
	}
	overview, err := h.TwinService.Overview(r.Context(), workspaceUUID)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	proposals := make([]twinProposalResponse, len(overview.Proposals))
	for index, detail := range overview.Proposals {
		proposals[index] = mapTwinProposal(detail.Proposal, detail.Review, detail.Version)
	}
	versions := make([]twinVersionResponse, len(overview.Versions))
	for index, version := range overview.Versions {
		versions[index] = mapTwinVersion(version)
	}
	var current *twinVersionResponse
	if overview.Current != nil {
		mapped := mapTwinVersion(*overview.Current)
		current = &mapped
	}
	var pending *twinProposalResponse
	if overview.Pending != nil {
		mapped := mapTwinProposal(*overview.Pending, nil, nil)
		pending = &mapped
	}
	writeJSON(w, http.StatusOK, twinOverviewResponse{CurrentVersion: current, PendingProposal: pending, Proposals: proposals, Versions: versions, CanManage: roleAllowed(member.Role, "owner", "admin")})
}

func (h *Handler) GetTwinProposal(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, workspaceUUID, ok := h.twinReadWorkspace(w, r, workspaceID)
	if !ok {
		return
	}
	proposalID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposal id")
	if !ok {
		return
	}
	detail, err := h.TwinService.ProposalDetail(r.Context(), workspaceUUID, proposalID)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapTwinProposalDetail(detail))
}

func (h *Handler) GetTwinVersion(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, workspaceUUID, ok := h.twinReadWorkspace(w, r, workspaceID)
	if !ok {
		return
	}
	versionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "versionId"), "version id")
	if !ok {
		return
	}
	detail, err := h.TwinService.VersionDetail(r.Context(), workspaceUUID, versionID)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapTwinVersionDetail(detail))
}

func (h *Handler) CreateTwinProposal(w http.ResponseWriter, r *http.Request) {
	var request struct {
		WikiRevisionID string `json:"wiki_revision_id"`
	}
	if !decodeTwinRequest(w, r, &request, false) {
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
	revisionID, ok := parseUUIDOrBadRequest(w, request.WikiRevisionID, "wiki revision id")
	if !ok {
		return
	}
	result, err := h.TwinService.EnsureProposal(r.Context(), workspaceUUID, revisionID, member.UserID)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, twinProposalResultResponse{Created: result.Created, Proposal: mapTwinProposal(result.Proposal, nil, nil)})
}

func (h *Handler) AcceptTwinProposal(w http.ResponseWriter, r *http.Request) {
	if !requireEmptyLMWikiBody(w, r) {
		return
	}
	workspaceUUID, proposalUUID, member, ok := h.twinReviewIDs(w, r)
	if !ok {
		return
	}
	result, err := h.TwinService.AcceptProposal(r.Context(), workspaceUUID, proposalUUID, member.UserID)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, twinVersionResultResponse{Created: result.Created, Version: mapTwinVersion(result.Version)})
}

func (h *Handler) RejectTwinProposal(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Reason string `json:"reason"`
	}
	if !decodeTwinRequest(w, r, &request, true) {
		return
	}
	workspaceUUID, proposalUUID, member, ok := h.twinReviewIDs(w, r)
	if !ok {
		return
	}
	detail, err := h.TwinService.RejectProposal(r.Context(), workspaceUUID, proposalUUID, member.UserID, request.Reason)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapTwinProposalDetail(detail))
}

func (h *Handler) twinReadWorkspace(w http.ResponseWriter, r *http.Request, workspaceID string) (db.Member, pgtype.UUID, bool) {
	workspaceMember, found := h.workspaceMember(w, r, workspaceID)
	if !found {
		return db.Member{}, pgtype.UUID{}, false
	}
	parsed, parsedOK := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !parsedOK {
		return db.Member{}, pgtype.UUID{}, false
	}
	return workspaceMember, parsed, true
}

func (h *Handler) twinReviewIDs(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, db.Member, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, db.Member{}, false
	}
	workspaceUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, db.Member{}, false
	}
	proposalUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "proposalId"), "proposal id")
	return workspaceUUID, proposalUUID, member, ok
}

func decodeTwinRequest(w http.ResponseWriter, r *http.Request, target any, allowEmpty bool) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2*1024*1024+1))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if errors.Is(err, io.EOF) && allowEmpty {
		return true
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func (h *Handler) writeTwinError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTwinNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Twin artifact not found", "code": "twin_not_found"})
	case errors.Is(err, service.ErrTwinWikiNotAccepted):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Wiki revision is not accepted", "code": "wiki_revision_not_accepted"})
	case errors.Is(err, service.ErrTwinAlreadyCurrent):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin is already current", "code": "twin_already_current"})
	case errors.Is(err, service.ErrTwinAlreadyDecided):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin proposal already decided", "code": "twin_proposal_already_decided"})
	case errors.Is(err, service.ErrTwinBaseStale):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin proposal base is stale", "code": "twin_base_stale"})
	case errors.Is(err, service.ErrTwinWikiStale):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin proposal Wiki is stale", "code": "twin_wiki_stale"})
	case errors.Is(err, service.ErrTwinInvalidReview):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Twin review", "code": "twin_review_invalid"})
	default:
		writeError(w, http.StatusInternalServerError, "Twin operation failed")
	}
}
