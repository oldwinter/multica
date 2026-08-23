package protocol

const (
	EventTwinProposalChanged   = "twin:proposal_changed"
	EventTwinVersionChanged    = "twin:version_changed"
	EventTwinBindingChanged    = "twin:binding_changed"
	EventTwinDepositionChanged = "twin:deposition_changed"
)

// Twin realtime payloads are invalidation signals, not data replication.
// Keep them structurally unable to carry assertions, citations, evidence,
// generated content, prompts, task results, or local custody metadata.
type TwinProposalChangedPayload struct {
	ProposalID string `json:"proposal_id"`
	State      string `json:"state"`
	VersionID  string `json:"version_id,omitempty"`
}

type TwinVersionChangedPayload struct {
	VersionID     string `json:"version_id"`
	ProposalID    string `json:"proposal_id"`
	VersionNumber int64  `json:"version_number"`
}

type TwinBindingChangedPayload struct {
	BindingID     string `json:"binding_id"`
	State         string `json:"state"`
	TwinVersionID string `json:"twin_version_id,omitempty"`
}

type TwinDepositionChangedPayload struct {
	DepositionID      string `json:"deposition_id"`
	ProposalID        string `json:"proposal_id"`
	TaskID            string `json:"task_id"`
	BaseTwinVersionID string `json:"base_twin_version_id"`
	State             string `json:"state"`
}
