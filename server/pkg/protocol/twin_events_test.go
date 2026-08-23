package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTwinRealtimePayloadsExposeOnlyInvalidationMetadata(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		keys    []string
	}{
		{name: EventTwinProposalChanged, payload: TwinProposalChangedPayload{ProposalID: "proposal-1", State: "accepted", VersionID: "version-1"}, keys: []string{"proposal_id", "state", "version_id"}},
		{name: EventTwinVersionChanged, payload: TwinVersionChangedPayload{VersionID: "version-1", ProposalID: "proposal-1", VersionNumber: 2}, keys: []string{"version_id", "proposal_id", "version_number"}},
		{name: EventTwinBindingChanged, payload: TwinBindingChangedPayload{BindingID: "binding-1", State: "enabled", TwinVersionID: "version-1"}, keys: []string{"binding_id", "state", "twin_version_id"}},
		{name: EventTwinDepositionChanged, payload: TwinDepositionChangedPayload{DepositionID: "deposition-1", ProposalID: "proposal-1", TaskID: "task-1", BaseTwinVersionID: "version-1", State: "accepted"}, keys: []string{"deposition_id", "proposal_id", "task_id", "base_twin_version_id", "state"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(test.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if len(decoded) != len(test.keys) {
				t.Fatalf("payload keys = %#v, want exactly %#v", decoded, test.keys)
			}
			for _, key := range test.keys {
				if _, ok := decoded[key]; !ok {
					t.Fatalf("payload missing %q: %s", key, raw)
				}
			}
			lower := strings.ToLower(string(raw))
			for _, forbidden := range []string{"content", "assertion", "citation", "evidence", "prompt", "result", "credential", "path"} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("payload leaked forbidden field %q: %s", forbidden, raw)
				}
			}
		})
	}
}
