package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/util"
)

const skillEvolutionBodyLimit = 16 << 10

type HTTP struct {
	evolution SkillEvolution
	skills    WorkspaceSkillLoader
	proposals ProposalRequester
	metrics   *Metrics
}

// ProposalRequester is the visible Improvement Room boundary. Production
// implementations recover a human-accepted executable_procedure
// recommendation; they never invoke Lifecycle.Generate's model port directly.
type ProposalRequester interface {
	RequestProposal(context.Context, pgtype.UUID, pgtype.UUID, pgtype.UUID, string) (ProposalRequestResult, error)
}

func NewHTTP(evolution SkillEvolution, skills WorkspaceSkillLoader, proposals ProposalRequester, metrics ...*Metrics) *HTTP {
	handler := &HTTP{evolution: evolution, skills: skills, proposals: proposals}
	if len(metrics) > 0 {
		handler.metrics = metrics[0]
	}
	return handler
}

func (h *HTTP) Routes() http.Handler {
	router := chi.NewRouter()
	router.Use(handler.RequireHumanActor)
	router.Get("/skills/{skillId}", h.overview)
	router.Put("/skills/{skillId}/loop", h.configure)
	router.Post("/skills/{skillId}/pause", h.pause)
	router.Post("/skills/{skillId}/proposals", h.generate)
	router.Post("/skills/{skillId}/fork", h.fork)
	router.Post("/skills/{skillId}/releases/{releaseId}/rollback", h.rollback)
	router.Get("/proposals/{proposalId}", h.proposal)
	router.Post("/proposals/{proposalId}/reject", h.reject)
	router.Post("/proposals/{proposalId}/publish", h.publish)
	return router
}

type loopRequest struct {
	Enabled          bool   `json:"enabled"`
	Mode             string `json:"mode"`
	CooldownSeconds  int32  `json:"cooldown_seconds"`
	MinimumSignals   int32  `json:"minimum_signals"`
	MaxEvidenceRefs  int32  `json:"max_evidence_refs"`
	MaxReplaySamples int32  `json:"max_replay_samples"`
	MaxCostUSDTicks  int64  `json:"max_cost_usd_ticks"`
	PolicyVersion    string `json:"policy_version"`
}

type idempotencyRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type decisionRequest struct {
	Reason         string `json:"reason,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
}

type forkRequest struct {
	Name           string `json:"name"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (h *HTTP) overview(w http.ResponseWriter, r *http.Request) {
	workspaceID, skillID, actor, role, ok := h.skillRequest(w, r, "skillId")
	if !ok {
		return
	}
	value, err := h.evolution.Overview(r.Context(), workspaceID, skillID, DefaultOverviewLimit)
	if err != nil {
		h.writeError(w, err)
		return
	}
	admin := role == "owner" || role == "admin"
	creator := value.Skill.Skill.CreatedBy.Valid && value.Skill.Skill.CreatedBy == actor.ID
	writeEvolutionJSON(w, http.StatusOK, overviewResponse(value, permissionsDTO{CanConfigure: admin || creator, CanPublish: admin, CanFork: admin}))
}

func (h *HTTP) configure(w http.ResponseWriter, r *http.Request) {
	workspaceID, skillID, actor, role, ok := h.skillRequest(w, r, "skillId")
	if !ok || !h.requireCreatorOrAdmin(w, r.Context(), workspaceID, skillID, actor.ID, role) {
		return
	}
	var request loopRequest
	if !decodeEvolutionJSON(w, r, &request) {
		return
	}
	value, err := h.evolution.Configure(r.Context(), actor, LoopConfig{
		WorkspaceID: workspaceID, SkillID: skillID, Enabled: request.Enabled, Mode: LoopMode(request.Mode),
		Cooldown: time.Duration(request.CooldownSeconds) * time.Second, MinimumSignals: int(request.MinimumSignals),
		MaxEvidenceRefs: int(request.MaxEvidenceRefs), MaxReplaySamples: int(request.MaxReplaySamples),
		MaxCostUSDTicks: request.MaxCostUSDTicks, PolicyVersion: request.PolicyVersion,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeEvolutionJSON(w, http.StatusOK, loopResponse(value))
}

func (h *HTTP) pause(w http.ResponseWriter, r *http.Request) {
	workspaceID, skillID, actor, role, ok := h.skillRequest(w, r, "skillId")
	if !ok || !h.requireCreatorOrAdmin(w, r.Context(), workspaceID, skillID, actor.ID, role) {
		return
	}
	var request idempotencyRequest
	if !decodeEvolutionJSON(w, r, &request) {
		return
	}
	if !validBoundaryToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	// Pause is a naturally idempotent state assignment. The key is validated
	// at the boundary for the common mutation contract but is not persisted.
	value, err := h.evolution.Pause(r.Context(), actor, workspaceID, skillID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeEvolutionJSON(w, http.StatusOK, loopResponse(value))
}

func (h *HTTP) generate(w http.ResponseWriter, r *http.Request) {
	workspaceID, skillID, actor, role, ok := h.skillRequest(w, r, "skillId")
	if !ok || !h.requireCreatorOrAdmin(w, r.Context(), workspaceID, skillID, actor.ID, role) {
		return
	}
	var request idempotencyRequest
	if !decodeEvolutionJSON(w, r, &request) {
		return
	}
	if !validBoundaryToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if h.proposals == nil {
		writeEvolutionError(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	value, err := h.proposals.RequestProposal(r.Context(), workspaceID, skillID, actor.ID, request.IdempotencyKey)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeEvolutionJSON(w, http.StatusAccepted, proposalRequestResponse(value))
}

func (h *HTTP) proposal(w http.ResponseWriter, r *http.Request) {
	workspaceID, _, _, _, ok := h.requestIdentity(w, r, "proposalId")
	if !ok {
		return
	}
	proposalID, ok := parseBoundaryUUID(w, chi.URLParam(r, "proposalId"))
	if !ok {
		return
	}
	value, err := h.evolution.ReadProposal(r.Context(), workspaceID, proposalID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeEvolutionJSON(w, http.StatusOK, proposalDetailResponse(value))
}

func (h *HTTP) reject(w http.ResponseWriter, r *http.Request) {
	workspaceID, actor, role, ok := h.memberRequest(w, r)
	if !ok {
		return
	}
	proposalID, ok := parseBoundaryUUID(w, chi.URLParam(r, "proposalId"))
	if !ok {
		return
	}
	view, err := h.evolution.ReadProposal(r.Context(), workspaceID, proposalID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if !h.requireCreatorOrAdmin(w, r.Context(), workspaceID, view.Detail.Proposal.SkillID, actor.ID, role) {
		return
	}
	var request decisionRequest
	if !decodeEvolutionJSON(w, r, &request) {
		return
	}
	if !validDecisionRequest(request) {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	value, err := h.evolution.Reject(r.Context(), RejectRequest{WorkspaceID: workspaceID, ProposalID: proposalID, Actor: actor, Reason: request.Reason, IdempotencyKey: request.IdempotencyKey})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeEvolutionJSON(w, http.StatusOK, proposalResponse(value))
}

func (h *HTTP) publish(w http.ResponseWriter, r *http.Request) {
	workspaceID, actor, role, ok := h.memberRequest(w, r)
	if !ok || !requireEvolutionAdmin(w, role) {
		return
	}
	proposalID, ok := parseBoundaryUUID(w, chi.URLParam(r, "proposalId"))
	if !ok {
		return
	}
	var request decisionRequest
	if !decodeEvolutionJSON(w, r, &request) {
		return
	}
	if !validDecisionRequest(request) {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	value, err := h.evolution.Publish(r.Context(), PublishRequest{WorkspaceID: workspaceID, ProposalID: proposalID, Actor: actor, Reason: request.Reason, IdempotencyKey: request.IdempotencyKey})
	if value.DecisionRecorded && !value.Replayed {
		h.metrics.RecordProposalAccepted()
	}
	if err != nil {
		if publicationUnknown(err) && validUUID(value.Release.ID) {
			proposal := proposalResponse(value.Proposal)
			writeEvolutionJSON(w, http.StatusAccepted, publicationDTO{Proposal: &proposal, Release: releaseResponse(value.Release)})
			return
		}
		h.writeError(w, err)
		return
	}
	if !value.Replayed && value.Release.Outcome == string(ReleaseOutcomeSucceeded) {
		h.metrics.RecordPublication(false)
	}
	proposal := proposalResponse(value.Proposal)
	writeEvolutionJSON(w, http.StatusOK, publicationDTO{Proposal: &proposal, Release: releaseResponse(value.Release)})
}

func (h *HTTP) rollback(w http.ResponseWriter, r *http.Request) {
	workspaceID, skillID, actor, role, ok := h.skillRequest(w, r, "skillId")
	if !ok || !requireEvolutionAdmin(w, role) {
		return
	}
	releaseID, ok := parseBoundaryUUID(w, chi.URLParam(r, "releaseId"))
	if !ok {
		return
	}
	var request idempotencyRequest
	if !decodeEvolutionJSON(w, r, &request) {
		return
	}
	if !validBoundaryToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	value, err := h.evolution.Rollback(r.Context(), RollbackRequest{WorkspaceID: workspaceID, SkillID: skillID, SourceReleaseID: releaseID, Actor: actor, IdempotencyKey: request.IdempotencyKey})
	if err != nil {
		if publicationUnknown(err) && validUUID(value.Release.ID) {
			writeEvolutionJSON(w, http.StatusAccepted, publicationDTO{Release: releaseResponse(value.Release)})
			return
		}
		h.writeError(w, err)
		return
	}
	if !value.Replayed && value.Release.Outcome == string(ReleaseOutcomeSucceeded) {
		h.metrics.RecordPublication(true)
	}
	writeEvolutionJSON(w, http.StatusOK, publicationDTO{Release: releaseResponse(value.Release)})
}

func (h *HTTP) fork(w http.ResponseWriter, r *http.Request) {
	workspaceID, sourceID, actor, role, ok := h.skillRequest(w, r, "skillId")
	if !ok || !requireEvolutionAdmin(w, role) {
		return
	}
	var request forkRequest
	if !decodeEvolutionJSON(w, r, &request) {
		return
	}
	if !validBoundaryToken(request.Name, 255) || !validBoundaryToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	value, err := h.evolution.Fork(r.Context(), ForkRequest{WorkspaceID: workspaceID, SourceSkillID: sourceID, NewName: request.Name, Actor: actor, IdempotencyKey: request.IdempotencyKey})
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeEvolutionJSON(w, http.StatusCreated, skillIdentityDTO{ID: util.UUIDToString(value.Skill.Skill.ID), Name: value.Skill.Skill.Name,
		BundleHash: value.Skill.Manifest.Hash, Ownership: string(value.Skill.Ownership.Class), OwnershipReason: string(value.Skill.Ownership.Reason), ForkRequired: value.Skill.Ownership.ForkRequired})
}

func (h *HTTP) skillRequest(w http.ResponseWriter, r *http.Request, param string) (pgtype.UUID, pgtype.UUID, DecisionActor, string, bool) {
	workspaceID, actor, role, ok := h.memberRequest(w, r)
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, DecisionActor{}, "", false
	}
	skillID, ok := parseBoundaryUUID(w, chi.URLParam(r, param))
	return workspaceID, skillID, actor, role, ok
}

func (h *HTTP) requestIdentity(w http.ResponseWriter, r *http.Request, _ string) (pgtype.UUID, pgtype.UUID, DecisionActor, string, bool) {
	workspaceID, actor, role, ok := h.memberRequest(w, r)
	return workspaceID, pgtype.UUID{}, actor, role, ok
}

func (h *HTTP) memberRequest(w http.ResponseWriter, r *http.Request) (pgtype.UUID, DecisionActor, string, bool) {
	if h == nil || h.evolution == nil || h.skills == nil {
		writeEvolutionError(w, http.StatusServiceUnavailable, "unavailable")
		return pgtype.UUID{}, DecisionActor{}, "", false
	}
	workspaceID, err := util.ParseUUID(middleware.WorkspaceIDFromContext(r.Context()))
	member, memberOK := middleware.MemberFromContext(r.Context())
	if err != nil || !memberOK || !member.UserID.Valid || member.WorkspaceID != workspaceID {
		writeEvolutionError(w, http.StatusNotFound, "not_found")
		return pgtype.UUID{}, DecisionActor{}, "", false
	}
	return workspaceID, DecisionActor{ID: member.UserID, Kind: ActorKindHuman}, member.Role, true
}

func (h *HTTP) requireCreatorOrAdmin(w http.ResponseWriter, ctx context.Context, workspaceID, skillID, actorID pgtype.UUID, role string) bool {
	if role == "owner" || role == "admin" {
		return true
	}
	skill, err := h.skills.Load(ctx, workspaceID, skillID)
	if err != nil {
		h.writeError(w, err)
		return false
	}
	if !skill.Skill.CreatedBy.Valid || skill.Skill.CreatedBy != actorID {
		writeEvolutionError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

func requireEvolutionAdmin(w http.ResponseWriter, role string) bool {
	if role == "owner" || role == "admin" {
		return true
	}
	writeEvolutionError(w, http.StatusForbidden, "forbidden")
	return false
}

func (h *HTTP) writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, ErrWorkspaceSkillNotFound), errors.Is(err, ErrPersistenceNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, ErrHumanActorRequired):
		status, code = http.StatusForbidden, "human_required"
	case errors.Is(err, ErrForkRequired):
		status, code = http.StatusConflict, "fork_required"
	case errors.Is(err, ErrStaleBase), errors.Is(err, ErrConcurrentBundleDrift), errors.Is(err, ErrPersistenceConflict),
		errors.Is(err, ErrDecisionConflict), errors.Is(err, ErrGenerationActive), errors.Is(err, ErrEvolutionCooldown),
		errors.Is(err, ErrRoomCandidateSourceDrift):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, ErrPublicationUnknown), errors.Is(err, ErrReleaseNotRetryable):
		status, code = http.StatusAccepted, "publication_unknown"
	case errors.Is(err, ErrEvolutionDisabled), errors.Is(err, ErrEvolutionPaused), errors.Is(err, ErrEvolutionObserveOnly),
		errors.Is(err, ErrInsufficientSignals), errors.Is(err, ErrEvaluationFailed), errors.Is(err, ErrRoomCandidateNotReady),
		errors.Is(err, ErrImprovementRoomUnavailable), errors.Is(err, ErrImprovementRoomContextTooBig):
		status, code = http.StatusUnprocessableEntity, "not_ready"
	case errors.Is(err, ErrLifecycleInvalidInput), errors.Is(err, ErrPersistenceInvalidInput),
		errors.Is(err, ErrWorkspaceSkillInvalidInput), errors.Is(err, ErrInvalidEvidenceRef), errors.Is(err, ErrRoomCandidateInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	}
	writeEvolutionError(w, status, code)
}

func decodeEvolutionJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, skillEvolutionBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			writeEvolutionError(w, http.StatusRequestEntityTooLarge, "body_too_large")
		} else {
			writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func parseBoundaryUUID(w http.ResponseWriter, value string) (pgtype.UUID, bool) {
	parsed, err := util.ParseUUID(value)
	if err != nil {
		writeEvolutionError(w, http.StatusBadRequest, "invalid_id")
		return pgtype.UUID{}, false
	}
	return parsed, true
}

func validBoundaryToken(value string, max int) bool {
	return value != "" && len(value) <= max && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n\t")
}

func validDecisionRequest(request decisionRequest) bool {
	return validBoundaryToken(request.IdempotencyKey, MaxIdempotencyKeyBytes) &&
		len([]rune(request.Reason)) <= MaxReviewReasonRunes && !strings.ContainsRune(request.Reason, 0)
}

func publicationUnknown(err error) bool {
	return errors.Is(err, ErrPublicationUnknown) || errors.Is(err, ErrReleaseNotRetryable)
}

func writeEvolutionJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeEvolutionError(w http.ResponseWriter, status int, code string) {
	writeEvolutionJSON(w, status, map[string]string{"error": code})
}
