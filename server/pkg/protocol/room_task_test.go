package protocol

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func validRoomTaskContextV1() RoomTaskContextV1 {
	return RoomTaskContextV1{
		WorkspaceID:  "11111111-2222-4333-8444-555555555555",
		RoomID:       "22222222-3333-4444-8555-666666666666",
		CycleID:      "33333333-4444-4555-8666-777777777777",
		TurnID:       "44444444-5555-4666-8777-888888888888",
		Title:        "Architecture council",
		Instructions: "Challenge assumptions.",
		Memory:       json.RawMessage(`{"summary":"Prefer explicit contracts"}`),
		Transcript: []RoomTaskTranscriptEntryV1{{
			ID:         "55555555-6666-4777-8888-999999999999",
			Ordinal:    1,
			EntryType:  "message",
			AuthorType: "member",
			AuthorID:   "66666666-7777-4888-8999-aaaaaaaaaaaa",
			Body:       "Review the retry boundary.",
			CreatedAt:  time.Date(2026, time.August, 13, 6, 0, 0, 0, time.UTC).Format(time.RFC3339),
		}},
	}
}

func TestRoomTaskContextV1EncodeThenParseRoundTrips(t *testing.T) {
	want := validRoomTaskContextV1()

	payload, err := EncodeRoomTaskContextV1(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseRoomTaskContextV1(payload)
	if err != nil {
		t.Fatal(err)
	}

	if got.Type != RoomTaskContextType || got.SchemaVersion != RoomTaskContextSchemaV1 {
		t.Fatalf("Room context discriminator = %q/%d", got.Type, got.SchemaVersion)
	}
	if got.TurnID != want.TurnID || len(got.Transcript) != 1 || got.Transcript[0].Body != want.Transcript[0].Body {
		t.Fatalf("Room context round trip = %+v", got)
	}
}

func TestRoomTaskContextV1RejectsUnsupportedVersionAndMalformedIdentity(t *testing.T) {
	for name, payload := range map[string][]byte{
		"legacy unversioned": []byte(`{"type":"room"}`),
		"future version":     []byte(`{"type":"room","schema_version":2}`),
		"malformed identity": []byte(`{"type":"room","schema_version":1,"workspace_id":"not-a-uuid"}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRoomTaskContextV1(payload)
			if !errors.Is(err, ErrInvalidRoomTaskContext) {
				t.Fatalf("ParseRoomTaskContextV1() error = %v, want ErrInvalidRoomTaskContext", err)
			}
		})
	}
}
