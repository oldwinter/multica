package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/featureflags"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type twinExecutionBindingResponse struct {
	ID            string    `json:"id"`
	ScopeType     string    `json:"scope_type"`
	ScopeID       string    `json:"scope_id"`
	State         string    `json:"state"`
	TwinVersionID string    `json:"twin_version_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type twinExecutionBindingsResponse struct {
	Bindings   []twinExecutionBindingResponse  `json:"bindings"`
	CanManage  bool                            `json:"can_manage"`
	KillSwitch service.TwinExecutionKillSwitch `json:"kill_switch"`
}

type twinExecutionPolicyExclusionResponse struct {
	BindingID string                     `json:"binding_id"`
	ScopeType service.TwinUsePolicyScope `json:"scope_type"`
	Code      string                     `json:"code"`
}

type twinExecutionPolicyResponse struct {
	State      service.TwinUsePolicyState             `json:"state"`
	ScopeType  *service.TwinUsePolicyScope            `json:"scope_type"`
	ScopeID    *string                                `json:"scope_id"`
	BindingID  *string                                `json:"binding_id"`
	Explicit   bool                                   `json:"explicit"`
	Reason     service.TwinPolicyDecisionReason       `json:"reason"`
	Exclusions []twinExecutionPolicyExclusionResponse `json:"exclusions"`
}

type twinExecutionBriefingResponse struct {
	Policy           twinExecutionPolicyResponse            `json:"policy"`
	TwinVersion      *service.TwinExecutionVersionReference `json:"twin_version"`
	Briefing         string                                 `json:"briefing"`
	BriefingDigest   string                                 `json:"briefing_digest"`
	AssertionIDs     []string                               `json:"assertion_ids"`
	CitationKeys     []string                               `json:"citation_keys"`
	CompilerVersion  string                                 `json:"compiler_version"`
	ByteCount        int                                    `json:"byte_count"`
	TokenCount       int                                    `json:"token_count"`
	Inject           bool                                   `json:"inject"`
	ExclusionReasons []string                               `json:"exclusion_reasons"`
}

type twinExecutionFeedbackResponse struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Rating    string    `json:"rating"`
	Note      *string   `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type twinExecutionDepositionResponse struct {
	ID                 string    `json:"id"`
	TaskID             string    `json:"task_id"`
	BaseTwinVersionID  string    `json:"base_twin_version_id"`
	ProposalID         string    `json:"proposal_id"`
	ReplacesProposalID *string   `json:"replaces_proposal_id,omitempty"`
	EvidenceDigest     string    `json:"evidence_digest"`
	State              string    `json:"state"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type twinExecutionAttributionResponse struct {
	TwinVersionID     string   `json:"twin_version_id"`
	TwinVersionNumber int64    `json:"twin_version_number"`
	TwinVersionDigest string   `json:"twin_version_digest"`
	Briefing          string   `json:"briefing"`
	BriefingDigest    string   `json:"briefing_digest"`
	AssertionIDs      []string `json:"assertion_ids"`
	CitationKeys      []string `json:"citation_keys"`
	PolicyScopeType   string   `json:"policy_scope_type"`
	PolicyScopeID     string   `json:"policy_scope_id"`
	PolicyState       string   `json:"policy_state"`
	CompilerVersion   string   `json:"compiler_version"`
	ByteCount         int      `json:"byte_count"`
	TokenCount        int      `json:"token_count"`
}

type twinExecutionTaskContextResponse struct {
	TaskID      string                            `json:"task_id"`
	Attribution *twinExecutionAttributionResponse `json:"attribution"`
	Feedback    *twinExecutionFeedbackResponse    `json:"feedback"`
	Depositions []twinExecutionDepositionResponse `json:"depositions"`
	Assertions  []service.TwinExecutionAssertion  `json:"assertions"`
	Citations   []service.TwinExecutionCitation   `json:"citations"`
}

type twinExecutionDepositionResultResponse struct {
	Created    bool                            `json:"created"`
	Proposal   twinProposalResponse            `json:"proposal"`
	Deposition twinExecutionDepositionResponse `json:"deposition"`
}

func (h *Handler) twinExecutionService(r *http.Request) *service.TwinExecutionService {
	execution := service.NewTwinExecutionService(h.Queries, featureflags.TwinExecutionEnabled(r.Context(), h.FeatureFlags))
	if creator, ok := any(h.TwinService).(service.TwinDepositionProposalCreator); ok {
		execution.DepositionCreator = creator
	}
	return execution
}

// ResolveTwinBriefingForClaim is the runtime adapter shared by singular and
// batch claims. An operator kill switch skips Twin bytes without blocking the
// underlying task; enabled policies still fail closed inside the claim path if
// compilation or attribution cannot complete.
func (h *Handler) ResolveTwinBriefingForClaim(ctx context.Context, input service.TwinBriefingClaimInput) (service.TwinBriefingClaimResolution, error) {
	startedAt := time.Now()
	if !featureflags.TwinExecutionEnabled(ctx, h.FeatureFlags) {
		compiled := service.TwinCompiledBriefing{
			PolicyDecision: service.TwinEffectiveUsePolicy{State: service.TwinUseOff, Reason: service.TwinPolicyNoExplicitBinding},
		}
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinBriefingCompilation(analytics.TwinBriefingCompilationMetric{
			Context: analytics.TwinMetricContext{WorkspaceID: input.WorkspaceID, TaskID: input.TaskID},
			State:   analytics.TwinCompilationStateExcluded, ExclusionCode: analytics.TwinExclusionKillSwitch,
			LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		return service.TwinBriefingClaimResolution{Compiled: compiled}, nil
	}
	workspaceID, err := util.ParseUUID(input.WorkspaceID)
	if err != nil {
		return service.TwinBriefingClaimResolution{}, fmt.Errorf("resolve Twin briefing workspace: %w", err)
	}
	var oneOff *service.TwinExecutionOneOffPolicy
	if input.OneOffPolicy != nil {
		versionID := pgtype.UUID{}
		if input.OneOffPolicy.VersionID != "" {
			versionID, err = util.ParseUUID(input.OneOffPolicy.VersionID)
			if err != nil {
				return service.TwinBriefingClaimResolution{}, fmt.Errorf("resolve one-off Twin version: %w", err)
			}
		}
		oneOff = &service.TwinExecutionOneOffPolicy{
			ID: "one-off:" + input.TaskID, RunID: input.RunID,
			State: input.OneOffPolicy.State, TwinVersionID: versionID,
		}
	}
	compiled, err := service.NewTwinExecutionService(h.Queries, true).CompileBriefing(ctx, workspaceID, service.TwinExecutionBriefingInput{
		Task: service.TwinTaskEligibility{
			TaskID: input.TaskID, WorkspaceID: input.WorkspaceID, AgentID: input.AgentID,
			ProjectID: input.ProjectID, IssueID: input.IssueID, RunID: input.RunID,
			Request: input.Request, Tags: append([]string(nil), input.Tags...), Eligible: true,
		},
		OneOff: oneOff,
	})
	if err != nil {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinBriefingCompilation(analytics.TwinBriefingCompilationMetric{
			Context: analytics.TwinMetricContext{WorkspaceID: input.WorkspaceID, TaskID: input.TaskID},
			State:   analytics.TwinCompilationStateFailed, ExclusionCode: analytics.TwinExclusionNone,
			LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		return service.TwinBriefingClaimResolution{}, err
	}
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinBriefingCompilation(analytics.TwinBriefingCompilationMetric{
		Context: analytics.TwinMetricContext{WorkspaceID: input.WorkspaceID, TaskID: input.TaskID},
		State:   twinCompilationMetricState(compiled), Scope: twinMetricScope(compiled.PolicyDecision.Scope),
		ExclusionCode: twinMetricExclusion(compiled), AssertionCount: len(compiled.SelectedAssertionIDs),
		CitationCount: len(compiled.CitationIDs), ExclusionCount: len(compiled.Exclusions),
		ByteCount: compiled.ByteCount, TokenCount: compiled.TokenCount, LatencyMS: time.Since(startedAt).Milliseconds(),
	}))
	return service.TwinBriefingClaimResolution{Compiled: compiled}, nil
}

func (h *Handler) ListTwinExecutionBindings(w http.ResponseWriter, r *http.Request) {
	member, workspaceID, ok := h.twinReadWorkspace(w, r, h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	execution := h.twinExecutionService(r)
	bindings, err := execution.ListBindings(r.Context(), workspaceID)
	if err != nil {
		h.writeTwinExecutionError(w, err)
		return
	}
	mapped := make([]twinExecutionBindingResponse, len(bindings))
	for index, binding := range bindings {
		mapped[index] = mapTwinExecutionBinding(binding)
	}
	writeJSON(w, http.StatusOK, twinExecutionBindingsResponse{
		Bindings: mapped, CanManage: roleAllowed(member.Role, "owner", "admin"), KillSwitch: execution.KillSwitch(),
	})
}

func (h *Handler) UpsertTwinExecutionBinding(w http.ResponseWriter, r *http.Request) {
	if !h.requireTwinExecutionHumanManager(w, r) {
		return
	}
	var request struct {
		ScopeType     string `json:"scope_type"`
		ScopeID       string `json:"scope_id"`
		State         string `json:"state"`
		TwinVersionID string `json:"twin_version_id"`
	}
	if !decodeTwinRequest(w, r, &request, false) {
		return
	}
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	scopeID, ok := parseUUIDOrBadRequest(w, request.ScopeID, "scope id")
	if !ok {
		return
	}
	versionID, ok := parseUUIDOrBadRequest(w, request.TwinVersionID, "Twin version id")
	if !ok {
		return
	}
	binding, err := h.twinExecutionService(r).UpsertBinding(r.Context(), service.TwinBindingInput{
		WorkspaceID: workspaceID, ScopeType: request.ScopeType, ScopeID: scopeID,
		State: request.State, TwinVersionID: versionID,
	})
	if err != nil {
		h.writeTwinExecutionError(w, err)
		return
	}
	h.publishTwinBindingChanged(uuidToString(workspaceID), requestUserID(r), binding)
	writeJSON(w, http.StatusOK, mapTwinExecutionBinding(binding))
}

func (h *Handler) DeleteTwinExecutionBinding(w http.ResponseWriter, r *http.Request) {
	if !h.requireTwinExecutionHumanManager(w, r) {
		return
	}
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	bindingID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "bindingId"), "binding id")
	if !ok {
		return
	}
	if err := h.twinExecutionService(r).DeleteBinding(r.Context(), workspaceID, bindingID); err != nil {
		h.writeTwinExecutionError(w, err)
		return
	}
	h.publishTwinBindingDeleted(uuidToString(workspaceID), requestUserID(r), uuidToString(bindingID))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PreviewTwinExecutionBriefing(w http.ResponseWriter, r *http.Request) {
	var request struct {
		AgentID       string   `json:"agent_id"`
		ProjectID     string   `json:"project_id"`
		IssueID       string   `json:"issue_id"`
		RunID         string   `json:"run_id"`
		OneOffState   string   `json:"one_off_state"`
		TwinVersionID string   `json:"twin_version_id"`
		Request       string   `json:"request"`
		Tags          []string `json:"tags"`
	}
	if !decodeTwinRequest(w, r, &request, false) {
		return
	}
	member, workspaceID, ok := h.twinReadWorkspace(w, r, h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	taskID := "preview"
	if request.RunID != "" {
		taskID = request.RunID
	}
	execution := h.twinExecutionService(r)
	effectiveRunID := request.RunID
	var oneOff *service.TwinExecutionOneOffPolicy
	if request.OneOffState != "" || request.TwinVersionID != "" {
		if effectiveRunID == "" {
			effectiveRunID = request.IssueID
		}
		prepared, err := execution.PrepareOneOffPreview(
			r.Context(), workspaceID, effectiveRunID,
			service.TwinUsePolicyState(request.OneOffState), request.TwinVersionID,
		)
		if err != nil {
			h.writeTwinExecutionError(w, err)
			return
		}
		oneOff = prepared
	}
	startedAt := time.Now()
	preview, err := execution.CompileBriefingPreview(r.Context(), workspaceID, service.TwinExecutionBriefingInput{Task: service.TwinTaskEligibility{
		TaskID: taskID, WorkspaceID: uuidToString(workspaceID), AgentID: request.AgentID,
		ProjectID: request.ProjectID, IssueID: request.IssueID, RunID: effectiveRunID,
		Request: request.Request, Tags: request.Tags, Eligible: true,
	}, OneOff: oneOff})
	if err != nil {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinBriefingCompilation(analytics.TwinBriefingCompilationMetric{
			Context: analytics.TwinMetricContext{UserID: uuidToString(member.UserID), WorkspaceID: uuidToString(workspaceID), TaskID: taskID},
			State:   analytics.TwinCompilationStateFailed, ExclusionCode: analytics.TwinExclusionNone,
			LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		h.writeTwinExecutionError(w, err)
		return
	}
	metricContext := analytics.TwinMetricContext{UserID: uuidToString(member.UserID), WorkspaceID: uuidToString(workspaceID), TaskID: taskID}
	compiled := preview.Briefing
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinBriefingCompilation(analytics.TwinBriefingCompilationMetric{
		Context: metricContext, State: twinCompilationMetricState(compiled), Scope: twinMetricScope(compiled.PolicyDecision.Scope),
		ExclusionCode: twinMetricExclusion(compiled), AssertionCount: len(compiled.SelectedAssertionIDs), CitationCount: len(compiled.CitationIDs),
		ExclusionCount: len(compiled.Exclusions), ByteCount: compiled.ByteCount, TokenCount: compiled.TokenCount,
		LatencyMS: time.Since(startedAt).Milliseconds(),
	}))
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinBriefingUse(analytics.TwinBriefingUseMetric{
		Context: metricContext, State: analytics.TwinUseStatePreviewed, Scope: twinMetricScope(compiled.PolicyDecision.Scope),
		ExclusionCode: twinMetricExclusion(compiled), AssertionCount: len(compiled.SelectedAssertionIDs), CitationCount: len(compiled.CitationIDs),
		ByteCount: compiled.ByteCount, TokenCount: compiled.TokenCount,
	}))
	writeJSON(w, http.StatusOK, mapTwinCompiledBriefing(preview))
}

func (h *Handler) GetTwinExecutionTaskContext(w http.ResponseWriter, r *http.Request) {
	_, workspaceID, ok := h.twinReadWorkspace(w, r, h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task id")
	if !ok {
		return
	}
	context, err := h.twinExecutionService(r).GetTaskContext(r.Context(), workspaceID, taskID)
	if err != nil {
		h.writeTwinExecutionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapTwinExecutionTaskContext(context))
}

func (h *Handler) UpsertTwinExecutionFeedback(w http.ResponseWriter, r *http.Request) {
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "this endpoint is only available to human actors")
		return
	}
	var request struct {
		Rating string  `json:"rating"`
		Note   *string `json:"note"`
	}
	if !decodeTwinRequest(w, r, &request, false) {
		return
	}
	member, workspaceID, ok := h.twinReadWorkspace(w, r, h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task id")
	if !ok {
		return
	}
	feedback, err := h.twinExecutionService(r).UpsertFeedback(r.Context(), service.TwinRunFeedbackInput{
		WorkspaceID: workspaceID, TaskID: taskID, Rating: request.Rating, Note: request.Note,
	})
	if err != nil {
		h.writeTwinExecutionError(w, err)
		return
	}
	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinRunFeedback(analytics.TwinRunFeedbackMetric{
		Context: analytics.TwinMetricContext{UserID: uuidToString(member.UserID), WorkspaceID: uuidToString(workspaceID), TaskID: uuidToString(taskID)},
		Rating:  analytics.TwinFeedbackRating(feedback.Rating),
	}))
	writeJSON(w, http.StatusOK, map[string]twinExecutionFeedbackResponse{"feedback": mapTwinExecutionFeedback(feedback)})
}

func (h *Handler) CreateTwinExecutionDeposition(w http.ResponseWriter, r *http.Request) {
	if !h.requireTwinExecutionHumanManager(w, r) {
		return
	}
	var request struct {
		ReplacesProposalID string          `json:"replaces_proposal_id"`
		EditedAssertions   json.RawMessage `json:"edited_assertions"`
	}
	if !decodeTwinRequest(w, r, &request, true) {
		return
	}
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task id")
	if !ok {
		return
	}
	member, found := h.workspaceMember(w, r, uuidToString(workspaceID))
	if !found {
		return
	}
	replacement := pgtype.UUID{}
	if request.ReplacesProposalID != "" {
		replacement, ok = parseUUIDOrBadRequest(w, request.ReplacesProposalID, "replaces proposal id")
		if !ok {
			return
		}
	}
	startedAt := time.Now()
	result, err := h.twinExecutionService(r).CreateDeposition(r.Context(), workspaceID, taskID, service.TwinDepositionRequest{
		RequestedByID: member.UserID, ReplacesProposalID: replacement, EditedAssertions: request.EditedAssertions,
	})
	if err != nil {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinProposalGeneration(analytics.TwinProposalGenerationMetric{
			Context: analytics.TwinMetricContext{UserID: uuidToString(member.UserID), WorkspaceID: uuidToString(workspaceID), TaskID: uuidToString(taskID)},
			Kind:    analytics.TwinProposalKindDeposition, State: analytics.TwinGenerationStateFailed,
			LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		h.writeTwinExecutionError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
		context, kind, assertions, citations, unsupported := twinProposalMetric(result.Proposal, uuidToString(member.UserID), uuidToString(taskID))
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.TwinProposalGeneration(analytics.TwinProposalGenerationMetric{
			Context: context, Kind: kind, State: analytics.TwinGenerationStateSucceeded,
			AssertionCount: assertions, CitationCount: citations, UnsupportedAssertionCount: unsupported,
			LatencyMS: time.Since(startedAt).Milliseconds(),
		}))
		h.publishTwinProposalChanged(uuidToString(workspaceID), uuidToString(member.UserID), uuidToString(result.Proposal.ID), "pending", "")
		h.publishTwinDepositionChanged(uuidToString(workspaceID), uuidToString(member.UserID), result.Deposition, "pending")
	}
	writeJSON(w, status, twinExecutionDepositionResultResponse{
		Created: result.Created, Proposal: mapTwinProposal(result.Proposal, nil, nil), Deposition: mapTwinExecutionDeposition(result.Deposition),
	})
}

func (h *Handler) GetTwinExecutionMetrics(w http.ResponseWriter, r *http.Request) {
	_, workspaceID, ok := h.twinReadWorkspace(w, r, h.resolveWorkspaceID(r))
	if !ok {
		return
	}
	metrics, err := h.twinExecutionService(r).GetMetrics(r.Context(), workspaceID)
	if err != nil {
		h.writeTwinExecutionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func (h *Handler) requireTwinExecutionHumanManager(w http.ResponseWriter, r *http.Request) bool {
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "this endpoint is only available to human actors")
		return false
	}
	_, ok := h.requireWorkspaceRole(w, r, h.resolveWorkspaceID(r), "workspace not found", "owner", "admin")
	return ok
}

func (h *Handler) writeTwinExecutionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTwinExecutionDisabled):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Twin execution is disabled", "code": "twin_execution_disabled"})
	case errors.Is(err, service.ErrTwinExecutionNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Twin execution record not found", "code": "twin_execution_not_found"})
	case errors.Is(err, service.ErrTwinExecutionConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin execution state conflicts with the request", "code": "twin_execution_conflict"})
	case errors.Is(err, service.ErrTwinBaseStale), errors.Is(err, service.ErrTwinWikiStale), errors.Is(err, service.ErrTwinDepositionEvidenceStale):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin deposition evidence is stale", "code": "twin_deposition_stale"})
	case errors.Is(err, service.ErrTwinExecutionTaskNotCompleted):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin deposition requires a completed task", "code": "twin_task_not_completed"})
	case errors.Is(err, service.ErrTwinExecutionAttributionMissing):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Twin task attribution is missing", "code": "twin_attribution_missing"})
	case errors.Is(err, service.ErrTwinDepositionUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Twin deposition generation is unavailable", "code": "twin_deposition_unavailable"})
	case errors.Is(err, service.ErrTwinGenerationUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Twin generation is unavailable", "code": "twin_generation_unavailable"})
	case errors.Is(err, service.ErrTwinExecutionUnsupportedVersion):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "Twin execution requires a signed schema-v2 version", "code": "twin_version_unsupported"})
	case errors.Is(err, service.ErrTwinGenerationDenied):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Twin generation egress is not allowed", "code": "twin_generation_denied"})
	case errors.Is(err, service.ErrTwinExecutionInvalidInput), errors.Is(err, service.ErrTwinUsePolicyInvalid), errors.Is(err, service.ErrTwinBriefingInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Twin execution request", "code": "twin_execution_invalid"})
	case errors.Is(err, service.ErrTwinGeneratorOutput), errors.Is(err, service.ErrTwinInvalidInput), errors.Is(err, service.ErrTwinInvalidAssertion), errors.Is(err, service.ErrTwinCitationMissing), errors.Is(err, service.ErrTwinUnsafeAssertion), errors.Is(err, service.ErrTwinContentTooLarge):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid Twin proposal output", "code": "twin_proposal_invalid"})
	default:
		writeError(w, http.StatusInternalServerError, "Twin execution operation failed")
	}
}

func mapTwinExecutionBinding(binding db.TwinBinding) twinExecutionBindingResponse {
	return twinExecutionBindingResponse{
		ID: uuidToString(binding.ID), ScopeType: binding.ScopeType, ScopeID: uuidToString(binding.ScopeID),
		State: binding.State, TwinVersionID: uuidToString(binding.TwinVersionID),
		CreatedAt: binding.CreatedAt.Time, UpdatedAt: binding.UpdatedAt.Time,
	}
}

func mapTwinCompiledBriefing(preview service.TwinExecutionBriefingPreview) twinExecutionBriefingResponse {
	compiled := preview.Briefing
	assertionIDs := append([]string(nil), compiled.SelectedAssertionIDs...)
	citationKeys := append([]string(nil), compiled.CitationIDs...)
	exclusionReasons := make([]string, len(compiled.Exclusions))
	for index, exclusion := range compiled.Exclusions {
		exclusionReasons[index] = string(exclusion.Code)
	}
	if assertionIDs == nil {
		assertionIDs = []string{}
	}
	if citationKeys == nil {
		citationKeys = []string{}
	}
	policyExclusions := make([]twinExecutionPolicyExclusionResponse, len(compiled.PolicyDecision.Exclusions))
	for index, exclusion := range compiled.PolicyDecision.Exclusions {
		policyExclusions[index] = twinExecutionPolicyExclusionResponse{BindingID: exclusion.BindingID, ScopeType: exclusion.Scope, Code: string(exclusion.Code)}
	}
	var scopeType *service.TwinUsePolicyScope
	var scopeID, bindingID *string
	if compiled.PolicyDecision.Explicit {
		scope := compiled.PolicyDecision.Scope
		scopeType = &scope
		scopeValue := compiled.PolicyDecision.ScopeID
		scopeID = &scopeValue
		bindingValue := compiled.PolicyDecision.BindingID
		bindingID = &bindingValue
	}
	return twinExecutionBriefingResponse{
		Policy: twinExecutionPolicyResponse{
			State: compiled.PolicyDecision.State, ScopeType: scopeType,
			ScopeID: scopeID, BindingID: bindingID,
			Explicit: compiled.PolicyDecision.Explicit, Reason: compiled.PolicyDecision.Reason,
			Exclusions: policyExclusions,
		},
		TwinVersion: preview.Version, Briefing: compiled.Briefing, BriefingDigest: compiled.Digest,
		AssertionIDs: assertionIDs, CitationKeys: citationKeys,
		CompilerVersion: compiled.CompilerVersion, ByteCount: compiled.ByteCount,
		TokenCount: compiled.TokenCount, Inject: compiled.Inject, ExclusionReasons: exclusionReasons,
	}
}

func mapTwinExecutionTaskContext(value service.TwinExecutionTaskContext) twinExecutionTaskContextResponse {
	response := twinExecutionTaskContextResponse{
		TaskID: value.TaskID, Depositions: make([]twinExecutionDepositionResponse, len(value.Depositions)),
		Assertions: value.Assertions, Citations: value.Citations,
	}
	for index, deposition := range value.Depositions {
		response.Depositions[index] = mapTwinExecutionDeposition(deposition)
	}
	if value.Attribution != nil {
		attribution := value.Attribution
		assertionIDs := make([]string, len(attribution.Assertions))
		for index, assertion := range attribution.Assertions {
			assertionIDs[index] = assertion.ID
		}
		response.Attribution = &twinExecutionAttributionResponse{
			TwinVersionID: attribution.TwinVersionID, TwinVersionNumber: attribution.VersionNumber,
			TwinVersionDigest: attribution.VersionDigest,
			Briefing:          attribution.Briefing, BriefingDigest: attribution.BriefingDigest,
			AssertionIDs: assertionIDs, CitationKeys: attribution.CitationKeys,
			PolicyScopeType: attribution.PolicyScopeType, PolicyScopeID: attribution.PolicyScopeID,
			PolicyState: attribution.PolicyState, CompilerVersion: attribution.CompilerVersion,
			ByteCount: attribution.ByteCount, TokenCount: attribution.TokenCount,
		}
	}
	if value.Feedback != nil {
		mapped := mapTwinExecutionFeedback(*value.Feedback)
		response.Feedback = &mapped
	}
	return response
}

func mapTwinExecutionFeedback(feedback db.TwinRunFeedback) twinExecutionFeedbackResponse {
	var note *string
	if feedback.Note.Valid {
		value := feedback.Note.String
		note = &value
	}
	return twinExecutionFeedbackResponse{
		ID: uuidToString(feedback.ID), TaskID: uuidToString(feedback.TaskID), Rating: feedback.Rating,
		Note: note, CreatedAt: feedback.CreatedAt.Time, UpdatedAt: feedback.UpdatedAt.Time,
	}
}

func mapTwinExecutionDeposition(deposition db.TwinDeposition) twinExecutionDepositionResponse {
	return twinExecutionDepositionResponse{
		ID: uuidToString(deposition.ID), TaskID: uuidToString(deposition.TaskID),
		BaseTwinVersionID: uuidToString(deposition.BaseTwinVersionID), ProposalID: uuidToString(deposition.ProposalID),
		ReplacesProposalID: uuidToPtr(deposition.ReplacesProposalID),
		EvidenceDigest:     deposition.EvidenceDigest, State: deposition.State,
		CreatedAt: deposition.CreatedAt.Time, UpdatedAt: deposition.UpdatedAt.Time,
	}
}
