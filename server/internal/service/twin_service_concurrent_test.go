package service

import (
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinInitialProposalAndAcceptanceAreConcurrentIdempotent(t *testing.T) {
	// Given
	fixture := newTwinServiceFixture(t)
	wiki := fixture.acceptedWiki(t, "Concurrent issue")
	const workers = 8
	results := make(chan TwinProposalResult, workers)
	errorsChannel := make(chan error, workers)
	var waitGroup sync.WaitGroup
	start := make(chan struct{})

	// When
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			result, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, wiki.ID, fixture.actorID)
			results <- result
			errorsChannel <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)
	close(errorsChannel)

	// Then
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent ensure proposal: %v", err)
		}
	}
	var proposal db.TwinProposal
	created := 0
	for result := range results {
		if !proposal.ID.Valid {
			proposal = result.Proposal
		}
		if result.Proposal.ID != proposal.ID {
			t.Fatalf("proposal IDs differ: %#v and %#v", proposal.ID, result.Proposal.ID)
		}
		if result.Created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created proposal results = %d, want 1", created)
	}

	acceptResults := make(chan TwinVersionResult, workers)
	errorsChannel = make(chan error, workers)
	start = make(chan struct{})
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			result, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, proposal.ID, fixture.actorID)
			acceptResults <- result
			errorsChannel <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(acceptResults)
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent proposal acceptance: %v", err)
		}
	}
	created = 0
	var versionID string
	for result := range acceptResults {
		if versionID == "" {
			versionID = result.Version.ID.String()
		}
		if result.Version.ID.String() != versionID {
			t.Fatalf("version IDs differ: %s and %s", versionID, result.Version.ID.String())
		}
		if result.Created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created version results = %d, want 1", created)
	}
}

func TestTwinAcceptProposalRollsBackReviewWhenVersionCreationFails(t *testing.T) {
	// Given
	fixture := newTwinServiceFixture(t)
	wiki := fixture.acceptedWiki(t, "Rollback issue")
	proposal, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, wiki.ID, fixture.actorID)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected version failure")
	fixture.service.beforeVersionCreate = func() error { return injected }

	// When
	_, err = fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID)

	// Then
	if !errors.Is(err, injected) {
		t.Fatalf("accept error = %v, want injected failure", err)
	}
	if _, err := fixture.queries.GetTwinProposalReview(fixture.ctx, db.GetTwinProposalReviewParams{WorkspaceID: fixture.workspaceID, ProposalID: proposal.Proposal.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("review after rollback error = %v, want pgx.ErrNoRows", err)
	}
	if _, err := fixture.queries.GetTwinVersionByProposal(fixture.ctx, db.GetTwinVersionByProposalParams{WorkspaceID: fixture.workspaceID, ProposalID: proposal.Proposal.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("version after rollback error = %v, want pgx.ErrNoRows", err)
	}
}
