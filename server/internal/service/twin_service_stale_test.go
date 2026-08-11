package service

import (
	"errors"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinAcceptProposalRejectsStaleSource(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	firstWiki := fixture.acceptedWiki(t, "First source")
	proposal, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.acceptedWiki(t, "Newer source")

	_, err = fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID)

	if !errors.Is(err, ErrTwinWikiStale) {
		t.Fatalf("stale source acceptance error = %v, want ErrTwinWikiStale", err)
	}
	assertNoTwinReviewOrVersion(t, fixture, proposal.Proposal)
}

func TestTwinAcceptProposalRejectsStaleBase(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	firstWiki := fixture.acceptedWiki(t, "First base")
	initial, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID); err != nil {
		t.Fatal(err)
	}
	secondWiki := fixture.acceptedWiki(t, "Evolution base")
	evolution, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, secondWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatal(err)
	}
	competing, err := fixture.queries.CreateTwinProposal(fixture.ctx, db.CreateTwinProposalParams{
		WorkspaceID:          fixture.workspaceID,
		Kind:                 "initial",
		SourceWikiRevisionID: secondWiki.ID,
		Content:              evolution.Proposal.Content,
		ContentDigest:        evolution.Proposal.ContentDigest,
		RequestedByID:        fixture.actorID,
	})
	if err != nil {
		t.Fatalf("create competing proposal: %v", err)
	}
	if _, err := fixture.queries.CreateTwinProposalReview(fixture.ctx, db.CreateTwinProposalReviewParams{WorkspaceID: fixture.workspaceID, ProposalID: competing.ID, Decision: "accepted", ReviewerID: fixture.actorID}); err != nil {
		t.Fatalf("review competing proposal: %v", err)
	}
	if _, err := fixture.queries.CreateTwinVersion(fixture.ctx, db.CreateTwinVersionParams{WorkspaceID: fixture.workspaceID, ProposalID: competing.ID, SignedOffByID: fixture.actorID}); err != nil {
		t.Fatalf("advance current Twin: %v", err)
	}

	_, err = fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, evolution.Proposal.ID, fixture.actorID)

	if !errors.Is(err, ErrTwinBaseStale) {
		t.Fatalf("stale base acceptance error = %v, want ErrTwinBaseStale", err)
	}
	assertNoTwinReviewOrVersion(t, fixture, evolution.Proposal)
}

func assertNoTwinReviewOrVersion(t *testing.T, fixture twinServiceFixture, proposal db.TwinProposal) {
	t.Helper()
	var reviewCount, versionCount int
	if err := fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM twin_proposal_review WHERE workspace_id = $1 AND proposal_id = $2`, fixture.workspaceID, proposal.ID).Scan(&reviewCount); err != nil {
		t.Fatal(err)
	}
	if err := fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM twin_version WHERE workspace_id = $1 AND proposal_id = $2`, fixture.workspaceID, proposal.ID).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if reviewCount != 0 || versionCount != 0 {
		t.Fatalf("stale proposal artifacts = reviews %d, versions %d; want zero", reviewCount, versionCount)
	}
}
