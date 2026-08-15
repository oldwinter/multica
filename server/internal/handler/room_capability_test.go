package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestDaemonCanClaimTaskRequiresRoomTasksCapabilityOnlyForRoomTasks(t *testing.T) {
	roomTask := db.AgentTaskQueue{RoomTurnID: pgtype.UUID{Valid: true}}
	ordinaryTask := db.AgentTaskQueue{}
	withoutCapability := httptest.NewRequest("POST", "/tasks/claim", nil)
	withCapability := httptest.NewRequest("POST", "/tasks/claim", nil)
	withCapability.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityRoomTasksV1)

	if daemonCanClaimTask(withoutCapability, roomTask) {
		t.Fatal("daemon without room-tasks-v1 admitted to Room task")
	}
	if !daemonCanClaimTask(withCapability, roomTask) {
		t.Fatal("daemon with room-tasks-v1 rejected from Room task")
	}
	if !daemonCanClaimTask(withoutCapability, ordinaryTask) {
		t.Fatal("ordinary task unexpectedly gated by Room capability")
	}
}
