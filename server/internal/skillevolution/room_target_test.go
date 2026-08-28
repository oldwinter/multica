package skillevolution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus/testutil"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

type workspaceSkillLoaderStub struct{ snapshot WorkspaceSkillSnapshot }

func (stub workspaceSkillLoaderStub) Load(context.Context, pgtype.UUID, pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	return stub.snapshot, nil
}

type roomProposalQueriesStub struct {
	loop     db.SkillEvolutionLoop
	base     db.SkillEvolutionRevision
	existing db.SkillEvolutionProposal
	created  db.SkillEvolutionProposal
	creates  int
}

func (stub *roomProposalQueriesStub) GetSkillEvolutionLoop(context.Context, db.GetSkillEvolutionLoopParams) (db.SkillEvolutionLoop, error) {
	return stub.loop, nil
}

func (stub *roomProposalQueriesStub) GetSkillEvolutionRevisionByHash(context.Context, db.GetSkillEvolutionRevisionByHashParams) (db.SkillEvolutionRevision, error) {
	return stub.base, nil
}

func (stub *roomProposalQueriesStub) GetSkillEvolutionProposalByGenerationKey(context.Context, db.GetSkillEvolutionProposalByGenerationKeyParams) (db.SkillEvolutionProposal, error) {
	if stub.existing.ID.Valid {
		return stub.existing, nil
	}
	return db.SkillEvolutionProposal{}, pgx.ErrNoRows
}

func (stub *roomProposalQueriesStub) CreateSkillEvolutionProposal(context.Context, db.CreateSkillEvolutionProposalParams) (db.SkillEvolutionProposal, error) {
	stub.creates++
	return stub.created, nil
}

func TestRoomSkillProposalTargetQueuesInsidePromotionAndReplaysIdentity(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	artifact := fixture.metadata.artifacts[0]
	artifact.TargetID = pgtype.UUID{}
	manifest, err := skillbundle.BuildValidatedManifest(fixture.base)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := &Lifecycle{skills: workspaceSkillLoaderStub{snapshot: WorkspaceSkillSnapshot{
		Skill: db.Skill{ID: fixture.skillID, WorkspaceID: fixture.workspaceID}, Bundle: fixture.base, Manifest: manifest,
		Ownership: Ownership{Class: OwnershipWorkspace, DirectEvolution: true},
	}}}
	target := NewRoomSkillProposalTarget(lifecycle, fixture.source)
	loopID, baseID, proposalID := testUUID(), testUUID(), testUUID()
	key := lifecycleKey("room-recommendation", artifact.IdempotencyKey)
	queries := &roomProposalQueriesStub{
		loop: db.SkillEvolutionLoop{ID: loopID, WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID,
			Enabled: true, Mode: string(LoopModePropose), MinimumSignals: 1, MaxEvidenceRefs: 10},
		base: db.SkillEvolutionRevision{ID: baseID, WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID,
			BundleHash: string(fixture.baseHash), OwnershipClass: string(OwnershipWorkspace)},
		created: db.SkillEvolutionProposal{ID: proposalID, WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID,
			LoopID: loopID, State: string(ProposalStateQueued), BaseRevisionID: baseID, BaseHash: string(fixture.baseHash),
			GenerationIdempotencyKey: key, RequestedByID: artifact.CreatedByUserID},
	}
	got, err := target.createQueuedProposal(context.Background(), queries, artifact)
	if err != nil || got != proposalID || queries.creates != 1 {
		t.Fatalf("create = (%v, %v), calls=%d", got, err, queries.creates)
	}
	queries.existing = queries.created
	got, err = target.createQueuedProposal(context.Background(), queries, artifact)
	if err != nil || got != proposalID || queries.creates != 1 {
		t.Fatalf("replay = (%v, %v), calls=%d", got, err, queries.creates)
	}
}

func TestRoomSkillProposalTargetRecoveryRequiresApprovedReview(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	fixture.metadata.review.Status = "rejected"
	target := NewRoomSkillProposalTarget(&Lifecycle{}, fixture.source)
	if _, err := target.ProcessRoomArtifactTarget(context.Background(), fixture.metadata.artifacts[0]); !errors.Is(err, ErrRoomCandidateNotReady) {
		t.Fatalf("recovery error = %v", err)
	}
}

func TestRoomSkillProposalTargetRecordsProductionGenerationMetrics(t *testing.T) {
	metrics := NewMetrics()
	target := &RoomSkillProposalTarget{metrics: metrics}
	target.recordGenerationMetrics(Generation{
		Proposal:   db.SkillEvolutionProposal{State: string(ProposalStateReady)},
		Candidate:  RevisionSnapshot{Revision: db.SkillEvolutionRevision{ID: testUUID()}},
		Validation: db.SkillEvolutionEvaluation{CostUsdTicks: pgtype.Int8{Int64: 3, Valid: true}},
		Replay:     db.SkillEvolutionEvaluation{CostUsdTicks: pgtype.Int8{Int64: 5, Valid: true}},
	}, nil, 2*time.Second)
	if got := testutil.ToFloat64(metrics.ProposalsGenerated); got != 1 {
		t.Fatalf("proposals generated = %v", got)
	}
	if got := testutil.ToFloat64(metrics.Revisions); got != 1 {
		t.Fatalf("revisions = %v", got)
	}
	if got := testutil.ToFloat64(metrics.CostUSDTicks); got != 8 {
		t.Fatalf("cost ticks = %v", got)
	}
	target.recordGenerationMetrics(Generation{}, ErrEvaluationFailed, time.Second)
	if got := testutil.ToFloat64(metrics.ValidationFailures); got != 1 {
		t.Fatalf("validation failures = %v", got)
	}
}
