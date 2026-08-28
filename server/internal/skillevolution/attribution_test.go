package skillevolution

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestAttributionRecorderRequiresLiveCapability(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	input.CapabilityProven = false

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonCapabilityNotProven)
	if repository.resolveCalls != 0 || repository.recordCalls != 0 {
		t.Fatalf("repository calls = resolve %d record %d, want none", repository.resolveCalls, repository.recordCalls)
	}
}

func TestAttributionRecorderContainsMissingInput(t *testing.T) {
	repository := newFakeAttributionRepository()

	report := newAttributionRecorder(repository).Record(context.Background(), AttributionInput{CapabilityProven: true})

	assertIneligibleAttribution(t, report, AttributionReasonInvalidInput)
	if repository.resolveCalls != 0 || repository.recordCalls != 0 {
		t.Fatal("missing input must not reach persistence")
	}
}

func TestAttributionRecorderRecordsExactRevision(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	if report.Eligibility != EvidenceEligibilityEligible || report.Reason != AttributionReasonExactRevisionMatch {
		t.Fatalf("report = %+v, want eligible exact match", report)
	}
	if report.Recorded != 1 || len(report.Outcomes) != 1 {
		t.Fatalf("recorded/outcomes = %d/%d, want 1/1", report.Recorded, len(report.Outcomes))
	}
	if repository.recordCalls != 1 || len(repository.rows) != 1 {
		t.Fatalf("repository record calls/rows = %d/%d, want 1/1", repository.recordCalls, len(repository.rows))
	}
	for _, row := range repository.rows {
		if row.Eligibility != EvidenceEligibilityEligible || row.Reason != string(AttributionReasonExactRevisionMatch) ||
			!row.ManifestDigest.Valid() || row.DispatchSnapshotID != input.DispatchSnapshotID || !row.TaskDispatchedAt.Equal(input.TaskDispatchedAt) {
			t.Fatalf("stored row = %+v", row)
		}
	}
}

func TestAttributionRecorderAcceptsBuiltinAndPluginWithoutWorkspaceRows(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	repository.revisions = nil
	input.DispatchedSkills = []DispatchSkillIdentity{
		{Source: skillbundle.SourceBuiltin, SkillID: "plan"},
		{Source: skillbundle.SourcePlugin, SkillID: "github/review"},
	}
	input.Manifest.Skills = []skillbundle.ExecutionRecord{
		{Source: skillbundle.SourceBuiltin, SkillID: "plan", BundleHash: string(attributionTestDigest('b'))},
		{Source: skillbundle.SourcePlugin, SkillID: "github/review", BundleHash: string(attributionTestDigest('c'))},
	}

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	if report.Eligibility != EvidenceEligibilityEligible || report.Reason != AttributionReasonNoWorkspaceSkill || report.Recorded != 0 || len(report.Outcomes) != 2 {
		t.Fatalf("report = %+v, want eligible report with no workspace rows", report)
	}
	if repository.resolveCalls != 0 || repository.recordCalls != 0 {
		t.Fatalf("repository calls = resolve %d record %d, want none", repository.resolveCalls, repository.recordCalls)
	}
}

func TestAttributionRecorderResolvesOnlyWorkspaceRecordsInMixedManifest(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	input.DispatchedSkills = append(input.DispatchedSkills,
		DispatchSkillIdentity{Source: skillbundle.SourceBuiltin, SkillID: "plan"},
		DispatchSkillIdentity{Source: skillbundle.SourcePlugin, SkillID: "github/review"},
	)
	input.Manifest.Skills = append(input.Manifest.Skills,
		skillbundle.ExecutionRecord{Source: skillbundle.SourceBuiltin, SkillID: "plan", BundleHash: string(attributionTestDigest('b'))},
		skillbundle.ExecutionRecord{Source: skillbundle.SourcePlugin, SkillID: "github/review", BundleHash: string(attributionTestDigest('c'))},
	)

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	if report.Eligibility != EvidenceEligibilityEligible || report.Recorded != 1 || len(report.Outcomes) != 3 {
		t.Fatalf("report = %+v, want one workspace row and three validated outcomes", report)
	}
	if repository.resolveCalls != 1 || repository.recordCalls != 1 || len(repository.rows) != 1 {
		t.Fatalf("repository calls/rows = %d/%d/%d, want 1/1/1", repository.resolveCalls, repository.recordCalls, len(repository.rows))
	}
}

func TestAttributionRecorderRejectsUnknownSources(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	input.DispatchedSkills[0].Source = "remote"
	input.Manifest.Skills[0].Source = "remote"

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonInvalidDispatch)
	if repository.resolveCalls != 0 || repository.recordCalls != 0 {
		t.Fatal("unknown source must be rejected before persistence")
	}
}

func TestAttributionRecorderUsesStoreRevisionAndIdempotencyQueries(t *testing.T) {
	pool := skillEvolutionTestPool(t)
	ctx := context.Background()
	queries := db.New(pool)
	store := NewStore(queries, pool)
	workspaceID, userID := testUUID(), testUUID()
	skillID, agentID, runtimeID, taskID := testUUID(), testUUID(), testUUID(), testUUID()
	seedPersistenceFixture(t, pool, workspaceID, userID, skillID, agentID)
	if _, err := pool.Exec(ctx, `INSERT INTO agent_runtime (id, workspace_id) VALUES ($1, $2)`, runtimeID, workspaceID); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := pool.Exec(ctx, `INSERT INTO agent_task_queue (id, agent_id, runtime_id, status, dispatched_at) VALUES ($1, $2, $3, 'dispatched', $4)`, taskID, agentID, runtimeID, dispatchedAt); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	revisionInput := testRevisionInput(t, workspaceID, skillID, "base", "base content")
	revisionInput.CreatedByID = userID
	revision, err := store.SaveRevision(ctx, revisionInput)
	if err != nil {
		t.Fatalf("save revision: %v", err)
	}
	skillIDString := attributionUUIDString(skillID)
	snapshot, err := store.RecordTaskDispatchSnapshot(ctx, TaskDispatchSnapshotInput{
		WorkspaceID:      workspaceID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeID:        runtimeID,
		TaskDispatchedAt: dispatchedAt,
		Skills:           []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: skillIDString}},
	})
	if err != nil {
		t.Fatalf("record dispatch snapshot: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, taskID); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	input := AttributionInput{
		WorkspaceID: workspaceID, TaskID: taskID, RuntimeID: runtimeID,
		DispatchSnapshotID: snapshot.Snapshot.ID, TaskDispatchedAt: dispatchedAt, CapabilityProven: true,
		DispatchedSkills: []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: skillIDString}},
		Manifest: skillbundle.ExecutionManifest{
			Version: skillbundle.ExecutionManifestVersion,
			Skills: []skillbundle.ExecutionRecord{{
				Source: skillbundle.SourceWorkspace, SkillID: skillIDString,
				BundleHash: string(revisionInput.BundleHash), RevisionID: attributionUUIDString(revision.Revision.ID),
			}},
		},
	}
	recorder := NewAttributionRecorder(store)

	first := recorder.Record(ctx, input)
	second := recorder.Record(ctx, input)

	if first.Eligibility != EvidenceEligibilityEligible || second.Eligibility != EvidenceEligibilityEligible {
		t.Fatalf("reports = %+v / %+v, want eligible", first, second)
	}
	rows, err := queries.ListSkillEvolutionTaskAttributions(ctx, db.ListSkillEvolutionTaskAttributionsParams{
		WorkspaceID: workspaceID,
		TaskID:      taskID,
	})
	if err != nil {
		t.Fatalf("list attribution rows: %v", err)
	}
	if len(rows) != 1 || rows[0].RevisionID != revision.Revision.ID || rows[0].BundleHash != string(revisionInput.BundleHash) {
		t.Fatalf("attribution rows = %+v, want one exact revision", rows)
	}
}

func TestAttributionRecorderRejectsMissingRevision(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	repository.revisions = nil

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonRevisionNotFound)
	if repository.recordCalls != 0 || len(repository.rows) != 0 {
		t.Fatal("missing revision must not persist partial attribution")
	}
}

func TestAttributionRecorderRejectsAmbiguousRevision(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	repository.revisions = append(repository.revisions, repository.revisions[0])

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonRevisionAmbiguous)
	if repository.recordCalls != 0 {
		t.Fatal("ambiguous revision must not be recorded")
	}
}

func TestAttributionRecorderRejectsRevisionHashMismatch(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	input.Manifest.Skills[0].RevisionID = attributionUUIDString(repository.revisions[0].ID)
	repository.revisions[0].BundleHash = attributionTestDigest('b')

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonRevisionHashMismatch)
	if repository.recordCalls != 0 {
		t.Fatal("hash mismatch must not be recorded")
	}
}

func TestAttributionRecorderRejectsNonUUIDRevisionClaim(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	input.Manifest.Skills[0].RevisionID = "not-an-evolution-revision"

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonRevisionNotProven)
	if repository.resolveCalls != 0 || repository.recordCalls != 0 {
		t.Fatal("invalid revision identity must not reach persistence")
	}
}

func TestAttributionRecorderDuplicateCallbackIsIdempotent(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	recorder := newAttributionRecorder(repository)

	first := recorder.Record(context.Background(), input)
	second := recorder.Record(context.Background(), input)

	if first.Eligibility != EvidenceEligibilityEligible || second.Eligibility != EvidenceEligibilityEligible {
		t.Fatalf("reports = %+v / %+v, want eligible", first, second)
	}
	if repository.recordCalls != 2 || len(repository.rows) != 1 {
		t.Fatalf("record calls/rows = %d/%d, want 2/1", repository.recordCalls, len(repository.rows))
	}
}

func TestAttributionRecorderContainsUnprovenTaskDispatch(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	repository.recordError = ErrPersistenceNotFound

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonTaskDispatchNotProven)
	if len(repository.rows) != 0 {
		t.Fatal("unproven task dispatch must not leave attribution rows")
	}
}

func TestAttributionRecorderConflictingRetryFailsClosed(t *testing.T) {
	repository := newFakeAttributionRepository()
	first := validRecorderInput(t, repository)
	recorder := newAttributionRecorder(repository)
	if report := recorder.Record(context.Background(), first); report.Eligibility != EvidenceEligibilityEligible {
		t.Fatalf("first report = %+v", report)
	}

	conflicting := first
	conflicting.Manifest.Skills = append([]skillbundle.ExecutionRecord(nil), first.Manifest.Skills...)
	conflicting.Manifest.Skills[0].BundleHash = string(attributionTestDigest('b'))
	repository.revisions = []attributionRevision{{
		ID: repository.revisions[0].ID, WorkspaceID: first.WorkspaceID,
		SkillID: repository.revisions[0].SkillID, Source: skillbundle.SourceWorkspace,
		BundleHash: attributionTestDigest('b'), OwnershipClass: OwnershipWorkspace,
	}}

	report := recorder.Record(context.Background(), conflicting)

	assertIneligibleAttribution(t, report, AttributionReasonPersistenceConflict)
	if len(repository.rows) != 1 {
		t.Fatalf("rows = %d, want original row only", len(repository.rows))
	}
	for _, row := range repository.rows {
		if row.BundleHash != attributionTestDigest('a') {
			t.Fatalf("stored hash = %q, want original", row.BundleHash)
		}
	}
}

func TestAttributionRecorderScopesRevisionToWorkspace(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	repository.revisions[0].WorkspaceID = attributionTestUUID()

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonRevisionNotFound)
	if repository.recordCalls != 0 {
		t.Fatal("cross-workspace revision must not be recorded")
	}
}

func TestAttributionRecorderRejectsIncompleteBundleList(t *testing.T) {
	repository := newFakeAttributionRepository()
	input := validRecorderInput(t, repository)
	input.DispatchedSkills = append(input.DispatchedSkills, DispatchSkillIdentity{
		Source:  skillbundle.SourceWorkspace,
		SkillID: uuid.NewString(),
	})

	report := newAttributionRecorder(repository).Record(context.Background(), input)

	assertIneligibleAttribution(t, report, AttributionReasonIncompleteManifest)
	if repository.resolveCalls != 0 || repository.recordCalls != 0 {
		t.Fatal("incomplete manifest must be rejected before revision lookup")
	}
}

func TestExecutionManifestNormalizerRejectsBeforeAttribution(t *testing.T) {
	skillID := uuid.NewString()
	validRecord := `{"source":"workspace","skill_id":"` + skillID + `","bundle_hash":"` + string(attributionTestDigest('a')) + `"}`
	tests := []struct {
		name string
		raw  string
	}{
		{name: "unknown version", raw: `{"version":2,"skills":[` + validRecord + `]}`},
		{name: "incomplete record", raw: `{"version":1,"skills":[{"source":"workspace","skill_id":"` + skillID + `"}]}`},
		{name: "duplicate identity", raw: `{"version":1,"skills":[` + validRecord + `,` + validRecord + `]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := skillbundle.NormalizeExecutionManifest(json.RawMessage(tc.raw)); err == nil {
				t.Fatal("NormalizeExecutionManifest succeeded, want rejection before recorder")
			}
		})
	}
}

type fakeAttributionRepository struct {
	revisions    []attributionRevision
	rows         map[string]TaskAttributionInput
	resolveCalls int
	recordCalls  int
	recordError  error
}

func newFakeAttributionRepository() *fakeAttributionRepository {
	return &fakeAttributionRepository{rows: make(map[string]TaskAttributionInput)}
}

func (f *fakeAttributionRepository) resolveAttributionRevisions(_ context.Context, match attributionRevisionMatch) ([]attributionRevision, error) {
	f.resolveCalls++
	result := make([]attributionRevision, 0, len(f.revisions))
	for _, revision := range f.revisions {
		if revision.WorkspaceID != match.WorkspaceID {
			continue
		}
		if match.RevisionID.Valid && revision.ID != match.RevisionID {
			continue
		}
		if !match.RevisionID.Valid && (revision.SkillID != match.SkillID || revision.BundleHash != match.BundleHash) {
			continue
		}
		result = append(result, revision)
	}
	return result, nil
}

func (f *fakeAttributionRepository) recordAttributionBatch(_ context.Context, inputs []TaskAttributionInput) error {
	f.recordCalls++
	if f.recordError != nil {
		return f.recordError
	}
	pending := make(map[string]TaskAttributionInput, len(inputs))
	for _, input := range inputs {
		key := attributionUUIDString(input.WorkspaceID) + "\x00" + attributionUUIDString(input.TaskID) + "\x00" + attributionUUIDString(input.SkillID)
		if existing, ok := f.rows[key]; ok && existing != input {
			return ErrPersistenceConflict
		}
		pending[key] = input
	}
	for key, input := range pending {
		f.rows[key] = input
	}
	return nil
}

func validRecorderInput(t *testing.T, repository *fakeAttributionRepository) AttributionInput {
	t.Helper()
	workspaceID := attributionTestUUID()
	taskID := attributionTestUUID()
	runtimeID := attributionTestUUID()
	skillID := attributionTestUUID()
	revisionID := attributionTestUUID()
	hash := attributionTestDigest('a')
	repository.revisions = []attributionRevision{{
		ID: revisionID, WorkspaceID: workspaceID, SkillID: skillID,
		Source: skillbundle.SourceWorkspace, BundleHash: hash, OwnershipClass: OwnershipWorkspace,
	}}
	skillIDString := attributionUUIDString(skillID)
	return AttributionInput{
		WorkspaceID: workspaceID, TaskID: taskID, RuntimeID: runtimeID,
		DispatchSnapshotID: attributionTestUUID(), TaskDispatchedAt: time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC),
		CapabilityProven: true,
		DispatchedSkills: []DispatchSkillIdentity{{Source: skillbundle.SourceWorkspace, SkillID: skillIDString}},
		Manifest: skillbundle.ExecutionManifest{
			Version: skillbundle.ExecutionManifestVersion,
			Skills: []skillbundle.ExecutionRecord{{
				Source: skillbundle.SourceWorkspace, SkillID: skillIDString, BundleHash: string(hash),
			}},
		},
	}
}

func assertIneligibleAttribution(t *testing.T, report AttributionReport, reason AttributionReason) {
	t.Helper()
	if report.Eligibility != EvidenceEligibilityIneligible || report.Reason != reason || report.Recorded != 0 {
		t.Fatalf("report = %+v, want ineligible %q with no records", report, reason)
	}
}

func attributionTestUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}

func attributionTestDigest(fill byte) Digest {
	return Digest("sha256:" + string(makeFilledBytes(fill, 64)))
}

func makeFilledBytes(fill byte, length int) []byte {
	value := make([]byte, length)
	for index := range value {
		value[index] = fill
	}
	return value
}
