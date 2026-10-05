package room

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRoomRealtimeEventNames(t *testing.T) {
	events := []string{
		EventRoomCreated,
		EventRoomUpdated,
		EventRoomEntry,
		EventRoomCycle,
		EventRoomTurn,
		EventRoomMemoryRevision,
		EventRoomReview,
		EventRoomRecommendationReview,
		EventRoomArtifact,
	}
	want := []string{
		"room:created",
		"room:updated",
		"room:entry",
		"room:cycle",
		"room:turn",
		"room:memory_revision",
		"room:review",
		"room:recommendation_review",
		"room:artifact",
	}
	if len(events) != len(want) {
		t.Fatalf("Room event count = %d, want %d", len(events), len(want))
	}
	seen := make(map[string]struct{}, len(events))
	for index, event := range events {
		if event != want[index] {
			t.Fatalf("Room event %d = %q, want %q", index, event, want[index])
		}
		if _, exists := seen[event]; exists {
			t.Fatalf("duplicate Room event %q", event)
		}
		seen[event] = struct{}{}
	}
}

func TestRoomRealtimePayloadsExcludeDatabaseJSONAndPrivateTaskState(t *testing.T) {
	roomID := realtimeTestUUID("00000000-0000-0000-0000-000000000001")
	cycleID := realtimeTestUUID("00000000-0000-0000-0000-000000000002")
	entryID := realtimeTestUUID("00000000-0000-0000-0000-000000000003")
	turnID := realtimeTestUUID("00000000-0000-0000-0000-000000000004")
	revisionID := realtimeTestUUID("00000000-0000-0000-0000-000000000005")
	artifactID := realtimeTestUUID("00000000-0000-0000-0000-000000000006")
	targetID := realtimeTestUUID("00000000-0000-0000-0000-000000000007")
	sensitive := []byte(`{"output":"private realtime sentinel"}`)

	roomRow := db.Room{
		ID: roomID, Status: "active", MemoryVersion: 4, ActiveCycleID: cycleID,
		Memory: sensitive, SuccessCriteria: sensitive, StopConditions: sensitive,
	}
	entry := db.RoomEntry{
		ID: entryID, RoomID: roomID, CycleID: cycleID, TurnID: turnID,
		Body: "private realtime sentinel", Mentions: sensitive,
	}
	cycle := db.RoomCycle{
		ID: cycleID, RoomID: roomID, Status: "running", Phase: "synthesizing",
		SynthesisError: sensitive,
	}
	turn := db.RoomTurn{
		ID: turnID, RoomID: roomID, CycleID: cycleID, Status: "completed",
		TurnKind: "synthesis", Attempt: 2, SessionID: pgtype.Text{String: "private realtime sentinel", Valid: true},
		WorkDir: pgtype.Text{String: "/private/realtime/sentinel", Valid: true}, Result: sensitive,
	}
	revision := db.RoomMemoryRevision{
		ID: revisionID, RoomID: roomID, CycleID: cycleID, ReviewStatus: "accepted",
		Version: 3, Synthesis: sensitive,
	}
	recommendationReview := db.RoomRecommendationReview{
		RoomID: roomID, MemoryRevisionID: revisionID, RecommendationKey: "r:1",
		Status: "approved", ArtifactID: artifactID,
	}
	artifact := db.RoomArtifact{
		ID: artifactID, RoomID: roomID, Kind: "wiki", TargetID: targetID,
		MemoryRevisionID: revisionID, RecommendationKey: pgtype.Text{String: "r:1", Valid: true},
		Body: "private realtime sentinel", CitationEntryIds: sensitive,
	}

	cases := map[string]any{
		EventRoomCreated:              roomEventPayload(roomRow),
		EventRoomUpdated:              roomEventPayload(roomRow),
		EventRoomEntry:                roomEntryEventPayload(entry),
		EventRoomCycle:                roomCycleEventPayload(cycle),
		EventRoomTurn:                 roomTurnEventPayload(turn),
		EventRoomMemoryRevision:       roomMemoryRevisionEventPayload(revision),
		EventRoomReview:               roomReviewEventPayload(roomRow, cycle, revision, "accept"),
		EventRoomRecommendationReview: roomRecommendationReviewEventPayload(recommendationReview),
		EventRoomArtifact:             roomArtifactEventPayload(artifact),
	}
	fixtureJSON, err := os.ReadFile("testdata/realtime_events.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]map[string]any
	if err := json.Unmarshal(fixtureJSON, &fixtures); err != nil {
		t.Fatal(err)
	}
	encodedSensitive := base64.StdEncoding.EncodeToString(sensitive)
	for event, payload := range cases {
		event, payload := event, payload
		t.Run(event, func(t *testing.T) {
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			wire := string(encoded)
			for _, forbidden := range []string{
				"private realtime sentinel", encodedSensitive,
				`"room":`, `"entry":`, `"cycle":`, `"turn":`, `"memory_revision":`,
				`"recommendation_review":`, `"artifact":`,
			} {
				if strings.Contains(wire, forbidden) {
					t.Fatalf("%s payload leaked %q: %s", event, forbidden, wire)
				}
			}
			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, fixtures[event]) {
				t.Fatalf("%s payload = %#v, want shared fixture %#v", event, decoded, fixtures[event])
			}
			if decoded["room_id"] != "00000000-0000-0000-0000-000000000001" {
				t.Fatalf("%s room_id = %#v", event, decoded["room_id"])
			}
			for key, value := range decoded {
				switch value.(type) {
				case map[string]any, []any:
					t.Fatalf("%s payload field %q is unbounded JSON: %s", event, key, wire)
				}
			}
		})
	}
}

func realtimeTestUUID(value string) pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.MustParse(value), Valid: true}
}
