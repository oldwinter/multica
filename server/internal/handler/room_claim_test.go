package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestClaimTaskByRuntimeRoomContextAndContinuity(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, "Room claim runtime", testutil.Cols{
		"device_info": "claim reclaim fixture",
	})
	agentID := dbfx.Agent(t, "Room claim agent", runtimeID)
	issueID := dbfx.Issue(t, "Room claim agent issue", testutil.Cols{"status": "in_progress"})
	roomID := dbfx.Insert(t, "room", testutil.Cols{
		"workspace_id":         testWorkspaceID,
		"title":                "Architecture council",
		"instructions":         "Challenge assumptions.",
		"created_by_user_id":   testUserID,
		"facilitator_agent_id": agentID,
		"memory":               testutil.Raw(`'{"summary":"Prefer explicit contracts","facts":[],"decisions":[],"open_questions":[]}'::jsonb`),
		"memory_version":       2,
	})
	cycleID := dbfx.Insert(t, "room_cycle", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"sequence":     1,
		"source":       "manual",
		"wake_key":     "manual:claim-test",
		"status":       "queued",
	})
	turnID := dbfx.Insert(t, "room_turn", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"cycle_id":     cycleID,
		"agent_id":     agentID,
		"status":       "queued",
	})
	contextJSON, err := protocol.EncodeRoomTaskContextV1(protocol.RoomTaskContextV1{
		WorkspaceID:    testWorkspaceID,
		RoomID:         roomID,
		CycleID:        cycleID,
		TurnID:         turnID,
		Title:          "Architecture council",
		Instructions:   "Challenge assumptions.",
		Memory:         json.RawMessage(`{"summary":"Prefer explicit contracts"}`),
		CostLimitTicks: int64Pointer(17),
		Transcript: []protocol.RoomTaskTranscriptEntryV1{{
			ID:         "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
			Ordinal:    1,
			EntryType:  "message",
			AuthorType: "member",
			AuthorID:   testUserID,
			Body:       "Review the retry boundary.",
			CreatedAt:  time.Date(2026, time.August, 13, 6, 0, 0, 0, time.UTC).Format(time.RFC3339),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id":          runtimeID,
		"context":             contextJSON,
		"room_turn_id":        turnID,
		"session_id":          "room-session-prior",
		"work_dir":            "/tmp/room-prior",
		"originator_user_id":  testUserID,
		"accountable_user_id": testUserID,
		"originator_source":   "direct_human",
	})
	ordinaryTaskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID,
		"issue_id":   issueID,
	})

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "room-claim")
	req = withURLParam(req, "runtimeId", runtimeID)
	var unsupportedResponse struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).
		Want(http.StatusOK).
		JSON(&unsupportedResponse)
	if unsupportedResponse.Task == nil || unsupportedResponse.Task.ID != ordinaryTaskID {
		t.Fatalf("daemon without Room capability got task %+v, want ordinary task %s", unsupportedResponse.Task, ordinaryTaskID)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status)
	if status != "queued" {
		t.Fatalf("Room task status after unsupported claim = %q, want queued", status)
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, ordinaryTaskID)

	req = newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "room-claim-capable")
	req = withURLParam(req, "runtimeId", runtimeID)
	req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityRoomTasksV1+","+protocol.DaemonCapabilityRoomCostLimitsV1)
	var response struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).
		Want(http.StatusOK).
		JSON(&response)
	if response.Task == nil {
		t.Fatal("Room task was not claimed")
	}
	claimed := response.Task
	if claimed.Kind != "room" || claimed.RoomID != roomID || claimed.RoomCycleID != cycleID || claimed.RoomTurnID != turnID {
		t.Fatalf("Room claim identity = kind %q room %q cycle %q turn %q", claimed.Kind, claimed.RoomID, claimed.RoomCycleID, claimed.RoomTurnID)
	}
	if claimed.RoomTitle != "Architecture council" || claimed.RoomInstructions != "Challenge assumptions." {
		t.Fatalf("Room claim prompt fields = title %q instructions %q", claimed.RoomTitle, claimed.RoomInstructions)
	}
	if claimed.RoomCostLimitTicks == nil || *claimed.RoomCostLimitTicks != 17 {
		t.Fatalf("Room claim cost limit = %v", claimed.RoomCostLimitTicks)
	}
	if !strings.Contains(string(claimed.RoomTranscript), "Review the retry boundary.") {
		t.Fatalf("Room transcript = %s", claimed.RoomTranscript)
	}
	if claimed.PriorSessionID != "room-session-prior" || claimed.PriorWorkDir != "/tmp/room-prior" {
		t.Fatalf("Room continuity = session %q workdir %q", claimed.PriorSessionID, claimed.PriorWorkDir)
	}
	if claimed.QuickCreatePrompt != "" {
		t.Fatalf("Room task was misclassified as quick create: %q", claimed.QuickCreatePrompt)
	}
}

func TestClaimTasksByRuntimeSkipsRoomTaskWithoutCapability(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, "Room batch capability runtime", testutil.Cols{
		"device_info": "claim reclaim fixture",
	})
	agentID := dbfx.Agent(t, "Room batch capability agent", runtimeID)
	issueID := dbfx.Issue(t, "Room batch capability agent issue", testutil.Cols{"status": "in_progress"})
	roomID := dbfx.Insert(t, "room", testutil.Cols{
		"workspace_id":         testWorkspaceID,
		"title":                "Batch capability room",
		"created_by_user_id":   testUserID,
		"facilitator_agent_id": agentID,
	})
	cycleID := dbfx.Insert(t, "room_cycle", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"sequence":     1,
		"source":       "manual",
		"wake_key":     "manual:batch-capability",
		"status":       "queued",
	})
	turnID := dbfx.Insert(t, "room_turn", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"cycle_id":     cycleID,
		"agent_id":     agentID,
		"status":       "queued",
	})
	roomTaskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id":   runtimeID,
		"context":      testutil.Raw("'{}'::jsonb"),
		"room_turn_id": turnID,
	})
	ordinaryTaskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID,
		"issue_id":   issueID,
	})

	var response batchClaimResponse
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/claim",
		map[string]any{"daemon_id": batchClaimTestDaemonID, "runtime_ids": []string{runtimeID}, "max_tasks": 1},
		testWorkspaceID, batchClaimTestDaemonID)
	testutil.Call(t, testHandler.ClaimTasksByRuntime, req).
		Want(http.StatusOK).
		JSON(&response)
	if len(response.Tasks) != 1 || response.Tasks[0].ID != ordinaryTaskID {
		t.Fatalf("batch daemon without Room capability got %+v, want ordinary task %s", response.Tasks, ordinaryTaskID)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id = $1`, roomTaskID).Scan(&status)
	if status != "queued" {
		t.Fatalf("batch-skipped Room task status = %q, want queued", status)
	}
}

func seedQueuedRoomTaskForSchema(t *testing.T, _ context.Context, runtimeID, agentID, label string, schemaVersion, priority int) string {
	t.Helper()
	roomID := dbfx.Insert(t, "room", testutil.Cols{
		"workspace_id":         testWorkspaceID,
		"title":                label,
		"created_by_user_id":   testUserID,
		"facilitator_agent_id": agentID,
		"capability_version":   schemaVersion,
	})
	cycleID := dbfx.Insert(t, "room_cycle", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"sequence":     1,
		"source":       "manual",
		"wake_key":     "manual:" + label,
		"status":       "running",
		"phase":        "gathering",
	})
	turnID := dbfx.Insert(t, "room_turn", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"cycle_id":     cycleID,
		"agent_id":     agentID,
		"status":       "queued",
		"turn_kind":    "participant",
		"attempt":      1,
	})
	roomContext := protocol.RoomTaskContextV1{
		WorkspaceID: testWorkspaceID,
		RoomID:      roomID,
		CycleID:     cycleID,
		TurnID:      turnID,
		Title:       label,
		Memory:      json.RawMessage(`{}`),
		Transcript:  []protocol.RoomTaskTranscriptEntryV1{},
		TurnKind:    "participant",
	}
	var contextJSON []byte
	var err error
	if schemaVersion >= protocol.RoomTaskContextSchemaV2 {
		contextJSON, err = protocol.EncodeRoomTaskContextV2(roomContext)
	} else {
		roomContext.TurnKind = ""
		contextJSON, err = protocol.EncodeRoomTaskContextV1(roomContext)
	}
	if err != nil {
		t.Fatal(err)
	}
	return dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id":          runtimeID,
		"status":              "queued",
		"priority":            priority,
		"context":             contextJSON,
		"room_turn_id":        turnID,
		"originator_user_id":  testUserID,
		"accountable_user_id": testUserID,
		"originator_source":   "direct_human",
	})
}

func assertRoomTaskDeferred(t *testing.T, ctx context.Context, taskID string) {
	t.Helper()
	var status string
	var fireAt time.Time
	if err := testPool.QueryRow(ctx, `
		SELECT status, fire_at FROM agent_task_queue WHERE id = $1
	`, taskID).Scan(&status, &fireAt); err != nil {
		t.Fatal(err)
	}
	if status != "deferred" {
		t.Fatalf("unsupported v2 Room task status = %q, want deferred", status)
	}
	if !fireAt.After(time.Now()) {
		t.Fatalf("unsupported v2 Room task fire_at = %s, want a future retry", fireAt)
	}
}

func TestClaimTaskByRuntimeV1DaemonMakesProgressPastV2RoomTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Room v2 singular gate runtime")
	agentID, _ := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Room v2 singular gate agent")
	roomTaskID := seedQueuedRoomTaskForSchema(t, ctx, runtimeID, agentID, "V2 singular gate", 2, 100)
	v1TaskID := seedQueuedRoomTaskForSchema(t, ctx, runtimeID, agentID, "V1 singular fallback", 1, 50)

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "room-v2-singular-gate")
	req = withURLParam(req, "runtimeId", runtimeID)
	req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityRoomTasksV1)
	var response struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, req).
		Want(http.StatusOK).
		JSON(&response)
	if response.Task == nil || response.Task.ID != v1TaskID {
		t.Fatalf("v1 daemon claimed %+v, want later compatible v1 task %s", response.Task, v1TaskID)
	}
	assertRoomTaskDeferred(t, ctx, roomTaskID)
}

func TestClaimTasksByRuntimeV1DaemonMakesProgressPastV2RoomTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Room v2 batch gate runtime")
	agentID, _ := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Room v2 batch gate agent")
	roomTaskID := seedQueuedRoomTaskForSchema(t, ctx, runtimeID, agentID, "V2 batch gate", 2, 100)
	v1TaskID := seedQueuedRoomTaskForSchema(t, ctx, runtimeID, agentID, "V1 batch fallback", 1, 50)

	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/claim",
		map[string]any{"daemon_id": batchClaimTestDaemonID, "runtime_ids": []string{runtimeID}, "max_tasks": 1},
		testWorkspaceID, batchClaimTestDaemonID)
	req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityRoomTasksV1)
	var response batchClaimResponse
	testutil.Call(t, testHandler.ClaimTasksByRuntime, req).
		Want(http.StatusOK).
		JSON(&response)
	if len(response.Tasks) != 1 || response.Tasks[0].ID != v1TaskID {
		t.Fatalf("v1 batch daemon claimed %+v, want later compatible v1 task %s", response.Tasks, v1TaskID)
	}
	assertRoomTaskDeferred(t, ctx, roomTaskID)
}

func TestClaimTaskByRuntimeUpgradedDaemonImmediatelyRestoresV2RoomTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Room v2 upgrade runtime")
	agentID, _ := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Room v2 upgrade agent")
	roomTaskID := seedQueuedRoomTaskForSchema(t, ctx, runtimeID, agentID, "V2 upgrade gate", 2, 100)

	claim := func(capabilities string) *AgentTaskResponse {
		t.Helper()
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "room-v2-upgrade")
		req = withURLParam(req, "runtimeId", runtimeID)
		req.Header.Set("X-Client-Capabilities", capabilities)
		var response struct {
			Task *AgentTaskResponse `json:"task"`
		}
		testutil.Call(t, testHandler.ClaimTaskByRuntime, req).
			Want(http.StatusOK).
			JSON(&response)
		return response.Task
	}

	if task := claim(protocol.DaemonCapabilityRoomTasksV1); task != nil {
		t.Fatalf("v1 daemon claimed unsupported v2 task %+v", task)
	}
	assertRoomTaskDeferred(t, ctx, roomTaskID)
	capabilities := protocol.DaemonCapabilityRoomTasksV1 + "," + protocol.DaemonCapabilityRoomOutcomesV2
	if task := claim(capabilities); task == nil || task.ID != roomTaskID {
		t.Fatalf("upgraded daemon claimed %+v, want restored v2 task %s", task, roomTaskID)
	}
}

func TestRoomTaskMessagesRequirePrivateAgentAccess(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, "Room task message privacy runtime", testutil.Cols{
		"device_info": "claim reclaim fixture",
	})
	agentID := dbfx.Agent(t, "Room task message privacy agent", runtimeID)
	roomID := dbfx.Insert(t, "room", testutil.Cols{
		"workspace_id":         testWorkspaceID,
		"title":                "Private transcript room",
		"created_by_user_id":   testUserID,
		"facilitator_agent_id": agentID,
	})
	cycleID := dbfx.Insert(t, "room_cycle", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"sequence":     1,
		"source":       "manual",
		"wake_key":     "manual:private-transcript",
		"status":       "running",
	})
	turnID := dbfx.Insert(t, "room_turn", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"room_id":      roomID,
		"cycle_id":     cycleID,
		"agent_id":     agentID,
		"status":       "running",
	})
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id":   runtimeID,
		"status":       "running",
		"room_turn_id": turnID,
	})
	dbfx.Insert(t, "task_message", testutil.Cols{
		"task_id": taskID,
		"seq":     1,
		"type":    "text",
		"content": "private tool transcript",
	})
	memberID := dbfx.User(t, "Room Plain Member", "room-plain-"+taskID+"@example.com")
	dbfx.Member(t, testWorkspaceID, memberID, "member")

	req := newRequest(http.MethodGet, "/api/tasks/"+taskID+"/messages", nil)
	req.Header.Set("X-User-ID", memberID)
	req = withURLParam(req, "taskId", taskID)
	member, err := testHandler.Queries.GetMemberByUserAndWorkspace(req.Context(), db.GetMemberByUserAndWorkspaceParams{
		UserID: parseUUID(memberID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(middleware.SetMemberContext(req.Context(), testWorkspaceID, member))
	var response struct {
		Error string `json:"error"`
	}
	w := testutil.Call(t, testHandler.ListTaskMessagesByUser, req).
		Want(http.StatusNotFound).
		JSON(&response)
	if response.Error != "task not found" || strings.Contains(w.Text(), "private tool transcript") {
		t.Fatalf("private Room task messages response=%+v body=%s", response, w.Text())
	}
}
