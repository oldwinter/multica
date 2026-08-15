package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestRoomsThroughRouterRequireMembershipAndKeepRuntimePrivate(t *testing.T) {
	ctx := context.Background()
	var facilitatorID string
	if err := testPool.QueryRow(ctx, `
		SELECT id::text FROM agent WHERE workspace_id = $1 ORDER BY created_at LIMIT 1
	`, testWorkspaceID).Scan(&facilitatorID); err != nil {
		t.Fatalf("load Room facilitator: %v", err)
	}

	unauthenticated, err := http.Get(testServer.URL + "/api/rooms")
	if err != nil {
		t.Fatal(err)
	}
	unauthenticated.Body.Close()
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated Room list = %d, want 401", unauthenticated.StatusCode)
	}

	createdResponse := authRequest(t, http.MethodPost, "/api/rooms", map[string]any{
		"title": "Router Room", "facilitator_agent_id": facilitatorID,
		"daily_turn_limit": 4, "schedule_interval_minutes": 15,
	})
	if createdResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createdResponse.Body)
		createdResponse.Body.Close()
		t.Fatalf("create Room through router = %d: %s", createdResponse.StatusCode, body)
	}
	var created struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
		Participants []json.RawMessage `json:"participants"`
	}
	readJSON(t, createdResponse, &created)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE room_turn_id IN (SELECT id FROM room_turn WHERE room_id = $1)`, created.Room.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM room_artifact WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM room_turn WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM room_cycle WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM room_entry WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM room_participant WHERE room_id = $1`, created.Room.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM room WHERE id = $1`, created.Room.ID)
	})
	if created.Room.ID == "" || len(created.Participants) != 2 {
		t.Fatalf("created Room = %+v", created)
	}

	listResponse := authRequest(t, http.MethodGet, "/api/rooms", nil)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("list Rooms through router = %d", listResponse.StatusCode)
	}
	var rooms []struct {
		ID string `json:"id"`
	}
	readJSON(t, listResponse, &rooms)
	if len(rooms) != 1 || rooms[0].ID != created.Room.ID {
		t.Fatalf("Room list = %+v", rooms)
	}

	detailResponse := authRequest(t, http.MethodGet, "/api/rooms/"+created.Room.ID, nil)
	if detailResponse.StatusCode != http.StatusOK {
		t.Fatalf("get Room through router = %d", detailResponse.StatusCode)
	}
	body, err := io.ReadAll(detailResponse.Body)
	detailResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, privateField := range []string{"session_id", "work_dir", "last_entry_ordinal", "last_cycle_sequence"} {
		if json.Valid(body) && containsJSONKey(body, privateField) {
			t.Fatalf("Room response exposed private field %q: %s", privateField, body)
		}
	}
}

func containsJSONKey(body []byte, key string) bool {
	var value any
	if json.Unmarshal(body, &value) != nil {
		return false
	}
	return containsRoomJSONKey(value, key)
}

func containsRoomJSONKey(value any, key string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			if childKey == key || containsRoomJSONKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsRoomJSONKey(child, key) {
				return true
			}
		}
	}
	return false
}
