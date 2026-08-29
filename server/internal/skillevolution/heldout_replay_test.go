package skillevolution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAcceptedRoomRecommendationCannotReplayItsSynthesisCorrection(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	workspaceID, userID, skillID := testUUID(), testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, skillID, testUUID())
	base := lifecycleBundle(skillID, "original")
	snapshot := lifecycleSnapshot(t, workspaceID, userID, skillID, base)
	skills := &memorySkillLoader{current: snapshot}
	synthesis := lifecycleEvidenceRef(workspaceID, skillID, "same-correction")
	candidate := ImprovementCandidate{
		Bundle: lifecycleBundle(skillID, "updated"), ObservedPattern: "repeated correction",
		ExpectedBenefit: "bounded improvement", RegressionRisk: "bounded risk",
		EvidenceDigests: []Digest{synthesis.Digest}, CostUSDTicks: 1,
	}
	candidate.AuthorizedChanges = BuildChangeAuthorizations(base, candidate.Bundle, candidate.EvidenceDigests)
	replay := &DeterministicReplayEvaluator{Outcome: ReplayOutcome{ReplayResult: ReplayResult{
		Result: EvaluationResultPassed, SampleCount: MinPassingReplaySamples,
	}}}
	lifecycle, err := NewLifecycle(NewStore(db.New(pool), pool), skills, &memoryPublisher{skills: skills}, &DeterministicImprover{}, replay)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Enable(context.Background(), DecisionActor{ID: userID, Kind: ActorKindHuman}, LoopConfig{
		WorkspaceID: workspaceID, SkillID: skillID, Mode: LoopModePropose, Cooldown: time.Hour,
		MinimumSignals: 1, MaxEvidenceRefs: 5, MaxReplaySamples: 2, MaxCostUSDTicks: 10, PolicyVersion: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	lifecycle.SetImprovementRecommendationSource(recommendationSourceFunc(func(_ context.Context, request RoomRecommendationRequest) (AcceptedImprovementRecommendation, error) {
		return AcceptedImprovementRecommendation{
			WorkspaceID: workspaceID, SkillID: skillID, RecommendationID: request.RecommendationID,
			ExpectedBaseHash: Digest(snapshot.Manifest.Hash), AcceptedByID: userID, Candidate: candidate,
			SynthesisEvidence: []ResolvedEvidence{{Ref: synthesis, Payload: []byte(`{"outcome":"needs_correction","correction":"same correction","reason":"bounded"}`)}},
			// The only available case is the synthesis correction, so it is not
			// eligible to appear again as held-out replay evidence.
			ReplayEvidence: nil,
		}, nil
	}))
	result, err := lifecycle.CreateProposalFromRoomRecommendation(context.Background(), RoomRecommendationRequest{
		WorkspaceID: workspaceID, SkillID: skillID, RecommendationID: uuid.NewString(), IdempotencyKey: "same-correction-overfit",
	})
	if !errors.Is(err, ErrEvaluationFailed) || result.Proposal.State != string(ProposalStateFailed) ||
		result.Replay.Result != string(EvaluationResultInconclusive) || replay.Calls != 0 {
		t.Fatalf("same-correction replay = (%+v, %v), evaluator calls=%d", result, err, replay.Calls)
	}
	if result.Replay.SafeMetrics == nil || !jsonEqual(result.Replay.SafeMetrics, []byte(`{"failures":0,"nondeterministic":false,"reason_code":"insufficient_held_out_samples","samples":0}`)) {
		t.Fatalf("content-free inconclusive replay metrics = %s", result.Replay.SafeMetrics)
	}
}
