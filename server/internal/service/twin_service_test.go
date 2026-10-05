package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinInitialProposalAcceptanceAndEvolution(t *testing.T) {
	// Given
	fixture := newTwinServiceFixture(t)
	firstWiki := fixture.acceptedWiki(t, "First issue")

	// When
	first, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("ensure initial proposal: %v", err)
	}
	repeated, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("repeat initial proposal: %v", err)
	}
	signed, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, first.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("accept initial proposal: %v", err)
	}
	repeatedSignOff, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, first.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("repeat initial sign-off: %v", err)
	}
	secondWiki := fixture.acceptedWiki(t, "Second issue")
	evolution, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, secondWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("ensure evolution proposal: %v", err)
	}

	// Then
	if !first.Created || first.Proposal.Kind != "initial" || first.Proposal.BaseTwinVersionID.Valid {
		t.Fatalf("initial proposal = %#v", first)
	}
	if repeated.Created || repeated.Proposal.ID != first.Proposal.ID {
		t.Fatalf("repeated proposal = %#v, want existing %#v", repeated, first)
	}
	if !signed.Created || signed.Version.VersionNumber != 1 || signed.Version.ProposalID != first.Proposal.ID {
		t.Fatalf("initial sign-off = %#v", signed)
	}
	if repeatedSignOff.Created || repeatedSignOff.Version.ID != signed.Version.ID {
		t.Fatalf("repeated sign-off = %#v, want existing %#v", repeatedSignOff, signed)
	}
	if !evolution.Created || evolution.Proposal.Kind != "evolution" || evolution.Proposal.BaseTwinVersionID != signed.Version.ID {
		t.Fatalf("evolution proposal = %#v, want base %#v", evolution, signed.Version.ID)
	}
	var content TwinProposalContent
	if err := json.Unmarshal(evolution.Proposal.Content, &content); err != nil {
		t.Fatalf("decode evolution content: %v", err)
	}
	if content.SourceWikiRevisionID != twinUUIDString(secondWiki.ID) || len(content.Assertions) != 1 || len(content.Diff.Added) != 1 || len(content.Diff.Removed) != 1 {
		t.Fatalf("evolution content = %#v", content)
	}
	if _, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID); !errors.Is(err, ErrTwinWikiStale) {
		t.Fatalf("old accepted Wiki build error = %v, want ErrTwinWikiStale", err)
	}
}

func TestTwinInitialProposalRequiresLatestAcceptedWiki(t *testing.T) {
	// Given
	fixture := newTwinServiceFixture(t)
	pending := fixture.wikiRevision(t, "Pending issue")

	// When
	_, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, pending.ID, fixture.actorID)

	// Then
	if !errors.Is(err, ErrTwinWikiNotAccepted) {
		t.Fatalf("pending Wiki build error = %v, want ErrTwinWikiNotAccepted", err)
	}
}

func TestTwinAcceptProposalRejectsOppositeAndStaleSource(t *testing.T) {
	// Given
	fixture := newTwinServiceFixture(t)
	firstWiki := fixture.acceptedWiki(t, "Review issue")
	proposal, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, firstWiki.ID, fixture.actorID)
	if err != nil {
		t.Fatal(err)
	}

	// When
	rejected, err := fixture.service.RejectProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID, "not ready")
	if err != nil {
		t.Fatalf("reject proposal: %v", err)
	}
	repeated, err := fixture.service.RejectProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID, "not ready")
	if err != nil {
		t.Fatalf("repeat rejection: %v", err)
	}
	_, oppositeErr := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID)

	// Then
	if !rejected.Created || rejected.Proposal.Review == nil || rejected.Proposal.Review.Decision != "rejected" {
		t.Fatalf("rejected proposal detail = %#v", rejected)
	}
	if repeated.Created || repeated.Proposal.Review == nil || repeated.Proposal.Review.Decision != "rejected" {
		t.Fatalf("repeated rejection detail = %#v", repeated)
	}
	if !errors.Is(oppositeErr, ErrTwinAlreadyDecided) {
		t.Fatalf("opposite review error = %v, want ErrTwinAlreadyDecided", oppositeErr)
	}
	if _, err := fixture.queries.GetCurrentTwinVersion(fixture.ctx, fixture.workspaceID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("current version after reject error = %v, want pgx.ErrNoRows", err)
	}
}

type twinServiceFixture struct {
	ctx          context.Context
	pool         *pgxpool.Pool
	queries      *db.Queries
	service      *TwinService
	workspaceID  pgtype.UUID
	actorID      pgtype.UUID
	egressPolicy LMWikiEgressPolicy
}

func newTwinServiceFixture(t *testing.T) twinServiceFixture {
	t.Helper()
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect Twin service database: %v", err)
	}
	t.Cleanup(pool.Close)
	var workspaceID, actorID pgtype.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('Twin Service Test', 'twin-service-' || gen_random_uuid()::text, '', 'TWN') RETURNING id`).Scan(&workspaceID); err != nil {
		t.Fatalf("create Twin service workspace: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&actorID); err != nil {
		t.Fatalf("create Twin service actor: %v", err)
	}
	queries := db.New(pool)
	policy, err := NewWikiService(queries, pool).UpdateSourcePolicy(ctx, workspaceID, actorID, LMWikiSourcePolicy{
		SourceClasses: []string{"issue"}, RemoteGenerationEnabled: true,
	})
	if err != nil {
		t.Fatalf("authorize Twin test evidence: %v", err)
	}
	t.Cleanup(func() {
		_ = queries.DeleteWorkspaceWikiTwinData(context.Background(), workspaceID)
		_ = queries.DeleteWorkspaceTwinProfile(context.Background(), workspaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID)
	})
	service := NewTwinService(queries, pool)
	service.ProposalGenerator = twinFixtureProposalGenerator{}
	return twinServiceFixture{
		ctx: ctx, pool: pool, queries: queries, service: service,
		workspaceID: workspaceID, actorID: actorID,
		egressPolicy: LMWikiEgressPolicy{
			RemoteGenerationEnabled: policy.RemoteGenerationEnabled,
			PolicyVersion:           policy.PolicyVersion, PolicyDigest: policy.PolicyDigest,
		},
	}
}

type twinFixtureProposalGenerator struct{}

func (twinFixtureProposalGenerator) Generate(_ context.Context, input TwinProposalGenerationInput) (TwinProposalCandidate, error) {
	assertions := make([]TwinAssertion, 0, len(input.BuilderInput.Citations))
	for _, citation := range input.BuilderInput.Citations {
		applicability := TwinAssertionApplicability{Keywords: []string{citation.SourceType}}
		switch citation.SourceType {
		case "issue":
			applicability = TwinAssertionApplicability{IssueID: citation.SourceID}
		case "project":
			applicability = TwinAssertionApplicability{ProjectID: citation.SourceID}
		}
		text := citation.Label
		if text == "" {
			text = citation.CitationKey
		}
		assertions = append(assertions, TwinAssertion{
			ID: "fixture:" + citation.SourceDigest, Type: TwinAssertionProcedure, Text: text,
			Applicability: applicability, EvidenceCitations: []string{citation.CitationKey}, Confidence: 0.9,
			Provenance: TwinAssertionProvenance{Kind: TwinProvenanceModel, Generator: "fixture-model"},
		})
	}
	return TwinProposalCandidate{Assertions: assertions}, nil
}

func (f twinServiceFixture) acceptedWiki(t *testing.T, title string) db.LmWikiRevision {
	t.Helper()
	revision := f.wikiRevision(t, title)
	if _, err := f.queries.CreateLMWikiReview(f.ctx, db.CreateLMWikiReviewParams{WorkspaceID: f.workspaceID, RevisionID: revision.ID, Decision: "accepted", ReviewerID: f.actorID}); err != nil {
		t.Fatalf("accept Twin source Wiki: %v", err)
	}
	return revision
}

func (f twinServiceFixture) wikiRevision(t *testing.T, title string) db.LmWikiRevision {
	t.Helper()
	snapshot, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{
		EgressPolicy: f.egressPolicy,
		Issues:       []LMWikiIssue{{ID: twinUUIDString(f.workspaceID), Number: 1, Title: title, Status: "todo"}},
	})
	if err != nil {
		t.Fatalf("build Twin source Wiki: %v", err)
	}
	revision, err := f.queries.CreateLMWikiRevision(f.ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID: f.workspaceID, SourceDigest: snapshot.SourceDigest, Content: snapshot.CanonicalJSON,
		SourcePolicyVersion: f.egressPolicy.PolicyVersion, SourcePolicyDigest: f.egressPolicy.PolicyDigest,
		RemoteGenerationEnabled: true, TriggerKind: "manual", RequestedByID: f.actorID,
	})
	if err != nil {
		t.Fatalf("create Twin source Wiki: %v", err)
	}
	citations, err := marshalLMWikiCitations(snapshot.Citations)
	if err != nil {
		t.Fatalf("marshal Twin source citations: %v", err)
	}
	if err := f.queries.CreateLMWikiCitations(f.ctx, db.CreateLMWikiCitationsParams{WorkspaceID: f.workspaceID, RevisionID: revision.ID, Citations: citations}); err != nil {
		t.Fatalf("create Twin source citations: %v", err)
	}
	return revision
}

func twinUUIDString(value pgtype.UUID) string {
	return value.String()
}
