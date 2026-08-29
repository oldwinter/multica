package skillevolution

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestExactFeedbackCoverageSerializesReviewAndAttributionTransactions(t *testing.T) {
	t.Run("attribution uncommitted makes review wait then cover", func(t *testing.T) {
		fixture := newCoverageConcurrencyFixture(t)
		written := make(chan struct{})
		release := make(chan struct{})
		fixture.store.attributionBatchBeforeCommit = func() {
			close(written)
			<-release
		}
		attributionDone := make(chan error, 1)
		go func() {
			_, err := fixture.store.recordAttributionBatch(context.Background(), []TaskAttributionInput{fixture.attribution})
			attributionDone <- err
		}()
		<-written

		reviewDone := make(chan error, 1)
		go func() { reviewDone <- fixture.createReview("coverage-race:attribution-first") }()
		assertStillBlocked(t, reviewDone, "review while attribution transaction is uncommitted")
		close(release)
		if err := <-attributionDone; err != nil {
			t.Fatalf("commit attribution: %v", err)
		}
		if err := <-reviewDone; err != nil {
			t.Fatalf("create review after attribution: %v", err)
		}
		fixture.assertCoveredExactlyOnce(t)
	})

	t.Run("review uncommitted makes attribution wait then reconcile", func(t *testing.T) {
		fixture := newCoverageConcurrencyFixture(t)
		written := make(chan struct{})
		release := make(chan struct{})
		fixture.reviews.reviewBeforeCommit = func() {
			close(written)
			<-release
		}
		reviewDone := make(chan error, 1)
		go func() { reviewDone <- fixture.createReview("coverage-race:review-first") }()
		<-written

		type attributionResult struct {
			value AttributionBatchResult
			err   error
		}
		attributionDone := make(chan attributionResult, 1)
		go func() {
			value, err := fixture.store.recordAttributionBatch(context.Background(), []TaskAttributionInput{fixture.attribution})
			attributionDone <- attributionResult{value: value, err: err}
		}()
		select {
		case result := <-attributionDone:
			t.Fatalf("attribution did not wait for uncommitted review: %+v", result)
		case <-time.After(150 * time.Millisecond):
		}
		close(release)
		if err := <-reviewDone; err != nil {
			t.Fatalf("commit review: %v", err)
		}
		result := <-attributionDone
		if result.err != nil || !result.value.Inserted || !result.value.Covered {
			t.Fatalf("attribution reconciliation = (%+v, %v)", result.value, result.err)
		}
		if replayed, err := fixture.store.recordAttributionBatch(context.Background(), []TaskAttributionInput{fixture.attribution}); err != nil || replayed.Inserted || replayed.Covered {
			t.Fatalf("idempotent attribution = (%+v, %v)", replayed, err)
		}
		fixture.assertCoveredExactlyOnce(t)
	})
}

type coverageConcurrencyFixture struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	store       *Store
	reviews     *exactCoverageTaskReviewRepository
	workspaceID pgtype.UUID
	userID      pgtype.UUID
	taskID      pgtype.UUID
	agentID     pgtype.UUID
	attribution TaskAttributionInput
}

func newCoverageConcurrencyFixture(t *testing.T) *coverageConcurrencyFixture {
	t.Helper()
	pool := skillEvolutionTestPool(t)
	workspaceID, userID, skillID, agentID, runtimeID, taskID :=
		testUUID(), testUUID(), testUUID(), testUUID(), testUUID(), testUUID()
	dbFixture := seedPersistenceFixture(t, pool, workspaceID, userID, skillID, agentID)
	dbFixture.Insert(t, "agent_runtime", testutil.Cols{"id": runtimeID, "workspace_id": workspaceID})
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	dbFixture.Task(t, util.UUIDToString(agentID), testutil.Cols{
		"id": taskID, "runtime_id": runtimeID, "status": "dispatched", "dispatched_at": dispatchedAt,
	})
	queries := db.New(pool)
	store := NewStore(queries, pool)
	revisionInput := testRevisionInput(t, workspaceID, skillID, "candidate", "coverage concurrency")
	revisionInput.CreatedByID = userID
	revision, err := store.SaveRevision(context.Background(), revisionInput)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.RecordTaskDispatchSnapshot(context.Background(), TaskDispatchSnapshotInput{
		WorkspaceID: workspaceID, TaskID: taskID, AgentID: agentID, RuntimeID: runtimeID, TaskDispatchedAt: dispatchedAt,
		Skills: []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: util.UUIDToString(skillID)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dbFixture.Exec(t, "UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1", taskID)
	return &coverageConcurrencyFixture{
		pool: pool, queries: queries, store: store,
		reviews:     newExactCoverageTaskReviewRepository(pool, queries, NewMetrics()),
		workspaceID: workspaceID, userID: userID, taskID: taskID, agentID: agentID,
		attribution: TaskAttributionInput{
			WorkspaceID: workspaceID, TaskID: taskID, RuntimeID: runtimeID, SkillID: skillID,
			RevisionID: revision.Revision.ID, ManifestVersion: SkillExecutionManifestVersion,
			Source: skillbundle.SourceWorkspace, BundleHash: revisionInput.BundleHash,
			ManifestDigest: testDigest("coverage-concurrency-manifest"), Eligibility: EvidenceEligibilityEligible,
			Reason: string(AttributionReasonExactRevisionMatch), DispatchSnapshotID: snapshot.Snapshot.ID,
			TaskDispatchedAt: dispatchedAt,
		},
	}
}

func (fixture *coverageConcurrencyFixture) createReview(key string) error {
	reviews := service.NewTaskRunReviewService(fixture.reviews, persistenceTaskReviewAccess{
		workspaceID: util.UUIDToString(fixture.workspaceID), reviewerID: util.UUIDToString(fixture.userID),
		taskID: util.UUIDToString(fixture.taskID), agentID: util.UUIDToString(fixture.agentID),
	})
	_, err := reviews.CreateTaskRunReview(context.Background(), util.UUIDToString(fixture.workspaceID), util.UUIDToString(fixture.userID), service.CreateTaskRunReviewInput{
		TaskID: util.UUIDToString(fixture.taskID), IdempotencyKey: key,
		Outcome: service.TaskRunReviewOutcomeHelpful, Target: service.TaskRunReviewTargetKnowledge, Reason: "coverage race",
	})
	return err
}

func (fixture *coverageConcurrencyFixture) assertCoveredExactlyOnce(t *testing.T) {
	t.Helper()
	row, err := fixture.queries.GetSkillEvolutionTaskAttribution(context.Background(), db.GetSkillEvolutionTaskAttributionParams{
		WorkspaceID: fixture.workspaceID, TaskID: fixture.taskID, SkillID: fixture.attribution.SkillID,
	})
	if err != nil || !row.FeedbackCoveredAt.Valid {
		t.Fatalf("covered attribution = (%+v, %v)", row.FeedbackCoveredAt, err)
	}
	var eligible, covered int
	if err := fixture.pool.QueryRow(context.Background(), `SELECT count(*), count(*) FILTER (WHERE feedback_covered_at IS NOT NULL)
		FROM skill_evolution_task_attribution WHERE workspace_id = $1 AND task_id = $2`, fixture.workspaceID, fixture.taskID).
		Scan(&eligible, &covered); err != nil || eligible != 1 || covered != 1 || covered > eligible {
		t.Fatalf("coverage counts = %d/%d, err=%v", eligible, covered, err)
	}
}

func assertStillBlocked(t *testing.T, done <-chan error, operation string) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("%s completed before lock release: %v", operation, err)
	case <-time.After(150 * time.Millisecond):
	}
}
