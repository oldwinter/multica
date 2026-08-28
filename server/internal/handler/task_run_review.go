package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const taskRunReviewRequestBodyLimit = 16 << 10

type taskRunReviewHTTPService interface {
	CreateTaskRunReview(context.Context, string, string, service.CreateTaskRunReviewInput) (service.TaskRunReviewEvidence, error)
	ListTaskRunReviewRefs(context.Context, string, string, string, int) (service.TaskRunReviewRefPage, error)
	LoadTaskRunReviewEvidence(context.Context, string, string, string) (service.TaskRunReviewEvidence, error)
	ListManualRerunRefs(context.Context, string, string, string, int) (service.ManualRerunPage, error)
	LoadManualRerunEvidence(context.Context, string, string, string) (service.ManualRerunEvidence, error)
}

// TaskRunReviewHTTPHandler is intentionally not registered in the central
// router here. It is a narrow task-owned leaf that can be composed when the
// persistence implementation lands without changing ordinary task execution.
type TaskRunReviewHTTPHandler struct {
	root    *Handler
	service taskRunReviewHTTPService
}

func NewTaskRunReviewHTTPHandler(root *Handler, taskReviewService *service.TaskRunReviewService) *TaskRunReviewHTTPHandler {
	return &TaskRunReviewHTTPHandler{root: root, service: taskReviewService}
}

// NewTaskRunReviewTaskAccess exposes the existing workspace/private-Agent
// visibility policy through the task service's narrow authorization port.
func NewTaskRunReviewTaskAccess(root *Handler) service.TaskRunReviewTaskAccess {
	return taskRunReviewTaskAccess{root: root}
}

func (h *TaskRunReviewHTTPHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID, reviewerID, ok := h.requireHumanReviewer(w, r)
	if !ok {
		return
	}
	var input service.CreateTaskRunReviewInput
	if !decodeTaskRunReviewJSON(w, r, &input) {
		return
	}
	input.TaskID = chi.URLParam(r, "taskId")
	evidence, err := h.service.CreateTaskRunReview(r.Context(), workspaceID, reviewerID, input)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, evidence)
}

func (h *TaskRunReviewHTTPHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID, reviewerID, ok := h.requireHumanReviewer(w, r)
	if !ok {
		return
	}
	cursor, limit, ok := taskRunReviewListParams(w, r)
	if !ok {
		return
	}
	page, err := h.service.ListTaskRunReviewRefs(r.Context(), workspaceID, reviewerID, cursor, limit)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *TaskRunReviewHTTPHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID, reviewerID, ok := h.requireHumanReviewer(w, r)
	if !ok {
		return
	}
	evidence, err := h.service.LoadTaskRunReviewEvidence(r.Context(), workspaceID, reviewerID, chi.URLParam(r, "reviewId"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func (h *TaskRunReviewHTTPHandler) ListManualReruns(w http.ResponseWriter, r *http.Request) {
	workspaceID, reviewerID, ok := h.requireHumanReviewer(w, r)
	if !ok {
		return
	}
	cursor, limit, ok := taskRunReviewListParams(w, r)
	if !ok {
		return
	}
	page, err := h.service.ListManualRerunRefs(r.Context(), workspaceID, reviewerID, cursor, limit)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *TaskRunReviewHTTPHandler) GetManualRerun(w http.ResponseWriter, r *http.Request) {
	workspaceID, reviewerID, ok := h.requireHumanReviewer(w, r)
	if !ok {
		return
	}
	evidence, err := h.service.LoadManualRerunEvidence(r.Context(), workspaceID, reviewerID, chi.URLParam(r, "taskId"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, evidence)
}

func (h *TaskRunReviewHTTPHandler) requireHumanReviewer(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	if h == nil || h.root == nil {
		writeError(w, http.StatusServiceUnavailable, "task run reviews are unavailable")
		return "", "", false
	}
	if isMachineCredentialActor(r) {
		writeError(w, http.StatusForbidden, "this endpoint is only available to human actors")
		return "", "", false
	}
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "task run reviews are unavailable")
		return "", "", false
	}
	reviewerID, ok := requireUserID(w, r)
	if !ok {
		return "", "", false
	}
	workspaceID := h.root.resolveWorkspaceID(r)
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspace id is invalid")
		return "", "", false
	}
	workspaceID = util.UUIDToString(workspaceUUID)
	actorType, _ := h.root.resolveActor(r, reviewerID, workspaceID)
	if actorType != "member" {
		writeError(w, http.StatusForbidden, "this endpoint is only available to human actors")
		return "", "", false
	}
	return workspaceID, reviewerID, true
}

func (h *TaskRunReviewHTTPHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTaskRunReviewInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrTaskRunReviewTaskActive), errors.Is(err, service.ErrTaskRunReviewSourceChanged):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrTaskRunReviewForbidden):
		writeError(w, http.StatusForbidden, "task run review source is not authorized")
	case errors.Is(err, service.ErrTaskRunReviewNotFound):
		writeError(w, http.StatusNotFound, "task run review source not found")
	case errors.Is(err, service.ErrTaskRunReviewUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "task run review request failed")
	}
}

func taskRunReviewListParams(w http.ResponseWriter, r *http.Request) (string, int, bool) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > service.MaxTaskRunReviewRefs {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return "", 0, false
		}
		limit = parsed
	}
	cursor := r.URL.Query().Get("cursor")
	if len(cursor) > service.MaxTaskRunReviewCursorLen {
		writeError(w, http.StatusBadRequest, "cursor is invalid")
		return "", 0, false
	}
	return cursor, limit, true
}

func decodeTaskRunReviewJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, taskRunReviewRequestBodyLimit)
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

type taskRunReviewTaskAccess struct{ root *Handler }

func (a taskRunReviewTaskAccess) ValidateWorkspaceMember(ctx context.Context, workspaceID, reviewerID string) error {
	if a.root == nil || a.root.Queries == nil {
		return service.ErrTaskRunReviewUnavailable
	}
	if _, err := a.root.getWorkspaceMember(ctx, reviewerID, workspaceID); errors.Is(err, pgx.ErrNoRows) {
		return service.ErrTaskRunReviewForbidden
	} else if err != nil {
		return service.ErrTaskRunReviewUnavailable
	}
	return nil
}

func (a taskRunReviewTaskAccess) LoadAuthorizedTask(ctx context.Context, workspaceID, reviewerID, taskID string) (service.TaskRunReviewTask, error) {
	if a.root == nil || a.root.Queries == nil {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewUnavailable
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewInvalid
	}
	taskUUID, err := util.ParseUUID(taskID)
	if err != nil {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewInvalid
	}
	task, err := a.root.Queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: taskUUID, WorkspaceID: workspaceUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewNotFound
	}
	if err != nil {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewUnavailable
	}
	agent, err := a.root.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: task.AgentID, WorkspaceID: workspaceUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewNotFound
	}
	if err != nil {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewUnavailable
	}
	allowed, err := a.canAccessTaskAgent(ctx, agent, reviewerID, workspaceID)
	if err != nil {
		return service.TaskRunReviewTask{}, err
	}
	if !allowed {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewForbidden
	}
	return service.TaskRunReviewTask{
		ID: util.UUIDToString(task.ID), WorkspaceID: workspaceID,
		AgentID: util.UUIDToString(task.AgentID), IssueID: util.UUIDToString(task.IssueID),
		ChatSessionID: util.UUIDToString(task.ChatSessionID), Status: task.Status,
	}, nil
}

// canAccessTaskAgent preserves canAccessPrivateAgent's member semantics while
// retaining query errors. Lists may hide an unauthorized private Agent, but
// must not turn a membership or invocation-target outage into an empty 200.
func (a taskRunReviewTaskAccess) canAccessTaskAgent(ctx context.Context, agent db.Agent, reviewerID, workspaceID string) (bool, error) {
	if util.UUIDToString(agent.OwnerID) == reviewerID {
		return true, nil
	}
	member, err := a.root.getWorkspaceMember(ctx, reviewerID, workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, service.ErrTaskRunReviewUnavailable
	}
	if roleAllowed(member.Role, "owner", "admin") {
		return true, nil
	}
	if agent.PermissionMode != "public_to" {
		return false, nil
	}
	targets, err := a.root.Queries.ListAgentInvocationTargets(ctx, agent.ID)
	if err != nil {
		return false, service.ErrTaskRunReviewUnavailable
	}
	return memberHitsInvocationTargets(targets, reviewerID), nil
}

func (a taskRunReviewTaskAccess) ValidateTargetSkill(ctx context.Context, workspaceID, skillID string) error {
	if a.root == nil || a.root.Queries == nil {
		return service.ErrTaskRunReviewUnavailable
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		return service.ErrTaskRunReviewInvalid
	}
	skillUUID, err := util.ParseUUID(skillID)
	if err != nil {
		return service.ErrTaskRunReviewInvalid
	}
	if _, err := a.root.Queries.GetSkillInWorkspace(ctx, db.GetSkillInWorkspaceParams{ID: skillUUID, WorkspaceID: workspaceUUID}); errors.Is(err, pgx.ErrNoRows) {
		return service.ErrTaskRunReviewNotFound
	} else if err != nil {
		return service.ErrTaskRunReviewUnavailable
	}
	return nil
}
