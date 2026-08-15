package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
)

func TestRoomMutationBodiesAreBounded(t *testing.T) {
	if testHandler == nil {
		t.Skip("handler not available")
	}
	roomID := testWorkspaceID
	payload := `{"status":"` + strings.Repeat("x", roomMutationBodyLimit) + `"}`
	tests := []struct {
		name string
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "create", path: "/api/rooms", call: testHandler.CreateRoom},
		{name: "message", path: "/api/rooms/" + roomID + "/messages", call: testHandler.PostRoomMessage},
		{name: "wake", path: "/api/rooms/" + roomID + "/wake", call: testHandler.WakeRoom},
		{name: "status", path: "/api/rooms/" + roomID + "/status", call: testHandler.SetRoomStatus},
		{name: "promotion", path: "/api/rooms/" + roomID + "/promotions", call: testHandler.PromoteRoomArtifact},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(payload))
			request.ContentLength = -1
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-User-ID", testUserID)
			request.Header.Set("X-Workspace-ID", testWorkspaceID)
			if test.name != "create" {
				request = withURLParam(request, "id", roomID)
			}
			recorder := httptest.NewRecorder()

			test.call(recorder, request)

			if recorder.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
			}
		})
	}
}

func TestRoomMutationArraysAreBounded(t *testing.T) {
	if testHandler == nil {
		t.Skip("handler not available")
	}
	roomID := testWorkspaceID
	ids := make([]string, maxRoomAgentTargets+1)
	for index := range ids {
		ids[index] = testWorkspaceID
	}
	participants := make([]map[string]string, maxRoomParticipantRequests+1)
	for index := range participants {
		participants[index] = map[string]string{"type": "agent", "id": testWorkspaceID}
	}
	tests := []struct {
		name string
		path string
		body map[string]any
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "participants", path: "/api/rooms", body: map[string]any{"participants": participants}, call: testHandler.CreateRoom},
		{name: "mentions", path: "/api/rooms/" + roomID + "/messages", body: map[string]any{"mention_agent_ids": ids}, call: testHandler.PostRoomMessage},
		{name: "targets", path: "/api/rooms/" + roomID + "/wake", body: map[string]any{"target_agent_ids": ids}, call: testHandler.WakeRoom},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := newRequest(http.MethodPost, test.path, test.body)
			if test.name != "participants" {
				request = withURLParam(request, "id", roomID)
			}
			recorder := httptest.NewRecorder()

			test.call(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "too many") {
				t.Fatalf("response does not explain array limit: %s", recorder.Body.String())
			}
		})
	}
}

func TestRoomHTTPWorkflowPersistsRefusalsAndHidesRuntimeState(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	var agentID string
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM agent WHERE workspace_id = $1 AND name = 'Handler Test Agent'
	`, testWorkspaceID).Scan(&agentID); err != nil {
		t.Fatal(err)
	}

	createRecorder := httptest.NewRecorder()
	createRequest := newRequest(http.MethodPost, "/api/rooms", map[string]any{
		"title": "Handler Room", "instructions": "Keep decisions explicit.",
		"facilitator_agent_id": agentID, "daily_turn_limit": 5,
	})
	testHandler.CreateRoom(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create Room: %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created roomDetailResponse
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	roomID := created.Room.ID
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE room_id = $1)`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM room_artifact WHERE room_id = $1`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM room_turn WHERE room_id = $1`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM room_cycle WHERE room_id = $1`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM room_entry WHERE room_id = $1`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM room_participant WHERE room_id = $1`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM room WHERE id = $1`, roomID)
	})

	messageRecorder := httptest.NewRecorder()
	messageRequest := newRequest(http.MethodPost, "/api/rooms/"+roomID+"/messages", map[string]any{
		"body": "Assess the recovery boundary.", "idempotency_key": "handler-message-1",
	})
	messageRequest = withURLParam(messageRequest, "id", roomID)
	testHandler.PostRoomMessage(messageRecorder, messageRequest)
	if messageRecorder.Code != http.StatusCreated {
		t.Fatalf("post Room message: %d: %s", messageRecorder.Code, messageRecorder.Body.String())
	}
	var messageResult struct {
		Tasks []string `json:"tasks"`
	}
	if err := json.Unmarshal(messageRecorder.Body.Bytes(), &messageResult); err != nil || len(messageResult.Tasks) != 1 {
		t.Fatalf("Room task response = %+v, err = %v", messageResult, err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'completed', result = '{"output":"Persist terminal projections atomically."}'::jsonb,
		    session_id = 'private-session', work_dir = '/private/work',
		    started_at = now(), completed_at = now()
		WHERE id = $1
	`, messageResult.Tasks[0]); err != nil {
		t.Fatal(err)
	}
	taskID, err := util.ParseUUID(messageResult.Tasks[0])
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := testHandler.RoomRuntime.SyncTask(ctx, taskID); err != nil || !changed {
		t.Fatalf("synchronize Room task = %v, %v", changed, err)
	}

	getRecorder := httptest.NewRecorder()
	getRequest := withURLParam(newRequest(http.MethodGet, "/api/rooms/"+roomID, nil), "id", roomID)
	testHandler.GetRoom(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get Room: %d: %s", getRecorder.Code, getRecorder.Body.String())
	}
	if strings.Contains(getRecorder.Body.String(), "private-session") || strings.Contains(getRecorder.Body.String(), "/private/work") {
		t.Fatalf("Room API leaked runtime continuity state: %s", getRecorder.Body.String())
	}
	var detail roomDetailResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Entries) != 2 || detail.Room.MemoryVersion != 1 {
		t.Fatalf("Room detail entries/memory = %d/%d", len(detail.Entries), detail.Room.MemoryVersion)
	}
	resultEntryID := detail.Entries[1].ID

	promoteRecorder := httptest.NewRecorder()
	promoteRequest := withURLParam(newRequest(http.MethodPost, "/api/rooms/"+roomID+"/promotions", map[string]any{
		"kind": "decision", "entry_id": resultEntryID,
		"idempotency_key": "handler-decision-1", "title": "Atomic terminal projections",
	}), "id", roomID)
	testHandler.PromoteRoomArtifact(promoteRecorder, promoteRequest)
	if promoteRecorder.Code != http.StatusCreated {
		t.Fatalf("promote Room artifact: %d: %s", promoteRecorder.Code, promoteRecorder.Body.String())
	}

	statusRecorder := httptest.NewRecorder()
	statusRequest := withURLParam(newRequest(http.MethodPut, "/api/rooms/"+roomID+"/status", map[string]any{"status": "paused"}), "id", roomID)
	testHandler.SetRoomStatus(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("pause Room: %d: %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	pausedRecorder := httptest.NewRecorder()
	pausedRequest := withURLParam(newRequest(http.MethodPost, "/api/rooms/"+roomID+"/messages", map[string]any{
		"body": "This message must remain visible.", "idempotency_key": "handler-message-paused",
	}), "id", roomID)
	testHandler.PostRoomMessage(pausedRecorder, pausedRequest)
	if pausedRecorder.Code != http.StatusConflict || !strings.Contains(pausedRecorder.Body.String(), "room_paused") {
		t.Fatalf("paused Room message: %d: %s", pausedRecorder.Code, pausedRecorder.Body.String())
	}
	var entryCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM room_entry WHERE room_id = $1`, roomID).Scan(&entryCount); err != nil {
		t.Fatal(err)
	}
	if entryCount != 3 {
		t.Fatalf("paused message entry count = %d, want 3", entryCount)
	}
}
