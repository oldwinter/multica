package service

import (
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinAcceptNewerProposalRejectsOlderPendingProposal(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	firstWiki := fixture.acceptedWiki(t, "First pending proposal")
	first, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("ensure first proposal: %v", err)
	}
	secondWiki := fixture.acceptedWiki(t, "Second pending proposal")
	second, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, secondWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("ensure second proposal: %v", err)
	}

	signed, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, second.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("accept newer proposal: %v", err)
	}
	olderReview, err := fixture.queries.GetTwinProposalReview(fixture.ctx, db.GetTwinProposalReviewParams{WorkspaceID: fixture.workspaceID, ProposalID: first.Proposal.ID})
	if err != nil {
		t.Fatalf("load older proposal review: %v", err)
	}
	if olderReview.Decision != "rejected" || olderReview.ReviewerID != fixture.actorID || !olderReview.Reason.Valid || olderReview.Reason.String != twinSupersededProposalReason || len([]rune(olderReview.Reason.String)) > TwinReviewReasonLimit {
		t.Fatalf("older proposal terminal review = %#v", olderReview)
	}
	overview, err := fixture.service.Overview(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load Twin overview: %v", err)
	}
	if overview.Pending != nil {
		t.Fatalf("pending proposal after newer sign-off = %#v", overview.Pending)
	}
	if overview.Current == nil || overview.Current.ID != signed.Version.ID || overview.Current.ProposalID != second.Proposal.ID {
		t.Fatalf("current Twin after newer sign-off = %#v", overview.Current)
	}

	repeated, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, second.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("repeat newer proposal acceptance: %v", err)
	}
	if repeated.Created || repeated.Version.ID != signed.Version.ID {
		t.Fatalf("repeated acceptance = %#v, want existing %#v", repeated, signed)
	}
	reloaded, err := fixture.queries.GetTwinProposalReview(fixture.ctx, db.GetTwinProposalReviewParams{WorkspaceID: fixture.workspaceID, ProposalID: first.Proposal.ID})
	if err != nil || reloaded.ID != olderReview.ID || reloaded.Decision != olderReview.Decision || reloaded.ReviewerID != olderReview.ReviewerID || reloaded.Reason != olderReview.Reason {
		t.Fatalf("older terminal review mutated: before=%#v after=%#v error=%v", olderReview, reloaded, err)
	}
}
