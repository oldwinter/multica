package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/room"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/llm"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

type acceptedOutcomesStub struct {
	ref      room.AcceptedOutcomeRef
	evidence room.AcceptedOutcomeEvidence
}

func (stub *acceptedOutcomesStub) ListAcceptedOutcomeRefs(context.Context, pgtype.UUID, int) ([]room.AcceptedOutcomeRef, error) {
	return []room.AcceptedOutcomeRef{stub.ref}, nil
}

func (stub *acceptedOutcomesStub) LoadAcceptedOutcomeEvidence(context.Context, pgtype.UUID, room.AcceptedOutcomeRef) (room.AcceptedOutcomeEvidence, error) {
	return stub.evidence, nil
}

type roomMetadataStub struct {
	value     db.Room
	artifacts []db.RoomArtifact
	review    db.RoomRecommendationReview
}

func (stub roomMetadataStub) GetRoom(context.Context, db.GetRoomParams) (db.Room, error) {
	return stub.value, nil
}

func (stub roomMetadataStub) ListRoomArtifacts(context.Context, db.ListRoomArtifactsParams) ([]db.RoomArtifact, error) {
	return append([]db.RoomArtifact(nil), stub.artifacts...), nil
}

func (stub roomMetadataStub) GetRoomRecommendationReview(context.Context, db.GetRoomRecommendationReviewParams) (db.RoomRecommendationReview, error) {
	return stub.review, nil
}

type proposalProcessorStub struct {
	artifact db.RoomArtifact
	result   Generation
	calls    int
}

func (stub *proposalProcessorStub) ProcessRoomArtifactTarget(_ context.Context, artifact db.RoomArtifact) (Generation, error) {
	stub.calls++
	stub.artifact = artifact
	return stub.result, nil
}

type improvementRoomQueuerStub struct {
	roomID pgtype.UUID
	calls  int
	keys   []string
}

func (stub *improvementRoomQueuerStub) EnsureImprovementRoom(
	_ context.Context,
	_, _, _ pgtype.UUID,
	key string,
) (ImprovementRoomQueueResult, error) {
	stub.calls++
	stub.keys = append(stub.keys, key)
	return ImprovementRoomQueueResult{RoomID: stub.roomID, EligibleSignals: 2}, nil
}

func TestRoomCandidateSourceStrictlyLoadsAcceptedImprovement(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	request := RoomRecommendationRequest{
		WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID,
		RecommendationID: improvementRecommendationID(fixture.outcomes.ref), IdempotencyKey: "accepted-1",
	}
	accepted, err := fixture.source.LoadAcceptedImprovement(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.ExpectedBaseHash != fixture.baseHash || len(accepted.Evidence) != 1 || accepted.Candidate.Bundle.ID != uuidText(fixture.skillID) {
		t.Fatalf("accepted = %+v", accepted)
	}
	fixture.outcomes.evidence.Ref.CycleID = testUUID()
	if _, err := fixture.source.LoadAcceptedImprovement(context.Background(), request); !errors.Is(err, ErrRoomCandidateSourceDrift) {
		t.Fatalf("changed accepted ref error = %v", err)
	}
	fixture.outcomes.evidence.Ref = fixture.outcomes.ref

	var envelope map[string]any
	if err := json.Unmarshal([]byte(fixture.outcomes.evidence.Body), &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["unexpected"] = true
	raw, _ := json.Marshal(envelope)
	fixture.outcomes.evidence.Body = string(raw)
	if _, err := fixture.source.LoadAcceptedImprovement(context.Background(), request); !errors.Is(err, ErrRoomCandidateInvalid) {
		t.Fatalf("unknown-field error = %v", err)
	}
}

func TestRoomCandidateEngineUsesOnlyBoundEvidenceAndReplayFailsClosed(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	roomRef, err := roomOutcomeEvidenceRef(fixture.outcomes.ref)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(roomOutcomePayload{
		RoomID: uuidText(fixture.outcomes.ref.RoomID), MemoryRevisionID: uuidText(fixture.outcomes.ref.MemoryRevisionID),
		CycleID: uuidText(fixture.outcomes.ref.CycleID), RecommendationKey: fixture.outcomes.ref.RecommendationKey,
		RecommendationKind: fixture.outcomes.ref.RecommendationKind, Body: fixture.outcomes.evidence.Body,
	})
	candidate, err := fixture.source.Improve(context.Background(), ImprovementRequest{
		Base: fixture.base, Evidence: []ResolvedEvidence{{Ref: fixture.signalRef, Payload: []byte(`{"reason":"bounded"}`)}, {Ref: roomRef, Payload: payload}},
		PolicyVersion: "v1", MaxCostUSDTicks: 1, MaxChangedFiles: 2, MaxPrimaryGrowth: 1024,
	})
	if err != nil || candidate.Bundle.Content == fixture.base.Content {
		t.Fatalf("improve = (%+v, %v)", candidate, err)
	}
	replay, err := fixture.source.Replay(context.Background(), ReplayRequest{
		Base: fixture.base, Candidate: candidate.Bundle,
		Evidence: []ResolvedEvidence{{Ref: fixture.signalRef, Payload: []byte(`{"outcome":"needs_correction","correction":"add the focused check","reason":"bounded"}`)}},
		Limits:   ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 1, PolicyVersion: "v1"},
	})
	if err != nil || replay.Result != EvaluationResultUnknown || replay.SampleCount != 1 || replay.ReasonCode != "behavioral_runner_unavailable" {
		t.Fatalf("replay = (%+v, %v)", replay, err)
	}
	replay, err = fixture.source.Replay(context.Background(), ReplayRequest{
		Base: fixture.base, Candidate: candidate.Bundle,
		Evidence: []ResolvedEvidence{{Ref: fixture.signalRef, Payload: []byte(`{"outcome":"helpful","reason":"mismatched state"}`)}},
		Limits:   ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 1, PolicyVersion: "v1"},
	})
	if err != nil || replay.Result != EvaluationResultFailed || replay.ReasonCode != "evidence_revalidation_failed" {
		t.Fatalf("semantic mismatch replay = (%+v, %v)", replay, err)
	}
	candidate.Bundle.Files = append(candidate.Bundle.Files, skillbundle.File{Path: "../escape", Content: "x"})
	replay, err = fixture.source.Replay(context.Background(), ReplayRequest{
		Base: fixture.base, Candidate: candidate.Bundle, Evidence: []ResolvedEvidence{{Ref: fixture.signalRef}},
		Limits: ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 1, PolicyVersion: "v1"},
	})
	if err != nil || replay.Result != EvaluationResultFailed {
		t.Fatalf("unsafe replay = (%+v, %v)", replay, err)
	}
}

func TestProductionAcceptedCurrentRoomRecommendationBecomesReadyAndPublishable(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	fixture := newRoomCandidateFixture(t)
	actorID := fixture.outcomes.evidence.ReviewedByUserID
	seedPersistenceFixture(t, pool, fixture.workspaceID, actorID, fixture.skillID, testUUID())
	skills := &memorySkillLoader{current: lifecycleSnapshot(t, fixture.workspaceID, actorID, fixture.skillID, fixture.base)}
	client := &replayJSONClientStub{enabled: true, responses: []llm.JSONGeneration{
		{Content: `{"response":"base missed the focused check"}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"response":"candidate performs the focused check"}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"winner":"candidate","base_pass":false,"candidate_pass":true}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"winner":"candidate","base_pass":false,"candidate_pass":true}`, PromptTokens: 1, CompletionTokens: 1},
	}}
	lifecycle, err := NewLifecycle(
		NewStore(db.New(pool), pool), skills, &memoryPublisher{skills: skills},
		NewProductionImprover(fixture.source, DefaultReplayTimeout), newProductionBehavioralEvaluator(client),
	)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle.SetImprovementRecommendationSource(fixture.source)
	actor := DecisionActor{ID: actorID, Kind: ActorKindHuman}
	if _, err := lifecycle.Enable(context.Background(), actor, LoopConfig{
		WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID, Mode: LoopModePropose,
		Cooldown: time.Hour, MinimumSignals: 1, MaxEvidenceRefs: 5, MaxReplaySamples: 1,
		MaxCostUSDTicks: 2_000_000, PolicyVersion: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	request := RoomRecommendationRequest{
		WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID,
		RecommendationID: improvementRecommendationID(fixture.outcomes.ref), IdempotencyKey: "production-room-ready",
	}
	generation, err := lifecycle.CreateProposalFromRoomRecommendation(context.Background(), request)
	if err != nil || generation.Proposal.State != string(ProposalStateReady) || generation.Replay.Result != string(EvaluationResultPassed) {
		t.Fatalf("production Room generation = (%+v, %v)", generation, err)
	}
	publication, err := lifecycle.Publish(context.Background(), PublishRequest{
		WorkspaceID: fixture.workspaceID, ProposalID: generation.Proposal.ID, Actor: actor,
		Reason: "bounded production replay passed", IdempotencyKey: "publish-production-room-ready",
	})
	if err != nil || publication.Release.Outcome != string(ReleaseOutcomeSucceeded) ||
		publication.Result.PostHash != Digest(generation.Proposal.CandidateHash.String) {
		t.Fatalf("publish production Room proposal = (%+v, %v)", publication, err)
	}
}

func TestRoomProposalRequesterNeverFallsBackToGenericGeneration(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	processor := &proposalProcessorStub{}
	requester := NewRoomProposalRequester(fixture.source, processor)
	_, err := requester.RequestProposal(context.Background(), fixture.workspaceID, fixture.skillID, pgtype.UUID{}, "scheduled-1")
	if err != nil {
		t.Fatal(err)
	}
	if processor.artifact.ID != fixture.metadata.artifacts[0].ID || processor.artifact.IdempotencyKey != "promotion-1" {
		t.Fatalf("recovered artifact = %+v", processor.artifact)
	}
	fixture.outcomes.ref.RecommendationKind = string(room.RecommendationTargetKnowledge)
	if _, err := requester.RequestProposal(context.Background(), fixture.workspaceID, fixture.skillID, pgtype.UUID{}, "scheduled-2"); !errors.Is(err, ErrRoomCandidateNotReady) {
		t.Fatalf("no-candidate error = %v", err)
	}
}

func TestRoomProposalRequesterQueuesVisibleRoomBeforeAnArtifactExists(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	fixture.outcomes.ref.RecommendationKind = string(room.RecommendationTargetKnowledge)
	processor := &proposalProcessorStub{}
	queuer := &improvementRoomQueuerStub{roomID: testUUID()}
	requester := NewRoomProposalRequester(fixture.source, processor, queuer)

	for range 2 {
		result, err := requester.RequestProposal(context.Background(), fixture.workspaceID, fixture.skillID, testUUID(), "request-1")
		if err != nil {
			t.Fatal(err)
		}
		if result.State != "improvement_room_queued" || result.RoomID != queuer.roomID || result.Generation != nil || result.EligibleSignals != 2 {
			t.Fatalf("queued result = %+v", result)
		}
	}
	if processor.calls != 0 || queuer.calls != 2 || queuer.keys[0] != queuer.keys[1] {
		t.Fatalf("processor calls=%d queuer=%+v", processor.calls, queuer)
	}
}

type roomCandidateFixture struct {
	workspaceID pgtype.UUID
	skillID     pgtype.UUID
	baseHash    Digest
	base        skillbundle.Skill
	signalRef   EvidenceRef
	outcomes    *acceptedOutcomesStub
	metadata    *roomMetadataStub
	source      *RoomCandidateSource
}

func newRoomCandidateFixture(t *testing.T) roomCandidateFixture {
	t.Helper()
	workspaceID, skillID, actorID := testUUID(), testUUID(), testUUID()
	base := skillbundle.Skill{ID: uuidText(skillID), Source: skillbundle.SourceWorkspace, Name: "focused-review", Description: "Review one bounded change.",
		Content: "---\nname: focused-review\ndescription: Review one bounded change.\n---\n\nOriginal."}
	baseManifest, err := skillbundle.BuildValidatedManifest(base)
	if err != nil {
		t.Fatal(err)
	}
	baseHash := Digest(baseManifest.Hash)
	signalRef := EvidenceRef{WorkspaceID: uuidText(workspaceID), Kind: EvidenceKindTaskReview, SourceID: uuidText(testUUID()),
		SourceRevisionID: uuidText(testUUID()), TargetSkillID: uuidText(skillID), SourceState: "needs_correction",
		Digest: testDigest("candidate-evidence"), Eligibility: EvidenceEligibilityEligible, ObservedAt: time.Now().UTC()}
	signal := NewSignalAdapter(EvidenceKindTaskReview,
		func(context.Context, SignalQuery) ([]EvidenceRef, error) { return []EvidenceRef{signalRef}, nil },
		func(context.Context, SignalQuery, EvidenceRef) (ResolvedEvidence, error) {
			return ResolvedEvidence{Ref: signalRef, Payload: []byte(`{"outcome":"needs_correction","correction":"add the focused check","reason":"bounded"}`)}, nil
		})
	envelope := roomCandidateEnvelope{SchemaVersion: 1, BaseSkillID: uuidText(skillID), BaseHash: string(baseHash),
		Bundle: roomCandidateBundle{ID: uuidText(skillID), Source: skillbundle.SourceWorkspace, Name: base.Name, Description: base.Description,
			Content: strings.Replace(base.Content, "Original.", "Updated with the focused check.", 1), Files: []roomCandidateFile{}},
		ObservedPattern: "reviews repeatedly miss the focused check", ExpectedBenefit: "the procedure makes the check explicit",
		RegressionRisk: "the added check may be too strict", EvidenceDigests: []string{string(signalRef.Digest)}}
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	ref := room.AcceptedOutcomeRef{WorkspaceID: workspaceID, RoomID: testUUID(), MemoryRevisionID: testUUID(), CycleID: testUUID(),
		RecommendationKey: string(testDigest("recommendation")), RecommendationKind: string(room.RecommendationTargetExecutableProcedure),
		SourceState: "accepted_current", Digest: string(testDigest("room-outcome")), ObservedAt: time.Now().UTC()}
	outcomes := &acceptedOutcomesStub{ref: ref, evidence: room.AcceptedOutcomeEvidence{Ref: ref, ReviewedByUserID: actorID, Body: string(body)}}
	artifact := db.RoomArtifact{
		ID: testUUID(), WorkspaceID: workspaceID, RoomID: ref.RoomID, Kind: string(room.RecommendationTargetExecutableProcedure),
		IdempotencyKey: "promotion-1", TargetID: testUUID(), Title: "Focused improvement", Body: string(body),
		Rationale: pgtype.Text{String: "accepted rationale", Valid: true}, CreatedByUserID: actorID,
		MemoryRevisionID: ref.MemoryRevisionID, RecommendationKey: pgtype.Text{String: ref.RecommendationKey, Valid: true},
	}
	artifact.SourceDigest = roomArtifactSourceDigest(artifact)
	metadata := &roomMetadataStub{
		value:     db.Room{ID: ref.RoomID, WorkspaceID: workspaceID, TemplateID: pgtype.Text{String: improvementRoomTemplateID, Valid: true}},
		artifacts: []db.RoomArtifact{artifact},
		review: db.RoomRecommendationReview{ID: testUUID(), WorkspaceID: workspaceID, RoomID: ref.RoomID,
			MemoryRevisionID: ref.MemoryRevisionID, RecommendationKey: ref.RecommendationKey, Status: "approved",
			ArtifactID: artifact.ID, ReviewedByUserID: actorID, ReviewedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}},
	}
	source, err := NewRoomCandidateSource(outcomes, metadata, signal)
	if err != nil {
		t.Fatal(err)
	}
	return roomCandidateFixture{workspaceID: workspaceID, skillID: skillID, baseHash: baseHash, base: base, signalRef: signalRef, outcomes: outcomes, metadata: metadata, source: source}
}
