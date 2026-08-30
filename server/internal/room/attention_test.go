package room

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func attentionUUID(value string) pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan(value)
	return id
}

func TestBuildRoomAttentionProjectionContracts(t *testing.T) {
	roomID := attentionUUID("11111111-1111-4111-8111-111111111111")
	cycleID := attentionUUID("22222222-2222-4222-8222-222222222222")
	revisionID := attentionUUID("33333333-3333-4333-8333-333333333333")

	tests := []struct {
		name            string
		kind            string
		input           roomAttentionInput
		wantSeverity    string
		wantTitle       string
		wantFocus       string
		wantIdentity    string
		wantKeyFragment string
	}{
		{
			name: "outcome review", kind: RoomInboxOutcomeReviewRequired,
			input:        roomAttentionInput{RoomID: roomID, CycleID: cycleID, MemoryRevisionID: revisionID},
			wantSeverity: "action_required", wantTitle: "Room outcome needs review",
			wantFocus: "outcome_review", wantIdentity: revisionID.String(), wantKeyFragment: ":outcome:",
		},
		{
			name: "recommendation review", kind: RoomInboxRecommendationReviewRequired,
			input:        roomAttentionInput{RoomID: roomID, CycleID: cycleID, MemoryRevisionID: revisionID, RecommendationKey: "create_artifact"},
			wantSeverity: "action_required", wantTitle: "Room recommendation needs review",
			wantFocus: "recommendation_review", wantIdentity: "create_artifact", wantKeyFragment: ":recommendation:",
		},
		{
			name: "failed cycle", kind: RoomInboxCycleFailed,
			input:        roomAttentionInput{RoomID: roomID, CycleID: cycleID, Phase: "synthesis", ReasonCode: "malformed_synthesis"},
			wantSeverity: "attention", wantTitle: "Room cycle needs attention",
			wantFocus: "cycle_failed", wantKeyFragment: ":failed",
		},
		{
			name: "blocked run", kind: RoomInboxCycleBlocked,
			input:        roomAttentionInput{RoomID: roomID, ReasonCode: "budget_exhausted"},
			wantSeverity: "attention", wantTitle: "Room run is blocked",
			wantFocus: "cycle_blocked", wantKeyFragment: ":blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildRoomAttentionProjection(tt.kind, tt.input)
			if err != nil {
				t.Fatalf("buildRoomAttentionProjection: %v", err)
			}
			if got.Type != tt.kind || got.Severity != tt.wantSeverity || got.Title != tt.wantTitle {
				t.Fatalf("projection = %#v", got)
			}
			if got.ReviewIdentity != tt.wantIdentity || !strings.Contains(got.AttentionKey, tt.wantKeyFragment) {
				t.Fatalf("identity = %q key = %q", got.ReviewIdentity, got.AttentionKey)
			}
			var details map[string]string
			if err := json.Unmarshal(got.Details, &details); err != nil {
				t.Fatalf("details: %v", err)
			}
			if details["room_id"] != roomID.String() || details["focus"] != tt.wantFocus {
				t.Fatalf("details = %#v", details)
			}
			if !strings.HasPrefix(details["route"], "/rooms?room="+roomID.String()) {
				t.Fatalf("stable Room route missing: %#v", details)
			}
			for _, forbidden := range []string{"objective", "transcript", "summary", "artifact", "content"} {
				if _, exists := details[forbidden]; exists {
					t.Fatalf("privacy-sensitive %q leaked in details %#v", forbidden, details)
				}
			}
		})
	}
}

func TestBuildRoomAttentionProjectionRejectsMissingOrUnboundedIdentities(t *testing.T) {
	roomID := attentionUUID("11111111-1111-4111-8111-111111111111")
	cycleID := attentionUUID("22222222-2222-4222-8222-222222222222")

	tests := []struct {
		kind  string
		input roomAttentionInput
	}{
		{RoomInboxOutcomeReviewRequired, roomAttentionInput{RoomID: roomID, CycleID: cycleID}},
		{RoomInboxRecommendationReviewRequired, roomAttentionInput{RoomID: roomID, CycleID: cycleID, MemoryRevisionID: cycleID, RecommendationKey: "raw recommendation prose must not become an identity"}},
		{RoomInboxCycleFailed, roomAttentionInput{RoomID: roomID}},
		{RoomInboxCycleBlocked, roomAttentionInput{}},
		{"room_unknown", roomAttentionInput{RoomID: roomID}},
	}

	for _, tt := range tests {
		if _, err := buildRoomAttentionProjection(tt.kind, tt.input); err == nil {
			t.Fatalf("buildRoomAttentionProjection(%q, %#v) succeeded", tt.kind, tt.input)
		}
	}
}

func TestBuildRoomAttentionProjectionDropsUnsafeMetadata(t *testing.T) {
	got, err := buildRoomAttentionProjection(RoomInboxCycleFailed, roomAttentionInput{
		RoomID:     attentionUUID("11111111-1111-4111-8111-111111111111"),
		CycleID:    attentionUUID("22222222-2222-4222-8222-222222222222"),
		Phase:      "synthesis with secret text",
		ReasonCode: strings.Repeat("x", 65),
	})
	if err != nil {
		t.Fatalf("buildRoomAttentionProjection: %v", err)
	}
	var details map[string]string
	if err := json.Unmarshal(got.Details, &details); err != nil {
		t.Fatalf("details: %v", err)
	}
	if _, exists := details["phase"]; exists {
		t.Fatalf("unsafe phase retained: %#v", details)
	}
	if _, exists := details["reason_code"]; exists {
		t.Fatalf("unsafe reason retained: %#v", details)
	}
}

func TestPublishRoomAttentionUsesRecipientScopedPrivacySafePayload(t *testing.T) {
	sink := &recordingEvents{}
	service := &Service{events: sink}
	item := db.InboxItem{
		ID:                 attentionUUID("44444444-4444-4444-8444-444444444444"),
		WorkspaceID:        attentionUUID("55555555-5555-4555-8555-555555555555"),
		RecipientType:      "member",
		RecipientID:        attentionUUID("66666666-6666-4666-8666-666666666666"),
		Type:               RoomInboxOutcomeReviewRequired,
		Severity:           "action_required",
		Title:              "Room outcome needs review",
		RoomID:             attentionUUID("11111111-1111-4111-8111-111111111111"),
		RoomCycleID:        attentionUUID("22222222-2222-4222-8222-222222222222"),
		RoomReviewIdentity: pgtype.Text{String: "33333333-3333-4333-8333-333333333333", Valid: true},
		RoomAttentionKey:   pgtype.Text{String: "internal-dedupe-key", Valid: true},
		Details:            []byte(`{"focus":"outcome_review","room_id":"11111111-1111-4111-8111-111111111111"}`),
	}

	service.publishRoomAttentionItems([]db.InboxItem{item})
	eventList := sink.snapshot()
	if len(eventList) != 1 || eventList[0].Type != protocol.EventInboxNew {
		t.Fatalf("events = %#v", eventList)
	}
	payload := eventList[0].Payload.(map[string]any)
	eventItem := payload["item"].(map[string]any)
	roomID, ok := eventItem["room_id"].(*string)
	if eventItem["recipient_id"] != item.RecipientID.String() || !ok || roomID == nil || *roomID != item.RoomID.String() {
		t.Fatalf("event item = %#v", eventItem)
	}
	if _, leaked := eventItem["room_attention_key"]; leaked {
		t.Fatalf("internal dedupe key leaked: %#v", eventItem)
	}
	if eventItem["body"] != (*string)(nil) {
		t.Fatalf("Room attention body must remain empty: %#v", eventItem["body"])
	}
}

func TestRoomRefusalNeedsAttentionSkipsDeliberateOrAlreadyRunningStates(t *testing.T) {
	for _, reason := range []string{"", "room_paused", "room_archived", "cycle_active"} {
		if roomRefusalNeedsAttention(reason) {
			t.Fatalf("roomRefusalNeedsAttention(%q) = true", reason)
		}
	}
	for _, reason := range []string{"budget_exhausted", "no_targets", "facilitator_unavailable"} {
		if !roomRefusalNeedsAttention(reason) {
			t.Fatalf("roomRefusalNeedsAttention(%q) = false", reason)
		}
	}
}
