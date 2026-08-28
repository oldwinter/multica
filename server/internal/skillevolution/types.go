package skillevolution

type LoopMode string

const (
	LoopModeObserve LoopMode = "observe"
	LoopModePropose LoopMode = "propose"
	LoopModePaused  LoopMode = "paused"
)

func (m LoopMode) Valid() bool {
	switch m {
	case LoopModeObserve, LoopModePropose, LoopModePaused:
		return true
	default:
		return false
	}
}

type ProposalState string

const (
	ProposalStateQueued             ProposalState = "queued"
	ProposalStateRunning            ProposalState = "running"
	ProposalStateReady              ProposalState = "ready"
	ProposalStateFailed             ProposalState = "failed"
	ProposalStateStale              ProposalState = "stale"
	ProposalStateRejected           ProposalState = "rejected"
	ProposalStatePublishing         ProposalState = "publishing"
	ProposalStatePublished          ProposalState = "published"
	ProposalStatePublicationUnknown ProposalState = "publication_unknown"
)

func (s ProposalState) Valid() bool {
	switch s {
	case ProposalStateQueued, ProposalStateRunning, ProposalStateReady,
		ProposalStateFailed, ProposalStateStale, ProposalStateRejected,
		ProposalStatePublishing, ProposalStatePublished, ProposalStatePublicationUnknown:
		return true
	default:
		return false
	}
}

func (s ProposalState) Terminal() bool {
	switch s {
	case ProposalStateFailed, ProposalStateStale, ProposalStateRejected,
		ProposalStatePublished, ProposalStatePublicationUnknown:
		return true
	default:
		return false
	}
}

type ReleaseKind string

const (
	ReleaseKindPublish  ReleaseKind = "publish"
	ReleaseKindRollback ReleaseKind = "rollback"
)

func (k ReleaseKind) Valid() bool {
	return k == ReleaseKindPublish || k == ReleaseKindRollback
}

type ReleaseOutcome string

const (
	ReleaseOutcomePending            ReleaseOutcome = "pending"
	ReleaseOutcomeSucceeded          ReleaseOutcome = "succeeded"
	ReleaseOutcomeFailed             ReleaseOutcome = "failed"
	ReleaseOutcomePublicationUnknown ReleaseOutcome = "publication_unknown"
)

func (o ReleaseOutcome) Valid() bool {
	switch o {
	case ReleaseOutcomePending, ReleaseOutcomeSucceeded, ReleaseOutcomeFailed, ReleaseOutcomePublicationUnknown:
		return true
	default:
		return false
	}
}

func (o ReleaseOutcome) Terminal() bool {
	return o.Valid() && o != ReleaseOutcomePending
}

type EvaluationResult string

const (
	EvaluationResultPassed       EvaluationResult = "passed"
	EvaluationResultFailed       EvaluationResult = "failed"
	EvaluationResultInconclusive EvaluationResult = "inconclusive"
	EvaluationResultUnknown      EvaluationResult = "unknown"
)

func (r EvaluationResult) Valid() bool {
	switch r {
	case EvaluationResultPassed, EvaluationResultFailed, EvaluationResultInconclusive, EvaluationResultUnknown:
		return true
	default:
		return false
	}
}

type RecommendationTarget string

const (
	RecommendationTargetKnowledge            RecommendationTarget = "knowledge"
	RecommendationTargetPreference           RecommendationTarget = "preference"
	RecommendationTargetConstraint           RecommendationTarget = "constraint"
	RecommendationTargetExecutableProcedure  RecommendationTarget = "executable_procedure"
	RecommendationTargetImplementationDefect RecommendationTarget = "implementation_defect"
	RecommendationTargetDecision             RecommendationTarget = "decision"
	RecommendationTargetUnsupported          RecommendationTarget = "unsupported"
)

func (t RecommendationTarget) Valid() bool {
	switch t {
	case RecommendationTargetKnowledge, RecommendationTargetPreference,
		RecommendationTargetConstraint, RecommendationTargetExecutableProcedure,
		RecommendationTargetImplementationDefect, RecommendationTargetDecision,
		RecommendationTargetUnsupported:
		return true
	default:
		return false
	}
}

const SkillExecutionManifestVersion = 1

// SkillExecutionManifest is the normalized, all-or-nothing resolved-bundle
// claim. Protocol decoding remains optional and belongs at the daemon boundary.
type SkillExecutionManifest struct {
	Version int                    `json:"version"`
	Skills  []SkillExecutionRecord `json:"skills"`
}

type SkillExecutionRecord struct {
	Source     string `json:"source"`
	SkillID    string `json:"skill_id"`
	BundleHash Digest `json:"bundle_hash"`
	RevisionID string `json:"revision_id,omitempty"`
}
