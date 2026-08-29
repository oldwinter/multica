package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
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
	skill    db.Skill
	files    []db.SkillFile
	loop     db.SkillEvolutionLoop
	base     db.SkillEvolutionRevision
	existing db.SkillEvolutionProposal
	created  db.SkillEvolutionProposal
	creates  int
	staled   int
}

func (stub *roomProposalQueriesStub) LockWorkspaceSkillForEvolution(context.Context, db.LockWorkspaceSkillForEvolutionParams) (db.Skill, error) {
	return stub.skill, nil
}

func (stub *roomProposalQueriesStub) LockWorkspaceSkillFilesForEvolution(context.Context, db.LockWorkspaceSkillFilesForEvolutionParams) ([]db.SkillFile, error) {
	return stub.files, nil
}

func (stub *roomProposalQueriesStub) StaleDriftedActiveSkillEvolutionProposals(_ context.Context, params db.StaleDriftedActiveSkillEvolutionProposalsParams) ([]db.SkillEvolutionProposal, error) {
	if stub.existing.ID.Valid && stub.existing.BaseHash != params.LiveHash {
		stub.existing.State = string(ProposalStateStale)
		stub.staled++
		return []db.SkillEvolutionProposal{stub.existing}, nil
	}
	return nil, nil
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
		skill: db.Skill{ID: fixture.skillID, WorkspaceID: fixture.workspaceID, Name: fixture.base.Name,
			Description: fixture.base.Description, Content: fixture.base.Content, Config: []byte(`{}`)},
		loop: db.SkillEvolutionLoop{ID: loopID, WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID,
			IsEnabled: true, Mode: string(LoopModePropose), MinimumSignals: 1, MaxEvidenceRefs: 10},
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

func TestRoomSkillProposalTargetStalesDriftedActiveProposalUnderSkillLock(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	artifact := fixture.metadata.artifacts[0]
	artifact.TargetID = pgtype.UUID{}
	drifted := fixture.base
	drifted.Content = strings.Replace(drifted.Content, "Original.", "Human edit.", 1)
	queries := &roomProposalQueriesStub{
		skill: db.Skill{ID: fixture.skillID, WorkspaceID: fixture.workspaceID, Name: drifted.Name,
			Description: drifted.Description, Content: drifted.Content, Config: []byte(`{}`)},
		existing: db.SkillEvolutionProposal{ID: testUUID(), WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID,
			State: string(ProposalStateReady), BaseHash: string(fixture.baseHash)},
	}
	target := NewRoomSkillProposalTarget(&Lifecycle{skills: workspaceSkillLoaderStub{}}, fixture.source)
	if _, err := target.createQueuedProposal(context.Background(), queries, artifact); !errors.Is(err, ErrStaleBase) {
		t.Fatalf("drift error = %v, want stale base", err)
	}
	if queries.staled != 1 || queries.existing.State != string(ProposalStateStale) || queries.creates != 0 {
		t.Fatalf("drift cleanup staled=%d state=%q creates=%d", queries.staled, queries.existing.State, queries.creates)
	}
}

func TestRoomSkillProposalTargetSerializesDriftCleanupAndNewBaseProposal(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	setupProductionRoomQueueSchema(t, pool)
	fixture := newRoomCandidateFixture(t)
	actorID := fixture.metadata.artifacts[0].CreatedByUserID
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: fixture.skillID, WorkspaceID: fixture.workspaceID, CreatorID: actorID,
		Name: fixture.base.Name, Description: fixture.base.Description, Content: fixture.base.Content, Config: `{}`,
	})
	mustExecPublisher(t, pool, `INSERT INTO workspace (id) VALUES ($1)`, fixture.workspaceID)
	mustExecPublisher(t, pool, `INSERT INTO member (workspace_id, user_id) VALUES ($1, $2)`, fixture.workspaceID, actorID)
	store := NewStore(db.New(pool), pool)
	loop, err := store.ConfigureLoop(context.Background(), LoopConfig{
		WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID, Enabled: true, Mode: LoopModePropose,
		Cooldown: time.Minute, MinimumSignals: 1, MaxEvidenceRefs: 5, MaxReplaySamples: 1,
		MaxCostUSDTicks: 10, PolicyVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	baseMetadata, err := revisionMetadataDigest(fixture.base, fixture.baseHash)
	if err != nil {
		t.Fatal(err)
	}
	baseRevision, err := store.SaveRevision(context.Background(), revisionInput(
		fixture.workspaceID, fixture.skillID, actorID, "base", OwnershipWorkspace,
		fixture.base, fixture.baseHash, baseMetadata,
	))
	if err != nil {
		t.Fatal(err)
	}
	oldProposal, err := store.CreateProposal(context.Background(), ProposalInput{
		WorkspaceID: fixture.workspaceID, SkillID: fixture.skillID, LoopID: loop.ID,
		BaseRevisionID: baseRevision.Revision.ID, BaseHash: fixture.baseHash,
		GenerationKey: "old-active-base", RequestedByID: actorID,
	})
	if err != nil {
		t.Fatal(err)
	}
	mustExecPublisher(t, pool, `UPDATE skill_evolution_proposal SET state = 'ready' WHERE id = $1`, oldProposal.ID)

	drifted := fixture.base
	drifted.Content = strings.Replace(drifted.Content, "Original.", "Human-edited base.", 1)
	driftedManifest, err := skillbundle.BuildValidatedManifest(drifted)
	if err != nil {
		t.Fatal(err)
	}
	driftedHash := Digest(driftedManifest.Hash)
	mustExecPublisher(t, pool, `UPDATE skill SET content = $1 WHERE id = $2`, drifted.Content, fixture.skillID)
	driftedMetadata, err := revisionMetadataDigest(drifted, driftedHash)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveRevision(context.Background(), revisionInput(
		fixture.workspaceID, fixture.skillID, actorID, "base", OwnershipWorkspace,
		drifted, driftedHash, driftedMetadata,
	)); err != nil {
		t.Fatal(err)
	}
	candidate := drifted
	candidate.Content += "\nRun the newly required check."
	envelope := roomCandidateEnvelope{
		SchemaVersion: 1, BaseSkillID: uuidText(fixture.skillID), BaseHash: string(driftedHash),
		Bundle: roomCandidateBundle{ID: uuidText(fixture.skillID), Source: skillbundle.SourceWorkspace,
			Name: candidate.Name, Description: candidate.Description, Content: candidate.Content, Files: []roomCandidateFile{}},
		ObservedPattern: "the check was skipped", ExpectedBenefit: "the check becomes explicit",
		RegressionRisk: "one additional bounded check", EvidenceDigests: []string{string(fixture.signalRef.Digest)},
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	fixture.outcomes.evidence.Body = string(body)
	artifact := fixture.metadata.artifacts[0]
	artifact.Body = string(body)
	artifact.TargetID = pgtype.UUID{}
	artifact.SourceDigest = roomArtifactSourceDigest(artifact)
	fixture.metadata.artifacts[0] = artifact
	target := NewRoomSkillProposalTarget(&Lifecycle{skills: NewWorkspaceSkillRepository(db.New(pool))}, fixture.source)

	ids := make(chan pgtype.UUID, 2)
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	start := make(chan struct{})
	for range 2 {
		ready.Add(1)
		go func() {
			ready.Done()
			<-start
			tx, beginErr := pool.Begin(context.Background())
			if beginErr != nil {
				errs <- beginErr
				return
			}
			id, createErr := target.createQueuedProposal(context.Background(), db.New(tx), artifact)
			if createErr == nil {
				createErr = tx.Commit(context.Background())
			} else {
				_ = tx.Rollback(context.Background())
			}
			ids <- id
			errs <- createErr
		}()
	}
	ready.Wait()
	close(start)
	firstID, secondID := <-ids, <-ids
	if firstErr, secondErr := <-errs, <-errs; firstErr != nil || secondErr != nil || firstID != secondID || !firstID.Valid {
		t.Fatalf("concurrent promotion ids=(%v,%v) errors=(%v,%v)", firstID, secondID, firstErr, secondErr)
	}
	var oldState string
	var queuedCount int
	if err := pool.QueryRow(context.Background(), `SELECT state FROM skill_evolution_proposal WHERE id = $1`, oldProposal.ID).Scan(&oldState); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM skill_evolution_proposal WHERE workspace_id = $1 AND skill_id = $2 AND state = 'queued' AND base_hash = $3`, fixture.workspaceID, fixture.skillID, driftedHash).Scan(&queuedCount); err != nil {
		t.Fatal(err)
	}
	if oldState != string(ProposalStateStale) || queuedCount != 1 {
		t.Fatalf("drift transition old=%q new queued=%d", oldState, queuedCount)
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
