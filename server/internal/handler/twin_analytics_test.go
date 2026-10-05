package handler

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/service"
)

func TestTwinProposalMetricCountsReturnsOnlyAggregates(t *testing.T) {
	content := []byte(`{
		"assertions":[
			{"id":"assertion-secret-a","text":"private instruction","evidence_citations":["citation-secret-a","citation-secret-b"]},
			{"id":"assertion-secret-b","text":"another private instruction","evidence_citations":["citation-secret-b"]},
			{"id":"assertion-secret-c","text":"unsupported private instruction","evidence_citations":[]}
		]
	}`)

	assertions, citations, unsupported := twinProposalMetricCounts(content)
	if assertions != 3 || citations != 2 || unsupported != 1 {
		t.Fatalf("counts = (%d, %d, %d), want (3, 2, 1)", assertions, citations, unsupported)
	}
}

func TestTwinMetricExclusionMapsCompilerDetailToBoundedDimensions(t *testing.T) {
	tests := []struct {
		name string
		code service.TwinBriefingExclusionCode
		want analytics.TwinExclusionCode
	}{
		{name: "policy", code: service.TwinBriefingPolicyOff, want: analytics.TwinExclusionPolicyOff},
		{name: "preview", code: service.TwinBriefingPreviewOnly, want: analytics.TwinExclusionPolicyOff},
		{name: "task unauthorized", code: service.TwinBriefingTaskUnauthorized, want: analytics.TwinExclusionUnauthorized},
		{name: "version unauthorized", code: service.TwinBriefingVersionUnauthorized, want: analytics.TwinExclusionUnauthorized},
		{name: "task local", code: service.TwinBriefingTaskLocalOnly, want: analytics.TwinExclusionLocalOnly},
		{name: "version local", code: service.TwinBriefingVersionLocalOnly, want: analytics.TwinExclusionLocalOnly},
		{name: "unsigned version", code: service.TwinBriefingUnsignedVersion, want: analytics.TwinExclusionNoSignedVersion},
		{name: "unsigned assertion", code: service.TwinBriefingUnsignedAssertion, want: analytics.TwinExclusionNoSignedVersion},
		{name: "stale", code: service.TwinBriefingStaleVersion, want: analytics.TwinExclusionStaleVersion},
		{name: "irrelevant", code: service.TwinBriefingIrrelevant, want: analytics.TwinExclusionIneligibleTask},
		{name: "budget", code: service.TwinBriefingOverBudget, want: analytics.TwinExclusionBudget},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := twinMetricExclusionCode(test.code)
			if got != test.want {
				t.Fatalf("exclusion = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTwinMetricExclusionUsesStablePrivacySafePrecedence(t *testing.T) {
	compiled := service.TwinCompiledBriefing{Exclusions: []service.TwinBriefingExclusion{
		{AssertionID: "assertion-secret", Code: service.TwinBriefingOverBudget},
		{AssertionID: "another-secret", Code: service.TwinBriefingTaskUnauthorized},
	}}
	if got := twinMetricExclusion(compiled); got != analytics.TwinExclusionUnauthorized {
		t.Fatalf("exclusion = %q, want %q", got, analytics.TwinExclusionUnauthorized)
	}
}
