package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/featureflags"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
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
	// Keep a handler-level backstop in addition to the router middleware: task
	// and cloud-node credentials inherit their owner's identity and must never
	// be able to trigger model egress through this endpoint.
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "this endpoint is only available to human actors")
		return
	}
	if !featureflags.TwinExecutionEnabled(r.Context(), h.FeatureFlags) {
		h.writeTwinExecutionError(w, service.ErrTwinExecutionDisabled)
		return
	}
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
	startedAt := time.Now()
	result, err := h.TwinService.EnsureProposal(r.Context(), workspaceUUID, revisionID, member.UserID)
	if err != nil {
		state := analytics.TwinGenerationStateFailed
		if errors.Is(err, service.ErrTwinWikiNotAccepted) || errors.Is(err, service.ErrTwinAlreadyCurrent) || errors.Is(err, service.ErrTwinBaseStale) || errors.Is(err, service.ErrTwinWikiStale) {
			state = analytics.TwinGenerationStateBlocked
		}
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinProposalGeneration(analytics.TwinProposalGenerationMetric{
			Context: analytics.TwinMetricContext{UserID: uuidToString(member.UserID), WorkspaceID: uuidToString(workspaceUUID)},
			State:   state, LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		h.writeTwinError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
		context, kind, assertions, citations, unsupported := twinProposalMetric(result.Proposal, uuidToString(member.UserID), "")
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinProposalGeneration(analytics.TwinProposalGenerationMetric{
			Context: context, Kind: kind, State: analytics.TwinGenerationStateSucceeded,
			AssertionCount: assertions, CitationCount: citations, UnsupportedAssertionCount: unsupported,
			LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		h.publishTwinProposalChanged(workspaceID, uuidToString(member.UserID), uuidToString(result.Proposal.ID), "pending", "")
	}
	writeJSON(w, status, twinProposalResultResponse{Created: result.Created, Proposal: mapTwinProposal(result.Proposal, nil, nil)})
}

func (h *Handler) CorrectTwinProposal(w http.ResponseWriter, r *http.Request) {
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "this endpoint is only available to human actors")
		return
	}
	var request struct {
		EditedAssertions json.RawMessage `json:"edited_assertions"`
	}
	if !decodeTwinRequest(w, r, &request, false) {
		return
	}
	workspaceUUID, proposalUUID, member, ok := h.twinReviewIDs(w, r)
	if !ok {
		return
	}
	startedAt := time.Now()
	result, err := h.TwinService.CorrectProposal(r.Context(), workspaceUUID, proposalUUID, member.UserID, request.EditedAssertions)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
		actorID := uuidToString(member.UserID)
		metricContext, kind, assertions, citations, unsupported := twinProposalMetric(result.Proposal, actorID, "")
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinProposalGeneration(analytics.TwinProposalGenerationMetric{
			Context: metricContext, Kind: kind, State: analytics.TwinGenerationStateSucceeded,
			AssertionCount: assertions, CitationCount: citations, UnsupportedAssertionCount: unsupported,
			LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		h.publishTwinProposalChanged(uuidToString(workspaceUUID), actorID, uuidToString(result.Proposal.ID), "pending", "")
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
		actorID := uuidToString(member.UserID)
		if detail, detailErr := h.TwinService.ProposalDetail(r.Context(), workspaceUUID, proposalUUID); detailErr == nil {
			context, kind, assertions, citations, _ := twinProposalMetric(detail.Proposal, actorID, "")
			obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinSignOff(analytics.TwinSignOffMetric{
				Context: context, Kind: kind, Decision: analytics.TwinReviewDecisionSigned,
				AssertionCount: assertions, CitationCount: citations,
			}))
			if kind == analytics.TwinProposalKindDeposition {
				obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinDepositionReview(analytics.TwinDepositionReviewMetric{
					Context: context, Decision: analytics.TwinReviewDecisionAccepted,
					AssertionCount: assertions, CitationCount: citations,
				}))
			}
		}
		h.publishTwinProposalChanged(uuidToString(workspaceUUID), actorID, uuidToString(proposalUUID), "accepted", uuidToString(result.Version.ID))
		h.publishTwinVersionChanged(uuidToString(workspaceUUID), actorID, result.Version)
		h.publishTwinDepositionReview(r.Context(), workspaceUUID, proposalUUID, actorID, "accepted")
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
	result, err := h.TwinService.RejectProposal(r.Context(), workspaceUUID, proposalUUID, member.UserID, request.Reason)
	if err != nil {
		h.writeTwinError(w, err)
		return
	}
	if result.Created {
		actorID := uuidToString(member.UserID)
		context, kind, assertions, citations, _ := twinProposalMetric(result.Proposal.Proposal, uuidToString(member.UserID), "")
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinSignOff(analytics.TwinSignOffMetric{
			Context: context, Kind: kind, Decision: analytics.TwinReviewDecisionRejected,
			AssertionCount: assertions, CitationCount: citations,
		}))
		if kind == analytics.TwinProposalKindDeposition {
			obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinDepositionReview(analytics.TwinDepositionReviewMetric{
				Context: context, Decision: analytics.TwinReviewDecisionRejected,
				AssertionCount: assertions, CitationCount: citations,
			}))
		}
		h.publishTwinProposalChanged(uuidToString(workspaceUUID), actorID, uuidToString(proposalUUID), "rejected", "")
		h.publishTwinDepositionReview(r.Context(), workspaceUUID, proposalUUID, actorID, "rejected")
	}
	writeJSON(w, http.StatusOK, mapTwinProposalDetail(result.Proposal))
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
	case errors.Is(err, service.ErrTwinProposalSuperseded):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin proposal is superseded", "code": "twin_proposal_superseded"})
	case errors.Is(err, service.ErrTwinProposalUnchanged):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin proposal edit has no changes", "code": "twin_proposal_unchanged"})
	case errors.Is(err, service.ErrTwinBaseStale):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin proposal base is stale", "code": "twin_base_stale"})
	case errors.Is(err, service.ErrTwinWikiStale):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin proposal Wiki is stale", "code": "twin_wiki_stale"})
	case errors.Is(err, service.ErrTwinInvalidReview):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Twin review", "code": "twin_review_invalid"})
	case errors.Is(err, service.ErrTwinGenerationDenied):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Twin generation egress is not allowed", "code": "twin_generation_denied"})
	case errors.Is(err, service.ErrTwinGenerationUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Twin generation is unavailable", "code": "twin_generation_unavailable"})
	case errors.Is(err, service.ErrTwinGeneratorOutput), errors.Is(err, service.ErrTwinInvalidInput), errors.Is(err, service.ErrTwinInvalidAssertion), errors.Is(err, service.ErrTwinCitationMissing), errors.Is(err, service.ErrTwinUnsafeAssertion), errors.Is(err, service.ErrTwinContentTooLarge):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid Twin proposal output", "code": "twin_proposal_invalid"})
	default:
		writeError(w, http.StatusInternalServerError, "Twin operation failed")
	}
}
