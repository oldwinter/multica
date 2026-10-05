package protocol

import (
	"encoding/json"
	"testing"
)

func TestWikiEventPayloadIsContentFree(t *testing.T) {
	payload := WikiEventPayload{
		PageID:         "page-1",
		Scope:          "user",
		RevisionID:     "revision-1",
		RevisionNumber: 2,
		ProposalID:     "proposal-1",
		ReviewStatus:   "accepted",
		RecipientID:    "user-1",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal WikiEventPayload: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal WikiEventPayload: %v", err)
	}
	for _, forbidden := range []string{"recipient_id", "content", "body", "path", "digest"} {
		if _, ok := got[forbidden]; ok {
			t.Errorf("WikiEventPayload exposed forbidden key %q", forbidden)
		}
	}
	if got["page_id"] != payload.PageID || got["scope"] != payload.Scope {
		t.Fatalf("WikiEventPayload identity = %+v, want page_id and scope", got)
	}
}

func TestLMWikiEventPayloadIsContentFree(t *testing.T) {
	payload := LMWikiEventPayload{
		PolicyVersion:  3,
		RevisionID:     "revision-1",
		RevisionNumber: 4,
		ReviewDecision: "accepted",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal LMWikiEventPayload: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal LMWikiEventPayload: %v", err)
	}
	for _, forbidden := range []string{"content", "body", "path", "digest", "citations"} {
		if _, ok := got[forbidden]; ok {
			t.Errorf("LMWikiEventPayload exposed forbidden key %q", forbidden)
		}
	}
}
