package service

import (
	"sort"
	"time"
)

const TwinActivationContractVersion = 1

type TwinActivationStageKey string

const (
	TwinActivationStageSourcePolicy TwinActivationStageKey = "source_policy"
	TwinActivationStageEvidence     TwinActivationStageKey = "evidence"
	TwinActivationStageSignedTwin   TwinActivationStageKey = "signed_twin"
	TwinActivationStagePreview      TwinActivationStageKey = "preview"
	TwinActivationStageBinding      TwinActivationStageKey = "binding"
	TwinActivationStageRun          TwinActivationStageKey = "attributed_run"
	TwinActivationStageFeedback     TwinActivationStageKey = "feedback"
	TwinActivationStageDeposition   TwinActivationStageKey = "deposition"
)

type TwinActivationActionKey string

const (
	TwinActivationActionInspectDisabled      TwinActivationActionKey = "inspect_disabled"
	TwinActivationActionConfigureSource      TwinActivationActionKey = "configure_source"
	TwinActivationActionReviewEvidence       TwinActivationActionKey = "review_evidence"
	TwinActivationActionRefreshEvidence      TwinActivationActionKey = "refresh_evidence"
	TwinActivationActionReviewTwin           TwinActivationActionKey = "review_twin"
	TwinActivationActionGenerateTwin         TwinActivationActionKey = "generate_twin"
	TwinActivationActionCompilePreview       TwinActivationActionKey = "compile_preview"
	TwinActivationActionConfigureBinding     TwinActivationActionKey = "configure_binding"
	TwinActivationActionRunWithTwin          TwinActivationActionKey = "run_with_twin"
	TwinActivationActionReviewRun            TwinActivationActionKey = "review_run"
	TwinActivationActionReviewDeposition     TwinActivationActionKey = "review_deposition"
	TwinActivationActionMonitorEffectiveness TwinActivationActionKey = "monitor_effectiveness"
)

type TwinActivationAction struct {
	Key             TwinActivationActionKey `json:"key"`
	Reason          string                  `json:"reason"`
	Target          string                  `json:"target"`
	ResponsibleRole string                  `json:"responsible_role"`
	CanAct          bool                    `json:"can_act"`
}

type TwinActivationBlocker struct {
	Kind            string `json:"kind"`
	Reason          string `json:"reason"`
	ResponsibleRole string `json:"responsible_role"`
}

type TwinActivationInspectionLink struct {
	Key    string `json:"key"`
	Target string `json:"target"`
}

type TwinActivationStage struct {
	Key      TwinActivationStageKey `json:"key"`
	Complete bool                   `json:"complete"`
	Count    int64                  `json:"count,omitempty"`
}

type TwinMaintenanceSeverity string

const (
	TwinMaintenanceSeverityHigh   TwinMaintenanceSeverity = "high"
	TwinMaintenanceSeverityMedium TwinMaintenanceSeverity = "medium"
	TwinMaintenanceSeverityLow    TwinMaintenanceSeverity = "low"
)

type TwinMaintenanceItem struct {
	ID            string                  `json:"id"`
	Kind          string                  `json:"kind"`
	Reason        string                  `json:"reason"`
	Severity      TwinMaintenanceSeverity `json:"severity"`
	OwnerRole     string                  `json:"owner_role"`
	SubjectType   string                  `json:"subject_type"`
	SubjectID     string                  `json:"subject_id,omitempty"`
	VersionNumber int64                   `json:"version_number,omitempty"`
	Count         int64                   `json:"count,omitempty"`
	CreatedAt     *time.Time              `json:"created_at,omitempty"`
	Action        TwinActivationActionKey `json:"action"`
}

// TwinActivationFacts is a content-free projection of persisted Wiki and Twin
// state. Callers must never place assertions, prompts, outputs, citations, or
// source locations in this structure.
type TwinActivationFacts struct {
	FeatureEnabled              bool
	CanManage                   bool
	SourcePolicyConfigured      bool
	PendingEvidenceRevisionID   string
	AcceptedEvidenceRevisionID  string
	PendingProposalID           string
	PendingProposalCreatedAt    *time.Time
	CurrentVersionID            string
	CurrentVersionNumber        int64
	CurrentVersionSourceID      string
	PreviewedVersionID          string
	ActiveBindingCount          int64
	ExplicitOffBindingCount     int64
	AttributedRunCount          int64
	FeedbackCount               int64
	PendingDepositionID         string
	PendingDepositionCreatedAt  *time.Time
	PendingDepositionCount      int64
	RecentMismatchCount         int64
	LowConfidenceAssertionCount int64
}

type TwinActivationReadiness struct {
	ContractVersion int                            `json:"contract_version"`
	Ready           bool                           `json:"ready"`
	CanManage       bool                           `json:"can_manage"`
	Stages          []TwinActivationStage          `json:"stages"`
	NextAction      TwinActivationAction           `json:"next_action"`
	Blockers        []TwinActivationBlocker        `json:"blockers"`
	InspectionLinks []TwinActivationInspectionLink `json:"inspection_links"`
	Maintenance     []TwinMaintenanceItem          `json:"maintenance"`
}

func BuildTwinActivationReadiness(facts TwinActivationFacts) TwinActivationReadiness {
	evidenceAccepted := facts.AcceptedEvidenceRevisionID != ""
	signedCurrent := facts.CurrentVersionID != "" && facts.CurrentVersionSourceID == facts.AcceptedEvidenceRevisionID
	previewedCurrent := signedCurrent && facts.PreviewedVersionID == facts.CurrentVersionID
	depositionClear := facts.PendingDepositionCount == 0

	stages := []TwinActivationStage{
		{Key: TwinActivationStageSourcePolicy, Complete: facts.SourcePolicyConfigured},
		{Key: TwinActivationStageEvidence, Complete: evidenceAccepted},
		{Key: TwinActivationStageSignedTwin, Complete: signedCurrent},
		{Key: TwinActivationStagePreview, Complete: previewedCurrent},
		{Key: TwinActivationStageBinding, Complete: facts.ActiveBindingCount > 0, Count: facts.ActiveBindingCount},
		{Key: TwinActivationStageRun, Complete: facts.AttributedRunCount > 0, Count: facts.AttributedRunCount},
		{Key: TwinActivationStageFeedback, Complete: facts.FeedbackCount > 0, Count: facts.FeedbackCount},
		{Key: TwinActivationStageDeposition, Complete: depositionClear, Count: facts.PendingDepositionCount},
	}

	nextAction := nextTwinActivationAction(facts, signedCurrent, previewedCurrent)
	readiness := TwinActivationReadiness{
		ContractVersion: TwinActivationContractVersion,
		Ready:           facts.FeatureEnabled && allTwinActivationStagesComplete(stages),
		CanManage:       facts.CanManage,
		Stages:          stages,
		NextAction:      nextAction,
		Blockers:        twinActivationBlockers(facts, nextAction, signedCurrent),
		InspectionLinks: []TwinActivationInspectionLink{
			{Key: "evidence_history", Target: "wiki"},
			{Key: "twin_history", Target: "twin"},
			{Key: "execution_evidence", Target: "use"},
		},
		Maintenance: twinMaintenanceQueue(facts),
	}
	return readiness
}

func twinActivationBlockers(facts TwinActivationFacts, action TwinActivationAction, signedCurrent bool) []TwinActivationBlocker {
	if action.Key == TwinActivationActionMonitorEffectiveness {
		return []TwinActivationBlocker{}
	}
	blockers := make([]TwinActivationBlocker, 0, 2)
	kind := "missing_state"
	switch {
	case !facts.FeatureEnabled:
		kind = "kill_switch"
	case facts.CurrentVersionID != "" && !signedCurrent:
		kind = "stale_version"
	case action.Key == TwinActivationActionConfigureBinding && facts.ExplicitOffBindingCount > 0:
		kind = "exclusion"
	case action.Key == TwinActivationActionReviewEvidence || action.Key == TwinActivationActionReviewTwin || action.Key == TwinActivationActionReviewDeposition:
		kind = "review_gate"
	}
	blockers = append(blockers, TwinActivationBlocker{
		Kind: kind, Reason: action.Reason, ResponsibleRole: action.ResponsibleRole,
	})
	if !action.CanAct {
		blockers = append(blockers, TwinActivationBlocker{
			Kind: "missing_capability", Reason: "owner_or_admin_required", ResponsibleRole: "owner_admin",
		})
	}
	return blockers
}

func nextTwinActivationAction(facts TwinActivationFacts, signedCurrent, previewedCurrent bool) TwinActivationAction {
	managerAction := func(key TwinActivationActionKey, reason, target string) TwinActivationAction {
		return TwinActivationAction{Key: key, Reason: reason, Target: target, ResponsibleRole: "owner_admin", CanAct: facts.CanManage}
	}
	memberAction := func(key TwinActivationActionKey, reason, target string) TwinActivationAction {
		return TwinActivationAction{Key: key, Reason: reason, Target: target, ResponsibleRole: "member", CanAct: true}
	}

	switch {
	case !facts.FeatureEnabled:
		return managerAction(TwinActivationActionInspectDisabled, "disabled_by_operator", "use")
	case !facts.SourcePolicyConfigured:
		return managerAction(TwinActivationActionConfigureSource, "source_policy_missing", "wiki")
	case facts.AcceptedEvidenceRevisionID == "" && facts.PendingEvidenceRevisionID != "":
		return managerAction(TwinActivationActionReviewEvidence, "evidence_review_pending", "wiki")
	case facts.AcceptedEvidenceRevisionID == "":
		return managerAction(TwinActivationActionRefreshEvidence, "accepted_evidence_missing", "wiki")
	case !signedCurrent && facts.PendingProposalID != "":
		return managerAction(TwinActivationActionReviewTwin, "twin_review_pending", "twin")
	case !signedCurrent:
		return managerAction(TwinActivationActionGenerateTwin, "signed_twin_missing_or_stale", "twin")
	case !previewedCurrent:
		return memberAction(TwinActivationActionCompilePreview, "current_twin_not_previewed", "use")
	case facts.ActiveBindingCount == 0 && facts.ExplicitOffBindingCount > 0:
		return managerAction(TwinActivationActionConfigureBinding, "explicit_off_binding", "use")
	case facts.ActiveBindingCount == 0:
		return managerAction(TwinActivationActionConfigureBinding, "active_binding_missing", "use")
	case facts.AttributedRunCount == 0:
		return memberAction(TwinActivationActionRunWithTwin, "attributed_run_missing", "use")
	case facts.FeedbackCount == 0:
		return memberAction(TwinActivationActionReviewRun, "run_feedback_missing", "use")
	case facts.PendingDepositionCount > 0:
		return managerAction(TwinActivationActionReviewDeposition, "deposition_review_pending", "twin")
	default:
		return memberAction(TwinActivationActionMonitorEffectiveness, "activation_loop_complete", "use")
	}
}

func twinMaintenanceQueue(facts TwinActivationFacts) []TwinMaintenanceItem {
	items := make([]TwinMaintenanceItem, 0, 5)
	if facts.PendingProposalID != "" {
		items = append(items, TwinMaintenanceItem{
			ID: "pending_proposal:" + facts.PendingProposalID, Kind: "pending_proposal", Reason: "review_required",
			Severity: TwinMaintenanceSeverityMedium, OwnerRole: "owner_admin", SubjectType: "twin_proposal",
			SubjectID: facts.PendingProposalID, CreatedAt: facts.PendingProposalCreatedAt, Action: TwinActivationActionReviewTwin,
		})
	}
	if facts.CurrentVersionID != "" && facts.AcceptedEvidenceRevisionID != "" && facts.CurrentVersionSourceID != facts.AcceptedEvidenceRevisionID {
		items = append(items, TwinMaintenanceItem{
			ID: "stale_signed_version:" + facts.CurrentVersionID, Kind: "stale_signed_version", Reason: "accepted_evidence_superseded_version",
			Severity: TwinMaintenanceSeverityHigh, OwnerRole: "owner_admin", SubjectType: "twin_version",
			SubjectID: facts.CurrentVersionID, VersionNumber: facts.CurrentVersionNumber, Action: TwinActivationActionGenerateTwin,
		})
	}
	if facts.RecentMismatchCount >= 2 {
		items = append(items, TwinMaintenanceItem{
			ID: "repeated_mismatch", Kind: "repeated_mismatch", Reason: "feedback_threshold_reached",
			Severity: TwinMaintenanceSeverityHigh, OwnerRole: "owner_admin", SubjectType: "workspace",
			Count: facts.RecentMismatchCount, Action: TwinActivationActionReviewRun,
		})
	}
	if facts.LowConfidenceAssertionCount > 0 && facts.CurrentVersionID != "" {
		items = append(items, TwinMaintenanceItem{
			ID: "low_confidence:" + facts.CurrentVersionID, Kind: "low_confidence", Reason: "signed_assertion_review_recommended",
			Severity: TwinMaintenanceSeverityLow, OwnerRole: "owner_admin", SubjectType: "twin_version",
			SubjectID: facts.CurrentVersionID, VersionNumber: facts.CurrentVersionNumber,
			Count: facts.LowConfidenceAssertionCount, Action: TwinActivationActionReviewTwin,
		})
	}
	if facts.PendingDepositionCount > 0 {
		items = append(items, TwinMaintenanceItem{
			ID: "pending_deposition:" + facts.PendingDepositionID, Kind: "pending_deposition", Reason: "review_required",
			Severity: TwinMaintenanceSeverityMedium, OwnerRole: "owner_admin", SubjectType: "twin_deposition",
			SubjectID: facts.PendingDepositionID, Count: facts.PendingDepositionCount,
			CreatedAt: facts.PendingDepositionCreatedAt, Action: TwinActivationActionReviewDeposition,
		})
	}

	severityRank := map[TwinMaintenanceSeverity]int{
		TwinMaintenanceSeverityHigh: 0, TwinMaintenanceSeverityMedium: 1, TwinMaintenanceSeverityLow: 2,
	}
	sort.Slice(items, func(left, right int) bool {
		if severityRank[items[left].Severity] != severityRank[items[right].Severity] {
			return severityRank[items[left].Severity] < severityRank[items[right].Severity]
		}
		return items[left].ID < items[right].ID
	})
	return items
}

func allTwinActivationStagesComplete(stages []TwinActivationStage) bool {
	for _, stage := range stages {
		if !stage.Complete {
			return false
		}
	}
	return true
}
