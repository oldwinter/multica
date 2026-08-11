package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	twinProposalDigest = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	twinWikiDigest     = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
)

func TestTwinQueriesImmutableWorkspaceScopedLifecycle(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	// Given
	ctx := context.Background()
	workspaceID := createLMWikiTestWorkspace(t, ctx, "twin-query")
	otherWorkspaceID := createLMWikiTestWorkspace(t, ctx, "twin-other")
	queries := db.New(testPool)
	revision := createAcceptedTwinWikiRevision(t, ctx, queries, workspaceID)
	proposalContent := json.RawMessage(`{"schema_version":1,"name":"Workspace Twin","assertions":[]}`)

	// When
	proposal, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
		WorkspaceID:          workspaceID,
		Kind:                 "initial",
		SourceWikiRevisionID: revision.ID,
		Content:              proposalContent,
		ContentDigest:        twinProposalDigest,
		RequestedByID:        parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create Twin proposal: %v", err)
	}
	beforeBytes := append([]byte(nil), proposal.Content...)
	review, err := queries.CreateTwinProposalReview(ctx, db.CreateTwinProposalReviewParams{
		WorkspaceID: workspaceID,
		ProposalID:  proposal.ID,
		Decision:    "accepted",
		ReviewerID:  parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("accept Twin proposal: %v", err)
	}
	repeatedReview, err := queries.CreateTwinProposalReview(ctx, db.CreateTwinProposalReviewParams{
		WorkspaceID: workspaceID,
		ProposalID:  proposal.ID,
		Decision:    "rejected",
		ReviewerID:  parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("repeat terminal Twin review: %v", err)
	}
	if repeatedReview.ID != review.ID || repeatedReview.Decision != "accepted" {
		t.Fatalf("terminal review changed: first=%#v repeated=%#v", review, repeatedReview)
	}
	reviews, err := queries.ListTwinProposalReviews(ctx, workspaceID)
	if err != nil || len(reviews) != 1 || reviews[0].ID != review.ID {
		t.Fatalf("proposal reviews = %#v, %v", reviews, err)
	}
	version, err := queries.CreateTwinVersion(ctx, db.CreateTwinVersionParams{
		WorkspaceID:   workspaceID,
		ProposalID:    proposal.ID,
		SignedOffByID: parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create Twin version: %v", err)
	}
	after, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: proposal.ID})
	if err != nil {
		t.Fatalf("read Twin proposal: %v", err)
	}
	current, err := queries.GetCurrentTwinVersion(ctx, workspaceID)
	if err != nil {
		t.Fatalf("read current Twin version: %v", err)
	}
	source, err := queries.GetTwinProposalSourceWikiRevision(ctx, db.GetTwinProposalSourceWikiRevisionParams{
		WorkspaceID: workspaceID,
		ProposalID:  proposal.ID,
	})
	if err != nil {
		t.Fatalf("read Twin proposal Wiki source: %v", err)
	}

	// Then
	if !bytes.Equal(beforeBytes, after.Content) {
		t.Fatalf("proposal bytes changed after sign-off: before=%s after=%s", beforeBytes, after.Content)
	}
	if !bytes.Equal(beforeBytes, version.Content) {
		t.Fatalf("version bytes = %s, want proposal bytes %s", version.Content, beforeBytes)
	}
	if version.VersionNumber != 1 || version.ProposalID != proposal.ID || version.SourceWikiRevisionID != revision.ID {
		t.Fatalf("version identity = %#v, want version 1 pinned to proposal and Wiki revision", version)
	}
	if version.ContentDigest != twinProposalDigest || version.SignedOffByID != parseUUID(testUserID) {
		t.Fatalf("version sign-off = %#v, want exact digest and signer", version)
	}
	if current.ID != version.ID || source.ID != revision.ID || source.SourceDigest != twinWikiDigest || !bytes.Equal(source.Content, revision.Content) {
		t.Fatalf("current/source mismatch: current=%#v source=%#v", current, source)
	}
	_, err = queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
		WorkspaceID:          otherWorkspaceID,
		Kind:                 "initial",
		SourceWikiRevisionID: revision.ID,
		Content:              proposalContent,
		ContentDigest:        twinProposalDigest,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign-source proposal create error = %v, want pgx.ErrNoRows", err)
	}
	byNaturalKey, err := queries.GetTwinProposalByNaturalKey(ctx, db.GetTwinProposalByNaturalKeyParams{
		WorkspaceID:          workspaceID,
		Kind:                 "initial",
		SourceWikiRevisionID: revision.ID,
	})
	if err != nil || byNaturalKey.ID != proposal.ID {
		t.Fatalf("natural-key proposal lookup = %#v, %v", byNaturalKey, err)
	}
	evolution, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
		WorkspaceID:          workspaceID,
		Kind:                 "evolution",
		SourceWikiRevisionID: revision.ID,
		BaseTwinVersionID:    version.ID,
		Content:              json.RawMessage(`{"schema_version":1,"assertions":[],"diff":{}}`),
		ContentDigest:        "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	})
	if err != nil {
		t.Fatalf("create pending evolution proposal: %v", err)
	}
	pending, err := queries.GetPendingTwinProposal(ctx, workspaceID)
	if err != nil || pending.ID != evolution.ID {
		t.Fatalf("pending proposal = %#v, %v", pending, err)
	}
	base, err := queries.GetTwinProposalBaseVersion(ctx, db.GetTwinProposalBaseVersionParams{
		WorkspaceID: workspaceID,
		ProposalID:  evolution.ID,
	})
	if err != nil || base.ID != version.ID {
		t.Fatalf("proposal base version = %#v, %v", base, err)
	}
	proposalHistory, err := queries.ListTwinProposals(ctx, db.ListTwinProposalsParams{
		WorkspaceID: workspaceID,
		ResultLimit: 10,
	})
	if err != nil || len(proposalHistory) != 2 {
		t.Fatalf("proposal history count = %d, %v, want 2", len(proposalHistory), err)
	}
	versionHistory, err := queries.ListTwinVersions(ctx, db.ListTwinVersionsParams{
		WorkspaceID: workspaceID,
		ResultLimit: 10,
	})
	if err != nil || len(versionHistory) != 1 || versionHistory[0].ID != version.ID {
		t.Fatalf("version history = %#v, %v", versionHistory, err)
	}
	foreignHistory, err := queries.ListTwinProposals(ctx, db.ListTwinProposalsParams{
		WorkspaceID: otherWorkspaceID,
		ResultLimit: 10,
	})
	if err != nil || len(foreignHistory) != 0 {
		t.Fatalf("wrong-workspace proposal history = %#v, %v, want empty", foreignHistory, err)
	}

	for name, lookup := range map[string]func() error{
		"proposal": func() error {
			_, lookupErr := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: otherWorkspaceID, ID: proposal.ID})
			return lookupErr
		},
		"version": func() error {
			_, lookupErr := queries.GetTwinVersion(ctx, db.GetTwinVersionParams{WorkspaceID: otherWorkspaceID, ID: version.ID})
			return lookupErr
		},
		"source": func() error {
			_, lookupErr := queries.GetTwinProposalSourceWikiRevision(ctx, db.GetTwinProposalSourceWikiRevisionParams{WorkspaceID: otherWorkspaceID, ProposalID: proposal.ID})
			return lookupErr
		},
	} {
		if lookupErr := lookup(); !errors.Is(lookupErr, pgx.ErrNoRows) {
			t.Fatalf("wrong-workspace %s lookup error = %v, want pgx.ErrNoRows", name, lookupErr)
		}
	}
}

func createAcceptedTwinWikiRevision(t *testing.T, ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) db.LmWikiRevision {
	t.Helper()

	revision, err := queries.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID:  workspaceID,
		SourceDigest: twinWikiDigest,
		Content:      json.RawMessage(`{"schema_version":1,"issues":[]}`),
		TriggerKind:  "manual",
	})
	if err != nil {
		t.Fatalf("create accepted Twin Wiki revision: %v", err)
	}
	if _, err := queries.CreateLMWikiReview(ctx, db.CreateLMWikiReviewParams{
		WorkspaceID: workspaceID,
		RevisionID:  revision.ID,
		Decision:    "accepted",
		ReviewerID:  parseUUID(testUserID),
	}); err != nil {
		t.Fatalf("accept Twin Wiki revision: %v", err)
	}
	return revision
}
