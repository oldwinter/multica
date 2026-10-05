package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	dbfx "github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinDepositionReplacementAndReviewLifecycle(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	wiki := fixture.acceptedWiki(t, "Record execution feedback")
	proposal, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, wiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("create initial Twin proposal: %v", err)
	}
	base, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("sign initial Twin proposal: %v", err)
	}
	prior, err := twinAssertions(base.Version.Content)
	if err != nil || len(prior) == 0 {
		t.Fatalf("decode base assertions = %#v, err = %v", prior, err)
	}
	frozenBaseContent := append([]byte(nil), base.Version.Content...)

	root := dbfx.New(fixture.pool, "", "")
	userID := root.User(t, "Twin deposition owner", "twin-deposition-"+time.Now().Format("150405.000000000")+"@example.com")
	workspace := dbfx.New(fixture.pool, fixture.workspaceID.String(), userID)
	workspace.Member(t, fixture.workspaceID.String(), userID, "owner")
	runtimeIDString := workspace.Runtime(t, "Twin deposition runtime")
	agentIDString := workspace.Agent(t, "Twin deposition Agent", runtimeIDString)
	issueIDString := workspace.Issue(t, "Twin deposition task", dbfx.Cols{"assignee_type": "agent", "assignee_id": agentIDString})
	taskIDString := workspace.Task(t, agentIDString, dbfx.Cols{
		"runtime_id": runtimeIDString, "issue_id": issueIDString, "status": "completed",
		"dispatched_at": dbfx.Raw("now() - interval '1 minute'"), "completed_at": dbfx.Raw("now()"),
	})
	taskID := twinExecutionUUIDFromString(t, taskIDString)
	agentID := twinExecutionUUIDFromString(t, agentIDString)
	runtimeID := twinExecutionUUIDFromString(t, runtimeIDString)
	actorID := twinExecutionUUIDFromString(t, userID)
	task, err := fixture.queries.GetAgentTask(fixture.ctx, taskID)
	if err != nil {
		t.Fatalf("load completed deposition task: %v", err)
	}
	store := NewTwinExecutionStore(fixture.queries)
	t.Cleanup(func() {
		_ = fixture.queries.DeleteWorkspaceTwinExecutionData(context.Background(), fixture.workspaceID)
	})
	if _, err := store.CreateTwinTaskAttributionForClaim(fixture.ctx, TwinTaskAttributionInput{
		WorkspaceID: fixture.workspaceID, TaskID: taskID, AgentID: agentID, RuntimeID: runtimeID,
		TaskDispatchedAt: task.DispatchedAt, TwinVersionID: base.Version.ID,
		Briefing: "Apply the signed execution assertion.", BriefingDigest: TwinBriefingDigest("Apply the signed execution assertion."),
		AssertionIDs: []string{prior[0].ID}, CitationKeys: append([]string(nil), prior[0].EvidenceCitations...),
		PolicyScopeType: "workspace", PolicyScopeID: fixture.workspaceID, PolicyState: "enabled", CompilerVersion: "test-v1",
	}); err != nil {
		t.Fatalf("create deposition attribution: %v", err)
	}
	if _, err := store.UpsertRunFeedback(fixture.ctx, TwinRunFeedbackInput{WorkspaceID: fixture.workspaceID, TaskID: taskID, Rating: "helped"}); err != nil {
		t.Fatalf("create deposition feedback: %v", err)
	}

	execution := NewTwinExecutionService(fixture.queries, true)
	execution.DepositionCreator = fixture.service
	type depositionResult struct {
		result TwinDepositionResult
		err    error
	}
	initialRequests := make(chan depositionResult, 2)
	var creators sync.WaitGroup
	for range 2 {
		creators.Add(1)
		go func() {
			defer creators.Done()
			result, createErr := execution.CreateDeposition(fixture.ctx, fixture.workspaceID, taskID, TwinDepositionRequest{RequestedByID: actorID})
			initialRequests <- depositionResult{result: result, err: createErr}
		}()
	}
	creators.Wait()
	close(initialRequests)
	var initial TwinDepositionResult
	initialCreated := 0
	for request := range initialRequests {
		if request.err != nil {
			t.Fatalf("create deterministic deposition concurrently: %v", request.err)
		}
		if request.result.Created {
			initialCreated++
		}
		if !initial.Proposal.ID.Valid {
			initial = request.result
		} else if initial.Proposal.ID != request.result.Proposal.ID || initial.Deposition.ID != request.result.Deposition.ID {
			t.Fatalf("concurrent deposition requests diverged: %#v != %#v", initial, request.result)
		}
	}
	if initialCreated != 1 || initial.Proposal.Kind != "deposition" || initial.Proposal.SchemaVersion != 2 || initial.Deposition.ReplacesProposalID.Valid {
		t.Fatalf("initial deposition = %#v", initial)
	}
	repeated, err := execution.CreateDeposition(fixture.ctx, fixture.workspaceID, taskID, TwinDepositionRequest{RequestedByID: actorID})
	if err != nil || repeated.Created || repeated.Proposal.ID != initial.Proposal.ID {
		t.Fatalf("repeated deposition = %#v, err = %v", repeated, err)
	}

	edited := make([]TwinAssertion, len(prior))
	for index := range prior {
		edited[index] = cloneTwinAssertion(prior[index])
	}
	edited[0].Text += " Record focused verification evidence."
	editedJSON, err := json.Marshal(edited)
	if err != nil {
		t.Fatalf("marshal edited deposition assertions: %v", err)
	}
	replacement, err := execution.CreateDeposition(fixture.ctx, fixture.workspaceID, taskID, TwinDepositionRequest{
		RequestedByID: actorID, ReplacesProposalID: initial.Proposal.ID, EditedAssertions: editedJSON,
	})
	if err != nil {
		t.Fatalf("create replacement deposition: %v", err)
	}
	if !replacement.Created || replacement.Proposal.ID == initial.Proposal.ID || replacement.Deposition.ReplacesProposalID != initial.Proposal.ID {
		t.Fatalf("replacement deposition = %#v", replacement)
	}
	detail, err := fixture.service.ProposalDetail(fixture.ctx, fixture.workspaceID, replacement.Proposal.ID)
	if err != nil {
		t.Fatalf("load deposition proposal detail: %v", err)
	}
	if detail.RunEvidence == nil || detail.RunEvidence.TaskID != taskID || detail.RunEvidence.BaseTwinVersionID != base.Version.ID ||
		detail.RunEvidence.EvidenceDigest != replacement.Deposition.EvidenceDigest || detail.RunEvidence.TaskStatus != "completed" ||
		!detail.RunEvidence.CompletedAt.Valid || detail.RunEvidence.FeedbackRating.String != "helped" {
		t.Fatalf("deposition run evidence = %#v", detail.RunEvidence)
	}
	replayedReplacement, err := execution.CreateDeposition(fixture.ctx, fixture.workspaceID, taskID, TwinDepositionRequest{
		RequestedByID: actorID, ReplacesProposalID: initial.Proposal.ID, EditedAssertions: editedJSON,
	})
	if err != nil || replayedReplacement.Created || replayedReplacement.Proposal.ID != replacement.Proposal.ID {
		t.Fatalf("replayed replacement = %#v, err = %v", replayedReplacement, err)
	}
	if _, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, actorID); !errors.Is(err, ErrTwinExecutionConflict) {
		t.Fatalf("accept superseded deposition error = %v, want conflict", err)
	}
	if _, err := store.UpsertRunFeedback(fixture.ctx, TwinRunFeedbackInput{WorkspaceID: fixture.workspaceID, TaskID: taskID, Rating: "mismatch"}); err != nil {
		t.Fatalf("change deposition feedback: %v", err)
	}
	if _, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, replacement.Proposal.ID, actorID); !errors.Is(err, ErrTwinDepositionEvidenceStale) {
		t.Fatalf("accept stale deposition evidence error = %v, want stale", err)
	}
	if _, err := store.UpsertRunFeedback(fixture.ctx, TwinRunFeedbackInput{WorkspaceID: fixture.workspaceID, TaskID: taskID, Rating: "helped"}); err != nil {
		t.Fatalf("restore deposition feedback: %v", err)
	}

	type reviewResult struct {
		result TwinVersionResult
		err    error
	}
	reviews := make(chan reviewResult, 2)
	var reviewers sync.WaitGroup
	for range 2 {
		reviewers.Add(1)
		go func() {
			defer reviewers.Done()
			result, acceptErr := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, replacement.Proposal.ID, actorID)
			reviews <- reviewResult{result: result, err: acceptErr}
		}()
	}
	reviewers.Wait()
	close(reviews)
	var signed TwinVersionResult
	created := 0
	for review := range reviews {
		if review.err != nil {
			t.Fatalf("accept replacement deposition concurrently: %v", review.err)
		}
		if review.result.Created {
			created++
		}
		if !signed.Version.ID.Valid {
			signed = review.result
		} else if signed.Version.ID != review.result.Version.ID {
			t.Fatalf("concurrent sign-off created different versions: %s != %s", signed.Version.ID.String(), review.result.Version.ID.String())
		}
	}
	if created != 1 || signed.Version.PriorVersionID != base.Version.ID {
		t.Fatalf("concurrent signed deposition versions created = %d, version = %#v", created, signed)
	}
	reloadedBase, err := fixture.queries.GetTwinVersion(fixture.ctx, db.GetTwinVersionParams{WorkspaceID: fixture.workspaceID, ID: base.Version.ID})
	if err != nil || !bytes.Equal(reloadedBase.Content, frozenBaseContent) {
		t.Fatalf("base Twin version mutated: err = %v", err)
	}
	accepted, err := store.GetDeposition(fixture.ctx, fixture.workspaceID, replacement.Deposition.ID)
	if err != nil || accepted.State != "accepted" {
		t.Fatalf("accepted deposition state = %#v, err = %v", accepted, err)
	}
	superseded, err := store.GetDeposition(fixture.ctx, fixture.workspaceID, initial.Deposition.ID)
	if err != nil || superseded.State != "rejected" {
		t.Fatalf("superseded deposition state = %#v, err = %v", superseded, err)
	}
	postAcceptReplay, err := execution.CreateDeposition(fixture.ctx, fixture.workspaceID, taskID, TwinDepositionRequest{
		RequestedByID: actorID, ReplacesProposalID: initial.Proposal.ID, EditedAssertions: editedJSON,
	})
	if err != nil || postAcceptReplay.Created || postAcceptReplay.Proposal.ID != replacement.Proposal.ID {
		t.Fatalf("post-accept replacement replay = %#v, err = %v", postAcceptReplay, err)
	}
}

func TestTwinDepositionEditCanRemoveAllAssertions(t *testing.T) {
	canonical, assertions, err := canonicalTwinDepositionEdit(json.RawMessage(`[]`))
	if err != nil || string(canonical) != "[]" || len(assertions) != 0 {
		t.Fatalf("empty deposition edit = %s, %#v, err = %v", canonical, assertions, err)
	}
}

func TestTwinDepositionEditCannotClaimExecutionOwnedScopes(t *testing.T) {
	raw := json.RawMessage(`[{"id":"review.focus","type":"quality_bar","text":"Keep review focused.","applicability":{"task_id":"task-private","workspace_id":"workspace-private","agent_id":"agent-private","keywords":["review"]},"evidence_citations":["issue:42"],"confidence":0.9}]`)
	_, assertions, err := canonicalTwinDepositionEdit(raw)
	if err != nil || len(assertions) != 1 {
		t.Fatalf("canonical deposition edit = %#v, err = %v", assertions, err)
	}
	applicability := assertions[0].Applicability
	if applicability.TaskID != "" || applicability.WorkspaceID != "" || applicability.AgentID != "" {
		t.Fatalf("human edit retained execution-owned scopes: %#v", applicability)
	}
}

var _ TwinDepositionProposalCreator = (*TwinService)(nil)
