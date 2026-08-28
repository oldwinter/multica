package skillevolution

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type overviewDTO struct {
	Skill       skillIdentityDTO `json:"skill"`
	Loop        *loopDTO         `json:"loop"`
	Revisions   []revisionDTO    `json:"revisions"`
	Proposals   []proposalDTO    `json:"proposals"`
	Releases    []releaseDTO     `json:"releases"`
	Permissions permissionsDTO   `json:"permissions"`
}

type skillIdentityDTO struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	BundleHash      string `json:"bundle_hash"`
	Ownership       string `json:"ownership"`
	OwnershipReason string `json:"ownership_reason"`
	ForkRequired    bool   `json:"fork_required"`
}

type permissionsDTO struct {
	CanConfigure bool `json:"can_configure"`
	CanPublish   bool `json:"can_publish"`
	CanFork      bool `json:"can_fork"`
}

type loopDTO struct {
	ID               string     `json:"id"`
	Enabled          bool       `json:"enabled"`
	Mode             string     `json:"mode"`
	CooldownSeconds  int32      `json:"cooldown_seconds"`
	MinimumSignals   int32      `json:"minimum_signals"`
	MaxEvidenceRefs  int32      `json:"max_evidence_refs"`
	MaxReplaySamples int32      `json:"max_replay_samples"`
	MaxCostUSDTicks  int64      `json:"max_cost_usd_ticks"`
	PolicyVersion    string     `json:"policy_version"`
	LastObservedAt   *time.Time `json:"last_observed_at,omitempty"`
	LastProposalAt   *time.Time `json:"last_proposal_at,omitempty"`
	NextEligibleAt   *time.Time `json:"next_eligible_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type revisionDTO struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	BundleHash       string    `json:"bundle_hash"`
	ByteCount        int64     `json:"byte_count"`
	SupportFileCount int32     `json:"support_file_count"`
	CreatedAt        time.Time `json:"created_at"`
}

type proposalDTO struct {
	ID             string    `json:"id"`
	SkillID        string    `json:"skill_id"`
	State          string    `json:"state"`
	BaseRevisionID string    `json:"base_revision_id"`
	BaseHash       string    `json:"base_hash"`
	CandidateID    *string   `json:"candidate_revision_id,omitempty"`
	CandidateHash  *string   `json:"candidate_hash,omitempty"`
	FailureReason  *string   `json:"failure_reason,omitempty"`
	StaleReason    *string   `json:"stale_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type releaseDTO struct {
	ID               string     `json:"id"`
	SkillID          string     `json:"skill_id"`
	ProposalID       *string    `json:"proposal_id,omitempty"`
	SourceReleaseID  *string    `json:"source_release_id,omitempty"`
	RevisionID       string     `json:"revision_id"`
	Kind             string     `json:"kind"`
	ExpectedBaseHash string     `json:"expected_base_hash"`
	PreHash          *string    `json:"pre_hash,omitempty"`
	PostHash         *string    `json:"post_hash,omitempty"`
	Outcome          string     `json:"outcome"`
	ErrorCode        *string    `json:"error_code,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type proposalDetailDTO struct {
	Proposal    proposalDTO           `json:"proposal"`
	Rationale   *ImprovementRationale `json:"rationale"`
	Diff        BundleDiff            `json:"diff"`
	Evidence    []evidenceDTO         `json:"evidence"`
	Evaluations []evaluationDTO       `json:"evaluations"`
	Reviews     []reviewDTO           `json:"reviews"`
}

type evidenceDTO struct {
	Kind             string    `json:"kind"`
	SourceID         string    `json:"source_id"`
	SourceRevisionID string    `json:"source_revision_id,omitempty"`
	SourceState      string    `json:"source_state"`
	Digest           string    `json:"digest"`
	ObservedAt       time.Time `json:"observed_at"`
}

type evaluationDTO struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Result         string          `json:"result"`
	Adapter        string          `json:"adapter"`
	AdapterVersion string          `json:"adapter_version"`
	PolicyVersion  string          `json:"policy_version"`
	ResultDigest   string          `json:"result_digest"`
	SafeMetrics    json.RawMessage `json:"safe_metrics"`
	CostUSDTicks   *int64          `json:"cost_usd_ticks,omitempty"`
	DurationMS     int64           `json:"duration_ms"`
	CreatedAt      time.Time       `json:"created_at"`
}

type reviewDTO struct {
	ID        string    `json:"id"`
	Decision  string    `json:"decision"`
	ActorID   string    `json:"actor_id"`
	Reason    *string   `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type publicationDTO struct {
	Proposal *proposalDTO `json:"proposal,omitempty"`
	Release  releaseDTO   `json:"release"`
}

type proposalRequestDTO struct {
	State    string       `json:"state"`
	RoomID   string       `json:"room_id"`
	Proposal *proposalDTO `json:"proposal,omitempty"`
}

func proposalRequestResponse(value ProposalRequestResult) proposalRequestDTO {
	result := proposalRequestDTO{State: value.State, RoomID: util.UUIDToString(value.RoomID)}
	if value.Generation != nil {
		proposal := proposalResponse(value.Generation.Proposal)
		result.Proposal = &proposal
	}
	return result
}

func overviewResponse(value EvolutionOverview, permissions permissionsDTO) overviewDTO {
	result := overviewDTO{
		Skill: skillIdentityDTO{
			ID: util.UUIDToString(value.Skill.Skill.ID), Name: value.Skill.Skill.Name,
			BundleHash: value.Skill.Manifest.Hash, Ownership: string(value.Skill.Ownership.Class),
			OwnershipReason: string(value.Skill.Ownership.Reason), ForkRequired: value.Skill.Ownership.ForkRequired,
		},
		Loop: nil, Revisions: make([]revisionDTO, 0, len(value.Revisions)),
		Proposals: make([]proposalDTO, 0, len(value.Proposals)), Releases: make([]releaseDTO, 0, len(value.Releases)),
		Permissions: permissions,
	}
	if value.Loop != nil {
		loop := loopResponse(*value.Loop)
		result.Loop = &loop
	}
	for _, revision := range value.Revisions {
		result.Revisions = append(result.Revisions, revisionResponse(revision))
	}
	for _, proposal := range value.Proposals {
		result.Proposals = append(result.Proposals, proposalSummaryResponse(proposal))
	}
	for _, release := range value.Releases {
		result.Releases = append(result.Releases, releaseResponse(release))
	}
	return result
}

func proposalDetailResponse(value ProposalView) proposalDetailDTO {
	result := proposalDetailDTO{
		Proposal: proposalResponse(value.Detail.Proposal), Rationale: value.Rationale,
		Diff:        BundleDiff{Metadata: []MetadataDiff{}, Files: []FileDiff{}},
		Evidence:    make([]evidenceDTO, 0, len(value.Detail.Evidence)),
		Evaluations: make([]evaluationDTO, 0, len(value.Detail.Evaluations)),
		Reviews:     make([]reviewDTO, 0, len(value.Detail.Reviews)),
	}
	if value.Candidate != nil {
		result.Diff = BuildBundleDiff(value.Base, *value.Candidate)
	}
	for _, item := range value.Detail.Evidence {
		result.Evidence = append(result.Evidence, evidenceResponse(item))
	}
	for _, item := range value.Detail.Evaluations {
		result.Evaluations = append(result.Evaluations, evaluationResponse(item))
	}
	for _, item := range value.Detail.Reviews {
		result.Reviews = append(result.Reviews, reviewResponse(item))
	}
	return result
}

func loopResponse(value db.SkillEvolutionLoop) loopDTO {
	return loopDTO{
		ID: util.UUIDToString(value.ID), Enabled: value.Enabled, Mode: value.Mode,
		CooldownSeconds: value.CooldownSeconds, MinimumSignals: value.MinimumSignals,
		MaxEvidenceRefs: value.MaxEvidenceRefs, MaxReplaySamples: value.MaxReplaySamples,
		MaxCostUSDTicks: value.MaxCostUsdTicks, PolicyVersion: value.PolicyVersion,
		LastObservedAt: optionalDTOTime(value.LastObservedAt), LastProposalAt: optionalDTOTime(value.LastProposalAt),
		NextEligibleAt: optionalDTOTime(value.NextEligibleAt), UpdatedAt: value.UpdatedAt.Time,
	}
}

func revisionResponse(value db.SkillEvolutionRevision) revisionDTO {
	return revisionDTO{ID: util.UUIDToString(value.ID), Kind: value.Kind, BundleHash: value.BundleHash,
		ByteCount: value.ByteCount, SupportFileCount: value.SupportFileCount, CreatedAt: value.CreatedAt.Time}
}

func proposalResponse(value db.SkillEvolutionProposal) proposalDTO {
	return proposalDTO{
		ID: util.UUIDToString(value.ID), SkillID: util.UUIDToString(value.SkillID), State: value.State,
		BaseRevisionID: util.UUIDToString(value.BaseRevisionID), BaseHash: value.BaseHash,
		CandidateID: optionalDTOUUID(value.CandidateRevisionID), CandidateHash: optionalDTOText(value.CandidateHash),
		FailureReason: optionalDTOText(value.FailureReason), StaleReason: optionalDTOText(value.StaleReason),
		CreatedAt: value.CreatedAt.Time, UpdatedAt: value.UpdatedAt.Time,
	}
}

func proposalSummaryResponse(value ProposalSummary) proposalDTO {
	return proposalDTO{
		ID: util.UUIDToString(value.ID), SkillID: util.UUIDToString(value.SkillID), State: value.State,
		BaseRevisionID: util.UUIDToString(value.BaseRevisionID), BaseHash: value.BaseHash,
		CandidateID: optionalDTOUUID(value.CandidateRevisionID), CandidateHash: optionalDTOText(value.CandidateHash),
		FailureReason: optionalDTOText(value.FailureReason), StaleReason: optionalDTOText(value.StaleReason),
		CreatedAt: value.CreatedAt.Time, UpdatedAt: value.UpdatedAt.Time,
	}
}

func releaseResponse(value db.SkillEvolutionRelease) releaseDTO {
	return releaseDTO{
		ID: util.UUIDToString(value.ID), SkillID: util.UUIDToString(value.SkillID),
		ProposalID: optionalDTOUUID(value.ProposalID), SourceReleaseID: optionalDTOUUID(value.SourceReleaseID),
		RevisionID: util.UUIDToString(value.RevisionID), Kind: value.Kind, ExpectedBaseHash: value.ExpectedBaseHash,
		PreHash: optionalDTOText(value.PreHash), PostHash: optionalDTOText(value.PostHash), Outcome: value.Outcome,
		ErrorCode: optionalDTOText(value.ErrorCode), CreatedAt: value.CreatedAt.Time, CompletedAt: optionalDTOTime(value.CompletedAt),
	}
}

func evidenceResponse(value db.SkillEvolutionEvidence) evidenceDTO {
	return evidenceDTO{Kind: value.Kind, SourceID: value.SourceID, SourceRevisionID: value.SourceRevisionID,
		SourceState: value.SourceState, Digest: value.Digest, ObservedAt: value.ObservedAt.Time}
}

func evaluationResponse(value db.SkillEvolutionEvaluation) evaluationDTO {
	metrics := json.RawMessage(append([]byte(nil), value.SafeMetrics...))
	if !json.Valid(metrics) {
		metrics = json.RawMessage(`{}`)
	}
	var cost *int64
	if value.CostUsdTicks.Valid {
		costValue := value.CostUsdTicks.Int64
		cost = &costValue
	}
	return evaluationDTO{ID: util.UUIDToString(value.ID), Kind: value.Kind, Result: value.Result,
		Adapter: value.Adapter, AdapterVersion: value.AdapterVersion, PolicyVersion: value.PolicyVersion,
		ResultDigest: value.ResultDigest, SafeMetrics: metrics, CostUSDTicks: cost, DurationMS: value.DurationMs,
		CreatedAt: value.CreatedAt.Time}
}

func reviewResponse(value db.SkillEvolutionReview) reviewDTO {
	return reviewDTO{ID: util.UUIDToString(value.ID), Decision: value.Decision, ActorID: util.UUIDToString(value.ActorID),
		Reason: optionalDTOText(value.Reason), CreatedAt: value.CreatedAt.Time}
}

func optionalDTOUUID(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	result := util.UUIDToString(value)
	return &result
}

func optionalDTOText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func optionalDTOTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
