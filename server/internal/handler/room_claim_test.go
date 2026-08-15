package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestClaimTaskByRuntimeRoomContextAndContinuity(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Room claim runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Room claim agent")

	var roomID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room (
			workspace_id, title, instructions, created_by_user_id, facilitator_agent_id,
			memory, memory_version
		) VALUES (
			$1, 'Architecture council', 'Challenge assumptions.', $2, $3,
			'{"summary":"Prefer explicit contracts","facts":[],"decisions":[],"open_questions":[]}'::jsonb, 2
		)
		RETURNING id
	`, testWorkspaceID, testUserID, agentID).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	var cycleID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room_cycle (workspace_id, room_id, sequence, source, wake_key, status)
		VALUES ($1, $2, 1, 'manual', 'manual:claim-test', 'queued')
		RETURNING id
	`, testWorkspaceID, roomID).Scan(&cycleID); err != nil {
		t.Fatal(err)
	}
	var turnID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room_turn (workspace_id, room_id, cycle_id, agent_id, status)
		VALUES ($1, $2, $3, $4, 'queued')
		RETURNING id
	`, testWorkspaceID, roomID, cycleID, agentID).Scan(&turnID); err != nil {
		t.Fatal(err)
	}
	contextJSON, err := protocol.EncodeRoomTaskContextV1(protocol.RoomTaskContextV1{
		WorkspaceID:  testWorkspaceID,
		RoomID:       roomID,
		CycleID:      cycleID,
		TurnID:       turnID,
		Title:        "Architecture council",
		Instructions: "Challenge assumptions.",
		Memory:       json.RawMessage(`{"summary":"Prefer explicit contracts"}`),
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
	var taskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority, context, room_turn_id,
			session_id, work_dir, originator_user_id, accountable_user_id, originator_source
		) VALUES (
			$1, $2, NULL, 'queued', 0, $3, $4,
			'room-session-prior', '/tmp/room-prior', $5, $5, 'direct_human'
		)
		RETURNING id
	`, agentID, runtimeID, contextJSON, turnID, testUserID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	ordinaryTaskID := seedQueuedIssueTask(t, ctx, agentID, runtimeID, issueID)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
		testPool.Exec(context.Background(), `DELETE FROM room_turn WHERE id = $1`, turnID)
		testPool.Exec(context.Background(), `DELETE FROM room_cycle WHERE id = $1`, cycleID)
		testPool.Exec(context.Background(), `DELETE FROM room WHERE id = $1`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	w := httptest.NewRecorder()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "room-claim")
	req = withURLParam(req, "runtimeId", runtimeID)
	testHandler.ClaimTaskByRuntime(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("claim Room task without capability: status %d: %s", w.Code, w.Body.String())
	}
	var unsupportedResponse struct {
		Task *AgentTaskResponse `json:"task"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &unsupportedResponse); err != nil {
		t.Fatal(err)
	}
	if unsupportedResponse.Task == nil || unsupportedResponse.Task.ID != ordinaryTaskID {
		t.Fatalf("daemon without Room capability got task %+v, want ordinary task %s", unsupportedResponse.Task, ordinaryTaskID)
	}
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "queued" {
		t.Fatalf("Room task status after unsupported claim = %q, want queued", status)
	}
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, ordinaryTaskID); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req = newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, "room-claim-capable")
	req = withURLParam(req, "runtimeId", runtimeID)
	req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityRoomTasksV1)
	testHandler.ClaimTaskByRuntime(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("claim Room task: status %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Task *AgentTaskResponse `json:"task"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Task == nil {
		t.Fatalf("Room task was not claimed: %s", w.Body.String())
	}
	claimed := response.Task
	if claimed.Kind != "room" || claimed.RoomID != roomID || claimed.RoomCycleID != cycleID || claimed.RoomTurnID != turnID {
		t.Fatalf("Room claim identity = kind %q room %q cycle %q turn %q", claimed.Kind, claimed.RoomID, claimed.RoomCycleID, claimed.RoomTurnID)
	}
	if claimed.RoomTitle != "Architecture council" || claimed.RoomInstructions != "Challenge assumptions." {
		t.Fatalf("Room claim prompt fields = title %q instructions %q", claimed.RoomTitle, claimed.RoomInstructions)
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
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Room batch capability runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Room batch capability agent")

	var roomID, cycleID, turnID, roomTaskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room (workspace_id, title, created_by_user_id, facilitator_agent_id)
		VALUES ($1, 'Batch capability room', $2, $3) RETURNING id
	`, testWorkspaceID, testUserID, agentID).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room_cycle (workspace_id, room_id, sequence, source, wake_key, status)
		VALUES ($1, $2, 1, 'manual', 'manual:batch-capability', 'queued') RETURNING id
	`, testWorkspaceID, roomID).Scan(&cycleID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room_turn (workspace_id, room_id, cycle_id, agent_id, status)
		VALUES ($1, $2, $3, $4, 'queued') RETURNING id
	`, testWorkspaceID, roomID, cycleID, agentID).Scan(&turnID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, context, room_turn_id)
		VALUES ($1, $2, 'queued', 0, '{}'::jsonb, $3) RETURNING id
	`, agentID, runtimeID, turnID).Scan(&roomTaskID); err != nil {
		t.Fatal(err)
	}
	ordinaryTaskID := seedQueuedIssueTask(t, ctx, agentID, runtimeID, issueID)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, roomTaskID)
		testPool.Exec(context.Background(), `DELETE FROM room_turn WHERE id = $1`, turnID)
		testPool.Exec(context.Background(), `DELETE FROM room_cycle WHERE id = $1`, cycleID)
		testPool.Exec(context.Background(), `DELETE FROM room WHERE id = $1`, roomID)
	})

	w := postBatchClaim(t, testWorkspaceID, []string{runtimeID}, 1)
	if w.Code != http.StatusOK {
		t.Fatalf("batch claim status %d: %s", w.Code, w.Body.String())
	}
	var response batchClaimResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Tasks) != 1 || response.Tasks[0].ID != ordinaryTaskID {
		t.Fatalf("batch daemon without Room capability got %+v, want ordinary task %s", response.Tasks, ordinaryTaskID)
	}
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, roomTaskID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "queued" {
		t.Fatalf("batch-skipped Room task status = %q, want queued", status)
	}
}

func TestRoomTaskMessagesRequirePrivateAgentAccess(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := createClaimReclaimRuntime(t, ctx, "Room task message privacy runtime")
	agentID, issueID := createClaimReclaimAgentAndIssue(t, ctx, runtimeID, "Room task message privacy agent")
	var roomID, cycleID, turnID, taskID, memberID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room (workspace_id, title, created_by_user_id, facilitator_agent_id)
		VALUES ($1, 'Private transcript room', $2, $3) RETURNING id
	`, testWorkspaceID, testUserID, agentID).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room_cycle (workspace_id, room_id, sequence, source, wake_key, status)
		VALUES ($1, $2, 1, 'manual', 'manual:private-transcript', 'running') RETURNING id
	`, testWorkspaceID, roomID).Scan(&cycleID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO room_turn (workspace_id, room_id, cycle_id, agent_id, status)
		VALUES ($1, $2, $3, $4, 'running') RETURNING id
	`, testWorkspaceID, roomID, cycleID, agentID).Scan(&turnID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, room_turn_id)
		VALUES ($1, $2, 'running', 0, $3) RETURNING id
	`, agentID, runtimeID, turnID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO task_message (task_id, seq, type, content)
		VALUES ($1, 1, 'text', 'private tool transcript')
	`, taskID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Room Plain Member', 'room-plain-' || gen_random_uuid()::text || '@example.com') RETURNING id
	`).Scan(&memberID); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')
	`, testWorkspaceID, memberID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, testWorkspaceID, memberID)
		testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, memberID)
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
		testPool.Exec(context.Background(), `DELETE FROM room_turn WHERE id = $1`, turnID)
		testPool.Exec(context.Background(), `DELETE FROM room_cycle WHERE id = $1`, cycleID)
		testPool.Exec(context.Background(), `DELETE FROM room WHERE id = $1`, roomID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/"+taskID+"/messages", nil)
	req.Header.Set("X-User-ID", memberID)
	req = withURLParam(req, "taskId", taskID)
	member, err := testHandler.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID: parseUUID(memberID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(middleware.SetMemberContext(req.Context(), testWorkspaceID, member))
	testHandler.ListTaskMessagesByUser(w, req)
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "private tool transcript") {
		t.Fatalf("private Room task messages status=%d body=%s", w.Code, w.Body.String())
	}
}
