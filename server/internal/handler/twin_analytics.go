package handler

import (
	"encoding/json"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func twinProposalMetricCounts(content []byte) (assertions, citations, unsupported int) {
	var proposal service.TwinProposalContent
	if json.Unmarshal(content, &proposal) != nil {
		return 0, 0, 0
	}
	seenCitations := make(map[string]struct{})
	for _, assertion := range proposal.Assertions {
		if len(assertion.EvidenceCitations) == 0 {
			unsupported++
		}
		for _, citation := range assertion.EvidenceCitations {
			seenCitations[citation] = struct{}{}
		}
	}
	return len(proposal.Assertions), len(seenCitations), unsupported
}

func twinProposalMetric(proposal db.TwinProposal, userID, taskID string) (analytics.TwinMetricContext, analytics.TwinProposalKind, int, int, int) {
	assertions, citations, unsupported := twinProposalMetricCounts(proposal.Content)
	kind := analytics.TwinProposalKind(proposal.Kind)
	if proposal.Kind == "correction" {
		kind = analytics.TwinProposalKindInitial
		if proposal.BaseTwinVersionID.Valid {
			kind = analytics.TwinProposalKindEvolution
		}
	}
	return analytics.TwinMetricContext{
		UserID: userID, WorkspaceID: uuidToString(proposal.WorkspaceID), TaskID: taskID,
	}, kind, assertions, citations, unsupported
}

func twinCompilationMetricState(compiled service.TwinCompiledBriefing) analytics.TwinCompilationState {
	if compiled.Inject || compiled.PreviewOnly {
		return analytics.TwinCompilationStateCompiled
	}
	return analytics.TwinCompilationStateExcluded
}

func twinMetricScope(scope service.TwinUsePolicyScope) analytics.TwinPolicyScope {
	return analytics.TwinPolicyScope(scope)
}

// twinMetricExclusion deliberately maps compiler detail to a bounded product
// dimension. Assertion IDs and raw exclusion data never cross this boundary.
func twinMetricExclusion(compiled service.TwinCompiledBriefing) analytics.TwinExclusionCode {
	best := analytics.TwinExclusionNone
	bestRank := 0
	for _, exclusion := range compiled.Exclusions {
		candidate, rank := twinMetricExclusionCode(exclusion.Code)
		if rank > bestRank {
			best, bestRank = candidate, rank
		}
	}
	return best
}

func twinMetricExclusionCode(code service.TwinBriefingExclusionCode) (analytics.TwinExclusionCode, int) {
	switch code {
	case service.TwinBriefingTaskUnauthorized, service.TwinBriefingVersionUnauthorized:
		return analytics.TwinExclusionUnauthorized, 90
	case service.TwinBriefingTaskLocalOnly, service.TwinBriefingVersionLocalOnly:
		return analytics.TwinExclusionLocalOnly, 80
	case service.TwinBriefingUnsignedVersion, service.TwinBriefingMutableProposal, service.TwinBriefingUnsignedAssertion:
		return analytics.TwinExclusionNoSignedVersion, 70
	case service.TwinBriefingStaleVersion:
		return analytics.TwinExclusionStaleVersion, 60
	case service.TwinBriefingTaskIneligible, service.TwinBriefingIrrelevant, service.TwinBriefingNoRelevantAssertion:
		return analytics.TwinExclusionIneligibleTask, 50
	case service.TwinBriefingOverBudget:
		return analytics.TwinExclusionBudget, 40
	case service.TwinBriefingPolicyOff, service.TwinBriefingPreviewOnly:
		return analytics.TwinExclusionPolicyOff, 30
	default:
		return analytics.TwinExclusionNone, 0
	}
}
