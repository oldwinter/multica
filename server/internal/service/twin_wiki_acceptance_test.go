package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAcceptedWikiBuildsReviewableTwinOnlyAfterExplicitRequest(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	wiki := NewWikiService(fixture.queries, fixture.pool)
	firstWiki := fixture.wikiIssues(t, LMWikiIssue{ID: uuidString(fixture.workspaceID), Number: 1, Title: "Keep", Status: "todo"})

	if _, err := wiki.Review(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID, "accepted", ""); err != nil {
		t.Fatalf("accept first Wiki: %v", err)
	}
	if proposals := fixture.proposals(t); len(proposals) != 0 {
		t.Fatalf("automatic proposals after Wiki acceptance = %d, want 0", len(proposals))
	}
	generated, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("explicitly build initial Twin: %v", err)
	}
	initial := generated.Proposal
	if initial.Kind != "initial" || initial.BaseTwinVersionID.Valid {
		t.Fatalf("initial proposal = %#v", initial)
	}
	if _, err := fixture.queries.GetTwinProposalReview(fixture.ctx, db.GetTwinProposalReviewParams{WorkspaceID: fixture.workspaceID, ProposalID: initial.ID}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("automatic proposal review error = %v, want pgx.ErrNoRows", err)
	}
	repaired, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil || repaired.Created || repaired.Proposal.ID != initial.ID {
		t.Fatalf("manual proposal repair = %#v, %v", repaired, err)
	}
	firstVersion, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, initial.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("accept initial Twin: %v", err)
	}
	profile, err := fixture.queries.GetTwinProfileByWorkspace(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load initial Twin review profile: %v", err)
	}
	var reviewSteps []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(profile.ReviewSteps, &reviewSteps); err != nil {
		t.Fatalf("decode initial Twin review steps: %v", err)
	}
	if len(reviewSteps) != 6 || reviewSteps[5].ID != "deposition" || reviewSteps[5].State != "current" {
		t.Fatalf("initial Twin review steps = %#v", reviewSteps)
	}
	secondWiki := fixture.wikiIssues(t,
		LMWikiIssue{ID: uuidString(fixture.workspaceID), Number: 1, Title: "Keep", Status: "todo"},
		LMWikiIssue{ID: uuidString(fixture.actorID), Number: 2, Title: "Add", Status: "todo"},
	)
	if _, err := wiki.Review(fixture.ctx, fixture.workspaceID, secondWiki.ID, fixture.actorID, "accepted", ""); err != nil {
		t.Fatalf("accept second Wiki: %v", err)
	}
	if _, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, secondWiki.ID, fixture.actorID); err != nil {
		t.Fatalf("explicitly build evolution Twin: %v", err)
	}
	evolution := fixture.latestProposal(t)
	if evolution.Kind != "evolution" || evolution.BaseTwinVersionID != firstVersion.Version.ID || evolution.SourceWikiRevisionID != secondWiki.ID {
		t.Fatalf("evolution proposal = %#v", evolution)
	}
	firstContent := decodeTwinProposal(t, initial.Content)
	secondContent := decodeTwinProposal(t, evolution.Content)
	if len(firstContent.Assertions) != 1 || len(secondContent.Assertions) != 2 {
		t.Fatalf("assertions = %d then %d", len(firstContent.Assertions), len(secondContent.Assertions))
	}
	if got, want := secondContent.Diff.Unchanged, []string{firstContent.Assertions[0].ID}; !slices.Equal(got, want) {
		t.Fatalf("unchanged IDs = %#v, want %#v", got, want)
	}
	addedID := secondContent.Assertions[0].ID
	if addedID == firstContent.Assertions[0].ID {
		addedID = secondContent.Assertions[1].ID
	}
	if got, want := secondContent.Diff.Added, []string{addedID}; !slices.Equal(got, want) || len(secondContent.Diff.Removed) != 0 {
		t.Fatalf("diff = %#v, want added %#v and no removals", secondContent.Diff, want)
	}
	secondVersion, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, evolution.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("accept evolution Twin: %v", err)
	}
	if secondVersion.Version.VersionNumber != 2 || secondVersion.Version.PriorVersionID != firstVersion.Version.ID || secondVersion.Version.SourceWikiRevisionID != secondWiki.ID {
		t.Fatalf("second version = %#v", secondVersion.Version)
	}
	profile, err = fixture.queries.GetTwinProfileByWorkspace(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load evolved Twin review profile: %v", err)
	}
	reviewSteps = nil
	if err := json.Unmarshal(profile.ReviewSteps, &reviewSteps); err != nil {
		t.Fatalf("decode evolved Twin review steps: %v", err)
	}
	if len(reviewSteps) != 6 || reviewSteps[5].ID != "deposition" || reviewSteps[5].State != "complete" {
		t.Fatalf("evolved Twin review steps = %#v", reviewSteps)
	}
	replayedFirstVersion, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, initial.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("replay initial Twin sign-off: %v", err)
	}
	if replayedFirstVersion.Created || replayedFirstVersion.Version.ID != firstVersion.Version.ID {
		t.Fatalf("replayed initial version = %#v, want existing %#v", replayedFirstVersion, firstVersion)
	}
	profile, err = fixture.queries.GetTwinProfileByWorkspace(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("load Twin review profile after historical replay: %v", err)
	}
	reviewSteps = nil
	if err := json.Unmarshal(profile.ReviewSteps, &reviewSteps); err != nil {
		t.Fatalf("decode Twin review steps after historical replay: %v", err)
	}
	if profile.ReviewDigest != secondVersion.Version.ContentDigest || len(reviewSteps) != 6 || reviewSteps[5].State != "complete" {
		t.Fatalf("profile after historical replay = digest %q steps %#v, want current digest %q", profile.ReviewDigest, reviewSteps, secondVersion.Version.ContentDigest)
	}

	rejectedWiki := fixture.wikiIssues(t, LMWikiIssue{ID: uuidString(fixture.actorID), Number: 3, Title: "Reject", Status: "todo"})
	if _, err := wiki.Review(fixture.ctx, fixture.workspaceID, rejectedWiki.ID, fixture.actorID, "rejected", "not representative"); err != nil {
		t.Fatalf("reject Wiki: %v", err)
	}
	current, err := fixture.queries.GetCurrentTwinVersion(fixture.ctx, fixture.workspaceID)
	if err != nil || current.ID != secondVersion.Version.ID {
		t.Fatalf("current after Wiki rejection = %#v, %v", current, err)
	}
	if proposals := fixture.proposals(t); len(proposals) != 2 {
		t.Fatalf("proposal count after Wiki rejection = %d, want 2", len(proposals))
	}

	thirdWiki := fixture.wikiIssues(t, LMWikiIssue{ID: uuidString(fixture.actorID), Number: 2, Title: "Add", Status: "todo"})
	if _, err := wiki.Review(fixture.ctx, fixture.workspaceID, thirdWiki.ID, fixture.actorID, "accepted", ""); err != nil {
		t.Fatalf("accept third Wiki: %v", err)
	}
	if _, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, thirdWiki.ID, fixture.actorID); err != nil {
		t.Fatalf("explicitly build third Twin: %v", err)
	}
	thirdProposal := fixture.latestProposal(t)
	thirdContent := decodeTwinProposal(t, thirdProposal.Content)
	if got, want := thirdContent.Diff.Removed, []string{firstContent.Assertions[0].ID}; !slices.Equal(got, want) {
		t.Fatalf("removed IDs = %#v, want %#v", got, want)
	}
	if got, want := thirdContent.Diff.Unchanged, []string{addedID}; !slices.Equal(got, want) || len(thirdContent.Diff.Added) != 0 {
		t.Fatalf("third diff = %#v, want unchanged %#v and no additions", thirdContent.Diff, want)
	}
	if _, err := fixture.service.RejectProposal(fixture.ctx, fixture.workspaceID, thirdProposal.ID, fixture.actorID, "keep version two"); err != nil {
		t.Fatalf("reject evolution Twin: %v", err)
	}
	current, err = fixture.queries.GetCurrentTwinVersion(fixture.ctx, fixture.workspaceID)
	if err != nil || current.ID != secondVersion.Version.ID {
		t.Fatalf("current after Twin rejection = %#v, %v", current, err)
	}
	repaired, err = fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, thirdWiki.ID, fixture.actorID)
	if err != nil || repaired.Created || repaired.Proposal.ID != thirdProposal.ID {
		t.Fatalf("rejected natural-key repair = %#v, %v", repaired, err)
	}
}

func TestWikiAcceptanceCommitsWhenExplicitTwinGenerationFails(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	generationFailure := errors.New("model unavailable")
	fixture.service.ProposalGenerator = DeterministicTwinProposalGenerator{Err: generationFailure}
	wiki := NewWikiService(fixture.queries, fixture.pool)
	revision := fixture.wikiRevision(t, "Accepted independently")

	if _, err := wiki.Review(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID, "accepted", ""); err != nil {
		t.Fatalf("accept Wiki despite unavailable model: %v", err)
	}
	if _, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID); !errors.Is(err, generationFailure) {
		t.Fatalf("explicit Twin generation error = %v, want %v", err, generationFailure)
	}
	review, err := fixture.queries.GetLMWikiReview(fixture.ctx, db.GetLMWikiReviewParams{WorkspaceID: fixture.workspaceID, RevisionID: revision.ID})
	if err != nil || review.Decision != "accepted" {
		t.Fatalf("persisted Wiki review = %#v, %v", review, err)
	}
	if proposals := fixture.proposals(t); len(proposals) != 0 {
		t.Fatalf("proposals after model failure = %d, want 0", len(proposals))
	}
}

func TestConcurrentWikiAcceptanceLeavesTwinBuildExplicit(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	wiki := NewWikiService(fixture.queries, fixture.pool)
	revision := fixture.wikiRevision(t, "Concurrent acceptance")
	const workers = 8
	start := make(chan struct{})
	errorsChannel := make(chan error, workers)
	var waitGroup sync.WaitGroup
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := wiki.Review(context.Background(), fixture.workspaceID, revision.ID, fixture.actorID, "accepted", "")
			errorsChannel <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent Wiki acceptance: %v", err)
		}
	}
	if proposals := fixture.proposals(t); len(proposals) != 0 {
		t.Fatalf("proposal count = %d, want 0 without explicit Build", len(proposals))
	}
}

func (f twinServiceFixture) wikiIssues(t *testing.T, issues ...LMWikiIssue) db.LmWikiRevision {
	t.Helper()
	snapshot, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{EgressPolicy: f.egressPolicy, Issues: issues})
	if err != nil {
		t.Fatalf("build Wiki snapshot: %v", err)
	}
	revision, err := f.queries.CreateLMWikiRevision(f.ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID: f.workspaceID, SourceDigest: snapshot.SourceDigest, Content: snapshot.CanonicalJSON,
		SourcePolicyVersion: f.egressPolicy.PolicyVersion, SourcePolicyDigest: f.egressPolicy.PolicyDigest,
		RemoteGenerationEnabled: true, TriggerKind: "manual", RequestedByID: f.actorID,
	})
	if err != nil {
		t.Fatalf("create Wiki revision: %v", err)
	}
	citations, err := marshalLMWikiCitations(snapshot.Citations)
	if err != nil {
		t.Fatalf("marshal Wiki citations: %v", err)
	}
	if err := f.queries.CreateLMWikiCitations(f.ctx, db.CreateLMWikiCitationsParams{WorkspaceID: f.workspaceID, RevisionID: revision.ID, Citations: citations}); err != nil {
		t.Fatalf("create Wiki citations: %v", err)
	}
	return revision
}

func (f twinServiceFixture) proposals(t *testing.T) []db.TwinProposal {
	t.Helper()
	proposals, err := f.queries.ListTwinProposals(f.ctx, db.ListTwinProposalsParams{WorkspaceID: f.workspaceID, ResultLimit: 100})
	if err != nil {
		t.Fatalf("list Twin proposals: %v", err)
	}
	return proposals
}

func (f twinServiceFixture) onlyProposal(t *testing.T) db.TwinProposal {
	t.Helper()
	proposals := f.proposals(t)
	if len(proposals) != 1 {
		t.Fatalf("proposal count = %d, want 1", len(proposals))
	}
	return proposals[0]
}

func (f twinServiceFixture) latestProposal(t *testing.T) db.TwinProposal {
	t.Helper()
	proposals := f.proposals(t)
	if len(proposals) == 0 {
		t.Fatal("no Twin proposals")
	}
	return proposals[0]
}

func decodeTwinProposal(t *testing.T, content []byte) TwinProposalContent {
	t.Helper()
	var proposal TwinProposalContent
	if err := json.Unmarshal(content, &proposal); err != nil {
		t.Fatalf("decode Twin proposal: %v", err)
	}
	return proposal
}
