package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildTwinActivationReadinessChoosesOneDeterministicAction(t *testing.T) {
	complete := TwinActivationFacts{
		FeatureEnabled: true, CanManage: true, SourcePolicyConfigured: true,
		AcceptedEvidenceRevisionID: "evidence:2", CurrentVersionID: "version:2",
		CurrentVersionSourceID: "evidence:2", PreviewedVersionID: "version:2",
		ActiveBindingCount: 1, AttributedRunCount: 1, FeedbackCount: 1,
	}

	for _, test := range []struct {
		name  string
		alter func(*TwinActivationFacts)
		want  TwinActivationActionKey
	}{
		{name: "operator disabled wins", alter: func(f *TwinActivationFacts) { f.FeatureEnabled = false; f.SourcePolicyConfigured = false }, want: TwinActivationActionInspectDisabled},
		{name: "source policy", alter: func(f *TwinActivationFacts) { f.SourcePolicyConfigured = false }, want: TwinActivationActionConfigureSource},
		{name: "pending evidence", alter: func(f *TwinActivationFacts) {
			f.AcceptedEvidenceRevisionID = ""
			f.PendingEvidenceRevisionID = "evidence:pending"
		}, want: TwinActivationActionReviewEvidence},
		{name: "stale signed version", alter: func(f *TwinActivationFacts) { f.CurrentVersionSourceID = "evidence:1" }, want: TwinActivationActionGenerateTwin},
		{name: "pending Twin review", alter: func(f *TwinActivationFacts) { f.CurrentVersionID = ""; f.PendingProposalID = "proposal:1" }, want: TwinActivationActionReviewTwin},
		{name: "preview", alter: func(f *TwinActivationFacts) { f.PreviewedVersionID = "" }, want: TwinActivationActionCompilePreview},
		{name: "binding", alter: func(f *TwinActivationFacts) { f.ActiveBindingCount = 0 }, want: TwinActivationActionConfigureBinding},
		{name: "run", alter: func(f *TwinActivationFacts) { f.AttributedRunCount = 0 }, want: TwinActivationActionRunWithTwin},
		{name: "feedback", alter: func(f *TwinActivationFacts) { f.FeedbackCount = 0 }, want: TwinActivationActionReviewRun},
		{name: "deposition", alter: func(f *TwinActivationFacts) { f.PendingDepositionCount = 1 }, want: TwinActivationActionReviewDeposition},
		{name: "complete", alter: func(_ *TwinActivationFacts) {}, want: TwinActivationActionMonitorEffectiveness},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts := complete
			test.alter(&facts)
			got := BuildTwinActivationReadiness(facts)
			if got.NextAction.Key != test.want {
				t.Fatalf("next action = %q, want %q", got.NextAction.Key, test.want)
			}
			if got.ContractVersion != TwinActivationContractVersion || len(got.Stages) != 8 {
				t.Fatalf("readiness contract = %#v", got)
			}
			if len(got.InspectionLinks) != 3 {
				t.Fatalf("inspection links = %#v", got.InspectionLinks)
			}
		})
	}
}

func TestBuildTwinActivationReadinessReturnsSafeSortedMaintenanceMetadata(t *testing.T) {
	facts := TwinActivationFacts{
		FeatureEnabled: true, CanManage: true, SourcePolicyConfigured: true,
		AcceptedEvidenceRevisionID: "evidence:2", PendingProposalID: "proposal:2",
		CurrentVersionID: "version:1", CurrentVersionNumber: 4, CurrentVersionSourceID: "evidence:1",
		PendingDepositionID: "deposition:1", PendingDepositionCount: 2,
		RecentMismatchCount: 3, LowConfidenceAssertionCount: 1,
	}
	got := BuildTwinActivationReadiness(facts)
	if len(got.Maintenance) != 5 {
		t.Fatalf("maintenance item count = %d, want 5", len(got.Maintenance))
	}
	if got.Maintenance[0].Severity != TwinMaintenanceSeverityHigh || got.Maintenance[1].Severity != TwinMaintenanceSeverityHigh {
		t.Fatalf("maintenance queue is not severity sorted: %#v", got.Maintenance)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal readiness: %v", err)
	}
	serialized := strings.ToLower(string(raw))
	for _, forbidden := range []string{"assertions", "prompt", "output", "citation", "credential", "local_path", "briefing"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("readiness payload contains forbidden field %q: %s", forbidden, raw)
		}
	}
}

func TestBuildTwinActivationReadinessIdentifiesExplicitOffAsAnExclusion(t *testing.T) {
	facts := TwinActivationFacts{
		FeatureEnabled: true, CanManage: false, SourcePolicyConfigured: true,
		AcceptedEvidenceRevisionID: "evidence:1", CurrentVersionID: "version:1",
		CurrentVersionSourceID: "evidence:1", PreviewedVersionID: "version:1",
		ExplicitOffBindingCount: 1,
	}
	got := BuildTwinActivationReadiness(facts)
	if got.NextAction.Key != TwinActivationActionConfigureBinding || got.NextAction.Reason != "explicit_off_binding" {
		t.Fatalf("next action = %#v", got.NextAction)
	}
	if len(got.Blockers) != 2 || got.Blockers[0].Kind != "exclusion" || got.Blockers[1].Kind != "missing_capability" {
		t.Fatalf("blockers = %#v", got.Blockers)
	}
}

func TestBuildTwinEffectivenessMetricsSuppressesSmallCohortsAndRequiresControl(t *testing.T) {
	metrics := buildTwinEffectivenessMetrics([]TwinExecutionCohortMetrics{
		{PolicyState: "off", SampleSize: 5, CompletedRuns: 4, RevisionCount: 1, CostedRuns: 4, UncostedRuns: 1},
		{PolicyState: "preview", SampleSize: 2, CompletedRuns: 2, FeedbackHelped: 2},
		{PolicyState: "enabled", SampleSize: 5, CompletedRuns: 5, FeedbackTotal: 4, FeedbackHelped: 3, RevisionCount: 1, CostedRuns: 5, DepositionTotal: 2, DepositionAccepted: 1},
	})
	if !metrics.Comparison.Eligible || metrics.Comparison.ControlState != "off" {
		t.Fatalf("comparison = %#v", metrics.Comparison)
	}
	if !metrics.Cohorts[1].DetailSuppressed || metrics.Cohorts[1].FeedbackHelped != nil || metrics.Cohorts[1].CostUSDTicks != nil {
		t.Fatalf("small preview cohort leaked detail: %#v", metrics.Cohorts[1])
	}
	if metrics.Cohorts[2].DetailSuppressed || metrics.Cohorts[2].HelpedRate == nil || *metrics.Cohorts[2].HelpedRate != 0.75 {
		t.Fatalf("enabled cohort = %#v", metrics.Cohorts[2])
	}
	if metrics.Cohorts[2].DepositionAcceptance == nil || *metrics.Cohorts[2].DepositionAcceptance != 0.5 {
		t.Fatalf("enabled deposition acceptance = %#v", metrics.Cohorts[2])
	}
	if metrics.Cohorts[0].HelpedRate != nil || metrics.Cohorts[0].DepositionAcceptance != nil {
		t.Fatalf("empty outcome rates must remain unavailable: %#v", metrics.Cohorts[0])
	}
}
