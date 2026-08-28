package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

const (
	MaxTaskRunReviewTextBytes = 4096
	MaxTaskRunReviewRefs      = 100
	MaxTaskRunReviewCursorLen = 256
)

var (
	ErrTaskRunReviewInvalid       = errors.New("invalid task run review")
	ErrTaskRunReviewNotFound      = errors.New("task run review source not found")
	ErrTaskRunReviewForbidden     = errors.New("task run review source is not authorized")
	ErrTaskRunReviewTaskActive    = errors.New("task run review requires a terminal task")
	ErrTaskRunReviewSourceChanged = errors.New("task run review source changed")
	ErrTaskRunReviewUnavailable   = errors.New("task run review source is unavailable")
)

type TaskRunReviewOutcome string

const (
	TaskRunReviewOutcomeHelpful         TaskRunReviewOutcome = "helpful"
	TaskRunReviewOutcomeNeedsCorrection TaskRunReviewOutcome = "needs_correction"
)

func (o TaskRunReviewOutcome) Valid() bool {
	return o == TaskRunReviewOutcomeHelpful || o == TaskRunReviewOutcomeNeedsCorrection
}

// TaskRunReviewTarget is deliberately task-owned. Skill evolution may adapt
// skill_procedure reviews, but the task domain remains authoritative for the
// human classification and does not depend on an evolution package.
type TaskRunReviewTarget string

const (
	TaskRunReviewTargetKnowledge      TaskRunReviewTarget = "knowledge"
	TaskRunReviewTargetTwinAssertion  TaskRunReviewTarget = "twin_assertion"
	TaskRunReviewTargetSkillProcedure TaskRunReviewTarget = "skill_procedure"
	TaskRunReviewTargetProductDefect  TaskRunReviewTarget = "product_defect"
)

func (t TaskRunReviewTarget) Valid() bool {
	switch t {
	case TaskRunReviewTargetKnowledge, TaskRunReviewTargetTwinAssertion,
		TaskRunReviewTargetSkillProcedure, TaskRunReviewTargetProductDefect:
		return true
	default:
		return false
	}
}

type CreateTaskRunReviewInput struct {
	TaskID     string               `json:"-"`
	Outcome    TaskRunReviewOutcome `json:"outcome"`
	Target     TaskRunReviewTarget  `json:"target"`
	SkillID    string               `json:"skill_id,omitempty"`
	Correction string               `json:"correction,omitempty"`
	Reason     string               `json:"reason"`
}

type TaskRunReviewRecord struct {
	ID          string               `json:"id"`
	WorkspaceID string               `json:"workspace_id"`
	TaskID      string               `json:"task_id"`
	ReviewerID  string               `json:"reviewer_id"`
	Outcome     TaskRunReviewOutcome `json:"outcome"`
	Target      TaskRunReviewTarget  `json:"target"`
	SkillID     string               `json:"skill_id,omitempty"`
	Correction  string               `json:"correction,omitempty"`
	Reason      string               `json:"reason"`
	Digest      string               `json:"digest"`
	CreatedAt   time.Time            `json:"created_at"`
}

// TaskRunReviewRef is the only list projection. It cannot carry correction or
// reason content; callers must use LoadTaskRunReviewEvidence, which repeats
// workspace/private-agent authorization and digest validation.
type TaskRunReviewRef struct {
	ID          string               `json:"id"`
	WorkspaceID string               `json:"workspace_id"`
	TaskID      string               `json:"task_id"`
	ReviewerID  string               `json:"reviewer_id"`
	Outcome     TaskRunReviewOutcome `json:"outcome"`
	Target      TaskRunReviewTarget  `json:"target"`
	SkillID     string               `json:"skill_id,omitempty"`
	Digest      string               `json:"digest"`
	CreatedAt   time.Time            `json:"created_at"`
}

type TaskRunReviewEvidence struct {
	TaskRunReviewRecord
}

type TaskRunReviewRefPage struct {
	Refs       []TaskRunReviewRef `json:"refs"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type TaskRunReviewTask struct {
	ID            string
	WorkspaceID   string
	AgentID       string
	IssueID       string
	ChatSessionID string
	Status        string
}

const ManualRerunClassificationCorrectionRequested = "correction_requested"

// ManualRerunRecord is a content-free source projection built from task queue
// lineage. Repository implementations must independently scope both child and
// source rows to WorkspaceID rather than trusting the no-FK lineage pointer.
type ManualRerunRecord struct {
	TaskID              string
	WorkspaceID         string
	AgentID             string
	Status              string
	SourceTaskID        string
	SourceWorkspaceID   string
	SourceAgentID       string
	IssueID             string
	SourceIssueID       string
	ChatSessionID       string
	SourceChatSessionID string
	SourceStatus        string
	RerunOfTaskID       string
	RetryOfTaskID       string
	OriginatorUserID    string
	OriginatorSource    string
	CreatedAt           time.Time
}

type ManualRerunRef struct {
	WorkspaceID    string    `json:"workspace_id"`
	TaskID         string    `json:"task_id"`
	SourceTaskID   string    `json:"source_task_id"`
	Classification string    `json:"classification"`
	Digest         string    `json:"digest"`
	ObservedAt     time.Time `json:"observed_at"`
}

type ManualRerunEvidence struct {
	ManualRerunRef
	RequestedByUserID string `json:"requested_by_user_id"`
	TaskStatus        string `json:"task_status"`
	SourceTaskStatus  string `json:"source_task_status"`
}

type ManualRerunPage struct {
	Refs       []ManualRerunRef `json:"refs"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type TaskRunReviewRepository interface {
	CreateTaskRunReview(context.Context, TaskRunReviewRecord) error
	ListTaskRunReviewRefs(context.Context, string, string, int) (TaskRunReviewRefPage, error)
	LoadTaskRunReview(context.Context, string, string) (TaskRunReviewRecord, error)
	ListManualReruns(context.Context, string, string, int) ([]ManualRerunRecord, string, error)
	LoadManualRerun(context.Context, string, string) (ManualRerunRecord, error)
}

// TaskRunReviewTaskAccess is implemented at the HTTP/task boundary so it can
// reuse the same private-Agent visibility decision as task transcripts.
type TaskRunReviewTaskAccess interface {
	ValidateWorkspaceMember(context.Context, string, string) error
	LoadAuthorizedTask(context.Context, string, string, string) (TaskRunReviewTask, error)
	ValidateTargetSkill(context.Context, string, string) error
}

type TaskRunReviewService struct {
	repository TaskRunReviewRepository
	access     TaskRunReviewTaskAccess
	newID      func() string
	now        func() time.Time
}

func NewTaskRunReviewService(repository TaskRunReviewRepository, access TaskRunReviewTaskAccess) *TaskRunReviewService {
	return &TaskRunReviewService{
		repository: repository,
		access:     access,
		newID:      func() string { return util.UUIDToString(dbid.NewV7()) },
		now:        time.Now,
	}
}

func (s *TaskRunReviewService) CreateTaskRunReview(ctx context.Context, workspaceID, reviewerID string, input CreateTaskRunReviewInput) (TaskRunReviewEvidence, error) {
	if s == nil || s.repository == nil || s.access == nil {
		return TaskRunReviewEvidence{}, ErrTaskRunReviewInvalid
	}
	var err error
	if workspaceID, err = canonicalTaskRunReviewID(workspaceID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if reviewerID, err = canonicalTaskRunReviewID(reviewerID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if input.TaskID, err = canonicalTaskRunReviewID(input.TaskID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if input.SkillID != "" {
		if input.SkillID, err = canonicalTaskRunReviewID(input.SkillID); err != nil {
			return TaskRunReviewEvidence{}, err
		}
	}
	input.Correction = strings.TrimSpace(input.Correction)
	input.Reason = strings.TrimSpace(input.Reason)
	if err := validateTaskRunReviewInput(input); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if err := s.access.ValidateWorkspaceMember(ctx, workspaceID, reviewerID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	task, err := s.access.LoadAuthorizedTask(ctx, workspaceID, reviewerID, input.TaskID)
	if err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if err := validateAuthorizedTask(task, workspaceID, input.TaskID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if !terminalTaskRunReviewStatus(task.Status) {
		return TaskRunReviewEvidence{}, ErrTaskRunReviewTaskActive
	}
	if input.SkillID != "" {
		if err := s.access.ValidateTargetSkill(ctx, workspaceID, input.SkillID); err != nil {
			return TaskRunReviewEvidence{}, err
		}
	}

	record := TaskRunReviewRecord{
		ID: s.newID(), WorkspaceID: workspaceID, TaskID: input.TaskID, ReviewerID: reviewerID,
		Outcome: input.Outcome, Target: input.Target, SkillID: input.SkillID,
		Correction: input.Correction, Reason: input.Reason,
		// PostgreSQL timestamptz preserves microseconds. Freeze the same precision
		// before hashing so an immediate load does not look like source drift.
		CreatedAt: s.now().UTC().Truncate(time.Microsecond),
	}
	record.Digest, err = canonicalTaskRunReviewDigest(record)
	if err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if err := s.repository.CreateTaskRunReview(ctx, record); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	return TaskRunReviewEvidence{TaskRunReviewRecord: record}, nil
}

func (s *TaskRunReviewService) ListTaskRunReviewRefs(ctx context.Context, workspaceID, reviewerID, cursor string, limit int) (TaskRunReviewRefPage, error) {
	if s == nil || s.repository == nil || s.access == nil {
		return TaskRunReviewRefPage{}, ErrTaskRunReviewInvalid
	}
	var err error
	if workspaceID, err = canonicalTaskRunReviewID(workspaceID); err != nil {
		return TaskRunReviewRefPage{}, err
	}
	if reviewerID, err = canonicalTaskRunReviewID(reviewerID); err != nil || !validListRequest(workspaceID, reviewerID, cursor, limit) {
		return TaskRunReviewRefPage{}, ErrTaskRunReviewInvalid
	}
	if err := s.access.ValidateWorkspaceMember(ctx, workspaceID, reviewerID); err != nil {
		return TaskRunReviewRefPage{}, err
	}
	page, err := s.repository.ListTaskRunReviewRefs(ctx, workspaceID, cursor, limit)
	if err != nil {
		return TaskRunReviewRefPage{}, err
	}
	if len(page.Refs) > limit || !validCursor(page.NextCursor) {
		return TaskRunReviewRefPage{}, ErrTaskRunReviewSourceChanged
	}
	refs := make([]TaskRunReviewRef, 0, len(page.Refs))
	for _, ref := range page.Refs {
		if err := validateTaskRunReviewRef(ref, workspaceID); err != nil {
			return TaskRunReviewRefPage{}, err
		}
		task, err := s.access.LoadAuthorizedTask(ctx, workspaceID, reviewerID, ref.TaskID)
		if err != nil {
			if errors.Is(err, ErrTaskRunReviewForbidden) || errors.Is(err, ErrTaskRunReviewNotFound) {
				continue
			}
			return TaskRunReviewRefPage{}, err
		}
		if err := validateAuthorizedTask(task, workspaceID, ref.TaskID); err != nil {
			return TaskRunReviewRefPage{}, err
		}
		if !terminalTaskRunReviewStatus(task.Status) {
			return TaskRunReviewRefPage{}, ErrTaskRunReviewSourceChanged
		}
		if ref.SkillID != "" {
			if err := s.access.ValidateTargetSkill(ctx, workspaceID, ref.SkillID); err != nil {
				if errors.Is(err, ErrTaskRunReviewForbidden) || errors.Is(err, ErrTaskRunReviewNotFound) {
					continue
				}
				return TaskRunReviewRefPage{}, err
			}
		}
		refs = append(refs, ref)
	}
	page.Refs = refs
	return page, nil
}

func (s *TaskRunReviewService) LoadTaskRunReviewEvidence(ctx context.Context, workspaceID, reviewerID, reviewID string) (TaskRunReviewEvidence, error) {
	if s == nil || s.repository == nil || s.access == nil {
		return TaskRunReviewEvidence{}, ErrTaskRunReviewInvalid
	}
	var err error
	if workspaceID, err = canonicalTaskRunReviewID(workspaceID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if reviewerID, err = canonicalTaskRunReviewID(reviewerID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if reviewID, err = canonicalTaskRunReviewID(reviewID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if err := s.access.ValidateWorkspaceMember(ctx, workspaceID, reviewerID); err != nil {
		return TaskRunReviewEvidence{}, err
	}
	record, err := s.repository.LoadTaskRunReview(ctx, workspaceID, reviewID)
	if err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if record.WorkspaceID != workspaceID || record.ID != reviewID {
		return TaskRunReviewEvidence{}, ErrTaskRunReviewSourceChanged
	}
	task, err := s.access.LoadAuthorizedTask(ctx, workspaceID, reviewerID, record.TaskID)
	if err != nil {
		return TaskRunReviewEvidence{}, err
	}
	if err := validateAuthorizedTask(task, workspaceID, record.TaskID); err != nil || !terminalTaskRunReviewStatus(task.Status) {
		return TaskRunReviewEvidence{}, ErrTaskRunReviewSourceChanged
	}
	if record.SkillID != "" {
		if err := s.access.ValidateTargetSkill(ctx, workspaceID, record.SkillID); err != nil {
			return TaskRunReviewEvidence{}, err
		}
	}
	want, err := canonicalTaskRunReviewDigest(record)
	if err != nil || record.Digest != want {
		return TaskRunReviewEvidence{}, ErrTaskRunReviewSourceChanged
	}
	return TaskRunReviewEvidence{TaskRunReviewRecord: record}, nil
}

func (s *TaskRunReviewService) ListManualRerunRefs(ctx context.Context, workspaceID, reviewerID, cursor string, limit int) (ManualRerunPage, error) {
	if s == nil || s.repository == nil || s.access == nil {
		return ManualRerunPage{}, ErrTaskRunReviewInvalid
	}
	var err error
	if workspaceID, err = canonicalTaskRunReviewID(workspaceID); err != nil {
		return ManualRerunPage{}, err
	}
	if reviewerID, err = canonicalTaskRunReviewID(reviewerID); err != nil || !validListRequest(workspaceID, reviewerID, cursor, limit) {
		return ManualRerunPage{}, ErrTaskRunReviewInvalid
	}
	if err := s.access.ValidateWorkspaceMember(ctx, workspaceID, reviewerID); err != nil {
		return ManualRerunPage{}, err
	}
	records, nextCursor, err := s.repository.ListManualReruns(ctx, workspaceID, cursor, limit)
	if err != nil {
		return ManualRerunPage{}, err
	}
	if len(records) > limit || !validCursor(nextCursor) {
		return ManualRerunPage{}, ErrTaskRunReviewSourceChanged
	}
	refs := make([]ManualRerunRef, 0, len(records))
	for _, record := range records {
		ref, err := s.authorizedManualRerun(ctx, workspaceID, reviewerID, record)
		if errors.Is(err, ErrTaskRunReviewForbidden) || errors.Is(err, ErrTaskRunReviewNotFound) {
			continue
		}
		if err != nil {
			return ManualRerunPage{}, err
		}
		refs = append(refs, ref)
	}
	return ManualRerunPage{Refs: refs, NextCursor: nextCursor}, nil
}

func (s *TaskRunReviewService) LoadManualRerunEvidence(ctx context.Context, workspaceID, reviewerID, taskID string) (ManualRerunEvidence, error) {
	if s == nil || s.repository == nil || s.access == nil {
		return ManualRerunEvidence{}, ErrTaskRunReviewInvalid
	}
	var err error
	if workspaceID, err = canonicalTaskRunReviewID(workspaceID); err != nil {
		return ManualRerunEvidence{}, err
	}
	if reviewerID, err = canonicalTaskRunReviewID(reviewerID); err != nil {
		return ManualRerunEvidence{}, err
	}
	if taskID, err = canonicalTaskRunReviewID(taskID); err != nil {
		return ManualRerunEvidence{}, err
	}
	if err := s.access.ValidateWorkspaceMember(ctx, workspaceID, reviewerID); err != nil {
		return ManualRerunEvidence{}, err
	}
	record, err := s.repository.LoadManualRerun(ctx, workspaceID, taskID)
	if err != nil {
		return ManualRerunEvidence{}, err
	}
	ref, err := s.authorizedManualRerun(ctx, workspaceID, reviewerID, record)
	if err != nil {
		return ManualRerunEvidence{}, err
	}
	return ManualRerunEvidence{
		ManualRerunRef: ref, RequestedByUserID: record.OriginatorUserID,
		TaskStatus: record.Status, SourceTaskStatus: record.SourceStatus,
	}, nil
}

func (s *TaskRunReviewService) authorizedManualRerun(ctx context.Context, workspaceID, reviewerID string, record ManualRerunRecord) (ManualRerunRef, error) {
	if err := validateManualRerunRecord(record, workspaceID); err != nil {
		return ManualRerunRef{}, err
	}
	child, err := s.access.LoadAuthorizedTask(ctx, workspaceID, reviewerID, record.TaskID)
	if err != nil {
		return ManualRerunRef{}, err
	}
	source, err := s.access.LoadAuthorizedTask(ctx, workspaceID, reviewerID, record.SourceTaskID)
	if err != nil {
		return ManualRerunRef{}, err
	}
	if err := validateAuthorizedTask(child, workspaceID, record.TaskID); err != nil ||
		validateAuthorizedTask(source, workspaceID, record.SourceTaskID) != nil ||
		child.AgentID != record.AgentID || child.Status != record.Status ||
		source.AgentID != record.SourceAgentID || source.Status != record.SourceStatus ||
		child.AgentID != source.AgentID || child.IssueID != source.IssueID ||
		child.ChatSessionID != source.ChatSessionID ||
		child.IssueID != record.IssueID || source.IssueID != record.SourceIssueID ||
		child.ChatSessionID != record.ChatSessionID || source.ChatSessionID != record.SourceChatSessionID ||
		!terminalTaskRunReviewStatus(source.Status) {
		return ManualRerunRef{}, ErrTaskRunReviewSourceChanged
	}
	digest, err := canonicalManualRerunDigest(record)
	if err != nil {
		return ManualRerunRef{}, err
	}
	return ManualRerunRef{
		WorkspaceID: workspaceID, TaskID: record.TaskID, SourceTaskID: record.SourceTaskID,
		Classification: ManualRerunClassificationCorrectionRequested,
		Digest:         digest, ObservedAt: record.CreatedAt.UTC(),
	}, nil
}

func validateTaskRunReviewInput(input CreateTaskRunReviewInput) error {
	if !input.Outcome.Valid() || !input.Target.Valid() || !validReviewText(input.Reason, true) || !validReviewText(input.Correction, false) {
		return ErrTaskRunReviewInvalid
	}
	if input.Outcome == TaskRunReviewOutcomeNeedsCorrection && input.Correction == "" {
		return ErrTaskRunReviewInvalid
	}
	if input.SkillID != "" && (!validTaskRunReviewID(input.SkillID) || input.Target != TaskRunReviewTargetSkillProcedure) {
		return ErrTaskRunReviewInvalid
	}
	return nil
}

func validateTaskRunReviewRef(ref TaskRunReviewRef, workspaceID string) error {
	if ref.WorkspaceID != workspaceID || !validTaskRunReviewID(ref.ID) || !validTaskRunReviewID(ref.TaskID) ||
		!validTaskRunReviewID(ref.ReviewerID) || !ref.Outcome.Valid() || !ref.Target.Valid() ||
		(ref.SkillID != "" && (!validTaskRunReviewID(ref.SkillID) || ref.Target != TaskRunReviewTargetSkillProcedure)) ||
		!validSHA256Digest(ref.Digest) || ref.CreatedAt.IsZero() {
		return ErrTaskRunReviewSourceChanged
	}
	return nil
}

func validateManualRerunRecord(record ManualRerunRecord, workspaceID string) error {
	if record.WorkspaceID != workspaceID || record.SourceWorkspaceID != workspaceID ||
		!validTaskRunReviewID(record.TaskID) || !validTaskRunReviewID(record.SourceTaskID) ||
		!validTaskRunReviewID(record.AgentID) || !validTaskRunReviewID(record.SourceAgentID) ||
		!validOptionalTaskRunReviewID(record.IssueID) || !validOptionalTaskRunReviewID(record.SourceIssueID) ||
		!validOptionalTaskRunReviewID(record.ChatSessionID) || !validOptionalTaskRunReviewID(record.SourceChatSessionID) ||
		!validTaskRunReviewID(record.OriginatorUserID) || record.OriginatorSource != "direct_human" ||
		record.RerunOfTaskID != record.SourceTaskID ||
		record.RetryOfTaskID != "" || record.TaskID == record.SourceTaskID || record.CreatedAt.IsZero() ||
		record.AgentID != record.SourceAgentID || record.IssueID != record.SourceIssueID ||
		record.ChatSessionID != record.SourceChatSessionID ||
		!validTaskRunReviewStatus(record.Status) || !validTaskRunReviewStatus(record.SourceStatus) {
		return ErrTaskRunReviewSourceChanged
	}
	return nil
}

func validateAuthorizedTask(task TaskRunReviewTask, workspaceID, taskID string) error {
	if task.ID != taskID || task.WorkspaceID != workspaceID || !validTaskRunReviewID(task.AgentID) ||
		!validOptionalTaskRunReviewID(task.IssueID) || !validOptionalTaskRunReviewID(task.ChatSessionID) ||
		!validTaskRunReviewStatus(task.Status) {
		return ErrTaskRunReviewSourceChanged
	}
	return nil
}

func terminalTaskRunReviewStatus(status string) bool {
	return status == "completed" || status == "failed" || status == "cancelled"
}

func validTaskRunReviewStatus(status string) bool {
	if status == "" || len(status) > 64 {
		return false
	}
	for _, r := range status {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validListRequest(workspaceID, reviewerID, cursor string, limit int) bool {
	return validTaskRunReviewID(workspaceID) && validTaskRunReviewID(reviewerID) && validCursor(cursor) && limit > 0 && limit <= MaxTaskRunReviewRefs
}

func validCursor(cursor string) bool {
	return len(cursor) <= MaxTaskRunReviewCursorLen && utf8.ValidString(cursor) && strings.IndexByte(cursor, 0) < 0
}

func validTaskRunReviewID(value string) bool {
	canonical, err := canonicalTaskRunReviewID(value)
	return err == nil && canonical == value
}

func validOptionalTaskRunReviewID(value string) bool {
	return value == "" || validTaskRunReviewID(value)
}

func canonicalTaskRunReviewID(value string) (string, error) {
	id, err := util.ParseUUID(value)
	if err != nil {
		return "", ErrTaskRunReviewInvalid
	}
	return util.UUIDToString(id), nil
}

func validReviewText(value string, required bool) bool {
	if value == "" {
		return !required
	}
	if len(value) > MaxTaskRunReviewTextBytes || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func canonicalTaskRunReviewDigest(record TaskRunReviewRecord) (string, error) {
	if !validTaskRunReviewID(record.ID) || !validTaskRunReviewID(record.WorkspaceID) ||
		!validTaskRunReviewID(record.TaskID) || !validTaskRunReviewID(record.ReviewerID) ||
		record.CreatedAt.IsZero() || validateTaskRunReviewInput(CreateTaskRunReviewInput{
		TaskID: record.TaskID, Outcome: record.Outcome, Target: record.Target, SkillID: record.SkillID,
		Correction: record.Correction, Reason: record.Reason,
	}) != nil {
		return "", ErrTaskRunReviewInvalid
	}
	return canonicalTaskSignalDigest("task-run-review-v1", []string{
		record.ID, record.WorkspaceID, record.TaskID, record.ReviewerID,
		string(record.Outcome), string(record.Target), record.SkillID,
		record.Correction, record.Reason, record.CreatedAt.UTC().Format(time.RFC3339Nano),
	}), nil
}

func canonicalManualRerunDigest(record ManualRerunRecord) (string, error) {
	if err := validateManualRerunRecord(record, record.WorkspaceID); err != nil {
		return "", err
	}
	return canonicalTaskSignalDigest("manual-rerun-v1", []string{
		record.WorkspaceID, record.TaskID, record.AgentID, record.Status,
		record.SourceTaskID, record.SourceAgentID, record.SourceStatus,
		record.IssueID, record.SourceIssueID, record.ChatSessionID, record.SourceChatSessionID,
		record.OriginatorUserID, record.OriginatorSource, record.CreatedAt.UTC().Format(time.RFC3339Nano),
		ManualRerunClassificationCorrectionRequested,
	}), nil
}

func canonicalTaskSignalDigest(namespace string, values []string) string {
	h := sha256.New()
	writeTaskSignalDigestValue(h, namespace)
	for _, value := range values {
		writeTaskSignalDigestValue(h, value)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func writeTaskSignalDigestValue(h interface{ Write([]byte) (int, error) }, value string) {
	_, _ = fmt.Fprintf(h, "%d:%s\n", len(value), value)
}

func validSHA256Digest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	raw := strings.TrimPrefix(value, "sha256:")
	if raw != strings.ToLower(raw) {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}
