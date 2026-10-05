package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
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

	createRequest := newRequest(http.MethodPost, "/api/rooms", map[string]any{
		"title": "Handler Room", "instructions": "Keep decisions explicit.",
		"facilitator_agent_id": agentID, "daily_turn_limit": 5,
	})
	var created roomDetailResponse
	testutil.Call(t, testHandler.CreateRoom, createRequest).
		Want(http.StatusCreated).
		JSON(&created)
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

	messageRequest := newRequest(http.MethodPost, "/api/rooms/"+roomID+"/messages", map[string]any{
		"body": "Assess the recovery boundary.", "idempotency_key": "handler-message-1",
	})
	messageRequest = withURLParam(messageRequest, "id", roomID)
	var messageResult struct {
		Tasks []string `json:"tasks"`
	}
	testutil.Call(t, testHandler.PostRoomMessage, messageRequest).
		Want(http.StatusCreated).
		JSON(&messageResult)
	if len(messageResult.Tasks) != 1 {
		t.Fatalf("Room task response = %+v", messageResult)
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

	getRequest := withURLParam(newRequest(http.MethodGet, "/api/rooms/"+roomID, nil), "id", roomID)
	var detail roomDetailResponse
	getResponse := testutil.Call(t, testHandler.GetRoom, getRequest).
		Want(http.StatusOK).
		JSON(&detail)
	if strings.Contains(getResponse.Text(), "private-session") || strings.Contains(getResponse.Text(), "/private/work") {
		t.Fatalf("Room API leaked runtime continuity state: %s", getResponse.Text())
	}
	if len(detail.Entries) != 2 || detail.Room.MemoryVersion != 1 {
		t.Fatalf("Room detail entries/memory = %d/%d", len(detail.Entries), detail.Room.MemoryVersion)
	}
	resultEntryID := detail.Entries[1].ID

	promoteRequest := withURLParam(newRequest(http.MethodPost, "/api/rooms/"+roomID+"/promotions", map[string]any{
		"kind": "decision", "entry_id": resultEntryID,
		"idempotency_key": "handler-decision-1", "title": "Atomic terminal projections",
	}), "id", roomID)
	testutil.Call(t, testHandler.PromoteRoomArtifact, promoteRequest).Want(http.StatusCreated)

	wikiPromoteRequest := withURLParam(newRequest(http.MethodPost, "/api/rooms/"+roomID+"/promotions", map[string]any{
		"kind": "wiki", "entry_id": resultEntryID,
		"idempotency_key": "handler-wiki-1", "title": "Atomic terminal projections",
	}), "id", roomID)
	wikiRefusal := testutil.Call(t, testHandler.PromoteRoomArtifact, wikiPromoteRequest).
		Want(http.StatusBadRequest)
	if !strings.Contains(wikiRefusal.Text(), "invalid room input") {
		t.Fatalf("unavailable Wiki proposal target response: %s", wikiRefusal.Text())
	}
	var wikiArtifactCount, wikiPageCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM room_artifact
		WHERE room_id = $1 AND kind = 'wiki' AND idempotency_key = 'handler-wiki-1'
	`, roomID).Scan(&wikiArtifactCount); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM wiki_page
		WHERE workspace_id = $1 AND path LIKE 'rooms/' || $2 || '/%'
	`, testWorkspaceID, roomID).Scan(&wikiPageCount); err != nil {
		t.Fatal(err)
	}
	if wikiArtifactCount != 0 || wikiPageCount != 0 {
		t.Fatalf("unavailable Wiki proposal target left artifact/page rows = %d/%d", wikiArtifactCount, wikiPageCount)
	}

	statusRequest := withURLParam(newRequest(http.MethodPut, "/api/rooms/"+roomID+"/status", map[string]any{"status": "paused"}), "id", roomID)
	testutil.Call(t, testHandler.SetRoomStatus, statusRequest).Want(http.StatusOK)
	pausedRequest := withURLParam(newRequest(http.MethodPost, "/api/rooms/"+roomID+"/messages", map[string]any{
		"body": "This message must remain visible.", "idempotency_key": "handler-message-paused",
	}), "id", roomID)
	pausedResponse := testutil.Call(t, testHandler.PostRoomMessage, pausedRequest).Want(http.StatusConflict)
	if !strings.Contains(pausedResponse.Text(), "room_paused") {
		t.Fatalf("paused Room message: %s", pausedResponse.Text())
	}
	var entryCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM room_entry WHERE room_id = $1`, roomID).Scan(&entryCount); err != nil {
		t.Fatal(err)
	}
	if entryCount != 3 {
		t.Fatalf("paused message entry count = %d, want 3", entryCount)
	}
}
