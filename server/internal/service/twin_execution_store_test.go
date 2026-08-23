package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dbfx "github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestValidateTwinTaskAttributionInput(t *testing.T) {
	valid := validTwinTaskAttributionInput()
	tests := []struct {
		name   string
		mutate func(*TwinTaskAttributionInput)
	}{
		{name: "digest mismatch", mutate: func(input *TwinTaskAttributionInput) { input.BriefingDigest = twinExecutionTestDigest("other") }},
		{name: "briefing byte limit", mutate: func(input *TwinTaskAttributionInput) {
			input.Briefing = strings.Repeat("x", twinStoredBriefingMaxBytes+1)
			input.BriefingDigest = TwinBriefingDigest(input.Briefing)
		}},
		{name: "preview is not execution", mutate: func(input *TwinTaskAttributionInput) { input.PolicyState = "preview" }},
		{name: "duplicate assertion", mutate: func(input *TwinTaskAttributionInput) { input.AssertionIDs = []string{"a", "a"} }},
		{name: "wrong agent scope", mutate: func(input *TwinTaskAttributionInput) {
			input.PolicyScopeType = "agent"
			input.PolicyScopeID = twinExecutionTestUUID(9)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if _, _, err := validateTwinTaskAttributionInput(input); !errors.Is(err, ErrTwinExecutionInvalidInput) {
				t.Fatalf("validation error = %v, want ErrTwinExecutionInvalidInput", err)
			}
		})
	}
}

func TestTwinExecutionStoreLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect Twin execution store database: %v", err)
	}
	t.Cleanup(pool.Close)

	root := dbfx.New(pool, "", "")
	userID := root.User(t, "Twin execution owner", "twin-execution-"+time.Now().Format("150405.000000000")+"@example.com")
	workspaceIDString := root.Workspace(t, "Twin execution", "twin-execution-"+time.Now().Format("150405000000000"))
	fixture := dbfx.New(pool, workspaceIDString, userID)
	runtimeIDString := fixture.Runtime(t, "Twin execution runtime")
	agentIDString := fixture.Agent(t, "Twin execution Agent", runtimeIDString)
	issueIDString := fixture.Issue(t, "Twin execution issue")
	taskIDString := fixture.Task(t, agentIDString, dbfx.Cols{
		"runtime_id": runtimeIDString, "issue_id": issueIDString,
		"status": "dispatched", "dispatched_at": dbfx.Raw("now()"),
	})

	queries := db.New(pool)
	workspaceID := twinExecutionUUIDFromString(t, workspaceIDString)
	agentID := twinExecutionUUIDFromString(t, agentIDString)
	runtimeID := twinExecutionUUIDFromString(t, runtimeIDString)
	taskID := twinExecutionUUIDFromString(t, taskIDString)
	actorID := twinExecutionUUIDFromString(t, userID)
	t.Cleanup(func() {
		_ = queries.DeleteWorkspaceTwinExecutionData(context.Background(), workspaceID)
		_ = queries.DeleteWorkspaceWikiTwinData(context.Background(), workspaceID)
	})

	revision, err := queries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID: workspaceID, SourceDigest: twinExecutionTestDigest("source"),
		Content: []byte(`{"schema_version":1}`), TriggerKind: "manual", RequestedByID: actorID,
	})
	if err != nil {
		t.Fatalf("create Twin execution Wiki revision: %v", err)
	}
	proposal, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
		WorkspaceID: workspaceID, Kind: "initial", SourceWikiRevisionID: revision.ID,
		Content:       []byte(`{"schema_version":1,"assertions":[]}`),
		ContentDigest: twinExecutionTestDigest("initial"), RequestedByID: actorID,
	})
	if err != nil {
		t.Fatalf("create Twin execution proposal: %v", err)
	}
	if _, err := queries.CreateTwinProposalReview(ctx, db.CreateTwinProposalReviewParams{
		WorkspaceID: workspaceID, ProposalID: proposal.ID, Decision: "accepted", ReviewerID: actorID,
	}); err != nil {
		t.Fatalf("accept Twin execution proposal: %v", err)
	}
	version, err := queries.CreateTwinVersion(ctx, db.CreateTwinVersionParams{
		WorkspaceID: workspaceID, ProposalID: proposal.ID, SignedOffByID: actorID,
	})
	if err != nil {
		t.Fatalf("sign Twin execution version: %v", err)
	}
	snapshotIssueID := twinExecutionUUIDFromString(t, fixture.Issue(t, "Twin one-off snapshot"))
	snapshotTask, err := queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agentID, RuntimeID: runtimeID, IssueID: snapshotIssueID,
		Priority: 1, TwinUseState: pgtype.Text{String: string(TwinUseEnabled), Valid: true}, TwinVersionID: version.ID,
	})
	if err != nil {
		t.Fatalf("create task with Twin use snapshot: %v", err)
	}
	if snapshotTask.TwinUseState.String != string(TwinUseEnabled) || snapshotTask.TwinVersionID != version.ID {
		t.Fatalf("persisted Twin use snapshot = state %q version %s", snapshotTask.TwinUseState.String, snapshotTask.TwinVersionID.String())
	}

	store := NewTwinExecutionStore(queries)
	binding, err := store.UpsertBinding(ctx, TwinBindingInput{
		WorkspaceID: workspaceID, ScopeType: "workspace", ScopeID: workspaceID,
		State: "enabled", TwinVersionID: version.ID,
	})
	if err != nil {
		t.Fatalf("upsert Twin binding: %v", err)
	}
	updatedBinding, err := store.UpsertBinding(ctx, TwinBindingInput{
		WorkspaceID: workspaceID, ScopeType: "workspace", ScopeID: workspaceID,
		State: "preview", TwinVersionID: version.ID,
	})
	if err != nil || updatedBinding.ID != binding.ID || updatedBinding.State != "preview" {
		t.Fatalf("updated Twin binding = %#v, err = %v", updatedBinding, err)
	}

	task, err := queries.GetAgentTask(ctx, taskID)
	if err != nil {
		t.Fatalf("load claimed task: %v", err)
	}
	briefing := "Use the signed quality bar."
	attributionInput := TwinTaskAttributionInput{
		WorkspaceID: workspaceID, TaskID: taskID, AgentID: agentID,
		RuntimeID: runtimeID, TaskDispatchedAt: task.DispatchedAt,
		TwinVersionID: version.ID, Briefing: briefing,
		BriefingDigest: TwinBriefingDigest(briefing), AssertionIDs: []string{"assertion:quality"},
		CitationKeys: []string{"issue:quality"}, PolicyScopeType: "workspace",
		PolicyScopeID: workspaceID, PolicyState: "enabled", CompilerVersion: "test-v1",
	}
	attribution, err := store.CreateTwinTaskAttributionForClaim(ctx, attributionInput)
	if err != nil {
		t.Fatalf("create Twin task attribution: %v", err)
	}
	repeatedAttribution, err := store.CreateTwinTaskAttributionForClaim(ctx, attributionInput)
	if err != nil || repeatedAttribution.ID != attribution.ID {
		t.Fatalf("repeat Twin task attribution = %#v, err = %v", repeatedAttribution, err)
	}
	conflictingAttribution := attributionInput
	conflictingAttribution.Briefing = "Different briefing"
	conflictingAttribution.BriefingDigest = TwinBriefingDigest(conflictingAttribution.Briefing)
	if _, err := store.CreateTwinTaskAttributionForClaim(ctx, conflictingAttribution); !errors.Is(err, ErrTwinExecutionConflict) {
		t.Fatalf("conflicting Twin attribution error = %v, want conflict", err)
	}

	note := "Useful"
	feedback, err := store.UpsertRunFeedback(ctx, TwinRunFeedbackInput{
		WorkspaceID: workspaceID, TaskID: taskID, Rating: "helped", Note: &note,
	})
	if err != nil {
		t.Fatalf("create Twin feedback: %v", err)
	}
	updatedFeedback, err := store.UpsertRunFeedback(ctx, TwinRunFeedbackInput{
		WorkspaceID: workspaceID, TaskID: taskID, Rating: "mismatch",
	})
	if err != nil || updatedFeedback.ID != feedback.ID || updatedFeedback.Rating != "mismatch" {
		t.Fatalf("updated Twin feedback = %#v, err = %v", updatedFeedback, err)
	}

	depositionProposal, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
		WorkspaceID: workspaceID, Kind: "evolution", SourceWikiRevisionID: revision.ID,
		BaseTwinVersionID: version.ID, Content: []byte(`{"schema_version":1,"assertions":[]}`),
		ContentDigest: twinExecutionTestDigest("deposition"), RequestedByID: actorID,
	})
	if err != nil {
		t.Fatalf("create deposition proposal: %v", err)
	}
	depositionInput := TwinDepositionInput{
		WorkspaceID: workspaceID, TaskID: taskID, BaseTwinVersionID: version.ID,
		ProposalID: depositionProposal.ID, EvidenceDigest: twinExecutionTestDigest("evidence"),
	}
	deposition, err := store.LinkDeposition(ctx, depositionInput)
	if err != nil {
		t.Fatalf("link Twin deposition: %v", err)
	}
	repeatedDeposition, err := store.LinkDeposition(ctx, depositionInput)
	if err != nil || repeatedDeposition.ID != deposition.ID {
		t.Fatalf("repeat Twin deposition = %#v, err = %v", repeatedDeposition, err)
	}
	conflictingDeposition := depositionInput
	conflictingDeposition.EvidenceDigest = twinExecutionTestDigest("different evidence")
	if _, err := store.LinkDeposition(ctx, conflictingDeposition); !errors.Is(err, ErrTwinExecutionConflict) {
		t.Fatalf("conflicting Twin deposition error = %v, want conflict", err)
	}
	accepted, err := store.UpdateDepositionState(ctx, workspaceID, deposition.ID, "accepted")
	if err != nil || accepted.State != "accepted" {
		t.Fatalf("accepted Twin deposition = %#v, err = %v", accepted, err)
	}
	if _, err := store.UpdateDepositionState(ctx, workspaceID, deposition.ID, "rejected"); !errors.Is(err, ErrTwinExecutionConflict) {
		t.Fatalf("opposite Twin deposition state error = %v, want conflict", err)
	}
	metrics, err := store.GetMetrics(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get Twin execution metrics: %v", err)
	}
	if metrics.AttributedRuns != 1 || metrics.FeedbackTotal != 1 || metrics.FeedbackMismatch != 1 ||
		metrics.DepositionsTotal != 1 || metrics.DepositionsAccepted != 1 || metrics.BindingsPreview != 1 {
		t.Fatalf("Twin execution metrics = %#v", metrics)
	}

	otherWorkspaceID := twinExecutionTestUUID(42)
	if _, err := store.GetTaskAttribution(ctx, otherWorkspaceID, attribution.ID); !errors.Is(err, ErrTwinExecutionNotFound) {
		t.Fatalf("cross-workspace attribution error = %v, want not found", err)
	}
	if err := store.DeleteBinding(ctx, workspaceID, binding.ID); err != nil {
		t.Fatalf("delete Twin binding: %v", err)
	}
}

func validTwinTaskAttributionInput() TwinTaskAttributionInput {
	briefing := "Bounded briefing"
	return TwinTaskAttributionInput{
		WorkspaceID: twinExecutionTestUUID(1), TaskID: twinExecutionTestUUID(2),
		AgentID: twinExecutionTestUUID(3), RuntimeID: twinExecutionTestUUID(4),
		TaskDispatchedAt: pgtype.Timestamptz{Time: time.Unix(100, 0), Valid: true},
		TwinVersionID:    twinExecutionTestUUID(5), Briefing: briefing,
		BriefingDigest: TwinBriefingDigest(briefing), AssertionIDs: []string{"assertion:1"},
		CitationKeys: []string{"issue:1"}, PolicyScopeType: "workspace",
		PolicyScopeID: twinExecutionTestUUID(1), PolicyState: "enabled", CompilerVersion: "test-v1",
	}
}

func twinExecutionTestUUID(seed byte) pgtype.UUID {
	var value [16]byte
	value[15] = seed
	return pgtype.UUID{Bytes: value, Valid: true}
}

func twinExecutionUUIDFromString(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil {
		t.Fatalf("parse Twin execution UUID %q: %v", value, err)
	}
	return parsed
}

func twinExecutionTestDigest(value string) string {
	return TwinBriefingDigest(value)
}
