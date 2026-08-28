package service

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTaskRunReviewRepositoryCursorRoundTrip(t *testing.T) {
	createdAt := pgtype.Timestamptz{Time: time.Date(2026, 8, 28, 20, 15, 4, 123456000, time.UTC), Valid: true}
	id := util.MustParseUUID(taskReviewReviewID)
	cursor := encodeTaskRunReviewCursor(createdAt, id)
	gotTime, gotID, err := decodeTaskRunReviewCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if !gotTime.Valid || !gotTime.Time.Equal(createdAt.Time) || gotID != id {
		t.Fatalf("decoded cursor = (%+v, %+v), want (%+v, %+v)", gotTime, gotID, createdAt, id)
	}
	for _, invalid := range []string{"v2:abc", "v1:not-base64!", "v1:"} {
		if _, _, err := decodeTaskRunReviewCursor(invalid); !errors.Is(err, ErrTaskRunReviewInvalid) {
			t.Fatalf("decode %q error = %v, want invalid", invalid, err)
		}
	}
}

func TestTaskRunReviewRepositoryMapsContentFreeAndManualRows(t *testing.T) {
	createdAt := pgtype.Timestamptz{Time: taskReviewNow, Valid: true}
	review := taskRunReviewRef(db.ListTaskRunReviewRecordsRow{
		ID: util.MustParseUUID(taskReviewReviewID), WorkspaceID: util.MustParseUUID(taskReviewWorkspaceID),
		TaskID: util.MustParseUUID(taskReviewTaskID), ReviewerID: util.MustParseUUID(taskReviewReviewerID),
		Outcome: string(TaskRunReviewOutcomeHelpful), Target: string(TaskRunReviewTargetKnowledge),
		Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", CreatedAt: createdAt,
	})
	if review.ID != taskReviewReviewID || review.SkillID != "" || review.CreatedAt != taskReviewNow {
		t.Fatalf("review ref = %#v", review)
	}

	manualRow := db.ListManualRerunsRow{
		TaskID: util.MustParseUUID(taskReviewTaskID), WorkspaceID: util.MustParseUUID(taskReviewWorkspaceID),
		SourceTaskID: util.MustParseUUID(taskReviewSourceTaskID), SourceWorkspaceID: util.MustParseUUID(taskReviewWorkspaceID),
		AgentID: util.MustParseUUID(taskReviewAgentID), SourceAgentID: util.MustParseUUID(taskReviewAgentID),
		IssueID: util.MustParseUUID(taskReviewIssueID), SourceIssueID: util.MustParseUUID(taskReviewIssueID),
		Status: "running", SourceStatus: "completed", RerunOfTaskID: util.MustParseUUID(taskReviewSourceTaskID),
		OriginatorUserID: util.MustParseUUID(taskReviewReviewerID),
		OriginatorSource: pgtype.Text{String: "direct_human", Valid: true}, CreatedAt: createdAt,
	}
	manual := manualRerunRecordFromList(manualRow)
	if manual.TaskID != taskReviewTaskID || manual.SourceAgentID != taskReviewAgentID ||
		manual.OriginatorUserID != taskReviewReviewerID || manual.OriginatorSource != "direct_human" || manual.RetryOfTaskID != "" {
		t.Fatalf("manual rerun = %#v", manual)
	}
}
