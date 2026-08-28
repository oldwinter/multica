package skillevolution

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestSignalAdaptersKeepDiscoveryContentFreeAndRevalidateLoad(t *testing.T) {
	workspaceID, skillID := testUUID(), testUUID()
	ref := EvidenceRef{
		WorkspaceID: uuid.UUID(workspaceID.Bytes).String(), Kind: EvidenceKindTaskReview,
		SourceID: uuid.NewString(), TargetSkillID: uuid.UUID(skillID.Bytes).String(),
		SourceState: "needs_correction", Digest: testDigest("review"),
		Eligibility: EvidenceEligibilityEligible, ObservedAt: time.Now().UTC(),
	}
	adapter := NewSignalAdapter(EvidenceKindTaskReview,
		func(context.Context, SignalQuery) ([]EvidenceRef, error) { return []EvidenceRef{ref}, nil },
		func(_ context.Context, _ SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			return ResolvedEvidence{Ref: expected, Payload: []byte(`{"correction":"bounded"}`)}, nil
		},
	)
	set, err := newSignalSet([]SignalSource{adapter})
	if err != nil {
		t.Fatal(err)
	}
	query := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, Limit: 5}
	refs, err := set.discover(context.Background(), query)
	if err != nil || len(refs) != 1 {
		t.Fatalf("discover = (%+v, %v)", refs, err)
	}
	if reflect.TypeOf(refs[0]).Field(0).Name != "WorkspaceID" {
		t.Fatal("discovery did not return the content-free EvidenceRef contract")
	}
	resolved, err := set.resolve(context.Background(), query, refs)
	if err != nil || string(resolved[0].Payload) != `{"correction":"bounded"}` {
		t.Fatalf("resolve = (%+v, %v)", resolved, err)
	}

	drifted := NewSignalAdapter(EvidenceKindTaskReview,
		func(context.Context, SignalQuery) ([]EvidenceRef, error) { return []EvidenceRef{ref}, nil },
		func(_ context.Context, _ SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			expected.Digest = testDigest("changed")
			return ResolvedEvidence{Ref: expected}, nil
		},
	)
	if _, err := drifted.Load(context.Background(), query, ref); !errors.Is(err, ErrSignalSourceDrift) {
		t.Fatalf("drift error = %v, want source drift", err)
	}
}

func TestCandidatePolicyAndBoundedReplayFailClosed(t *testing.T) {
	base := lifecycleBundle(testUUID(), "original")
	ref := lifecycleEvidenceRef(testUUID(), testUUID(), "policy")
	candidate := ImprovementCandidate{
		Bundle:          lifecycleBundle(pgtype.UUID{Bytes: uuid.MustParse(base.ID), Valid: true}, "updated"),
		ObservedPattern: "repeated mismatch", ExpectedBenefit: "more precise output", RegressionRisk: "may be too strict",
		EvidenceDigests: []Digest{ref.Digest},
	}
	resolved := []ResolvedEvidence{{Ref: ref, Payload: []byte(`{"reason":"bounded"}`)}}
	passed := ValidateCandidatePolicy(base, candidate, resolved, DefaultCandidatePolicy())
	if passed.Result != EvaluationResultPassed || !passed.Digest.Valid() {
		t.Fatalf("valid candidate outcome = %+v", passed)
	}

	unsafe := candidate
	unsafe.Bundle.Content += "\napi_key = abcdefghijklmnopqrstuvwxyz"
	failed := ValidateCandidatePolicy(base, unsafe, resolved, DefaultCandidatePolicy())
	if failed.Result != EvaluationResultFailed || !containsString(failed.RuleCodes, "secret_like_content") {
		t.Fatalf("unsafe candidate outcome = %+v", failed)
	}

	engine := replayEngineFunc(func(context.Context, ReplayRequest) (ReplayResult, error) {
		return ReplayResult{Result: EvaluationResultPassed, SampleCount: 1}, nil
	})
	evaluator := NewProductionReplayEvaluator(engine, "room-replay", "v1")
	outcome, err := evaluator.Evaluate(context.Background(), ReplayRequest{
		Base: base, Candidate: candidate.Bundle, Evidence: resolved,
		Limits: ReplayLimits{Timeout: time.Second, MaxSamples: 2, MaxCostUSDTicks: 10, PolicyVersion: "v1"},
	})
	if err != nil || outcome.Result != EvaluationResultInconclusive || outcome.ReasonCode != "low_sample" {
		t.Fatalf("bounded replay = (%+v, %v)", outcome, err)
	}
	zeroSamples := enforceReplayLimits(ReplayOutcome{ReplayResult: ReplayResult{
		Result: EvaluationResultPassed,
	}}, ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 10, PolicyVersion: "v1"})
	if zeroSamples.Result != EvaluationResultInconclusive || zeroSamples.ReasonCode != "no_samples" {
		t.Fatalf("zero-sample replay = %+v", zeroSamples)
	}
	ignoresCancellation := NewProductionReplayEvaluator(replayEngineFunc(func(context.Context, ReplayRequest) (ReplayResult, error) {
		time.Sleep(20 * time.Millisecond)
		return ReplayResult{Result: EvaluationResultPassed, SampleCount: 2}, nil
	}), "room-replay", "v1")
	timedOut, err := ignoresCancellation.Evaluate(context.Background(), ReplayRequest{
		Base: base, Candidate: candidate.Bundle, Evidence: resolved,
		Limits: ReplayLimits{Timeout: time.Millisecond, MaxSamples: 2, MaxCostUSDTicks: 10, PolicyVersion: "v1"},
	})
	if err != nil || timedOut.Result != EvaluationResultUnknown || timedOut.ReasonCode != "timeout" {
		t.Fatalf("replay timeout = (%+v, %v)", timedOut, err)
	}
}

func TestProductionImproverEnforcesTimeoutAndCostBounds(t *testing.T) {
	workspaceID, skillID := testUUID(), testUUID()
	ref := lifecycleEvidenceRef(workspaceID, skillID, "improver")
	request := ImprovementRequest{
		Base: lifecycleBundle(skillID, "original"), Evidence: []ResolvedEvidence{{Ref: ref, Payload: []byte(`{}`)}},
		PolicyVersion: "v1", MaxCostUSDTicks: 5, MaxChangedFiles: 2, MaxPrimaryGrowth: 100,
	}
	tooExpensive := NewProductionImprover(improvementEngineFunc(func(context.Context, ImprovementRequest) (ImprovementCandidate, error) {
		return ImprovementCandidate{
			Bundle: lifecycleBundle(skillID, "updated"), ObservedPattern: "pattern", ExpectedBenefit: "benefit",
			RegressionRisk: "risk", EvidenceDigests: []Digest{ref.Digest}, CostUSDTicks: 6,
		}, nil
	}), time.Second)
	if _, err := tooExpensive.Improve(context.Background(), request); !errors.Is(err, ErrImproverLimit) {
		t.Fatalf("cost error = %v, want limit", err)
	}

	timedOut := NewProductionImprover(improvementEngineFunc(func(ctx context.Context, _ ImprovementRequest) (ImprovementCandidate, error) {
		<-ctx.Done()
		return ImprovementCandidate{}, ctx.Err()
	}), time.Millisecond)
	if _, err := timedOut.Improve(context.Background(), request); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v, want deadline", err)
	}
}

func TestLifecycleDBBackedGenerateRejectPublishStaleAndRollback(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	workspaceID, userID, skillID := testUUID(), testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, skillID, testUUID())

	baseBundle := lifecycleBundle(skillID, "original")
	baseSnapshot := lifecycleSnapshot(t, workspaceID, userID, skillID, baseBundle)
	skills := &memorySkillLoader{current: baseSnapshot}
	ref := lifecycleEvidenceRef(workspaceID, skillID, "canonical")
	source := lifecycleSignalSource(ref)
	candidateBundle := lifecycleBundle(skillID, "updated")
	improver := &DeterministicImprover{Candidate: ImprovementCandidate{
		Bundle: candidateBundle, ObservedPattern: "repeated mismatch", ExpectedBenefit: "more precise output",
		RegressionRisk: "may be too strict", EvidenceDigests: []Digest{ref.Digest}, CostUSDTicks: 3,
	}}
	replay := &DeterministicReplayEvaluator{Outcome: ReplayOutcome{ReplayResult: ReplayResult{
		Result: EvaluationResultPassed, SampleCount: 2, CostUSDTicks: 2,
	}}}
	publisher := &memoryPublisher{skills: skills}
	lifecycle, err := NewLifecycle(NewStore(db.New(pool), pool), skills, publisher, improver, replay, source)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle.now = func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	actor := DecisionActor{ID: userID, Kind: ActorKindHuman}
	config := LoopConfig{
		WorkspaceID: workspaceID, SkillID: skillID, Mode: LoopModePropose, Cooldown: time.Hour,
		MinimumSignals: 1, MaxEvidenceRefs: 10, MaxReplaySamples: 2, MaxCostUSDTicks: 20, PolicyVersion: "v1",
	}
	loop, err := lifecycle.Enable(ctx, actor, config)
	if err != nil || !loop.Enabled || loop.Mode != string(LoopModePropose) {
		t.Fatalf("Enable = (%+v, %v)", loop, err)
	}
	if publisher.calls != 0 {
		t.Fatal("enable invoked publisher")
	}

	generation, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: skillID, RequestedByID: userID, GenerationKey: "generate-accept",
	})
	if err != nil || generation.Proposal.State != string(ProposalStateReady) ||
		generation.Validation.Result != string(EvaluationResultPassed) || generation.Replay.Result != string(EvaluationResultPassed) {
		t.Fatalf("Generate = (%+v, %v)", generation, err)
	}
	if publisher.calls != 0 {
		t.Fatal("generation automatically invoked publisher")
	}
	idempotentGeneration, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: skillID, RequestedByID: userID, GenerationKey: "generate-accept",
	})
	if err != nil || !idempotentGeneration.Replayed || idempotentGeneration.Proposal.ID != generation.Proposal.ID || improver.Calls != 1 {
		t.Fatalf("idempotent Generate = (%+v, %v), improver calls=%d", idempotentGeneration, err, improver.Calls)
	}
	detail, err := lifecycle.ReadProposal(ctx, workspaceID, generation.Proposal.ID)
	if err != nil || len(detail.Detail.Evidence) != 1 || len(detail.Detail.Evaluations) != 2 || len(detail.Detail.Reviews) != 0 ||
		detail.Base.Revision.ID != generation.Proposal.BaseRevisionID || detail.Candidate == nil || detail.Candidate.Revision.ID != generation.Proposal.CandidateRevisionID {
		t.Fatalf("proposal detail before review = (%+v, %v)", detail, err)
	}

	publication, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: generation.Proposal.ID, Actor: actor,
		Reason: "reviewed", IdempotencyKey: "publish-accept",
	})
	if err != nil || publication.Proposal.State != string(ProposalStatePublished) ||
		publication.Release.Outcome != string(ReleaseOutcomeSucceeded) || publisher.calls != 1 {
		t.Fatalf("Publish = (%+v, %v), calls=%d", publication, err, publisher.calls)
	}
	if _, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: generation.Proposal.ID,
		Actor:  DecisionActor{ID: testUUID(), Kind: ActorKindHuman},
		Reason: "reviewed", IdempotencyKey: "publish-accept",
	}); !errors.Is(err, ErrPersistenceConflict) || publisher.calls != 1 {
		t.Fatalf("mismatched Publish replay error = %v, calls=%d", err, publisher.calls)
	}
	detail, err = lifecycle.ReadProposal(ctx, workspaceID, generation.Proposal.ID)
	if err != nil || len(detail.Detail.Reviews) != 1 || detail.Detail.Reviews[0].Decision != "publish" {
		t.Fatalf("proposal reviews after publish = (%+v, %v)", detail.Detail.Reviews, err)
	}
	if _, err := lifecycle.Rollback(ctx, RollbackRequest{
		WorkspaceID: workspaceID, SkillID: skillID, SourceReleaseID: publication.Release.ID,
		Actor: actor, IdempotencyKey: "publish-accept",
	}); !errors.Is(err, ErrPersistenceConflict) || publisher.calls != 1 {
		t.Fatalf("cross-operation release key error = %v, calls=%d", err, publisher.calls)
	}

	rolledBack, err := lifecycle.Rollback(ctx, RollbackRequest{
		WorkspaceID: workspaceID, SkillID: skillID, SourceReleaseID: publication.Release.ID,
		Actor: actor, IdempotencyKey: "rollback-accept",
	})
	if err != nil || rolledBack.Release.Outcome != string(ReleaseOutcomeSucceeded) || publisher.calls != 2 ||
		rolledBack.Result.PostHash != Digest(baseSnapshot.Manifest.Hash) {
		t.Fatalf("Rollback = (%+v, %v), calls=%d", rolledBack, err, publisher.calls)
	}
	overview, err := lifecycle.Overview(ctx, workspaceID, skillID, DefaultOverviewLimit)
	if err != nil || len(overview.Releases) != 2 || overview.Releases[0].Kind != string(ReleaseKindRollback) {
		t.Fatalf("append-only release history = (%+v, %v)", overview.Releases, err)
	}
	savedSnapshot := skills.current
	skills.current = WorkspaceSkillSnapshot{}
	if _, err := lifecycle.Rollback(ctx, RollbackRequest{
		WorkspaceID: workspaceID, SkillID: skillID, SourceReleaseID: publication.Release.ID,
		Actor: actor, IdempotencyKey: "rollback-loader-failure",
	}); !errors.Is(err, ErrWorkspaceSkillNotFound) {
		t.Fatalf("rollback loader error = %v", err)
	}
	storedOverview, err := lifecycle.store.GetOverview(ctx, workspaceID, skillID, DefaultOverviewLimit)
	if err != nil || len(storedOverview.Releases) != 2 {
		t.Fatalf("rollback loader failure persisted release = (%d, %v)", len(storedOverview.Releases), err)
	}
	skills.current = savedSnapshot

	// A second Skill exercises rejection and stale manual edits without an
	// independent publisher path.
	rejectionSkillID := testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, rejectionSkillID, testUUID())
	rejectionBase := lifecycleBundle(rejectionSkillID, "original")
	skills.current = lifecycleSnapshot(t, workspaceID, userID, rejectionSkillID, rejectionBase)
	ref.TargetSkillID = uuid.UUID(rejectionSkillID.Bytes).String()
	source = lifecycleSignalSource(ref)
	rejectionCandidate := lifecycleBundle(rejectionSkillID, "updated")
	improver.Candidate.Bundle = rejectionCandidate
	lifecycle.signals, _ = newSignalSet([]SignalSource{source})
	config.SkillID = rejectionSkillID
	if _, err := lifecycle.Enable(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	rejectedGeneration, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: rejectionSkillID, RequestedByID: userID, GenerationKey: "generate-reject",
	})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := lifecycle.Reject(ctx, RejectRequest{
		WorkspaceID: workspaceID, ProposalID: rejectedGeneration.Proposal.ID, Actor: actor,
		Reason: "not enough benefit", IdempotencyKey: "reject-human",
	})
	if err != nil || rejected.State != string(ProposalStateRejected) || publisher.calls != 2 {
		t.Fatalf("Reject = (%+v, %v), calls=%d", rejected, err, publisher.calls)
	}
	rejectedReplay, err := lifecycle.Reject(ctx, RejectRequest{
		WorkspaceID: workspaceID, ProposalID: rejectedGeneration.Proposal.ID, Actor: actor,
		Reason: "not enough benefit", IdempotencyKey: "reject-human",
	})
	if err != nil || rejectedReplay.ID != rejected.ID {
		t.Fatalf("idempotent Reject = (%+v, %v)", rejectedReplay, err)
	}
	if _, err := lifecycle.Reject(ctx, RejectRequest{
		WorkspaceID: workspaceID, ProposalID: rejectedGeneration.Proposal.ID, Actor: actor,
		Reason: "different", IdempotencyKey: "reject-human",
	}); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("mismatched Reject replay error = %v", err)
	}

	staleSkillID := testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, staleSkillID, testUUID())
	staleBase := lifecycleBundle(staleSkillID, "original")
	skills.current = lifecycleSnapshot(t, workspaceID, userID, staleSkillID, staleBase)
	ref.TargetSkillID = uuid.UUID(staleSkillID.Bytes).String()
	lifecycle.signals, _ = newSignalSet([]SignalSource{lifecycleSignalSource(ref)})
	improver.Candidate.Bundle = lifecycleBundle(staleSkillID, "updated")
	config.SkillID = staleSkillID
	if _, err := lifecycle.Enable(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	staleGeneration, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: staleSkillID, RequestedByID: userID, GenerationKey: "generate-stale",
	})
	if err != nil {
		t.Fatal(err)
	}
	humanEdit := lifecycleBundle(staleSkillID, "human-edit")
	skills.current = lifecycleSnapshot(t, workspaceID, userID, staleSkillID, humanEdit)
	beforeCalls := publisher.calls
	stalePublication, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: staleGeneration.Proposal.ID, Actor: actor,
		Reason: "reviewed", IdempotencyKey: "publish-stale",
	})
	if !errors.Is(err, ErrStaleBase) || stalePublication.Proposal.State != string(ProposalStateStale) || publisher.calls != beforeCalls ||
		skills.current.Manifest.Hash != lifecycleSnapshot(t, workspaceID, userID, staleSkillID, humanEdit).Manifest.Hash {
		t.Fatalf("stale publish = (%+v, %v), calls=%d", stalePublication, err, publisher.calls)
	}

	if _, err := lifecycle.Pause(ctx, DecisionActor{ID: userID, Kind: ActorKindMachine}, workspaceID, staleSkillID); !errors.Is(err, ErrHumanActorRequired) {
		t.Fatalf("machine pause error = %v, want human actor", err)
	}

	unknownSkillID := testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, unknownSkillID, testUUID())
	unknownBase := lifecycleBundle(unknownSkillID, "original")
	skills.current = lifecycleSnapshot(t, workspaceID, userID, unknownSkillID, unknownBase)
	ref.TargetSkillID = uuid.UUID(unknownSkillID.Bytes).String()
	lifecycle.signals, _ = newSignalSet([]SignalSource{lifecycleSignalSource(ref)})
	improver.Candidate.Bundle = lifecycleBundle(unknownSkillID, "updated")
	config.SkillID = unknownSkillID
	if _, err := lifecycle.Enable(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	unknownGeneration, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: unknownSkillID, RequestedByID: userID, GenerationKey: "generate-unknown",
	})
	if err != nil {
		t.Fatal(err)
	}
	publisher.err = &PublicationUnknownError{ExpectedPostHash: Digest(unknownGeneration.Proposal.CandidateHash.String), Cause: errors.New("ambiguous commit")}
	beforeCalls = publisher.calls
	unknownPublication, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: unknownGeneration.Proposal.ID, Actor: actor,
		Reason: "reviewed", IdempotencyKey: "publish-unknown",
	})
	if !errors.Is(err, ErrPublicationUnknown) || unknownPublication.Proposal.State != string(ProposalStatePublicationUnknown) ||
		unknownPublication.Release.Outcome != string(ReleaseOutcomePublicationUnknown) || publisher.calls != beforeCalls+1 {
		t.Fatalf("unknown Publish = (%+v, %v), calls=%d", unknownPublication, err, publisher.calls)
	}
	unknownCalls := publisher.calls
	if _, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: unknownGeneration.Proposal.ID, Actor: actor,
		Reason: "reviewed", IdempotencyKey: "publish-unknown",
	}); !errors.Is(err, ErrReleaseNotRetryable) || publisher.calls != unknownCalls {
		t.Fatalf("unknown publication retry error = %v, calls=%d", err, publisher.calls)
	}
	publisher.err = nil

	recordingSkillID := testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, recordingSkillID, testUUID())
	recordingBase := lifecycleBundle(recordingSkillID, "original")
	skills.current = lifecycleSnapshot(t, workspaceID, userID, recordingSkillID, recordingBase)
	ref.TargetSkillID = uuid.UUID(recordingSkillID.Bytes).String()
	lifecycle.signals, _ = newSignalSet([]SignalSource{lifecycleSignalSource(ref)})
	improver.Candidate.Bundle = lifecycleBundle(recordingSkillID, "updated")
	config.SkillID = recordingSkillID
	if _, err := lifecycle.Enable(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	recordingGeneration, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: recordingSkillID, RequestedByID: userID, GenerationKey: "generate-recording",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := lifecycle.store.(*Store)
	flaky := &flakyLifecycleStore{Store: store, failSucceededTransition: true}
	lifecycle.store = flaky
	beforeCalls = publisher.calls
	recordingPublication, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: recordingGeneration.Proposal.ID, Actor: actor,
		Reason: "reviewed", IdempotencyKey: "publish-recording",
	})
	if !errors.Is(err, ErrPublicationUnknown) || recordingPublication.Release.Outcome != string(ReleaseOutcomePublicationUnknown) ||
		recordingPublication.Proposal.State != string(ProposalStatePublicationUnknown) ||
		publisher.calls != beforeCalls+1 || flaky.succeededFailures != 1 {
		t.Fatalf("release recording failure = (%+v, %v), calls=%d failures=%d", recordingPublication, err, publisher.calls, flaky.succeededFailures)
	}
	recordingCalls := publisher.calls
	if _, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: recordingGeneration.Proposal.ID, Actor: actor,
		Reason: "reviewed", IdempotencyKey: "publish-recording",
	}); !errors.Is(err, ErrReleaseNotRetryable) || publisher.calls != recordingCalls {
		t.Fatalf("recording failure retry error = %v, calls=%d", err, publisher.calls)
	}
	lifecycle.store = store

	failedSkillID := testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, failedSkillID, testUUID())
	failedBase := lifecycleBundle(failedSkillID, "original")
	skills.current = lifecycleSnapshot(t, workspaceID, userID, failedSkillID, failedBase)
	ref.TargetSkillID = uuid.UUID(failedSkillID.Bytes).String()
	lifecycle.signals, _ = newSignalSet([]SignalSource{lifecycleSignalSource(ref)})
	unsafeCandidate := lifecycleBundle(failedSkillID, "updated")
	unsafeCandidate.Content += "\napi_key = abcdefghijklmnopqrstuvwxyz"
	improver.Candidate.Bundle = unsafeCandidate
	config.SkillID = failedSkillID
	if _, err := lifecycle.Enable(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	beforeCalls = publisher.calls
	beforeReplayCalls := replay.Calls
	failedGeneration, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: failedSkillID, RequestedByID: userID, GenerationKey: "generate-validation-failure",
	})
	if !errors.Is(err, ErrEvaluationFailed) || failedGeneration.Proposal.State != string(ProposalStateFailed) ||
		publisher.calls != beforeCalls || replay.Calls != beforeReplayCalls {
		t.Fatalf("validation failure = (%+v, %v), publisher=%d replay=%d", failedGeneration, err, publisher.calls, replay.Calls)
	}
	failedView, err := lifecycle.ReadProposal(ctx, workspaceID, failedGeneration.Proposal.ID)
	if err != nil || len(failedView.Detail.Evaluations) != 1 ||
		failedView.Detail.Evaluations[0].Result != string(EvaluationResultFailed) || failedView.Candidate != nil {
		t.Fatalf("inspectable validation failure = (%+v, %v)", failedView, err)
	}
}

func TestLifecycleDBBackedDisabledObservePausedAndCooldownGates(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	workspaceID, userID, skillID := testUUID(), testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, skillID, testUUID())
	base := lifecycleBundle(skillID, "original")
	skills := &memorySkillLoader{current: lifecycleSnapshot(t, workspaceID, userID, skillID, base)}
	ref := lifecycleEvidenceRef(workspaceID, skillID, "gates")
	improver := &DeterministicImprover{Candidate: ImprovementCandidate{
		Bundle: lifecycleBundle(skillID, "updated"), ObservedPattern: "pattern", ExpectedBenefit: "benefit",
		RegressionRisk: "risk", EvidenceDigests: []Digest{ref.Digest},
	}}
	replay := &DeterministicReplayEvaluator{Outcome: ReplayOutcome{ReplayResult: ReplayResult{Result: EvaluationResultPassed, SampleCount: 1}}}
	publisher := &memoryPublisher{skills: skills}
	lifecycle, err := NewLifecycle(NewStore(db.New(pool), pool), skills, publisher, improver, replay, lifecycleSignalSource(ref))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	lifecycle.now = func() time.Time { return now }
	actor := DecisionActor{ID: userID, Kind: ActorKindHuman}
	config := LoopConfig{
		WorkspaceID: workspaceID, SkillID: skillID, Enabled: false, Mode: LoopModeObserve,
		Cooldown: time.Hour, MinimumSignals: 1, MaxEvidenceRefs: 5, MaxReplaySamples: 1,
		MaxCostUSDTicks: 10, PolicyVersion: "v1",
	}
	if _, err := lifecycle.Configure(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Observe(ctx, ObserveRequest{WorkspaceID: workspaceID, SkillID: skillID}); !errors.Is(err, ErrEvolutionDisabled) {
		t.Fatalf("disabled Observe error = %v", err)
	}
	if _, err := lifecycle.Enable(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Generate(ctx, GenerateRequest{WorkspaceID: workspaceID, SkillID: skillID, GenerationKey: "observe-only"}); !errors.Is(err, ErrEvolutionObserveOnly) {
		t.Fatalf("observe-mode Generate error = %v", err)
	}
	observation, err := lifecycle.Observe(ctx, ObserveRequest{WorkspaceID: workspaceID, SkillID: skillID})
	if err != nil || len(observation.References) != 1 {
		t.Fatalf("Observe = (%+v, %v)", observation, err)
	}
	config.Enabled = true
	config.Mode = LoopModePropose
	if _, err := lifecycle.Configure(ctx, actor, config); err != nil {
		t.Fatal(err)
	}
	generation, err := lifecycle.Generate(ctx, GenerateRequest{WorkspaceID: workspaceID, SkillID: skillID, GenerationKey: "first"})
	if err != nil || generation.Proposal.State != string(ProposalStateReady) {
		t.Fatalf("propose-mode Generate = (%+v, %v)", generation, err)
	}
	if _, err := lifecycle.Generate(ctx, GenerateRequest{WorkspaceID: workspaceID, SkillID: skillID, GenerationKey: "during-cooldown"}); !errors.Is(err, ErrEvolutionCooldown) {
		t.Fatalf("cooldown Generate error = %v", err)
	}
	paused, err := lifecycle.Pause(ctx, actor, workspaceID, skillID)
	if err != nil || paused.Mode != string(LoopModePaused) {
		t.Fatalf("Pause = (%+v, %v)", paused, err)
	}
	if _, err := lifecycle.Publish(ctx, PublishRequest{
		WorkspaceID: workspaceID, ProposalID: generation.Proposal.ID, Actor: actor,
		Reason: "reviewed", IdempotencyKey: "paused-publish",
	}); !errors.Is(err, ErrEvolutionPaused) || publisher.calls != 0 {
		t.Fatalf("paused Publish error = %v, calls=%d", err, publisher.calls)
	}
}

func TestLifecycleDBBackedDisabledDefaultAndAcceptedRoomRecommendation(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	workspaceID, userID, skillID := testUUID(), testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, skillID, testUUID())
	base := lifecycleBundle(skillID, "original")
	snapshot := lifecycleSnapshot(t, workspaceID, userID, skillID, base)
	skills := &memorySkillLoader{current: snapshot}
	ref := lifecycleEvidenceRef(workspaceID, skillID, "room-accepted")
	ref.Kind = EvidenceKindRoomOutcome
	improver := &DeterministicImprover{}
	replay := &DeterministicReplayEvaluator{Outcome: ReplayOutcome{ReplayResult: ReplayResult{
		Result: EvaluationResultPassed, SampleCount: 1,
	}}}
	lifecycle, err := NewLifecycle(NewStore(db.New(pool), pool), skills, &memoryPublisher{skills: skills}, improver, replay)
	if err != nil {
		t.Fatal(err)
	}
	overview, err := lifecycle.Overview(ctx, workspaceID, skillID, DefaultOverviewLimit)
	if err != nil || overview.Loop != nil || overview.Skill.Manifest.Hash != snapshot.Manifest.Hash || len(overview.Proposals) != 0 {
		t.Fatalf("disabled-default Overview = (%+v, %v)", overview, err)
	}
	actor := DecisionActor{ID: userID, Kind: ActorKindHuman}
	if _, err := lifecycle.Enable(ctx, actor, LoopConfig{
		WorkspaceID: workspaceID, SkillID: skillID, Mode: LoopModePropose, Cooldown: time.Hour,
		MinimumSignals: 1, MaxEvidenceRefs: 5, MaxReplaySamples: 1, MaxCostUSDTicks: 10, PolicyVersion: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	lifecycle.SetImprovementRecommendationSource(recommendationSourceFunc(func(_ context.Context, request RoomRecommendationRequest) (AcceptedImprovementRecommendation, error) {
		return AcceptedImprovementRecommendation{
			WorkspaceID: workspaceID, SkillID: skillID, RecommendationID: request.RecommendationID,
			ExpectedBaseHash: Digest(snapshot.Manifest.Hash), AcceptedByID: userID,
			Candidate: ImprovementCandidate{
				Bundle: lifecycleBundle(skillID, "room-updated"), ObservedPattern: "repeated correction",
				ExpectedBenefit: "clearer output", RegressionRisk: "narrower guidance",
				EvidenceDigests: []Digest{ref.Digest}, CostUSDTicks: 1,
			},
			Evidence: []ResolvedEvidence{{Ref: ref, Payload: []byte(`{"accepted":true}`)}},
		}, nil
	}))
	request := RoomRecommendationRequest{
		WorkspaceID: workspaceID, SkillID: skillID, RecommendationID: uuid.NewString(), IdempotencyKey: "room-proposal",
	}
	generation, err := lifecycle.CreateProposalFromRoomRecommendation(ctx, request)
	if err != nil || generation.Proposal.State != string(ProposalStateReady) || improver.Calls != 0 {
		t.Fatalf("room generation = (%+v, %v), improver calls=%d", generation, err, improver.Calls)
	}
	view, err := lifecycle.ReadProposal(ctx, workspaceID, generation.Proposal.ID)
	if err != nil || view.Rationale == nil || view.Rationale.ObservedPattern != "repeated correction" || len(view.Detail.Evidence) != 1 {
		t.Fatalf("room proposal view = (%+v, %v)", view, err)
	}
	replayed, err := lifecycle.CreateProposalFromRoomRecommendation(ctx, request)
	if err != nil || !replayed.Replayed || replayed.Proposal.ID != generation.Proposal.ID {
		t.Fatalf("room proposal replay = (%+v, %v)", replayed, err)
	}
}

func TestLifecycleCanceledGenerationIsTerminalized(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	workspaceID, userID, skillID := testUUID(), testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, skillID, testUUID())
	base := lifecycleBundle(skillID, "original")
	skills := &memorySkillLoader{current: lifecycleSnapshot(t, workspaceID, userID, skillID, base)}
	ref := lifecycleEvidenceRef(workspaceID, skillID, "cancel-generation")
	ctx, cancel := context.WithCancel(context.Background())
	improver := NewProductionImprover(improvementEngineFunc(func(context.Context, ImprovementRequest) (ImprovementCandidate, error) {
		cancel()
		return ImprovementCandidate{}, context.Canceled
	}), time.Second)
	lifecycle, err := NewLifecycle(NewStore(db.New(pool), pool), skills, &memoryPublisher{skills: skills}, improver,
		&DeterministicReplayEvaluator{}, lifecycleSignalSource(ref))
	if err != nil {
		t.Fatal(err)
	}
	actor := DecisionActor{ID: userID, Kind: ActorKindHuman}
	if _, err := lifecycle.Enable(context.Background(), actor, LoopConfig{
		WorkspaceID: workspaceID, SkillID: skillID, Mode: LoopModePropose, Cooldown: time.Hour,
		MinimumSignals: 1, MaxEvidenceRefs: 5, MaxReplaySamples: 1, MaxCostUSDTicks: 10, PolicyVersion: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	generation, err := lifecycle.Generate(ctx, GenerateRequest{
		WorkspaceID: workspaceID, SkillID: skillID, RequestedByID: userID, GenerationKey: "cancel-running",
	})
	if !errors.Is(err, ErrGenerationFailed) || generation.Proposal.State != string(ProposalStateFailed) {
		t.Fatalf("canceled generation = (%+v, %v)", generation, err)
	}
	persisted, err := lifecycle.store.GetProposal(context.Background(), workspaceID, generation.Proposal.ID)
	if err != nil || persisted.State != string(ProposalStateFailed) {
		t.Fatalf("persisted canceled generation = (%+v, %v)", persisted, err)
	}
}

type replayEngineFunc func(context.Context, ReplayRequest) (ReplayResult, error)

func (f replayEngineFunc) Replay(ctx context.Context, request ReplayRequest) (ReplayResult, error) {
	return f(ctx, request)
}

type improvementEngineFunc func(context.Context, ImprovementRequest) (ImprovementCandidate, error)

func (f improvementEngineFunc) Improve(ctx context.Context, request ImprovementRequest) (ImprovementCandidate, error) {
	return f(ctx, request)
}

type recommendationSourceFunc func(context.Context, RoomRecommendationRequest) (AcceptedImprovementRecommendation, error)

func (f recommendationSourceFunc) LoadAcceptedImprovement(ctx context.Context, request RoomRecommendationRequest) (AcceptedImprovementRecommendation, error) {
	return f(ctx, request)
}

type flakyLifecycleStore struct {
	*Store
	failSucceededTransition bool
	succeededFailures       int
}

func (s *flakyLifecycleStore) TransitionRelease(ctx context.Context, input ReleaseTransition) (db.SkillEvolutionRelease, error) {
	if s.failSucceededTransition && input.NextOutcome == ReleaseOutcomeSucceeded {
		s.failSucceededTransition = false
		s.succeededFailures++
		return db.SkillEvolutionRelease{}, errors.New("release ledger unavailable")
	}
	return s.Store.TransitionRelease(ctx, input)
}

type memorySkillLoader struct {
	current WorkspaceSkillSnapshot
}

func (l *memorySkillLoader) Load(_ context.Context, workspaceID, skillID pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	if l == nil || l.current.Skill.WorkspaceID != workspaceID || l.current.Skill.ID != skillID {
		return WorkspaceSkillSnapshot{}, ErrWorkspaceSkillNotFound
	}
	return l.current, nil
}

type memoryPublisher struct {
	skills *memorySkillLoader
	calls  int
	err    error
}

func (p *memoryPublisher) Publish(_ context.Context, request PublishSkillRequest) (PublishSkillResult, error) {
	if p == nil || p.skills == nil {
		return PublishSkillResult{}, ErrPublisherTransactionsRequired
	}
	p.calls++
	if p.err != nil {
		return PublishSkillResult{}, p.err
	}
	current := p.skills.current
	if Digest(current.Manifest.Hash) != request.ExpectedBaseHash {
		return PublishSkillResult{}, &StaleBaseError{Expected: request.ExpectedBaseHash, Current: Digest(current.Manifest.Hash)}
	}
	updated := lifecycleSnapshot(nil, current.Skill.WorkspaceID, current.Skill.CreatedBy, current.Skill.ID, request.Bundle)
	p.skills.current = updated
	return PublishSkillResult{Snapshot: updated, PreHash: Digest(current.Manifest.Hash), PostHash: Digest(updated.Manifest.Hash)}, nil
}

func lifecycleSignalSource(ref EvidenceRef) SignalSource {
	return NewSignalAdapter(ref.Kind,
		func(context.Context, SignalQuery) ([]EvidenceRef, error) { return []EvidenceRef{ref}, nil },
		func(_ context.Context, _ SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			return ResolvedEvidence{Ref: expected, Payload: []byte(`{"correction":"be precise"}`)}, nil
		},
	)
}

func lifecycleEvidenceRef(workspaceID, skillID pgtype.UUID, seed string) EvidenceRef {
	return EvidenceRef{
		WorkspaceID: uuid.UUID(workspaceID.Bytes).String(), Kind: EvidenceKindTaskReview,
		SourceID: uuid.NewSHA1(uuid.Nil, []byte(seed)).String(), TargetSkillID: uuid.UUID(skillID.Bytes).String(),
		SourceState: "needs_correction", Digest: testDigest(seed), Eligibility: EvidenceEligibilityEligible,
		ObservedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC),
	}
}

func lifecycleBundle(skillID pgtype.UUID, body string) skillbundle.Skill {
	content := "---\nname: deploy\ndescription: Deploy safely\n---\n\n" + body
	return skillbundle.Skill{
		ID: uuid.UUID(skillID.Bytes).String(), Source: skillbundle.SourceWorkspace,
		Name: "deploy", Description: "Deploy safely", Content: content,
		Files: []skillbundle.File{{Path: "references/checklist.md", Content: "verify output"}},
	}
}

func lifecycleSnapshot(t *testing.T, workspaceID, creatorID, skillID pgtype.UUID, bundle skillbundle.Skill) WorkspaceSkillSnapshot {
	manifest, err := skillbundle.BuildValidatedManifest(bundle)
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		panic(err)
	}
	return WorkspaceSkillSnapshot{
		Skill:     db.Skill{ID: skillID, WorkspaceID: workspaceID, CreatedBy: creatorID},
		Ownership: workspaceOwnership(), Bundle: cloneSkillBundle(bundle), Manifest: manifest,
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
