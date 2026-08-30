package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
)

func TestWikiReviewedProposalSignalListIsBoundedAndContentFree(t *testing.T) {
	accepted := wikiReviewedProposalTestRow("accepted")
	rejected := wikiReviewedProposalTestRow("rejected")
	rejected.proposalID = util.MustParseUUID("00000000-0000-0000-0000-000000000107")
	thirdValid := wikiReviewedProposalTestRow("accepted")
	thirdValid.proposalID = util.MustParseUUID("00000000-0000-0000-0000-000000000110")
	thirdValid.acceptedRevisionSourceRefID = thirdValid.proposalID
	pending := wikiReviewedProposalTestRow("accepted")
	pending.decision = "pending"
	personal := wikiReviewedProposalTestRow("accepted")
	personal.scope = "user"
	wrongSource := wikiReviewedProposalTestRow("accepted")
	wrongSource.acceptedRevisionSourceKind = "human"
	store := &fakeWikiReviewedProposalSignalStore{
		listRows: []wikiReviewedProposalSignalRow{accepted, rejected, thirdValid, pending, personal, wrongSource},
	}
	adapter := &wikiReviewedProposalSignalAdapter{store: store}

	refs, err := adapter.ListReviewedProposalSignals(context.Background(), accepted.workspaceID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if store.listLimit != 2 || len(refs) != 2 || refs[0].Decision != "accepted" || refs[1].Decision != "rejected" {
		t.Fatalf("list result = limit %d refs %#v", store.listLimit, refs)
	}
	for _, name := range []string{"Content", "Path", "Title", "Rationale", "ReviewReason", "Citations"} {
		if _, ok := reflect.TypeOf(WikiReviewedProposalSignalRef{}).FieldByName(name); ok {
			t.Fatalf("content-free ref unexpectedly has %s", name)
		}
	}
	if _, err := adapter.ListReviewedProposalSignals(context.Background(), accepted.workspaceID, MaxWikiReviewedProposalSignals+1); !errors.Is(err, ErrInvalidWikiReviewedProposalSignal) {
		t.Fatalf("over-limit error = %v", err)
	}
	if _, err := adapter.ListReviewedProposalSignals(context.Background(), pgtype.UUID{}, 1); !errors.Is(err, ErrInvalidWikiReviewedProposalSignal) {
		t.Fatalf("missing workspace error = %v", err)
	}
}

func TestWikiReviewedProposalSignalLoadUsesActualAcceptedRevision(t *testing.T) {
	row := wikiReviewedProposalTestRow("accepted")
	row.evidencePath = "procedures/reviewed.md"
	row.evidenceTitle = "Human reviewed procedure"
	row.evidenceContent = "Use the bounded accepted revision."
	row.acceptedRevisionPathDigest = wikiReviewedProposalStringDigest(row.evidencePath)
	row.acceptedRevisionTitleDigest = wikiReviewedProposalStringDigest(row.evidenceTitle)
	row.acceptedRevisionContentDigest = wikiReviewedProposalStringDigest(row.evidenceContent)
	ref := wikiReviewedProposalRef(row)
	store := &fakeWikiReviewedProposalSignalStore{loadRow: row}
	adapter := &wikiReviewedProposalSignalAdapter{store: store}

	evidence, err := adapter.LoadReviewedProposalSignal(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Content != "Use the bounded accepted revision." || evidence.Path != "procedures/reviewed.md" ||
		evidence.AcceptedRevisionContentDigest != row.acceptedRevisionContentDigest {
		t.Fatalf("accepted evidence = %#v", evidence)
	}
	if evidence.ProposalContentDigest == evidence.AcceptedRevisionContentDigest {
		t.Fatal("fixture must prove accepted human override is not attributed to proposal content")
	}
	if len(evidence.Citations) != 2 || evidence.Citations[0] != "task:00000000-0000-0000-0000-000000000108" {
		t.Fatalf("citations = %#v", evidence.Citations)
	}
}

func TestWikiReviewedProposalSignalLoadDetectsDigestDrift(t *testing.T) {
	row := wikiReviewedProposalTestRow("accepted")
	ref := wikiReviewedProposalRef(row)
	store := &fakeWikiReviewedProposalSignalStore{loadRow: row}
	adapter := &wikiReviewedProposalSignalAdapter{store: store}

	store.loadRow.evidenceContent = "content changed without its digest"
	if _, err := adapter.LoadReviewedProposalSignal(context.Background(), ref); !errors.Is(err, ErrWikiReviewedProposalDrift) {
		t.Fatalf("content drift error = %v", err)
	}

	store.loadRow = row
	store.loadRow.acceptedRevisionContentDigest = wikiReviewedProposalStringDigest("new accepted content")
	store.loadRow.evidenceContent = "new accepted content"
	if _, err := adapter.LoadReviewedProposalSignal(context.Background(), ref); !errors.Is(err, ErrWikiReviewedProposalDrift) {
		t.Fatalf("accepted digest drift error = %v", err)
	}

	store.loadRow = row
	store.loadRow.proposalDigestValid = false
	if _, err := adapter.LoadReviewedProposalSignal(context.Background(), ref); !errors.Is(err, ErrWikiReviewedProposalDrift) {
		t.Fatalf("proposal digest drift error = %v", err)
	}
}

func TestWikiReviewedProposalSignalLoadRejectsIneligibleCustodyAndLifecycle(t *testing.T) {
	valid := wikiReviewedProposalTestRow("accepted")
	ref := wikiReviewedProposalRef(valid)
	otherID := util.MustParseUUID("00000000-0000-0000-0000-000000000199")

	tests := []struct {
		name string
		edit func(*wikiReviewedProposalSignalRow)
	}{
		{name: "pending", edit: func(row *wikiReviewedProposalSignalRow) { row.decision = "pending" }},
		{name: "personal", edit: func(row *wikiReviewedProposalSignalRow) { row.scope = "user" }},
		{name: "local only", edit: func(row *wikiReviewedProposalSignalRow) { row.scope = "local" }},
		{name: "wrong accepted source kind", edit: func(row *wikiReviewedProposalSignalRow) { row.acceptedRevisionSourceKind = "human" }},
		{name: "wrong accepted source ref", edit: func(row *wikiReviewedProposalSignalRow) { row.acceptedRevisionSourceRefID = otherID }},
		{name: "missing accepted revision", edit: func(row *wikiReviewedProposalSignalRow) { row.acceptedRevisionID = pgtype.UUID{} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := valid
			tc.edit(&row)
			adapter := &wikiReviewedProposalSignalAdapter{store: &fakeWikiReviewedProposalSignalStore{loadRow: row}}
			if _, err := adapter.LoadReviewedProposalSignal(context.Background(), ref); !errors.Is(err, ErrWikiReviewedProposalUnavailable) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	otherWorkspace := valid
	otherWorkspace.workspaceID = otherID
	adapter := &wikiReviewedProposalSignalAdapter{store: &fakeWikiReviewedProposalSignalStore{loadRow: otherWorkspace}}
	if _, err := adapter.LoadReviewedProposalSignal(context.Background(), ref); !errors.Is(err, ErrWikiReviewedProposalUnavailable) {
		t.Fatalf("cross-workspace error = %v", err)
	}

	adapter = &wikiReviewedProposalSignalAdapter{store: &fakeWikiReviewedProposalSignalStore{loadErr: pgx.ErrNoRows}}
	if _, err := adapter.LoadReviewedProposalSignal(context.Background(), ref); !errors.Is(err, ErrWikiReviewedProposalUnavailable) {
		t.Fatalf("deleted error = %v", err)
	}
}

func TestWikiReviewedProposalSignalLoadReturnsRejectedProposalEvidence(t *testing.T) {
	row := wikiReviewedProposalTestRow("rejected")
	ref := wikiReviewedProposalRef(row)
	adapter := &wikiReviewedProposalSignalAdapter{store: &fakeWikiReviewedProposalSignalStore{loadRow: row}}

	evidence, err := adapter.LoadReviewedProposalSignal(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Ref.Decision != "rejected" || evidence.Content != "Rejected proposed content" ||
		evidence.ReviewReason != "Too broad for this page." || evidence.AcceptedRevisionContentDigest != "" {
		t.Fatalf("rejected evidence = %#v", evidence)
	}
}

func TestWikiReviewedProposalSignalSQLKeepsListBoundedAndRevalidatesLoadCustody(t *testing.T) {
	for _, required := range []string{
		"proposal.workspace_id = $1", "page.scope IN ('workspace', 'project')",
		"revision.source_kind = 'agent_proposal'", "revision.source_ref_id = proposal.id",
		"ORDER BY proposal.reviewed_at DESC", "LIMIT $2",
	} {
		if !strings.Contains(listWikiReviewedProposalSignalsSQL, required) {
			t.Fatalf("list query missing %q", required)
		}
	}
	for _, required := range []string{
		"page.workspace_id = proposal.workspace_id", "page.scope IN ('workspace', 'project')",
		"proposal.workspace_id = $1", "proposal.id = $2", "proposal.status IN ('accepted', 'rejected')",
		"revision.source_kind = 'agent_proposal'", "revision.source_ref_id = proposal.id", "<= $3",
	} {
		if !strings.Contains(loadWikiReviewedProposalSignalSQL, required) {
			t.Fatalf("load query missing %q", required)
		}
	}
}

type fakeWikiReviewedProposalSignalStore struct {
	listRows  []wikiReviewedProposalSignalRow
	listErr   error
	listLimit int32
	loadRow   wikiReviewedProposalSignalRow
	loadErr   error
}

func (f *fakeWikiReviewedProposalSignalStore) listReviewedProposalSignals(_ context.Context, _ pgtype.UUID, limit int32) ([]wikiReviewedProposalSignalRow, error) {
	f.listLimit = limit
	return append([]wikiReviewedProposalSignalRow(nil), f.listRows...), f.listErr
}

func (f *fakeWikiReviewedProposalSignalStore) loadReviewedProposalSignal(_ context.Context, _, _ pgtype.UUID) (wikiReviewedProposalSignalRow, error) {
	return f.loadRow, f.loadErr
}

func wikiReviewedProposalTestRow(decision string) wikiReviewedProposalSignalRow {
	row := wikiReviewedProposalSignalRow{
		workspaceID:           util.MustParseUUID("00000000-0000-0000-0000-000000000101"),
		proposalID:            util.MustParseUUID("00000000-0000-0000-0000-000000000102"),
		pageID:                util.MustParseUUID("00000000-0000-0000-0000-000000000103"),
		agentID:               util.MustParseUUID("00000000-0000-0000-0000-000000000104"),
		reviewedByID:          util.MustParseUUID("00000000-0000-0000-0000-000000000105"),
		decision:              decision,
		proposalContentDigest: wikiReviewedProposalStringDigest("Rejected proposed content"),
		reviewedAt:            pgtype.Timestamptz{Time: time.Date(2026, time.August, 28, 10, 0, 0, 0, time.UTC), Valid: true},
		scope:                 "workspace",
		proposalPathDigest:    wikiReviewedProposalStringDigest("procedures/proposed.md"),
		proposalTitleDigest:   wikiReviewedProposalStringDigest("Proposed procedure"),
		rationale:             "The prior procedure missed a bounded check.",
		reviewReason:          "Too broad for this page.",
		evidenceRefs:          []byte(`["task:00000000-0000-0000-0000-000000000108","room:00000000-0000-0000-0000-000000000109"]`),
		proposalDigestValid:   true,
		evidencePath:          "procedures/proposed.md",
		evidenceTitle:         "Proposed procedure",
		evidenceContent:       "Rejected proposed content",
	}
	row.rationaleDigest = wikiReviewedProposalStringDigest(row.rationale)
	row.evidenceRefsDigest = wikiReviewedProposalStringDigest(string(row.evidenceRefs))
	row.reviewReasonDigest = wikiReviewedProposalStringDigest(row.reviewReason)
	if decision == "accepted" {
		row.acceptedRevisionID = util.MustParseUUID("00000000-0000-0000-0000-000000000106")
		row.acceptedRevisionSourceKind = "agent_proposal"
		row.acceptedRevisionSourceRefID = row.proposalID
		row.evidencePath = "procedures/accepted.md"
		row.evidenceTitle = "Accepted procedure"
		row.evidenceContent = "Human-overridden accepted content"
		row.acceptedRevisionPathDigest = wikiReviewedProposalStringDigest(row.evidencePath)
		row.acceptedRevisionTitleDigest = wikiReviewedProposalStringDigest(row.evidenceTitle)
		row.acceptedRevisionContentDigest = wikiReviewedProposalStringDigest(row.evidenceContent)
		row.reviewReason = ""
		row.reviewReasonDigest = wikiReviewedProposalStringDigest("")
	}
	return row
}
