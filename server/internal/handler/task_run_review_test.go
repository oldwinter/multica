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
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

const (
	handlerTaskReviewWorkspaceID = "11111111-1111-4111-8111-111111111111"
	handlerTaskReviewUserID      = "22222222-2222-4222-8222-222222222222"
	handlerTaskReviewTaskID      = "33333333-3333-4333-8333-333333333333"
	handlerTaskReviewReviewID    = "44444444-4444-4444-8444-444444444444"
)

type fakeTaskRunReviewHTTPService struct {
	createInput service.CreateTaskRunReviewInput
	workspaceID string
	reviewerID  string
	calls       int
	err         error
}

func (f *fakeTaskRunReviewHTTPService) CreateTaskRunReview(_ context.Context, workspaceID, reviewerID string, input service.CreateTaskRunReviewInput) (service.TaskRunReviewEvidence, error) {
	f.calls++
	f.workspaceID, f.reviewerID, f.createInput = workspaceID, reviewerID, input
	if f.err != nil {
		return service.TaskRunReviewEvidence{}, f.err
	}
	return service.TaskRunReviewEvidence{TaskRunReviewRecord: service.TaskRunReviewRecord{
		ID: handlerTaskReviewReviewID, WorkspaceID: workspaceID, TaskID: input.TaskID,
		ReviewerID: reviewerID, Outcome: input.Outcome, Target: input.Target,
		Correction: input.Correction, Reason: input.Reason,
		Digest:    "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CreatedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC),
	}}, nil
}

func (f *fakeTaskRunReviewHTTPService) ListTaskRunReviewRefs(context.Context, string, string, string, int) (service.TaskRunReviewRefPage, error) {
	f.calls++
	if f.err != nil {
		return service.TaskRunReviewRefPage{}, f.err
	}
	return service.TaskRunReviewRefPage{Refs: []service.TaskRunReviewRef{{
		ID: handlerTaskReviewReviewID, WorkspaceID: handlerTaskReviewWorkspaceID,
		TaskID: handlerTaskReviewTaskID, ReviewerID: handlerTaskReviewUserID,
		Outcome: service.TaskRunReviewOutcomeHelpful, Target: service.TaskRunReviewTargetKnowledge,
		Digest:    "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CreatedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC),
	}}}, nil
}

func (f *fakeTaskRunReviewHTTPService) LoadTaskRunReviewEvidence(context.Context, string, string, string) (service.TaskRunReviewEvidence, error) {
	f.calls++
	return service.TaskRunReviewEvidence{}, f.err
}

func (f *fakeTaskRunReviewHTTPService) ListManualRerunRefs(context.Context, string, string, string, int) (service.ManualRerunPage, error) {
	f.calls++
	return service.ManualRerunPage{}, f.err
}

func (f *fakeTaskRunReviewHTTPService) LoadManualRerunEvidence(context.Context, string, string, string) (service.ManualRerunEvidence, error) {
	f.calls++
	return service.ManualRerunEvidence{}, f.err
}

func taskRunReviewRequest(method, target, body string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", handlerTaskReviewUserID)
	routeContext := chi.NewRouteContext()
	for key, value := range params {
		routeContext.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

func TestTaskRunReviewHTTPCreateContract(t *testing.T) {
	fake := &fakeTaskRunReviewHTTPService{}
	h := &TaskRunReviewHTTPHandler{root: &Handler{}, service: fake}
	req := taskRunReviewRequest(http.MethodPost,
		"/api/task-run-reviews?workspace_id="+strings.ToUpper(handlerTaskReviewWorkspaceID),
		`{"idempotency_key":"task-review:handler-contract","outcome":"needs_correction","target":"product_defect","correction":"Bound the retry.","reason":"The task retried forever."}`,
		map[string]string{"taskId": handlerTaskReviewTaskID},
	)
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	if fake.calls != 1 || fake.workspaceID != handlerTaskReviewWorkspaceID || fake.reviewerID != handlerTaskReviewUserID ||
		fake.createInput.TaskID != handlerTaskReviewTaskID || fake.createInput.IdempotencyKey != "task-review:handler-contract" {
		t.Fatalf("service call = %#v", fake)
	}
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response["id"] != handlerTaskReviewReviewID {
		t.Fatalf("response = %s, error = %v", w.Body.String(), err)
	}
	if _, exposed := response["idempotency_key"]; exposed {
		t.Fatalf("response exposed idempotency key: %s", w.Body.String())
	}
}

func TestTaskRunReviewHTTPRejectsMachineActorsBeforeService(t *testing.T) {
	for _, source := range []string{"task_token", "cloud_pat"} {
		t.Run(source, func(t *testing.T) {
			fake := &fakeTaskRunReviewHTTPService{}
			h := &TaskRunReviewHTTPHandler{root: &Handler{}, service: fake}
			req := taskRunReviewRequest(http.MethodGet,
				"/api/task-run-reviews?workspace_id="+handlerTaskReviewWorkspaceID, "", nil,
			)
			req.Header.Set("X-Actor-Source", source)
			w := httptest.NewRecorder()
			h.List(w, req)
			if w.Code != http.StatusForbidden || fake.calls != 0 {
				t.Fatalf("status = %d, calls = %d: %s", w.Code, fake.calls, w.Body.String())
			}
		})
	}
}

func TestTaskRunReviewHTTPRejectsUnknownAndOversizeBodies(t *testing.T) {
	for _, test := range []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "unknown field", body: `{"outcome":"helpful","target":"knowledge","reason":"ok","feedback":"leak"}`, wantStatus: http.StatusBadRequest},
		{name: "oversize", body: `{"outcome":"helpful","target":"knowledge","reason":"` + strings.Repeat("x", taskRunReviewRequestBodyLimit) + `"}`, wantStatus: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeTaskRunReviewHTTPService{}
			h := &TaskRunReviewHTTPHandler{root: &Handler{}, service: fake}
			req := taskRunReviewRequest(http.MethodPost,
				"/api/task-run-reviews?workspace_id="+handlerTaskReviewWorkspaceID,
				test.body, map[string]string{"taskId": handlerTaskReviewTaskID},
			)
			w := httptest.NewRecorder()
			h.Create(w, req)
			if w.Code != test.wantStatus || fake.calls != 0 {
				t.Fatalf("status = %d, want %d; calls = %d: %s", w.Code, test.wantStatus, fake.calls, w.Body.String())
			}
		})
	}
}

func TestTaskRunReviewHTTPCanonicalizesUUIDBoundariesBeforeService(t *testing.T) {
	tests := []struct {
		name string
		call func(*TaskRunReviewHTTPHandler, http.ResponseWriter, *http.Request)
		req  *http.Request
	}{
		{
			name: "workspace",
			call: (*TaskRunReviewHTTPHandler).List,
			req:  taskRunReviewRequest(http.MethodGet, "/api/task-run-reviews?workspace_id=not-a-uuid", "", nil),
		},
		{
			name: "create task",
			call: (*TaskRunReviewHTTPHandler).Create,
			req: taskRunReviewRequest(http.MethodPost, "/api/task-run-reviews?workspace_id="+handlerTaskReviewWorkspaceID,
				`{"idempotency_key":"uuid-boundary","outcome":"helpful","target":"knowledge","reason":"bounded"}`,
				map[string]string{"taskId": "not-a-uuid"}),
		},
		{
			name: "review",
			call: (*TaskRunReviewHTTPHandler).Get,
			req: taskRunReviewRequest(http.MethodGet, "/api/task-run-reviews/not-a-uuid?workspace_id="+handlerTaskReviewWorkspaceID,
				"", map[string]string{"reviewId": "not-a-uuid"}),
		},
		{
			name: "manual rerun task",
			call: (*TaskRunReviewHTTPHandler).GetManualRerun,
			req: taskRunReviewRequest(http.MethodGet, "/api/task-run-reviews/manual-reruns/not-a-uuid?workspace_id="+handlerTaskReviewWorkspaceID,
				"", map[string]string{"taskId": "not-a-uuid"}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeTaskRunReviewHTTPService{}
			h := &TaskRunReviewHTTPHandler{root: &Handler{}, service: fake}
			result := testutil.Call(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				test.call(h, w, r)
			}), test.req).Want(http.StatusBadRequest)
			if fake.calls != 0 || !strings.Contains(result.Body.String(), "invalid") {
				t.Fatalf("service calls=%d body=%s", fake.calls, result.Body.String())
			}
		})
	}
}

func TestTaskRunReviewHTTPValidCanonicalUUIDPreservesNotFoundSemantics(t *testing.T) {
	fake := &fakeTaskRunReviewHTTPService{err: service.ErrTaskRunReviewNotFound}
	h := &TaskRunReviewHTTPHandler{root: &Handler{}, service: fake}
	req := taskRunReviewRequest(http.MethodGet,
		"/api/task-run-reviews/"+handlerTaskReviewReviewID+"?workspace_id="+strings.ToUpper(handlerTaskReviewWorkspaceID),
		"", map[string]string{"reviewId": handlerTaskReviewReviewID},
	)
	result := testutil.Call(t, h.Get, req).Want(http.StatusNotFound)
	if fake.calls != 1 || !strings.Contains(result.Text(), "not found") {
		t.Fatalf("service calls=%d body=%s", fake.calls, result.Text())
	}
}

func TestTaskRunReviewHTTPListIsContentFree(t *testing.T) {
	fake := &fakeTaskRunReviewHTTPService{}
	h := &TaskRunReviewHTTPHandler{root: &Handler{}, service: fake}
	req := taskRunReviewRequest(http.MethodGet,
		"/api/task-run-reviews?workspace_id="+handlerTaskReviewWorkspaceID+"&limit=10", "", nil,
	)
	w := httptest.NewRecorder()
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "correction") || strings.Contains(w.Body.String(), "reason") {
		t.Fatalf("list response exposed review content: %s", w.Body.String())
	}
}

func TestTaskRunReviewHTTPMapsLifecycleErrors(t *testing.T) {
	for _, test := range []struct {
		err  error
		want int
	}{
		{service.ErrTaskRunReviewInvalid, http.StatusBadRequest},
		{service.ErrTaskRunReviewTaskActive, http.StatusConflict},
		{service.ErrTaskRunReviewSourceChanged, http.StatusConflict},
		{service.ErrTaskRunReviewForbidden, http.StatusForbidden},
		{service.ErrTaskRunReviewNotFound, http.StatusNotFound},
		{service.ErrTaskRunReviewUnavailable, http.StatusServiceUnavailable},
		{errors.New("database offline"), http.StatusInternalServerError},
	} {
		fake := &fakeTaskRunReviewHTTPService{err: test.err}
		h := &TaskRunReviewHTTPHandler{root: &Handler{}, service: fake}
		req := taskRunReviewRequest(http.MethodGet,
			"/api/task-run-reviews?workspace_id="+handlerTaskReviewWorkspaceID, "", nil,
		)
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != test.want {
			t.Fatalf("error %v: status = %d, want %d: %s", test.err, w.Code, test.want, w.Body.String())
		}
	}
}

func TestTaskRunReviewHTTPCreateIdempotencyLivePostgres(t *testing.T) {
	if testHandler == nil || testPool == nil || dbfx == nil {
		t.Skip("handler database fixture is unavailable")
	}
	var schemaReady bool
	dbfx.QueryRow(t, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'task_run_review'
			  AND column_name = 'idempotency_key'
		)
	`).Scan(&schemaReady)
	if !schemaReady {
		t.Skip("task run review idempotency migrations are not applied")
	}

	runtimeID := dbfx.Runtime(t, "task review idempotency runtime")
	agentID := dbfx.Agent(t, "task review idempotency agent", runtimeID)
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"status": "completed", "completed_at": testutil.Raw("now()"),
	})
	dbfx.Cleanup(t, `DELETE FROM task_run_review WHERE task_id = $1`, taskID)
	svc := service.NewTaskRunReviewService(
		service.NewDBTaskRunReviewRepository(testHandler.Queries),
		NewTaskRunReviewTaskAccess(testHandler),
	)
	h := NewTaskRunReviewHTTPHandler(testHandler, svc)
	requestBody := map[string]any{
		"idempotency_key": "task-review:http-live",
		"outcome":         "needs_correction",
		"target":          "product_defect",
		"correction":      "Bound the retry.",
		"reason":          "The task retried forever.",
	}
	post := func(body any) *http.Request {
		req := testutil.JSONRequest(http.MethodPost, "/api/tasks/"+taskID+"/review?workspace_id="+testWorkspaceID, body)
		req = testutil.WithHeaders(req, "X-User-ID", testUserID)
		return testutil.WithURLParams(req, "taskId", taskID)
	}

	firstResponse := testutil.Call(t, h.Create, post(requestBody)).Want(http.StatusCreated)
	var first service.TaskRunReviewEvidence
	firstResponse.JSON(&first)
	if strings.Contains(firstResponse.Text(), "idempotency_key") || strings.Contains(firstResponse.Text(), "task-review:http-live") {
		t.Fatalf("create response exposed idempotency key: %s", firstResponse.Text())
	}
	var replayed service.TaskRunReviewEvidence
	testutil.Call(t, h.Create, post(requestBody)).Want(http.StatusCreated).JSON(&replayed)
	if replayed.ID != first.ID || replayed.Digest != first.Digest || !replayed.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("HTTP replay diverged: first=%#v replayed=%#v", first, replayed)
	}

	conflict := map[string]any{}
	for key, value := range requestBody {
		conflict[key] = value
	}
	conflict["reason"] = "Different canonical payload."
	testutil.Call(t, h.Create, post(conflict)).Want(http.StatusConflict)
	for _, body := range []map[string]any{
		{"outcome": "helpful", "target": "knowledge", "reason": "missing key"},
		{"idempotency_key": " \t ", "outcome": "helpful", "target": "knowledge", "reason": "blank key"},
		{"idempotency_key": strings.Repeat("x", service.MaxTaskRunReviewIdempotencyKeyBytes+1), "outcome": "helpful", "target": "knowledge", "reason": "long key"},
	} {
		testutil.Call(t, h.Create, post(body)).Want(http.StatusBadRequest)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM task_run_review WHERE task_id = $1`, taskID); count != 1 {
		t.Fatalf("HTTP retries persisted %d rows, want 1", count)
	}
}
