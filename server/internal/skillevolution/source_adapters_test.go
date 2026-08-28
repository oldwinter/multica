package skillevolution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/service"
)

type taskReviewSignalsStub struct {
	ref      service.TaskRunReviewRef
	evidence service.TaskRunReviewEvidence
}

func (stub *taskReviewSignalsStub) ListTaskRunReviewRefs(context.Context, string, string, string, int) (service.TaskRunReviewRefPage, error) {
	return service.TaskRunReviewRefPage{Refs: []service.TaskRunReviewRef{stub.ref}}, nil
}

func (stub *taskReviewSignalsStub) LoadTaskRunReviewEvidence(context.Context, string, string, string) (service.TaskRunReviewEvidence, error) {
	return stub.evidence, nil
}

type manualSignalsStub struct {
	ref      service.ManualRerunRef
	evidence service.ManualRerunEvidence
}

func (stub *manualSignalsStub) ListManualRerunRefs(context.Context, string, string, string, int) (service.ManualRerunPage, error) {
	return service.ManualRerunPage{Refs: []service.ManualRerunRef{stub.ref}}, nil
}

func (stub *manualSignalsStub) LoadManualRerunEvidence(context.Context, string, string, string) (service.ManualRerunEvidence, error) {
	return stub.evidence, nil
}

type exactSkillTaskIndexStub struct {
	eligible bool
	tasks    []pgtype.UUID
}

func (stub *exactSkillTaskIndexStub) ListExactSkillTaskIDs(context.Context, pgtype.UUID, pgtype.UUID, int) ([]pgtype.UUID, error) {
	return append([]pgtype.UUID(nil), stub.tasks...), nil
}

type twinSignalsStub struct{ feedback service.TwinFeedbackSignalRef }

func (stub twinSignalsStub) ListFeedbackRefs(context.Context, pgtype.UUID, pgtype.UUID, int) ([]service.TwinFeedbackSignalRef, error) {
	return []service.TwinFeedbackSignalRef{stub.feedback}, nil
}

func (stub twinSignalsStub) LoadFeedbackEvidence(context.Context, pgtype.UUID, service.TwinFeedbackSignalRef) (service.TwinFeedbackSignalEvidence, error) {
	return service.TwinFeedbackSignalEvidence{Ref: stub.feedback, Rating: stub.feedback.State}, nil
}

func (twinSignalsStub) ListAcceptedDepositionRefs(context.Context, pgtype.UUID, pgtype.UUID, int) ([]service.TwinAcceptedDepositionSignalRef, error) {
	return nil, nil
}

func (twinSignalsStub) LoadAcceptedDepositionEvidence(context.Context, pgtype.UUID, service.TwinAcceptedDepositionSignalRef) (service.TwinAcceptedDepositionSignalEvidence, error) {
	return service.TwinAcceptedDepositionSignalEvidence{}, nil
}

func (stub *exactSkillTaskIndexStub) HasExactSkillTask(context.Context, pgtype.UUID, pgtype.UUID, pgtype.UUID) (bool, error) {
	return stub.eligible, nil
}

func TestTaskReviewSourceTargetsOneSkillAndRevalidatesDigest(t *testing.T) {
	workspaceID, actorID, skillID, taskID, reviewID := testUUID(), testUUID(), testUUID(), testUUID(), testUUID()
	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := service.TaskRunReviewRef{
		ID: uuidText(reviewID), WorkspaceID: uuidText(workspaceID), TaskID: uuidText(taskID), ReviewerID: uuidText(actorID),
		Outcome: service.TaskRunReviewOutcomeNeedsCorrection, Target: service.TaskRunReviewTargetSkillProcedure,
		SkillID: uuidText(skillID), Digest: string(testDigest("task-source")), CreatedAt: now,
	}
	stub := &taskReviewSignalsStub{ref: ref, evidence: service.TaskRunReviewEvidence{TaskRunReviewRecord: service.TaskRunReviewRecord{
		ID: ref.ID, WorkspaceID: ref.WorkspaceID, TaskID: ref.TaskID, ReviewerID: ref.ReviewerID,
		Outcome: ref.Outcome, Target: ref.Target, SkillID: ref.SkillID, Digest: ref.Digest, CreatedAt: ref.CreatedAt,
		Correction: "keep the change bounded", Reason: "the prior run changed unrelated files",
	}}}
	source := NewTaskReviewSignalSource(stub)
	query := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, ActorID: actorID, Limit: 5}
	refs, err := source.List(context.Background(), query)
	if err != nil || len(refs) != 1 || refs[0].TargetSkillID != uuidText(skillID) {
		t.Fatalf("list = (%+v, %v)", refs, err)
	}
	resolved, err := source.Load(context.Background(), query, refs[0])
	if err != nil || len(resolved.Payload) == 0 {
		t.Fatalf("load = (%s, %v)", resolved.Payload, err)
	}
	stub.evidence.Digest = string(testDigest("changed"))
	if _, err := source.Load(context.Background(), query, refs[0]); !errors.Is(err, ErrSignalSourceDrift) {
		t.Fatalf("drift error = %v", err)
	}
}

func TestManualRerunSourceRequiresExactAttributionAtListAndLoad(t *testing.T) {
	workspaceID, actorID, skillID, taskID, sourceTaskID := testUUID(), testUUID(), testUUID(), testUUID(), testUUID()
	ref := service.ManualRerunRef{
		WorkspaceID: uuidText(workspaceID), TaskID: uuidText(taskID), SourceTaskID: uuidText(sourceTaskID),
		Classification: service.ManualRerunClassificationCorrectionRequested,
		Digest:         string(testDigest("rerun")), ObservedAt: time.Now().UTC(),
	}
	stub := &manualSignalsStub{ref: ref, evidence: service.ManualRerunEvidence{
		ManualRerunRef: ref, RequestedByUserID: uuid.NewString(), TaskStatus: "completed", SourceTaskStatus: "completed",
	}}
	index := &exactSkillTaskIndexStub{eligible: true}
	source := NewManualRerunSignalSource(stub, index)
	query := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, ActorID: actorID, Limit: 5}
	refs, err := source.List(context.Background(), query)
	if err != nil || len(refs) != 1 || refs[0].TargetSkillID != uuidText(skillID) {
		t.Fatalf("eligible list = (%+v, %v)", refs, err)
	}
	index.eligible = false
	if _, err := source.Load(context.Background(), query, refs[0]); !errors.Is(err, ErrSignalSourceDrift) {
		t.Fatalf("lost-attribution error = %v", err)
	}
	refs, err = source.List(context.Background(), query)
	if err != nil || len(refs) != 0 {
		t.Fatalf("ineligible list = (%+v, %v)", refs, err)
	}
}

func TestTwinSourceFactoryBindsEveryReadToCurrentActor(t *testing.T) {
	workspaceID, skillID, taskID, actorID := testUUID(), testUUID(), testUUID(), testUUID()
	feedback := service.TwinFeedbackSignalRef{
		WorkspaceID: workspaceID, FeedbackID: testUUID(), TaskID: taskID, AttributionID: testUUID(), TwinVersionID: testUUID(),
		State: "helped", Digest: string(testDigest("twin-feedback")), ObservedAt: time.Now().UTC(),
	}
	var actors []pgtype.UUID
	factory := TwinSignalsFactory(func(actor pgtype.UUID) TwinSignals {
		actors = append(actors, actor)
		return twinSignalsStub{feedback: feedback}
	})
	source := NewTwinFeedbackSignalSource(factory, &exactSkillTaskIndexStub{eligible: true, tasks: []pgtype.UUID{taskID}})
	withoutActor := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, Limit: 5}
	refs, err := source.List(context.Background(), withoutActor)
	if err != nil || len(refs) != 0 || len(actors) != 0 {
		t.Fatalf("actor-less list = (%+v, %v), factory actors=%v", refs, err, actors)
	}
	query := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, ActorID: actorID, Limit: 5}
	refs, err = source.List(context.Background(), query)
	if err != nil || len(refs) != 1 || len(actors) != 1 || actors[0] != actorID {
		t.Fatalf("authorized list = (%+v, %v), factory actors=%v", refs, err, actors)
	}
	if _, err := source.Load(context.Background(), query, refs[0]); err != nil {
		t.Fatal(err)
	}
	if len(actors) != 2 || actors[1] != actorID {
		t.Fatalf("load factory actors = %v", actors)
	}
}
