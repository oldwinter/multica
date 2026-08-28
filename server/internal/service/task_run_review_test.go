package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	taskReviewWorkspaceID      = "11111111-1111-4111-8111-111111111111"
	taskReviewOtherWorkspaceID = "22222222-2222-4222-8222-222222222222"
	taskReviewReviewerID       = "33333333-3333-4333-8333-333333333333"
	taskReviewTaskID           = "44444444-4444-4444-8444-444444444444"
	taskReviewSourceTaskID     = "55555555-5555-4555-8555-555555555555"
	taskReviewAgentID          = "66666666-6666-4666-8666-666666666666"
	taskReviewSourceAgentID    = "77777777-7777-4777-8777-777777777777"
	taskReviewSkillID          = "88888888-8888-4888-8888-888888888888"
	taskReviewReviewID         = "99999999-9999-4999-8999-999999999999"
	taskReviewForbiddenTaskID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	taskReviewIssueID          = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	taskReviewOtherIssueID     = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
)

var taskReviewNow = time.Date(2026, 8, 28, 10, 11, 12, 13, time.UTC)

type fakeTaskRunReviewRepository struct {
	created       []TaskRunReviewRecord
	refs          TaskRunReviewRefPage
	records       map[string]TaskRunReviewRecord
	manual        []ManualRerunRecord
	manualRecords map[string]ManualRerunRecord
}

func (f *fakeTaskRunReviewRepository) CreateTaskRunReview(_ context.Context, record TaskRunReviewRecord) error {
	f.created = append(f.created, record)
	if f.records == nil {
		f.records = make(map[string]TaskRunReviewRecord)
	}
	f.records[record.ID] = record
	return nil
}

func (f *fakeTaskRunReviewRepository) ListTaskRunReviewRefs(context.Context, string, string, int) (TaskRunReviewRefPage, error) {
	return f.refs, nil
}

func (f *fakeTaskRunReviewRepository) LoadTaskRunReview(_ context.Context, _ string, id string) (TaskRunReviewRecord, error) {
	record, ok := f.records[id]
	if !ok {
		return TaskRunReviewRecord{}, ErrTaskRunReviewNotFound
	}
	return record, nil
}

func (f *fakeTaskRunReviewRepository) ListManualReruns(context.Context, string, string, int) ([]ManualRerunRecord, string, error) {
	return f.manual, "", nil
}

func (f *fakeTaskRunReviewRepository) LoadManualRerun(_ context.Context, _ string, id string) (ManualRerunRecord, error) {
	record, ok := f.manualRecords[id]
	if !ok {
		return ManualRerunRecord{}, ErrTaskRunReviewNotFound
	}
	return record, nil
}

type fakeTaskRunReviewAccess struct {
	tasks     map[string]TaskRunReviewTask
	forbidden map[string]bool
	skills    map[string]bool
	loads     []string
	memberErr error
}

func (f *fakeTaskRunReviewAccess) ValidateWorkspaceMember(context.Context, string, string) error {
	return f.memberErr
}

func (f *fakeTaskRunReviewAccess) LoadAuthorizedTask(_ context.Context, workspaceID, _ string, taskID string) (TaskRunReviewTask, error) {
	f.loads = append(f.loads, taskID)
	if f.forbidden[taskID] {
		return TaskRunReviewTask{}, ErrTaskRunReviewForbidden
	}
	task, ok := f.tasks[taskID]
	if !ok || task.WorkspaceID != workspaceID {
		return TaskRunReviewTask{}, ErrTaskRunReviewNotFound
	}
	return task, nil
}

func (f *fakeTaskRunReviewAccess) ValidateTargetSkill(_ context.Context, _ string, skillID string) error {
	if !f.skills[skillID] {
		return ErrTaskRunReviewNotFound
	}
	return nil
}

func newTaskRunReviewServiceForTest(repo *fakeTaskRunReviewRepository, access *fakeTaskRunReviewAccess) *TaskRunReviewService {
	svc := NewTaskRunReviewService(repo, access)
	svc.newID = func() string { return taskReviewReviewID }
	svc.now = func() time.Time { return taskReviewNow }
	return svc
}

func terminalReviewTask(status string) TaskRunReviewTask {
	return TaskRunReviewTask{ID: taskReviewTaskID, WorkspaceID: taskReviewWorkspaceID, AgentID: taskReviewAgentID, IssueID: taskReviewIssueID, Status: status}
}

func validCreateTaskRunReviewInput() CreateTaskRunReviewInput {
	return CreateTaskRunReviewInput{
		TaskID: taskReviewTaskID, Outcome: TaskRunReviewOutcomeNeedsCorrection,
		Target: TaskRunReviewTargetSkillProcedure, SkillID: taskReviewSkillID,
		Correction: "Use the bounded retry policy.", Reason: "The run retried without a cap.",
	}
}

func TestTaskRunReviewRequiresTerminalAuthorizedTask(t *testing.T) {
	for _, test := range []struct {
		status  string
		wantErr error
	}{
		{status: "queued", wantErr: ErrTaskRunReviewTaskActive},
		{status: "running", wantErr: ErrTaskRunReviewTaskActive},
		{status: "completed"},
		{status: "failed"},
		{status: "cancelled"},
	} {
		t.Run(test.status, func(t *testing.T) {
			repo := &fakeTaskRunReviewRepository{}
			access := &fakeTaskRunReviewAccess{
				tasks:  map[string]TaskRunReviewTask{taskReviewTaskID: terminalReviewTask(test.status)},
				skills: map[string]bool{taskReviewSkillID: true},
			}
			got, err := newTaskRunReviewServiceForTest(repo, access).CreateTaskRunReview(
				context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, validCreateTaskRunReviewInput(),
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CreateTaskRunReview() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				if len(repo.created) != 0 {
					t.Fatal("non-terminal task persisted a review")
				}
				return
			}
			if got.ID != taskReviewReviewID || got.Digest == "" || len(repo.created) != 1 {
				t.Fatalf("created evidence = %#v, records = %d", got, len(repo.created))
			}
		})
	}

	access := &fakeTaskRunReviewAccess{
		tasks:     map[string]TaskRunReviewTask{taskReviewTaskID: terminalReviewTask("completed")},
		forbidden: map[string]bool{taskReviewTaskID: true}, skills: map[string]bool{taskReviewSkillID: true},
	}
	_, err := newTaskRunReviewServiceForTest(&fakeTaskRunReviewRepository{}, access).CreateTaskRunReview(
		context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, validCreateTaskRunReviewInput(),
	)
	if !errors.Is(err, ErrTaskRunReviewForbidden) {
		t.Fatalf("private task error = %v, want forbidden", err)
	}
}

func TestTaskRunReviewValidatesClosedTargetAndBoundedReason(t *testing.T) {
	base := validCreateTaskRunReviewInput()
	tests := []struct {
		name   string
		mutate func(*CreateTaskRunReviewInput)
	}{
		{name: "unknown target", mutate: func(in *CreateTaskRunReviewInput) { in.Target = "runtime" }},
		{name: "skill on knowledge target", mutate: func(in *CreateTaskRunReviewInput) { in.Target = TaskRunReviewTargetKnowledge }},
		{name: "correction required", mutate: func(in *CreateTaskRunReviewInput) { in.Correction = "" }},
		{name: "reason required", mutate: func(in *CreateTaskRunReviewInput) { in.Reason = "" }},
		{name: "reason bounded", mutate: func(in *CreateTaskRunReviewInput) { in.Reason = string(make([]byte, MaxTaskRunReviewTextBytes+1)) }},
		{name: "controls rejected", mutate: func(in *CreateTaskRunReviewInput) { in.Reason = "bad\x01reason" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.mutate(&input)
			_, err := newTaskRunReviewServiceForTest(&fakeTaskRunReviewRepository{}, &fakeTaskRunReviewAccess{}).CreateTaskRunReview(
				context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, input,
			)
			if !errors.Is(err, ErrTaskRunReviewInvalid) {
				t.Fatalf("error = %v, want invalid", err)
			}
		})
	}
}

func TestTaskRunReviewLoadReauthorizesAndRevalidatesDigest(t *testing.T) {
	repo := &fakeTaskRunReviewRepository{}
	access := &fakeTaskRunReviewAccess{
		tasks:  map[string]TaskRunReviewTask{taskReviewTaskID: terminalReviewTask("completed")},
		skills: map[string]bool{taskReviewSkillID: true},
	}
	svc := newTaskRunReviewServiceForTest(repo, access)
	created, err := svc.CreateTaskRunReview(context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, validCreateTaskRunReviewInput())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := svc.LoadTaskRunReviewEvidence(context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, created.ID)
	if err != nil || loaded.Digest != created.Digest {
		t.Fatalf("LoadTaskRunReviewEvidence() = %#v, %v", loaded, err)
	}

	mutated := repo.records[created.ID]
	mutated.Correction = "A different correction"
	repo.records[created.ID] = mutated
	if _, err := svc.LoadTaskRunReviewEvidence(context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, created.ID); !errors.Is(err, ErrTaskRunReviewSourceChanged) {
		t.Fatalf("mutated review error = %v, want source changed", err)
	}

	if _, ok := reflect.TypeOf(TaskRunReviewRef{}).FieldByName("Correction"); ok {
		t.Fatal("TaskRunReviewRef must not expose correction content")
	}
	if _, ok := reflect.TypeOf(TaskRunReviewRef{}).FieldByName("Reason"); ok {
		t.Fatal("TaskRunReviewRef must not expose reason content")
	}
}

func TestTaskRunReviewCanonicalizesUUIDsBeforeAuthorizationAndDigest(t *testing.T) {
	repo := &fakeTaskRunReviewRepository{}
	access := &fakeTaskRunReviewAccess{
		tasks:  map[string]TaskRunReviewTask{taskReviewTaskID: terminalReviewTask("completed")},
		skills: map[string]bool{taskReviewSkillID: true},
	}
	input := validCreateTaskRunReviewInput()
	input.TaskID = strings.ToUpper(input.TaskID)
	input.SkillID = strings.ToUpper(input.SkillID)
	evidence, err := newTaskRunReviewServiceForTest(repo, access).CreateTaskRunReview(
		context.Background(), strings.ToUpper(taskReviewWorkspaceID), strings.ToUpper(taskReviewReviewerID), input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.WorkspaceID != taskReviewWorkspaceID || evidence.TaskID != taskReviewTaskID || evidence.SkillID != taskReviewSkillID || evidence.ReviewerID != taskReviewReviewerID {
		t.Fatalf("noncanonical UUIDs survived boundary normalization: %#v", evidence)
	}
}

func TestTaskRunReviewListRequiresWorkspaceMembershipBeforeRepositoryRead(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "removed member", err: ErrTaskRunReviewForbidden},
		{name: "membership store unavailable", err: ErrTaskRunReviewUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			access := &fakeTaskRunReviewAccess{memberErr: test.err}
			_, err := newTaskRunReviewServiceForTest(&fakeTaskRunReviewRepository{}, access).ListTaskRunReviewRefs(
				context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, "", 10,
			)
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
			if len(access.loads) != 0 {
				t.Fatalf("task rows loaded before workspace authorization: %v", access.loads)
			}
		})
	}
}

func validTaskRunReviewRef(id, taskID string) TaskRunReviewRef {
	return TaskRunReviewRef{
		ID: id, WorkspaceID: taskReviewWorkspaceID, TaskID: taskID, ReviewerID: taskReviewReviewerID,
		Outcome: TaskRunReviewOutcomeHelpful, Target: TaskRunReviewTargetKnowledge,
		Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", CreatedAt: taskReviewNow,
	}
}

func TestTaskRunReviewListHidesUnauthorizedPrivateAgentRefs(t *testing.T) {
	repo := &fakeTaskRunReviewRepository{refs: TaskRunReviewRefPage{Refs: []TaskRunReviewRef{
		validTaskRunReviewRef(taskReviewReviewID, taskReviewTaskID),
		validTaskRunReviewRef("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", taskReviewForbiddenTaskID),
	}}}
	access := &fakeTaskRunReviewAccess{
		tasks:     map[string]TaskRunReviewTask{taskReviewTaskID: terminalReviewTask("completed")},
		forbidden: map[string]bool{taskReviewForbiddenTaskID: true},
	}
	page, err := newTaskRunReviewServiceForTest(repo, access).ListTaskRunReviewRefs(
		context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, "", 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Refs) != 1 || page.Refs[0].TaskID != taskReviewTaskID {
		t.Fatalf("refs = %#v, want only authorized task", page.Refs)
	}
}

func validManualRerunRecord() ManualRerunRecord {
	return ManualRerunRecord{
		TaskID: taskReviewTaskID, WorkspaceID: taskReviewWorkspaceID, AgentID: taskReviewAgentID, Status: "running",
		SourceTaskID: taskReviewSourceTaskID, SourceWorkspaceID: taskReviewWorkspaceID,
		SourceAgentID: taskReviewAgentID, IssueID: taskReviewIssueID, SourceIssueID: taskReviewIssueID,
		SourceStatus:  "completed",
		RerunOfTaskID: taskReviewSourceTaskID, OriginatorUserID: taskReviewReviewerID,
		OriginatorSource: "direct_human", CreatedAt: taskReviewNow,
	}
}

func manualRerunAccess() *fakeTaskRunReviewAccess {
	return &fakeTaskRunReviewAccess{tasks: map[string]TaskRunReviewTask{
		taskReviewTaskID:       {ID: taskReviewTaskID, WorkspaceID: taskReviewWorkspaceID, AgentID: taskReviewAgentID, IssueID: taskReviewIssueID, Status: "running"},
		taskReviewSourceTaskID: {ID: taskReviewSourceTaskID, WorkspaceID: taskReviewWorkspaceID, AgentID: taskReviewAgentID, IssueID: taskReviewIssueID, Status: "completed"},
	}}
}

func TestManualRerunUsesOnlyRerunLineageAndClassifiesCorrectionRequest(t *testing.T) {
	record := validManualRerunRecord()
	repo := &fakeTaskRunReviewRepository{manualRecords: map[string]ManualRerunRecord{record.TaskID: record}}
	evidence, err := newTaskRunReviewServiceForTest(repo, manualRerunAccess()).LoadManualRerunEvidence(
		context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, record.TaskID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Classification != ManualRerunClassificationCorrectionRequested {
		t.Fatalf("classification = %q, want correction_requested", evidence.Classification)
	}
	if evidence.Classification == "success" || evidence.Digest == "" {
		t.Fatalf("manual rerun was treated as success: %#v", evidence)
	}
	if evidence.RequestedByUserID != taskReviewReviewerID {
		t.Fatalf("requested_by_user_id = %q, want persisted human originator", evidence.RequestedByUserID)
	}

	retry := record
	retry.RetryOfTaskID = taskReviewSourceTaskID
	repo.manualRecords[retry.TaskID] = retry
	if _, err := newTaskRunReviewServiceForTest(repo, manualRerunAccess()).LoadManualRerunEvidence(
		context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, retry.TaskID,
	); !errors.Is(err, ErrTaskRunReviewSourceChanged) {
		t.Fatalf("retry lineage error = %v, want source changed", err)
	}
}

func TestManualRerunRequiresDirectHumanOriginator(t *testing.T) {
	for _, source := range []string{"", "delegation", "rule_owner"} {
		record := validManualRerunRecord()
		record.OriginatorSource = source
		repo := &fakeTaskRunReviewRepository{manualRecords: map[string]ManualRerunRecord{record.TaskID: record}}
		if _, err := newTaskRunReviewServiceForTest(repo, manualRerunAccess()).LoadManualRerunEvidence(
			context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, record.TaskID,
		); !errors.Is(err, ErrTaskRunReviewSourceChanged) {
			t.Fatalf("originator source %q: error = %v, want source changed", source, err)
		}
	}
}

func TestManualRerunIndependentlyValidatesSourceAndChildWorkspace(t *testing.T) {
	record := validManualRerunRecord()
	record.SourceWorkspaceID = taskReviewOtherWorkspaceID
	repo := &fakeTaskRunReviewRepository{manualRecords: map[string]ManualRerunRecord{record.TaskID: record}}
	if _, err := newTaskRunReviewServiceForTest(repo, manualRerunAccess()).LoadManualRerunEvidence(
		context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, record.TaskID,
	); !errors.Is(err, ErrTaskRunReviewSourceChanged) {
		t.Fatalf("cross-workspace source error = %v, want source changed", err)
	}

	record = validManualRerunRecord()
	access := manualRerunAccess()
	source := access.tasks[taskReviewSourceTaskID]
	source.WorkspaceID = taskReviewOtherWorkspaceID
	access.tasks[taskReviewSourceTaskID] = source
	repo.manualRecords[record.TaskID] = record
	if _, err := newTaskRunReviewServiceForTest(repo, access).LoadManualRerunEvidence(
		context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, record.TaskID,
	); !errors.Is(err, ErrTaskRunReviewNotFound) {
		t.Fatalf("authorized reload cross-workspace error = %v, want not found", err)
	}
}

func TestManualRerunRejectsUnrelatedNoFKLineage(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ManualRerunRecord, *fakeTaskRunReviewAccess)
	}{
		{name: "different agent", mutate: func(record *ManualRerunRecord, access *fakeTaskRunReviewAccess) {
			record.SourceAgentID = taskReviewSourceAgentID
			source := access.tasks[taskReviewSourceTaskID]
			source.AgentID = taskReviewSourceAgentID
			access.tasks[taskReviewSourceTaskID] = source
		}},
		{name: "different issue", mutate: func(record *ManualRerunRecord, access *fakeTaskRunReviewAccess) {
			record.SourceIssueID = taskReviewOtherIssueID
			source := access.tasks[taskReviewSourceTaskID]
			source.IssueID = taskReviewOtherIssueID
			access.tasks[taskReviewSourceTaskID] = source
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := validManualRerunRecord()
			access := manualRerunAccess()
			test.mutate(&record, access)
			repo := &fakeTaskRunReviewRepository{manualRecords: map[string]ManualRerunRecord{record.TaskID: record}}
			if _, err := newTaskRunReviewServiceForTest(repo, access).LoadManualRerunEvidence(
				context.Background(), taskReviewWorkspaceID, taskReviewReviewerID, record.TaskID,
			); !errors.Is(err, ErrTaskRunReviewSourceChanged) {
				t.Fatalf("unrelated lineage error = %v, want source changed", err)
			}
		})
	}
}
