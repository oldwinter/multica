package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const taskRunReviewCursorPrefix = "v1:"

// DBTaskRunReviewRepository owns the task-domain persistence projection. It
// intentionally returns content-free list rows; full review text is loaded by
// ID only after the service repeats task authorization.
type DBTaskRunReviewRepository struct {
	queries *db.Queries
}

func NewDBTaskRunReviewRepository(queries *db.Queries) *DBTaskRunReviewRepository {
	return &DBTaskRunReviewRepository{queries: queries}
}

var _ TaskRunReviewRepository = (*DBTaskRunReviewRepository)(nil)

func (r *DBTaskRunReviewRepository) CreateTaskRunReview(ctx context.Context, record TaskRunReviewRecord) (TaskRunReviewRecord, error) {
	if r == nil || r.queries == nil {
		return TaskRunReviewRecord{}, ErrTaskRunReviewUnavailable
	}
	wantDigest, err := canonicalTaskRunReviewDigest(record)
	if err != nil || record.Digest != wantDigest {
		return TaskRunReviewRecord{}, ErrTaskRunReviewInvalid
	}
	params, err := taskRunReviewCreateParams(record)
	if err != nil {
		return TaskRunReviewRecord{}, err
	}
	created, err := r.queries.CreateTaskRunReview(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskRunReviewRecord{}, ErrTaskRunReviewSourceChanged
	}
	if err != nil {
		return TaskRunReviewRecord{}, taskRunReviewRepositoryError(err)
	}
	return taskRunReviewRecord(created), nil
}

func (r *DBTaskRunReviewRepository) ListTaskRunReviewRefs(ctx context.Context, workspaceID, cursor string, limit int) (TaskRunReviewRefPage, error) {
	if r == nil || r.queries == nil {
		return TaskRunReviewRefPage{}, ErrTaskRunReviewUnavailable
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil || limit < 1 || limit > MaxTaskRunReviewRefs {
		return TaskRunReviewRefPage{}, ErrTaskRunReviewInvalid
	}
	afterCreatedAt, afterID, err := decodeTaskRunReviewCursor(cursor)
	if err != nil {
		return TaskRunReviewRefPage{}, err
	}
	rows, err := r.queries.ListTaskRunReviewRecords(ctx, db.ListTaskRunReviewRecordsParams{
		WorkspaceID: workspaceUUID, AfterCreatedAt: afterCreatedAt, AfterID: afterID, PageSize: int32(limit + 1),
	})
	if err != nil {
		return TaskRunReviewRefPage{}, taskRunReviewRepositoryError(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	refs := make([]TaskRunReviewRef, len(rows))
	for index, row := range rows {
		refs[index] = taskRunReviewRef(row)
		if err := validateTaskRunReviewRef(refs[index], workspaceID); err != nil {
			return TaskRunReviewRefPage{}, ErrTaskRunReviewSourceChanged
		}
	}
	nextCursor := ""
	if hasMore {
		nextCursor = encodeTaskRunReviewCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID)
	}
	return TaskRunReviewRefPage{Refs: refs, NextCursor: nextCursor}, nil
}

func (r *DBTaskRunReviewRepository) LoadTaskRunReview(ctx context.Context, workspaceID, reviewID string) (TaskRunReviewRecord, error) {
	if r == nil || r.queries == nil {
		return TaskRunReviewRecord{}, ErrTaskRunReviewUnavailable
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		return TaskRunReviewRecord{}, ErrTaskRunReviewInvalid
	}
	reviewUUID, err := util.ParseUUID(reviewID)
	if err != nil {
		return TaskRunReviewRecord{}, ErrTaskRunReviewInvalid
	}
	row, err := r.queries.LoadTaskRunReview(ctx, db.LoadTaskRunReviewParams{WorkspaceID: workspaceUUID, ID: reviewUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskRunReviewRecord{}, ErrTaskRunReviewNotFound
	}
	if err != nil {
		return TaskRunReviewRecord{}, taskRunReviewRepositoryError(err)
	}
	record := taskRunReviewRecord(row)
	if record.WorkspaceID != workspaceID || record.ID != reviewID {
		return TaskRunReviewRecord{}, ErrTaskRunReviewSourceChanged
	}
	return record, nil
}

func (r *DBTaskRunReviewRepository) ListManualReruns(ctx context.Context, workspaceID, cursor string, limit int) ([]ManualRerunRecord, string, error) {
	if r == nil || r.queries == nil {
		return nil, "", ErrTaskRunReviewUnavailable
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil || limit < 1 || limit > MaxTaskRunReviewRefs {
		return nil, "", ErrTaskRunReviewInvalid
	}
	afterCreatedAt, afterID, err := decodeTaskRunReviewCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	rows, err := r.queries.ListManualReruns(ctx, db.ListManualRerunsParams{
		WorkspaceID: workspaceUUID, AfterCreatedAt: afterCreatedAt, AfterID: afterID, PageSize: int32(limit + 1),
	})
	if err != nil {
		return nil, "", taskRunReviewRepositoryError(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	records := make([]ManualRerunRecord, len(rows))
	for index, row := range rows {
		records[index] = manualRerunRecordFromList(row)
		if err := validateManualRerunRecord(records[index], workspaceID); err != nil {
			return nil, "", ErrTaskRunReviewSourceChanged
		}
	}
	nextCursor := ""
	if hasMore {
		nextCursor = encodeTaskRunReviewCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].TaskID)
	}
	return records, nextCursor, nil
}

func (r *DBTaskRunReviewRepository) LoadManualRerun(ctx context.Context, workspaceID, taskID string) (ManualRerunRecord, error) {
	if r == nil || r.queries == nil {
		return ManualRerunRecord{}, ErrTaskRunReviewUnavailable
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		return ManualRerunRecord{}, ErrTaskRunReviewInvalid
	}
	taskUUID, err := util.ParseUUID(taskID)
	if err != nil {
		return ManualRerunRecord{}, ErrTaskRunReviewInvalid
	}
	row, err := r.queries.LoadManualRerun(ctx, db.LoadManualRerunParams{WorkspaceID: workspaceUUID, TaskID: taskUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ManualRerunRecord{}, ErrTaskRunReviewNotFound
	}
	if err != nil {
		return ManualRerunRecord{}, taskRunReviewRepositoryError(err)
	}
	record := manualRerunRecordFromLoad(row)
	if err := validateManualRerunRecord(record, workspaceID); err != nil || record.TaskID != taskID {
		return ManualRerunRecord{}, ErrTaskRunReviewSourceChanged
	}
	return record, nil
}

func taskRunReviewCreateParams(record TaskRunReviewRecord) (db.CreateTaskRunReviewParams, error) {
	id, err := util.ParseUUID(record.ID)
	if err != nil {
		return db.CreateTaskRunReviewParams{}, ErrTaskRunReviewInvalid
	}
	workspaceID, err := util.ParseUUID(record.WorkspaceID)
	if err != nil {
		return db.CreateTaskRunReviewParams{}, ErrTaskRunReviewInvalid
	}
	taskID, err := util.ParseUUID(record.TaskID)
	if err != nil {
		return db.CreateTaskRunReviewParams{}, ErrTaskRunReviewInvalid
	}
	reviewerID, err := util.ParseUUID(record.ReviewerID)
	if err != nil {
		return db.CreateTaskRunReviewParams{}, ErrTaskRunReviewInvalid
	}
	var skillID pgtype.UUID
	if record.SkillID != "" {
		skillID, err = util.ParseUUID(record.SkillID)
		if err != nil {
			return db.CreateTaskRunReviewParams{}, ErrTaskRunReviewInvalid
		}
	}
	return db.CreateTaskRunReviewParams{
		ID: id, WorkspaceID: workspaceID, TaskID: taskID, ReviewerID: reviewerID,
		Outcome: string(record.Outcome), Target: string(record.Target), SkillID: skillID,
		Correction: pgtype.Text{String: record.Correction, Valid: record.Correction != ""},
		Reason:     record.Reason, IdempotencyKey: record.IdempotencyKey, Digest: record.Digest,
		CreatedAt: pgtype.Timestamptz{Time: record.CreatedAt, Valid: true},
	}, nil
}

func taskRunReviewRecord(row db.TaskRunReview) TaskRunReviewRecord {
	return TaskRunReviewRecord{
		ID: util.UUIDToString(row.ID), WorkspaceID: util.UUIDToString(row.WorkspaceID),
		TaskID: util.UUIDToString(row.TaskID), ReviewerID: util.UUIDToString(row.ReviewerID),
		Outcome: TaskRunReviewOutcome(row.Outcome), Target: TaskRunReviewTarget(row.Target),
		SkillID: util.UUIDToString(row.SkillID), Correction: nullableTaskRunReviewText(row.Correction),
		Reason: row.Reason, IdempotencyKey: row.IdempotencyKey, Digest: row.Digest, CreatedAt: nullableTaskRunReviewTime(row.CreatedAt),
	}
}

func taskRunReviewRef(row db.ListTaskRunReviewRecordsRow) TaskRunReviewRef {
	return TaskRunReviewRef{
		ID: util.UUIDToString(row.ID), WorkspaceID: util.UUIDToString(row.WorkspaceID),
		TaskID: util.UUIDToString(row.TaskID), ReviewerID: util.UUIDToString(row.ReviewerID),
		Outcome: TaskRunReviewOutcome(row.Outcome), Target: TaskRunReviewTarget(row.Target),
		SkillID: util.UUIDToString(row.SkillID), Digest: row.Digest,
		CreatedAt: nullableTaskRunReviewTime(row.CreatedAt),
	}
}

func manualRerunRecordFromList(row db.ListManualRerunsRow) ManualRerunRecord {
	return buildManualRerunRecord(
		row.TaskID, row.WorkspaceID, row.SourceTaskID, row.SourceWorkspaceID,
		row.AgentID, row.SourceAgentID, row.IssueID, row.SourceIssueID,
		row.ChatSessionID, row.SourceChatSessionID, row.Status, row.SourceStatus,
		row.RerunOfTaskID, row.RetryOfTaskID, row.OriginatorUserID,
		nullableTaskRunReviewText(row.OriginatorSource), row.CreatedAt,
	)
}

func manualRerunRecordFromLoad(row db.LoadManualRerunRow) ManualRerunRecord {
	return buildManualRerunRecord(
		row.TaskID, row.WorkspaceID, row.SourceTaskID, row.SourceWorkspaceID,
		row.AgentID, row.SourceAgentID, row.IssueID, row.SourceIssueID,
		row.ChatSessionID, row.SourceChatSessionID, row.Status, row.SourceStatus,
		row.RerunOfTaskID, row.RetryOfTaskID, row.OriginatorUserID,
		nullableTaskRunReviewText(row.OriginatorSource), row.CreatedAt,
	)
}

func buildManualRerunRecord(
	taskID, workspaceID, sourceTaskID, sourceWorkspaceID pgtype.UUID,
	agentID, sourceAgentID, issueID, sourceIssueID pgtype.UUID,
	chatSessionID, sourceChatSessionID pgtype.UUID,
	status, sourceStatus string,
	rerunOfTaskID, retryOfTaskID, originatorUserID pgtype.UUID,
	originatorSource string,
	createdAt pgtype.Timestamptz,
) ManualRerunRecord {
	return ManualRerunRecord{
		TaskID: util.UUIDToString(taskID), WorkspaceID: util.UUIDToString(workspaceID),
		SourceTaskID: util.UUIDToString(sourceTaskID), SourceWorkspaceID: util.UUIDToString(sourceWorkspaceID),
		AgentID: util.UUIDToString(agentID), SourceAgentID: util.UUIDToString(sourceAgentID),
		IssueID: util.UUIDToString(issueID), SourceIssueID: util.UUIDToString(sourceIssueID),
		ChatSessionID: util.UUIDToString(chatSessionID), SourceChatSessionID: util.UUIDToString(sourceChatSessionID),
		Status: status, SourceStatus: sourceStatus, RerunOfTaskID: util.UUIDToString(rerunOfTaskID),
		RetryOfTaskID: util.UUIDToString(retryOfTaskID), OriginatorUserID: util.UUIDToString(originatorUserID),
		OriginatorSource: originatorSource, CreatedAt: nullableTaskRunReviewTime(createdAt),
	}
}

func sameTaskRunReviewRecord(left, right TaskRunReviewRecord) bool {
	return left.ID == right.ID && left.WorkspaceID == right.WorkspaceID && left.TaskID == right.TaskID &&
		left.ReviewerID == right.ReviewerID && left.Outcome == right.Outcome && left.Target == right.Target &&
		left.SkillID == right.SkillID && left.Correction == right.Correction && left.Reason == right.Reason &&
		left.IdempotencyKey == right.IdempotencyKey && left.Digest == right.Digest && left.CreatedAt.Equal(right.CreatedAt)
}

func nullableTaskRunReviewText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableTaskRunReviewTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}

func encodeTaskRunReviewCursor(createdAt pgtype.Timestamptz, id pgtype.UUID) string {
	if !createdAt.Valid || !id.Valid {
		return ""
	}
	payload := createdAt.Time.UTC().Format(time.RFC3339Nano) + "\n" + util.UUIDToString(id)
	return taskRunReviewCursorPrefix + base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeTaskRunReviewCursor(cursor string) (pgtype.Timestamptz, pgtype.UUID, error) {
	if cursor == "" {
		return pgtype.Timestamptz{}, pgtype.UUID{}, nil
	}
	if len(cursor) > MaxTaskRunReviewCursorLen || !strings.HasPrefix(cursor, taskRunReviewCursorPrefix) {
		return pgtype.Timestamptz{}, pgtype.UUID{}, ErrTaskRunReviewInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(cursor, taskRunReviewCursorPrefix))
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, ErrTaskRunReviewInvalid
	}
	parts := strings.Split(string(raw), "\n")
	if len(parts) != 2 {
		return pgtype.Timestamptz{}, pgtype.UUID{}, ErrTaskRunReviewInvalid
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil || createdAt.IsZero() {
		return pgtype.Timestamptz{}, pgtype.UUID{}, ErrTaskRunReviewInvalid
	}
	id, err := util.ParseUUID(parts[1])
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, ErrTaskRunReviewInvalid
	}
	return pgtype.Timestamptz{Time: createdAt.UTC(), Valid: true}, id, nil
}

func taskRunReviewRepositoryError(err error) error {
	return fmt.Errorf("%w: %v", ErrTaskRunReviewUnavailable, err)
}
