package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestDaemonCanClaimTaskRequiresRoomTasksCapabilityOnlyForRoomTasks(t *testing.T) {
	contextV1 := protocol.RoomTaskContextV1{
		WorkspaceID: "11111111-2222-4333-8444-555555555555", RoomID: "22222222-3333-4444-8555-666666666666",
		CycleID: "33333333-4444-4555-8666-777777777777", TurnID: "44444444-5555-4666-8777-888888888888",
		Title: "Room", Memory: json.RawMessage(`{}`), Transcript: []protocol.RoomTaskTranscriptEntryV1{},
	}
	v1Payload, err := protocol.EncodeRoomTaskContextV1(contextV1)
	if err != nil {
		t.Fatal(err)
	}
	contextV1.TurnKind = "synthesis"
	v2Payload, err := protocol.EncodeRoomTaskContextV2(contextV1)
	if err != nil {
		t.Fatal(err)
	}
	costLimit := int64(17)
	contextV1.CostLimitTicks = &costLimit
	cappedPayload, err := protocol.EncodeRoomTaskContextV2(contextV1)
	if err != nil {
		t.Fatal(err)
	}
	roomTask := db.AgentTaskQueue{RoomTurnID: pgtype.UUID{Valid: true}, Context: v1Payload}
	v2RoomTask := db.AgentTaskQueue{RoomTurnID: pgtype.UUID{Valid: true}, Context: v2Payload}
	cappedRoomTask := db.AgentTaskQueue{RoomTurnID: pgtype.UUID{Valid: true}, Context: cappedPayload}
	ordinaryTask := db.AgentTaskQueue{}
	withoutCapability := httptest.NewRequest("POST", "/tasks/claim", nil)
	withCapability := httptest.NewRequest("POST", "/tasks/claim", nil)
	withCapability.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityRoomTasksV1)
	withOutcomeCapability := httptest.NewRequest("POST", "/tasks/claim", nil)
	withOutcomeCapability.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityRoomOutcomesV2)
	withCostCapability := httptest.NewRequest("POST", "/tasks/claim", nil)
	withCostCapability.Header.Set(
		"X-Client-Capabilities",
		protocol.DaemonCapabilityRoomOutcomesV2+","+protocol.DaemonCapabilityRoomCostLimitsV1,
	)

	if daemonRoomTaskCompatibility(withoutCapability, roomTask) != roomTaskClaimUnsupported {
		t.Fatal("daemon without room-tasks-v1 admitted to Room task")
	}
	if daemonRoomTaskCompatibility(withCapability, roomTask) != roomTaskClaimCompatible {
		t.Fatal("daemon with room-tasks-v1 rejected from Room task")
	}
	if daemonRoomTaskCompatibility(withCapability, v2RoomTask) != roomTaskClaimUnsupported {
		t.Fatal("v1 daemon admitted to v2 Room outcome task")
	}
	if daemonRoomTaskCompatibility(withOutcomeCapability, v2RoomTask) != roomTaskClaimCompatible {
		t.Fatal("outcome-capable daemon rejected from v2 Room task")
	}
	if daemonRoomTaskCompatibility(withOutcomeCapability, cappedRoomTask) != roomTaskClaimUnsupported {
		t.Fatal("outcome-only daemon admitted to capped Room task")
	}
	if daemonRoomTaskCompatibility(withCostCapability, cappedRoomTask) != roomTaskClaimCompatible {
		t.Fatal("cost-bound outcome daemon rejected from capped Room task")
	}
	if daemonRoomTaskCompatibility(withoutCapability, ordinaryTask) != roomTaskClaimCompatible {
		t.Fatal("ordinary task unexpectedly gated by Room capability")
	}
	invalidRoomTask := db.AgentTaskQueue{
		RoomTurnID: pgtype.UUID{Valid: true},
		Context:    []byte(`{"type":"room","schema_version":999}`),
	}
	if daemonRoomTaskCompatibility(withOutcomeCapability, invalidRoomTask) != roomTaskClaimInvalid {
		t.Fatal("invalid Room context was classified as a capability mismatch")
	}
}
