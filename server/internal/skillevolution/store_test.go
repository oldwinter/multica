package skillevolution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestValidateRevisionInputRequiresCanonicalBundle(t *testing.T) {
	skillID := testUUID()
	bundle := skillbundle.Skill{
		ID: uuid.UUID(skillID.Bytes).String(), Source: skillbundle.SourceWorkspace,
		Name: "deploy", Description: "Deploy safely", Content: "primary",
		Files: []skillbundle.File{{Path: "refs/checklist.md", Content: "check"}},
	}
	manifest, err := skillbundle.BuildValidatedManifest(bundle)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := CanonicalEvidenceDigest("revision_metadata", []DigestPart{{Key: "name", Value: bundle.Name}})
	if err != nil {
		t.Fatal(err)
	}
	input := RevisionInput{
		WorkspaceID: testUUID(), SkillID: skillID, Kind: "candidate", Ownership: OwnershipWorkspace,
		Source: bundle.Source, BundleHash: Digest(manifest.Hash), MetadataDigest: metadata,
		Name: bundle.Name, Description: bundle.Description, PrimaryContent: bundle.Content,
		Files: []RevisionFileInput{{Path: bundle.Files[0].Path, Content: bundle.Files[0].Content}},
	}
	if _, _, err := validateRevisionInput(input); err != nil {
		t.Fatalf("valid revision rejected: %v", err)
	}
	input.BundleHash = testDigest("different")
	if _, _, err := validateRevisionInput(input); !errors.Is(err, ErrPersistenceInvalidInput) {
		t.Fatalf("hash mismatch error = %v, want invalid input", err)
	}
}

func TestStorePersistenceIsolationIdempotencyAndAppendOnlyHistory(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	queries := db.New(pool)
	store := NewStore(queries, pool)

	workspaceA, workspaceB := testUUID(), testUUID()
	userA := testUUID()
	skillA, skillB := testUUID(), testUUID()
	agentA, agentB := testUUID(), testUUID()
	runtimeA := testUUID()
	seedPersistenceFixture(t, pool, workspaceA, userA, skillA, agentA)
	seedPersistenceFixture(t, pool, workspaceB, testUUID(), skillB, agentB)
	fixture := testutil.New(pool, util.UUIDToString(workspaceA), util.UUIDToString(userA))

	loop, err := store.ConfigureLoop(ctx, LoopConfig{
		WorkspaceID: workspaceA, SkillID: skillA, Enabled: true, Mode: LoopModePropose,
		Cooldown: time.Hour, MinimumSignals: 1, MaxEvidenceRefs: 10, MaxReplaySamples: 2,
		MaxCostUSDTicks: 100, PolicyVersion: "v1",
	})
	if err != nil {
		t.Fatalf("ConfigureLoop: %v", err)
	}
	if _, err := store.GetLoop(ctx, workspaceB, skillA); !errors.Is(err, ErrPersistenceNotFound) {
		t.Fatalf("cross-workspace loop read error = %v, want not found", err)
	}

	baseInput := testRevisionInput(t, workspaceA, skillA, "base", "base content")
	baseInput.CreatedByID = userA
	base, err := store.SaveRevision(ctx, baseInput)
	if err != nil {
		t.Fatalf("SaveRevision: %v", err)
	}
	replayed, err := store.SaveRevision(ctx, baseInput)
	if err != nil || replayed.Revision.ID != base.Revision.ID {
		t.Fatalf("idempotent SaveRevision = (%v, %v), want same id", replayed.Revision.ID, err)
	}
	conflicting := baseInput
	conflicting.Kind = "release"
	if _, err := store.SaveRevision(ctx, conflicting); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("conflicting immutable revision error = %v, want conflict", err)
	}
	if _, err := store.GetRevisionSnapshot(ctx, workspaceB, base.Revision.ID); !errors.Is(err, ErrPersistenceNotFound) {
		t.Fatalf("cross-workspace revision read error = %v, want not found", err)
	}

	proposalInput := ProposalInput{
		WorkspaceID: workspaceA, SkillID: skillA, LoopID: loop.ID,
		BaseRevisionID: base.Revision.ID, BaseHash: baseInput.BundleHash,
		GenerationKey: "generation-1", RequestedByID: userA,
	}
	proposal, err := store.CreateProposal(ctx, proposalInput)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	idempotent, err := store.CreateProposal(ctx, proposalInput)
	if err != nil || idempotent.ID != proposal.ID {
		t.Fatalf("idempotent CreateProposal = (%v, %v), want same id", idempotent.ID, err)
	}
	changedRequester := proposalInput
	changedRequester.RequestedByID = pgtype.UUID{}
	if _, err := store.CreateProposal(ctx, changedRequester); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed proposal requester error = %v, want conflict", err)
	}
	concurrent := proposalInput
	concurrent.GenerationKey = "generation-2"
	if _, err := store.CreateProposal(ctx, concurrent); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("overlapping generation error = %v, want conflict", err)
	}

	proposal, err = store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: workspaceA, ProposalID: proposal.ID,
		ExpectedState: ProposalStateQueued, NextState: ProposalStateRunning,
	})
	if err != nil {
		t.Fatalf("start proposal: %v", err)
	}
	if _, err := store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: workspaceA, ProposalID: proposal.ID,
		ExpectedState: ProposalStateQueued, NextState: ProposalStateRunning,
	}); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("stale proposal CAS error = %v, want conflict", err)
	}

	candidateInput := testRevisionInput(t, workspaceA, skillA, "candidate", "candidate content")
	candidateInput.CreatedByID = userA
	candidate, err := store.SaveRevision(ctx, candidateInput)
	if err != nil {
		t.Fatalf("save candidate: %v", err)
	}
	rationale := ImprovementRationale{
		ObservedPattern: "deployment corrections repeat", ExpectedBenefit: "deployments become consistent", RegressionRisk: "stricter checks may slow a run",
	}
	rationaleDigest, err := improvementRationaleDigest(ImprovementCandidate{
		ObservedPattern: rationale.ObservedPattern, ExpectedBenefit: rationale.ExpectedBenefit,
		RegressionRisk: rationale.RegressionRisk, EvidenceDigests: []Digest{testDigest("evidence")},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err = store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: workspaceA, ProposalID: proposal.ID, ExpectedState: ProposalStateRunning,
		NextState: ProposalStateReady, CandidateRevisionID: candidate.Revision.ID,
		CandidateHash: candidateInput.BundleHash, RationaleDigest: rationaleDigest,
		ObservedPattern: rationale.ObservedPattern, ExpectedBenefit: rationale.ExpectedBenefit, RegressionRisk: rationale.RegressionRisk,
	})
	if err != nil {
		t.Fatalf("ready proposal: %v", err)
	}

	ref := EvidenceRef{
		WorkspaceID: uuid.UUID(workspaceA.Bytes).String(), Kind: EvidenceKindManualRerun,
		SourceID: uuid.NewString(), TargetSkillID: uuid.UUID(skillA.Bytes).String(),
		SourceState: "completed", Digest: testDigest("evidence"),
		Eligibility: EvidenceEligibilityEligible, ObservedAt: time.Now().UTC(),
	}
	evidence, err := store.RecordEvidence(ctx, proposal.ID, ref)
	if err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}
	replayedEvidence, err := store.RecordEvidence(ctx, proposal.ID, ref)
	if err != nil || replayedEvidence.ID != evidence.ID {
		t.Fatalf("idempotent evidence = (%v, %v), want same id", replayedEvidence.ID, err)
	}
	changedRef := ref
	changedRef.Digest = testDigest("changed")
	if _, err := store.RecordEvidence(ctx, proposal.ID, changedRef); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed evidence error = %v, want conflict", err)
	}
	changedObservedAt := ref
	changedObservedAt.ObservedAt = changedObservedAt.ObservedAt.Add(time.Second)
	if _, err := store.RecordEvidence(ctx, proposal.ID, changedObservedAt); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed evidence observation error = %v, want conflict", err)
	}

	evaluationInput := EvaluationInput{
		WorkspaceID: workspaceA, ProposalID: proposal.ID, Kind: "deterministic_validation",
		Result: EvaluationResultPassed, Adapter: "validator", AdapterVersion: "v1", PolicyVersion: "v1",
		ResultDigest: testDigest("evaluation"), SafeMetrics: []byte(`{"checks":2}`), Duration: time.Second,
		IdempotencyKey: "evaluation-1",
	}
	evaluation, err := store.RecordEvaluation(ctx, evaluationInput)
	if err != nil {
		t.Fatalf("RecordEvaluation: %v", err)
	}
	replayedEvaluation, err := store.RecordEvaluation(ctx, evaluationInput)
	if err != nil || replayedEvaluation.ID != evaluation.ID {
		t.Fatalf("idempotent evaluation = (%v, %v), want same id", replayedEvaluation.ID, err)
	}
	changedEvaluation := evaluationInput
	changedEvaluation.AdapterVersion = "v2"
	if _, err := store.RecordEvaluation(ctx, changedEvaluation); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed evaluation error = %v, want conflict", err)
	}

	if _, err := store.TransitionProposal(ctx, ProposalTransition{
		WorkspaceID: workspaceA, ProposalID: proposal.ID, ExpectedState: ProposalStateReady,
		NextState: ProposalStatePublishing,
	}); err != nil {
		t.Fatalf("start proposal publication: %v", err)
	}
	releaseInput := ReleaseInput{
		WorkspaceID: workspaceA, SkillID: skillA, ProposalID: proposal.ID, RevisionID: candidate.Revision.ID,
		Kind: ReleaseKindPublish, ExpectedBaseHash: baseInput.BundleHash, ActorID: userA, IdempotencyKey: "release-1",
	}
	if _, err := store.CreateRelease(ctx, releaseInput); !errors.Is(err, ErrPersistenceNotFound) {
		t.Fatalf("release without review error = %v, want not found", err)
	}
	reviewInput := ReviewInput{
		WorkspaceID: workspaceA, ProposalID: proposal.ID, CandidateRevisionID: candidate.Revision.ID,
		Decision: "rejected", ActorID: userA, Reason: "needs more proof", IdempotencyKey: "review-rejected",
	}
	review, err := store.RecordReview(ctx, reviewInput)
	if err != nil {
		t.Fatalf("RecordReview: %v", err)
	}
	replayedReview, err := store.RecordReview(ctx, reviewInput)
	if err != nil || replayedReview.ID != review.ID {
		t.Fatalf("idempotent review = (%v, %v), want same id", replayedReview.ID, err)
	}
	changedReview := reviewInput
	changedReview.Decision = "publish"
	if _, err := store.RecordReview(ctx, changedReview); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed review error = %v, want conflict", err)
	}
	if _, err := store.CreateRelease(ctx, releaseInput); !errors.Is(err, ErrPersistenceNotFound) {
		t.Fatalf("release with rejected-only review error = %v, want not found", err)
	}
	publishReview := reviewInput
	publishReview.Decision = "publish"
	publishReview.Reason = "evidence verified"
	publishReview.IdempotencyKey = "review-publish"
	if _, err := store.RecordReview(ctx, publishReview); err != nil {
		t.Fatalf("RecordReview publish: %v", err)
	}
	release, err := store.CreateRelease(ctx, releaseInput)
	if err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}
	replayedRelease, err := store.CreateRelease(ctx, releaseInput)
	if err != nil || replayedRelease.ID != release.ID {
		t.Fatalf("idempotent release = (%v, %v), want same id", replayedRelease.ID, err)
	}
	changedRelease := releaseInput
	changedRelease.ActorID = testUUID()
	if _, err := store.CreateRelease(ctx, changedRelease); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed release actor error = %v, want conflict", err)
	}
	succeededRelease, err := store.TransitionRelease(ctx, ReleaseTransition{
		WorkspaceID: workspaceA, ReleaseID: release.ID, ExpectedOutcome: ReleaseOutcomePending,
		NextOutcome: ReleaseOutcomeSucceeded, PreHash: baseInput.BundleHash, PostHash: candidateInput.BundleHash,
	})
	if err != nil {
		t.Fatalf("complete release: %v", err)
	}
	rollbackInput := ReleaseInput{
		WorkspaceID: workspaceA, SkillID: skillA, SourceReleaseID: succeededRelease.ID, RevisionID: base.Revision.ID,
		Kind: ReleaseKindRollback, ExpectedBaseHash: candidateInput.BundleHash, ActorID: userA, IdempotencyKey: "rollback-1",
	}
	rollback, err := store.CreateRelease(ctx, rollbackInput)
	if err != nil {
		t.Fatalf("CreateRelease rollback: %v", err)
	}
	if _, err := store.TransitionRelease(ctx, ReleaseTransition{
		WorkspaceID: workspaceA, ReleaseID: rollback.ID, ExpectedOutcome: ReleaseOutcomePending,
		NextOutcome: ReleaseOutcomeSucceeded, PreHash: candidateInput.BundleHash, PostHash: baseInput.BundleHash,
	}); err != nil {
		t.Fatalf("complete rollback: %v", err)
	}
	detail, err := store.GetProposalDetail(ctx, workspaceA, proposal.ID)
	if err != nil || len(detail.Evidence) != 1 || len(detail.Reviews) != 2 || detail.Rationale == nil || detail.Rationale.ObservedPattern != rationale.ObservedPattern {
		t.Fatalf("proposal detail = (%+v, %v), want one evidence and append-only review decisions", detail, err)
	}

	taskID := testUUID()
	fixture.Insert(t, "agent_runtime", testutil.Cols{"id": runtimeA, "workspace_id": workspaceA})
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	fixture.Insert(t, "agent_task_queue", testutil.Cols{
		"id": taskID, "agent_id": agentA, "runtime_id": runtimeA,
		"status": "dispatched", "dispatched_at": dispatchedAt,
	})
	snapshot, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
		WorkspaceID: workspaceA, TaskID: taskID, AgentID: agentA, RuntimeID: runtimeA, TaskDispatchedAt: dispatchedAt,
		Skills: []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillA)}},
	})
	if err != nil {
		t.Fatalf("RecordTaskDispatchSnapshot: %v", err)
	}
	if replayed, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
		WorkspaceID: workspaceA, TaskID: taskID, AgentID: agentA, RuntimeID: runtimeA, TaskDispatchedAt: dispatchedAt,
		Skills: []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillA)}},
	}); err != nil || replayed.Snapshot.ID != snapshot.Snapshot.ID {
		t.Fatalf("idempotent task dispatch snapshot = (%+v, %v)", replayed, err)
	}
	if got := string(snapshot.Snapshot.Identities); !jsonEqual(snapshot.Snapshot.Identities, []byte(`[{"source":"workspace","skill_id":"`+util.UUIDToString(skillA)+`"}]`)) ||
		strings.Contains(got, "hash") || snapshot.Snapshot.IdentityCount != 1 || snapshot.Snapshot.ContractVersion != TaskDispatchContractVersion {
		t.Fatalf("content-free dispatch snapshot = %q count=%d version=%d", got, snapshot.Snapshot.IdentityCount, snapshot.Snapshot.ContractVersion)
	}
	changedSnapshot := TaskDispatchSnapshotInput{
		WorkspaceID: workspaceA, TaskID: taskID, AgentID: agentA, RuntimeID: runtimeA, TaskDispatchedAt: dispatchedAt,
		Skills: []DispatchSkillIdentity{{Source: skillbundle.SourceBuiltin, SkillID: "plan"}},
	}
	if _, err := store.RecordTaskDispatchSnapshot(ctx, changedSnapshot); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed dispatch snapshot error = %v, want conflict", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, taskID); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	attributionInput := TaskAttributionInput{
		WorkspaceID: workspaceA, TaskID: taskID, RuntimeID: runtimeA, SkillID: skillA,
		RevisionID: candidate.Revision.ID, ManifestVersion: 1, Source: skillbundle.SourceWorkspace,
		BundleHash: candidateInput.BundleHash, ManifestDigest: testDigest("manifest"),
		Eligibility: EvidenceEligibilityEligible, Reason: "matched_revision",
		DispatchSnapshotID: snapshot.Snapshot.ID, TaskDispatchedAt: dispatchedAt,
	}
	attribution, err := store.RecordTaskAttribution(ctx, attributionInput)
	if err != nil {
		t.Fatalf("RecordTaskAttribution: %v", err)
	}
	replayedAttribution, err := store.RecordTaskAttribution(ctx, attributionInput)
	if err != nil || replayedAttribution.ID != attribution.ID {
		t.Fatalf("idempotent attribution = (%v, %v), want same id", replayedAttribution.ID, err)
	}
	changedAttribution := attributionInput
	changedAttribution.Source = skillbundle.SourceBuiltin
	if _, err := store.RecordTaskAttribution(ctx, changedAttribution); !errors.Is(err, ErrPersistenceConflict) {
		t.Fatalf("changed attribution source error = %v, want conflict", err)
	}
	coverageMetrics := NewMetrics()
	coverage := &exactFeedbackCoverageRecorder{queries: queries, metrics: coverageMetrics}
	reviewerB, reviewerC := testUUID(), testUUID()
	for _, reviewerID := range []pgtype.UUID{reviewerB, reviewerC} {
		fixture.InsertNoID(t, "member", testutil.Cols{
			"workspace_id": workspaceA, "user_id": reviewerID,
		}, "workspace_id = $1 AND user_id = $2", workspaceA, reviewerID)
	}
	reviewRepository := service.NewDBTaskRunReviewRepository(queries)
	createCoveredReview := func(reviewerID pgtype.UUID, key string, recorder service.TaskRunReviewCoverageRecorder) {
		t.Helper()
		reviews := service.NewTaskRunReviewService(reviewRepository, persistenceTaskReviewAccess{
			workspaceID: util.UUIDToString(workspaceA), reviewerID: util.UUIDToString(reviewerID),
			taskID: util.UUIDToString(taskID), agentID: util.UUIDToString(agentA),
		})
		reviews.SetCoverageRecorder(recorder)
		if _, err := reviews.CreateTaskRunReview(ctx, util.UUIDToString(workspaceA), util.UUIDToString(reviewerID), service.CreateTaskRunReviewInput{
			TaskID: util.UUIDToString(taskID), IdempotencyKey: key,
			Outcome: service.TaskRunReviewOutcomeHelpful, Target: service.TaskRunReviewTargetKnowledge,
			Reason: "covered review",
		}); err != nil {
			t.Fatalf("create covered review %q: %v", key, err)
		}
	}
	createCoveredReview(userA, "coverage:first-review", coverage)
	// An idempotent replay still invokes the post-persistence hook.
	createCoveredReview(userA, "coverage:first-review", coverage)
	createCoveredReview(reviewerB, "coverage:second-reviewer", coverage)
	// A fresh recorder models a process restart; a third reviewer and key still
	// cannot increment the durable task-level coverage marker again.
	createCoveredReview(reviewerC, "coverage:after-restart", &exactFeedbackCoverageRecorder{queries: db.New(pool), metrics: coverageMetrics})
	unattributedTaskID := testTask(t, pool, agentA)
	if err := coverage.RecordTaskRunReviewCoverage(ctx, util.UUIDToString(workspaceA), util.UUIDToString(unattributedTaskID)); err != nil {
		t.Fatalf("record unattributed feedback coverage: %v", err)
	}
	if got := promtest.ToFloat64(coverageMetrics.FeedbackCoveredRuns); got != 1 {
		t.Fatalf("durable feedback covered runs = %v, want 1", got)
	}
	coveredAttribution, err := queries.GetSkillEvolutionTaskAttribution(ctx, db.GetSkillEvolutionTaskAttributionParams{
		WorkspaceID: workspaceA, TaskID: taskID, SkillID: skillA,
	})
	if err != nil || !coveredAttribution.FeedbackCoveredAt.Valid {
		t.Fatalf("durable feedback marker = (%+v, %v)", coveredAttribution.FeedbackCoveredAt, err)
	}

	fastTaskID := testUUID()
	fastDispatchedAt := dispatchedAt.Add(time.Second)
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (
id, agent_id, runtime_id, status, dispatched_at, completed_at
) VALUES ($1, $2, $3, 'completed', $4, $4)`, fastTaskID, agentA, runtimeA, fastDispatchedAt); err != nil {
		t.Fatalf("seed fast completed task: %v", err)
	}
	worker, err := NewAttributionWorker(store, 2, time.Second)
	if err != nil {
		t.Fatalf("NewAttributionWorker: %v", err)
	}
	fastDispatch := handler.TaskDispatchEvent{
		WorkspaceID: util.UUIDToString(workspaceA), TaskID: util.UUIDToString(fastTaskID),
		AgentID: util.UUIDToString(agentA), RuntimeID: util.UUIDToString(runtimeA), DispatchedAt: fastDispatchedAt,
		Skills: []handler.TaskDispatchSkill{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillA)}},
	}
	fastCompletion := handler.TaskCompletionEvent{
		WorkspaceID: fastDispatch.WorkspaceID, TaskID: fastDispatch.TaskID, AgentID: fastDispatch.AgentID,
		RuntimeID: fastDispatch.RuntimeID, DispatchedAt: fastDispatchedAt, CapabilityProven: true,
		SkillExecutionManifest: executionManifestJSON(t, skillbundle.ExecutionRecord{
			Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillA),
			BundleHash: string(candidateInput.BundleHash), RevisionID: util.UUIDToString(candidate.Revision.ID),
		}),
	}
	if !worker.OfferTaskDispatch(fastDispatch) || !worker.OfferTaskCompletion(fastCompletion) {
		t.Fatal("attribution worker rejected fast completion events")
	}
	worker.Close()
	fastSnapshot, err := store.GetTaskDispatchSnapshot(ctx, workspaceA, fastTaskID, agentA, runtimeA, fastDispatchedAt)
	if err != nil {
		t.Fatalf("delayed completed-task snapshot: %v", err)
	}
	fastAttribution, err := queries.GetSkillEvolutionTaskAttribution(ctx, db.GetSkillEvolutionTaskAttributionParams{
		WorkspaceID: workspaceA, TaskID: fastTaskID, SkillID: skillA,
	})
	if err != nil || fastAttribution.DispatchSnapshotID != fastSnapshot.Snapshot.ID ||
		!sameDatabaseTime(fastAttribution.TaskDispatchedAt, fastDispatchedAt) {
		t.Fatalf("delayed completed-task attribution = (%+v, %v)", fastAttribution, err)
	}
	if _, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
		WorkspaceID: workspaceA, TaskID: fastTaskID, AgentID: agentA, RuntimeID: runtimeA,
		TaskDispatchedAt: fastDispatchedAt.Add(time.Second),
		Skills:           []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillA)}},
	}); !errors.Is(err, ErrPersistenceNotFound) {
		t.Fatalf("wrong dispatched_at snapshot error = %v, want not found", err)
	}
	for _, terminalStatus := range []string{"failed", "cancelled"} {
		lateTaskID := testUUID()
		if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (
id, agent_id, runtime_id, status, dispatched_at, completed_at
) VALUES ($1, $2, $3, $4, $5, $5)`, lateTaskID, agentA, runtimeA, terminalStatus, fastDispatchedAt); err != nil {
			t.Fatalf("seed late %s task: %v", terminalStatus, err)
		}
		if _, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
			WorkspaceID: workspaceA, TaskID: lateTaskID, AgentID: agentA, RuntimeID: runtimeA,
			TaskDispatchedAt: fastDispatchedAt,
			Skills:           []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillA)}},
		}); !errors.Is(err, ErrPersistenceNotFound) {
			t.Fatalf("late %s snapshot error = %v, want not found", terminalStatus, err)
		}

		terminalTaskID := testUUID()
		if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (
id, agent_id, runtime_id, status, dispatched_at
) VALUES ($1, $2, $3, 'dispatched', $4)`, terminalTaskID, agentA, runtimeA, fastDispatchedAt); err != nil {
			t.Fatalf("seed pre-terminal %s task: %v", terminalStatus, err)
		}
		terminalSnapshot, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
			WorkspaceID: workspaceA, TaskID: terminalTaskID, AgentID: agentA, RuntimeID: runtimeA,
			TaskDispatchedAt: fastDispatchedAt,
			Skills:           []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillA)}},
		})
		if err != nil {
			t.Fatalf("record pre-terminal %s snapshot: %v", terminalStatus, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE agent_task_queue SET status = $2, completed_at = $3 WHERE id = $1`, terminalTaskID, terminalStatus, fastDispatchedAt); err != nil {
			t.Fatalf("mark task %s: %v", terminalStatus, err)
		}
		terminalAttribution := attributionInput
		terminalAttribution.TaskID = terminalTaskID
		terminalAttribution.DispatchSnapshotID = terminalSnapshot.Snapshot.ID
		terminalAttribution.TaskDispatchedAt = fastDispatchedAt
		terminalAttribution.ManifestDigest = testDigest("terminal-" + terminalStatus)
		if _, err := store.RecordTaskAttribution(ctx, terminalAttribution); !errors.Is(err, ErrPersistenceNotFound) {
			t.Fatalf("%s attribution error = %v, want not found", terminalStatus, err)
		}
	}

	reclaimedAt := dispatchedAt.Add(time.Minute)
	if _, err := pool.Exec(ctx, `UPDATE agent_task_queue SET status = 'dispatched', dispatched_at = $2, completed_at = NULL WHERE id = $1`, taskID, reclaimedAt); err != nil {
		t.Fatalf("reclaim task: %v", err)
	}
	reclaimed, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
		WorkspaceID: workspaceA, TaskID: taskID, AgentID: agentA, RuntimeID: runtimeA, TaskDispatchedAt: reclaimedAt,
		Skills: []DispatchSkillIdentity{{Source: skillbundle.SourceBuiltin, SkillID: "plan"}},
	})
	if err != nil || reclaimed.Snapshot.ID == snapshot.Snapshot.ID {
		t.Fatalf("reclaimed snapshot = (%+v, %v), want new identity", reclaimed, err)
	}
	frozen, err := store.GetTaskDispatchSnapshot(ctx, workspaceA, taskID, agentA, runtimeA, dispatchedAt)
	if err != nil || frozen.Snapshot.ID != snapshot.Snapshot.ID || frozen.Skills[0].SkillID != util.UUIDToString(skillA) {
		t.Fatalf("frozen prior dispatch = (%+v, %v)", frozen, err)
	}

	if _, err := store.ConfigureLoop(ctx, LoopConfig{
		WorkspaceID: workspaceB, SkillID: skillB, Enabled: true, Mode: LoopModeObserve,
		Cooldown: time.Hour, MinimumSignals: 1, MaxEvidenceRefs: 10, MaxReplaySamples: 2,
		MaxCostUSDTicks: 100, PolicyVersion: "v1",
	}); err != nil {
		t.Fatalf("configure other workspace loop: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sys_cron_executions (job_name, scope_kind, scope_id) VALUES
('skill_evolution', 'workspace', $1), ('other_job', 'workspace', $1), ('skill_evolution', 'workspace', $2)`,
		uuid.UUID(workspaceA.Bytes).String(), uuid.UUID(workspaceB.Bytes).String()); err != nil {
		t.Fatalf("seed scheduler cleanup rows: %v", err)
	}
	if err := queries.DeleteWorkspaceSkillEvolutionData(ctx, workspaceA); err != nil {
		t.Fatalf("DeleteWorkspaceSkillEvolutionData: %v", err)
	}
	for _, table := range []string{
		"skill_evolution_task_attribution", "skill_evolution_task_dispatch_snapshot", "task_run_review",
		"skill_evolution_release", "skill_evolution_review", "skill_evolution_evaluation", "skill_evolution_evidence",
		"skill_evolution_proposal", "skill_evolution_revision_file", "skill_evolution_revision", "skill_evolution_loop",
	} {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE workspace_id = $1", workspaceA).Scan(&count); err != nil || count != 0 {
			t.Fatalf("cleanup %s count/error = %d/%v", table, count, err)
		}
	}
	if _, err := store.GetLoop(ctx, workspaceB, skillB); err != nil {
		t.Fatalf("cleanup removed other workspace loop: %v", err)
	}
	var cronCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sys_cron_executions`).Scan(&cronCount); err != nil || cronCount != 2 {
		t.Fatalf("selective scheduler cleanup count/error = %d/%v, want 2", cronCount, err)
	}
}

func TestTaskReviewAndManualRerunQueriesHideCrossWorkspaceRows(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	queries := db.New(pool)
	workspaceA, workspaceB := testUUID(), testUUID()
	userA, userB := testUUID(), testUUID()
	skillA, skillB := testUUID(), testUUID()
	agentA, agentB := testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceA, userA, skillA, agentA)
	seedPersistenceFixture(t, pool, workspaceB, userB, skillB, agentB)

	sourceA, rerunA := testUUID(), testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status, completed_at) VALUES ($1, $2, 'completed', now())`, sourceA, agentA); err != nil {
		t.Fatalf("seed manual rerun source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status, completed_at, rerun_of_task_id, originator_user_id, originator_source) VALUES ($1, $2, 'completed', now(), $3, $4, 'direct_human')`, rerunA, agentA, sourceA, userA); err != nil {
		t.Fatalf("seed manual rerun: %v", err)
	}
	automatedRerun := testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status, completed_at, rerun_of_task_id, originator_user_id, originator_source) VALUES ($1, $2, 'completed', now(), $3, $4, 'automation')`, automatedRerun, agentA, sourceA, userA); err != nil {
		t.Fatalf("seed automated rerun: %v", err)
	}
	agentC := testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO agent (id, workspace_id) VALUES ($1, $2)`, agentC, workspaceA); err != nil {
		t.Fatalf("seed second same-workspace agent: %v", err)
	}
	otherAgentSource, otherAgentRerun := testUUID(), testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status, completed_at) VALUES ($1, $2, 'completed', now())`, otherAgentSource, agentC); err != nil {
		t.Fatalf("seed other-agent source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status, completed_at, rerun_of_task_id, originator_user_id, originator_source) VALUES ($1, $2, 'completed', now(), $3, $4, 'direct_human')`, otherAgentRerun, agentA, otherAgentSource, userA); err != nil {
		t.Fatalf("seed mismatched-agent rerun: %v", err)
	}
	scopeMismatchRerun := testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, issue_id, status, completed_at, rerun_of_task_id, originator_user_id, originator_source) VALUES ($1, $2, $3, 'completed', now(), $4, $5, 'direct_human')`, scopeMismatchRerun, agentA, testUUID(), sourceA, userA); err != nil {
		t.Fatalf("seed mismatched-scope rerun: %v", err)
	}
	nonterminalSource, nonterminalRerun := testUUID(), testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status) VALUES ($1, $2, 'running')`, nonterminalSource, agentA); err != nil {
		t.Fatalf("seed nonterminal source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status, rerun_of_task_id, originator_user_id, originator_source) VALUES ($1, $2, 'running', $3, $4, 'direct_human')`, nonterminalRerun, agentA, nonterminalSource, userA); err != nil {
		t.Fatalf("seed nonterminal-source rerun: %v", err)
	}
	rows, err := queries.ListManualReruns(ctx, db.ListManualRerunsParams{WorkspaceID: workspaceA, PageSize: 10})
	if err != nil || len(rows) != 1 || rows[0].SourceWorkspaceID != workspaceA {
		t.Fatalf("ListManualReruns = (%+v, %v), want one same-workspace row", rows, err)
	}
	for _, hiddenID := range []pgtype.UUID{automatedRerun, otherAgentRerun, scopeMismatchRerun, nonterminalRerun} {
		if _, err := queries.LoadManualRerun(ctx, db.LoadManualRerunParams{WorkspaceID: workspaceA, TaskID: hiddenID}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("invalid manual rerun %v error = %v, want no rows", hiddenID, err)
		}
	}
	if _, err := queries.LoadManualRerun(ctx, db.LoadManualRerunParams{WorkspaceID: workspaceB, TaskID: rerunA}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace manual rerun error = %v, want no rows", err)
	}

	crossWorkspaceRerun := testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, status, completed_at, rerun_of_task_id, originator_user_id, originator_source) VALUES ($1, $2, 'completed', now(), $3, $4, 'direct_human')`, crossWorkspaceRerun, agentA, sourceA, userA); err != nil {
		t.Fatalf("seed cross-workspace child: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_task_queue SET rerun_of_task_id = $1 WHERE id = $2`, testTask(t, pool, agentB), crossWorkspaceRerun); err != nil {
		t.Fatalf("point rerun across workspaces: %v", err)
	}
	if _, err := queries.LoadManualRerun(ctx, db.LoadManualRerunParams{WorkspaceID: workspaceA, TaskID: crossWorkspaceRerun}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace source error = %v, want no rows", err)
	}

	reviewID := testUUID()
	firstReviewAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	review, err := queries.CreateTaskRunReview(ctx, db.CreateTaskRunReviewParams{
		ID: reviewID, WorkspaceID: workspaceA, TaskID: sourceA, ReviewerID: userA,
		Outcome: "needs_correction", Target: "skill_procedure", SkillID: skillA,
		Correction: pgtype.Text{String: "check output", Valid: true}, Reason: "incorrect output", IdempotencyKey: "store-test:review",
		Digest:    string(testDigest("task-review")),
		CreatedAt: pgtype.Timestamptz{Time: firstReviewAt, Valid: true},
	})
	if err != nil || review.ID != reviewID {
		t.Fatalf("CreateTaskRunReview = (%+v, %v)", review, err)
	}
	if _, err := queries.LoadTaskRunReview(ctx, db.LoadTaskRunReviewParams{WorkspaceID: workspaceB, ID: reviewID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace review error = %v, want no rows", err)
	}
	if _, err := queries.CreateTaskRunReview(ctx, db.CreateTaskRunReviewParams{
		ID: testUUID(), WorkspaceID: workspaceA, TaskID: sourceA, ReviewerID: userA,
		Outcome: "helpful", Target: "skill_procedure", SkillID: skillB,
		Reason: "reviewed", IdempotencyKey: "store-test:cross-skill", Digest: string(testDigest("cross-skill")),
		CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace target Skill error = %v, want no rows", err)
	}

	repository := service.NewDBTaskRunReviewRepository(queries)
	workspaceAString := util.UUIDToString(workspaceA)
	serviceReview, err := service.NewTaskRunReviewService(repository, persistenceTaskReviewAccess{
		workspaceID: workspaceAString, reviewerID: util.UUIDToString(userA), taskID: util.UUIDToString(sourceA), agentID: util.UUIDToString(agentA),
	}).CreateTaskRunReview(ctx, workspaceAString, util.UUIDToString(userA), service.CreateTaskRunReviewInput{
		TaskID: util.UUIDToString(sourceA), IdempotencyKey: "store-test:service-review", Outcome: service.TaskRunReviewOutcomeHelpful,
		Target: service.TaskRunReviewTargetKnowledge, Reason: "useful run",
	})
	if err != nil {
		t.Fatalf("create task review through repository: %v", err)
	}
	firstPage, err := repository.ListTaskRunReviewRefs(ctx, workspaceAString, "", 1)
	if err != nil || len(firstPage.Refs) != 1 || firstPage.Refs[0].ID != serviceReview.ID || firstPage.NextCursor == "" {
		t.Fatalf("first review page = (%+v, %v)", firstPage, err)
	}
	secondPage, err := repository.ListTaskRunReviewRefs(ctx, workspaceAString, firstPage.NextCursor, 1)
	if err != nil || len(secondPage.Refs) != 1 || secondPage.Refs[0].ID != util.UUIDToString(reviewID) || secondPage.NextCursor != "" {
		t.Fatalf("second review page = (%+v, %v)", secondPage, err)
	}
	loadedReview, err := repository.LoadTaskRunReview(ctx, workspaceAString, util.UUIDToString(reviewID))
	if err != nil || loadedReview.Correction != "check output" || loadedReview.Reason != "incorrect output" {
		t.Fatalf("loaded review = (%+v, %v)", loadedReview, err)
	}
	manualRecords, manualCursor, err := repository.ListManualReruns(ctx, workspaceAString, "", 10)
	if err != nil || len(manualRecords) != 1 || manualCursor != "" || manualRecords[0].OriginatorUserID != util.UUIDToString(userA) {
		t.Fatalf("manual repository list = (%+v, %q, %v)", manualRecords, manualCursor, err)
	}
	loadedManual, err := repository.LoadManualRerun(ctx, workspaceAString, util.UUIDToString(rerunA))
	if err != nil || loadedManual.SourceTaskID != util.UUIDToString(sourceA) || loadedManual.OriginatorSource != "direct_human" {
		t.Fatalf("loaded manual rerun = (%+v, %v)", loadedManual, err)
	}
}

func TestWikiRoomProposalSourceIdentityPreservesAgentIdempotency(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	queries := db.New(pool)
	workspaceID, pageID, agentID, roomID := testUUID(), testUUID(), testUUID(), testUUID()
	if _, err := pool.Exec(ctx, `INSERT INTO workspace (id) VALUES ($1)`, workspaceID); err != nil {
		t.Fatalf("seed Wiki proposal workspace: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO wiki_page (id, workspace_id, scope, current_revision_number) VALUES ($1, $2, 'workspace', 1)`, pageID, workspaceID); err != nil {
		t.Fatalf("seed Wiki proposal page: %v", err)
	}

	agentParams := db.CreateWikiPageEditProposalParams{
		WorkspaceID: workspaceID, AgentID: agentID, IdempotencyKey: "agent-proposal", BaseRevisionNumber: 1,
		ProposedPath: "guide.md", ProposedTitle: "Guide", ProposedContent: "agent draft", Rationale: "agent rationale",
		EvidenceRefs: []byte(`[]`), PageID: pageID,
	}
	agentFirst, err := queries.CreateWikiPageEditProposal(ctx, agentParams)
	if err != nil {
		t.Fatalf("CreateWikiPageEditProposal: %v", err)
	}
	agentReplay, err := queries.CreateWikiPageEditProposal(ctx, agentParams)
	if err != nil || agentReplay.ID != agentFirst.ID || agentFirst.SourceKind != "agent" || !agentFirst.AgentID.Valid || agentFirst.SourceRefID.Valid {
		t.Fatalf("agent idempotency/source = (%+v, %+v, %v)", agentFirst, agentReplay, err)
	}

	roomParams := db.CreateRoomWikiPageEditProposalParams{
		WorkspaceID: workspaceID, SourceRefID: roomID, IdempotencyKey: "room-proposal", BaseRevisionNumber: 1,
		ProposedPath: "guide.md", ProposedTitle: "Guide", ProposedContent: "room draft", Rationale: "room rationale",
		EvidenceRefs: []byte(`[]`), PageID: pageID,
	}
	roomFirst, err := queries.CreateRoomWikiPageEditProposal(ctx, roomParams)
	if err != nil {
		t.Fatalf("CreateRoomWikiPageEditProposal: %v", err)
	}
	roomReplay, err := queries.CreateRoomWikiPageEditProposal(ctx, roomParams)
	if err != nil || roomReplay.ID != roomFirst.ID || roomFirst.SourceKind != "room" || roomFirst.AgentID.Valid || roomFirst.SourceRefID != roomID {
		t.Fatalf("room idempotency/source = (%+v, %+v, %v)", roomFirst, roomReplay, err)
	}
	loaded, err := queries.GetRoomWikiPageEditProposalByIdempotencyKey(ctx, db.GetRoomWikiPageEditProposalByIdempotencyKeyParams{
		WorkspaceID: workspaceID, SourceRefID: roomID, IdempotencyKey: roomParams.IdempotencyKey,
	})
	if err != nil || loaded.ID != roomFirst.ID {
		t.Fatalf("GetRoomWikiPageEditProposalByIdempotencyKey = (%v, %v), want %v", loaded.ID, err, roomFirst.ID)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO wiki_page_edit_proposal (
workspace_id, page_id, base_revision_number, proposed_path, proposed_title, proposed_content,
content_digest, rationale, evidence_refs, agent_id, idempotency_key, source_kind, source_ref_id
) VALUES ($1, $2, 1, 'bad', 'Bad', 'bad', $3, 'bad', '[]', $4, 'bad-source', 'room', $5)`,
		workspaceID, pageID, testDigest("bad"), agentID, roomID); err == nil {
		t.Fatal("invalid room proposal with agent identity unexpectedly inserted")
	}
}

func TestListScheduledLoopsIncludesOnlyEnabledDueObserveAndProposeModes(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	store := NewStore(db.New(pool), pool)
	workspaceID, userID, agentID := testUUID(), testUUID(), testUUID()
	skillIDs := []pgtype.UUID{testUUID(), testUUID(), testUUID(), testUUID()}
	seedPersistenceFixture(t, pool, workspaceID, userID, skillIDs[0], agentID)
	for _, skillID := range skillIDs[1:] {
		if _, err := pool.Exec(ctx, `INSERT INTO skill (id, workspace_id, created_by) VALUES ($1, $2, $3)`, skillID, workspaceID, userID); err != nil {
			t.Fatalf("seed scheduled Skill: %v", err)
		}
	}
	configs := []LoopConfig{
		{WorkspaceID: workspaceID, SkillID: skillIDs[0], Enabled: true, Mode: LoopModeObserve},
		{WorkspaceID: workspaceID, SkillID: skillIDs[1], Enabled: true, Mode: LoopModePropose},
		{WorkspaceID: workspaceID, SkillID: skillIDs[2], Enabled: true, Mode: LoopModePaused},
		{WorkspaceID: workspaceID, SkillID: skillIDs[3], Enabled: false, Mode: LoopModePropose},
	}
	for i := range configs {
		configs[i].Cooldown = time.Hour
		configs[i].MinimumSignals = 1
		configs[i].MaxEvidenceRefs = 10
		configs[i].MaxReplaySamples = 2
		configs[i].MaxCostUSDTicks = 100
		configs[i].PolicyVersion = "v1"
		if _, err := store.ConfigureLoop(ctx, configs[i]); err != nil {
			t.Fatalf("configure scheduled loop %d: %v", i, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE skill_evolution_loop SET next_eligible_at = now() + interval '1 hour' WHERE skill_id = $1`, skillIDs[1]); err != nil {
		t.Fatalf("defer propose loop: %v", err)
	}
	rows, err := store.ListScheduledLoops(ctx, time.Now().UTC(), pgtype.UUID{}, 10)
	if err != nil || len(rows) != 1 || rows[0].SkillID != skillIDs[0] || rows[0].Mode != string(LoopModeObserve) {
		t.Fatalf("scheduled loops = (%+v, %v), want due observe only", rows, err)
	}
	if rows, err := store.ListScheduledLoops(ctx, time.Now().UTC(), rows[0].ID, 10); err != nil || len(rows) != 0 {
		t.Fatalf("scheduled keyset tail = (%+v, %v), want empty", rows, err)
	}
}

func TestTaskAttributionWorkspaceGuardSerializesWorkspaceCleanup(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	workspaceID, userID, skillID, agentID, runtimeID, taskID :=
		testUUID(), testUUID(), testUUID(), testUUID(), testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, skillID, agentID)
	if _, err := pool.Exec(ctx, `INSERT INTO agent_runtime (id, workspace_id) VALUES ($1, $2)`, runtimeID, workspaceID); err != nil {
		t.Fatalf("seed attribution runtime: %v", err)
	}
	store := NewStore(db.New(pool), pool)
	revisionInput := testRevisionInput(t, workspaceID, skillID, "candidate", "guarded candidate")
	revisionInput.CreatedByID = userID
	revision, err := store.SaveRevision(ctx, revisionInput)
	if err != nil {
		t.Fatalf("save guarded revision: %v", err)
	}
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (
id, agent_id, runtime_id, status, dispatched_at, completed_at
) VALUES ($1, $2, $3, 'completed', $4, $4)`, taskID, agentID, runtimeID, dispatchedAt); err != nil {
		t.Fatalf("seed guarded task: %v", err)
	}
	snapshot, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
		WorkspaceID: workspaceID, TaskID: taskID, AgentID: agentID, RuntimeID: runtimeID, TaskDispatchedAt: dispatchedAt,
		Skills: []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillID)}},
	})
	if err != nil {
		t.Fatalf("record guarded snapshot: %v", err)
	}

	writerTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin attribution writer: %v", err)
	}
	writerDone := false
	t.Cleanup(func() {
		if !writerDone {
			_ = writerTx.Rollback(context.Background())
		}
	})
	writerStore := NewStore(db.New(writerTx), nil)
	if _, err := writerStore.RecordTaskAttribution(ctx, TaskAttributionInput{
		WorkspaceID: workspaceID, TaskID: taskID, RuntimeID: runtimeID, SkillID: skillID,
		RevisionID: revision.Revision.ID, ManifestVersion: SkillExecutionManifestVersion,
		Source: skillbundle.SourceWorkspace, BundleHash: revisionInput.BundleHash, ManifestDigest: testDigest("guarded-manifest"),
		Eligibility: EvidenceEligibilityEligible, Reason: string(AttributionReasonExactRevisionMatch),
		DispatchSnapshotID: snapshot.Snapshot.ID, TaskDispatchedAt: dispatchedAt,
	}); err != nil {
		t.Fatalf("record guarded attribution: %v", err)
	}

	cleanupAttempt, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin blocked cleanup: %v", err)
	}
	lockCtx, cancelLock := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancelLock()
	var lockedWorkspace pgtype.UUID
	err = cleanupAttempt.QueryRow(lockCtx, `SELECT id FROM workspace WHERE id = $1 FOR UPDATE`, workspaceID).Scan(&lockedWorkspace)
	if !errors.Is(err, context.DeadlineExceeded) {
		_ = cleanupAttempt.Rollback(context.Background())
		t.Fatalf("workspace delete lock error = %v, want attribution writer to hold KEY SHARE", err)
	}
	_ = cleanupAttempt.Rollback(context.Background())

	if err := writerTx.Commit(ctx); err != nil {
		t.Fatalf("commit attribution writer: %v", err)
	}
	writerDone = true

	cleanupTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin workspace cleanup: %v", err)
	}
	cleanupDone := false
	t.Cleanup(func() {
		if !cleanupDone {
			_ = cleanupTx.Rollback(context.Background())
		}
	})
	if err := cleanupTx.QueryRow(ctx, `SELECT id FROM workspace WHERE id = $1 FOR UPDATE`, workspaceID).Scan(&lockedWorkspace); err != nil {
		t.Fatalf("lock workspace for cleanup: %v", err)
	}
	cleanupQueries := db.New(cleanupTx)
	if err := cleanupQueries.DeleteWorkspaceSkillEvolutionData(ctx, workspaceID); err != nil {
		t.Fatalf("delete guarded evolution data: %v", err)
	}
	if _, err := cleanupTx.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, workspaceID); err != nil {
		t.Fatalf("delete guarded workspace: %v", err)
	}
	if err := cleanupTx.Commit(ctx); err != nil {
		t.Fatalf("commit workspace cleanup: %v", err)
	}
	cleanupDone = true

	for table, column := range map[string]string{
		"workspace":                        "id",
		"skill_evolution_task_attribution": "workspace_id",
	} {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE "+column+" = $1", workspaceID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("post-cleanup %s count/error = %d/%v", table, count, err)
		}
	}
}

type persistenceTaskReviewAccess struct {
	workspaceID string
	reviewerID  string
	taskID      string
	agentID     string
}

func (a persistenceTaskReviewAccess) ValidateWorkspaceMember(_ context.Context, workspaceID, reviewerID string) error {
	if workspaceID != a.workspaceID || reviewerID != a.reviewerID {
		return service.ErrTaskRunReviewForbidden
	}
	return nil
}

func (a persistenceTaskReviewAccess) LoadAuthorizedTask(_ context.Context, workspaceID, reviewerID, taskID string) (service.TaskRunReviewTask, error) {
	if workspaceID != a.workspaceID || reviewerID != a.reviewerID || taskID != a.taskID {
		return service.TaskRunReviewTask{}, service.ErrTaskRunReviewNotFound
	}
	return service.TaskRunReviewTask{ID: taskID, WorkspaceID: workspaceID, AgentID: a.agentID, Status: "completed"}, nil
}

func (a persistenceTaskReviewAccess) ValidateTargetSkill(context.Context, string, string) error {
	return service.ErrTaskRunReviewNotFound
}

func skillEvolutionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("connect to Postgres: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("Postgres unavailable: %v", err)
	}
	schema := "skill_evolution_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		admin.Close()
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open schema pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `
CREATE TABLE workspace (id UUID PRIMARY KEY);
CREATE TABLE skill (id UUID NOT NULL, workspace_id UUID NOT NULL, created_by UUID NULL);
CREATE TABLE member (workspace_id UUID NOT NULL, user_id UUID NOT NULL);
CREATE TABLE agent (id UUID NOT NULL, workspace_id UUID NOT NULL);
CREATE TABLE agent_runtime (id UUID NOT NULL, workspace_id UUID NOT NULL);
CREATE TABLE agent_task_queue (
    id UUID NOT NULL, agent_id UUID NOT NULL, runtime_id UUID NULL,
    issue_id UUID NULL, chat_session_id UUID NULL,
    status TEXT NOT NULL, dispatched_at TIMESTAMPTZ NULL, completed_at TIMESTAMPTZ NULL,
    rerun_of_task_id UUID NULL, retry_of_task_id UUID NULL,
    originator_user_id UUID NULL, originator_source TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE wiki_page (
    id UUID NOT NULL, workspace_id UUID NULL, scope TEXT NOT NULL,
    current_revision_number BIGINT NOT NULL
);
CREATE TABLE wiki_page_edit_proposal (
    id UUID NOT NULL DEFAULT gen_random_uuid(), workspace_id UUID NOT NULL, page_id UUID NOT NULL,
    base_revision_number BIGINT NOT NULL CHECK (base_revision_number > 0), proposed_path TEXT NOT NULL,
    proposed_title TEXT NOT NULL, proposed_content TEXT NOT NULL,
    content_digest TEXT NOT NULL CHECK (content_digest ~ '^sha256:[0-9a-f]{64}$'),
    rationale TEXT NOT NULL, evidence_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    agent_id UUID NOT NULL, idempotency_key TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending',
    reviewed_by_id UUID, review_reason TEXT, reviewed_at TIMESTAMPTZ, accepted_revision_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX wiki_page_edit_proposal_idempotency_uidx
    ON wiki_page_edit_proposal (workspace_id, agent_id, idempotency_key);
CREATE TABLE sys_cron_executions (
    job_name TEXT NOT NULL, scope_kind TEXT NOT NULL, scope_id TEXT NOT NULL
)`); err != nil {
		t.Fatalf("create prerequisite tables: %v", err)
	}
	applySkillEvolutionMigrations(t, pool)
	return pool
}

func applySkillEvolutionMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(files, func(i, j int) bool { return filepath.Base(files[i]) < filepath.Base(files[j]) })
	for _, file := range files {
		name := filepath.Base(file)
		prefix := strings.SplitN(name, "_", 2)[0]
		if (prefix < "482" || prefix > "512") && (prefix < "515" || prefix > "525") {
			continue
		}
		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
}

func seedPersistenceFixture(t *testing.T, pool *pgxpool.Pool, workspaceID, userID, skillID, agentID pgtype.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO workspace (id) VALUES ($1) ON CONFLICT DO NOTHING`, workspaceID); err != nil {
		t.Fatalf("seed persistence workspace: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO skill (id, workspace_id, created_by) VALUES ($1, $2, $3)`, skillID, workspaceID, userID); err != nil {
		t.Fatalf("seed persistence Skill: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO member (workspace_id, user_id) VALUES ($1, $2)`, workspaceID, userID); err != nil {
		t.Fatalf("seed persistence member: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO agent (id, workspace_id) VALUES ($1, $2)`, agentID, workspaceID); err != nil {
		t.Fatalf("seed persistence fixture: %v", err)
	}
}

func testRevisionInput(t *testing.T, workspaceID, skillID pgtype.UUID, kind, content string) RevisionInput {
	t.Helper()
	bundle := skillbundle.Skill{
		ID: uuid.UUID(skillID.Bytes).String(), Source: skillbundle.SourceWorkspace,
		Name: "deploy", Description: "Deploy safely", Content: content,
		Files: []skillbundle.File{{Path: "refs/checklist.md", Content: "check"}},
	}
	manifest, err := skillbundle.BuildValidatedManifest(bundle)
	if err != nil {
		t.Fatal(err)
	}
	return RevisionInput{
		WorkspaceID: workspaceID, SkillID: skillID, Kind: kind, Ownership: OwnershipWorkspace,
		Source: bundle.Source, BundleHash: Digest(manifest.Hash), MetadataDigest: testDigest(content + "-metadata"),
		Name: bundle.Name, Description: bundle.Description, PrimaryContent: bundle.Content,
		Files: []RevisionFileInput{{Path: bundle.Files[0].Path, Content: bundle.Files[0].Content}},
	}
}

func testTask(t *testing.T, pool *pgxpool.Pool, agentID pgtype.UUID) pgtype.UUID {
	t.Helper()
	id := testUUID()
	if _, err := pool.Exec(context.Background(), `INSERT INTO agent_task_queue (id, agent_id, status, completed_at) VALUES ($1, $2, 'completed', now())`, id, agentID); err != nil {
		t.Fatal(err)
	}
	return id
}

func testUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}

func testDigest(value string) Digest {
	digest, err := CanonicalEvidenceDigest("store_test", []DigestPart{{Key: "value", Value: value}})
	if err != nil {
		panic(fmt.Sprintf("test digest: %v", err))
	}
	return digest
}
