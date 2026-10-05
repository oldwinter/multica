package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinProposalCorrectionIsAppendOnlyAndMustBeReviewedAsHead(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	wiki := fixture.acceptedWiki(t, "Review user-visible changes")
	initial, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, wiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("create initial Twin proposal: %v", err)
	}
	frozen := append([]byte(nil), initial.Proposal.Content...)
	assertions, err := twinAssertions(initial.Proposal.Content)
	if err != nil || len(assertions) == 0 {
		t.Fatalf("decode initial assertions = %#v, err = %v", assertions, err)
	}

	unchangedJSON, err := json.Marshal(assertions)
	if err != nil {
		t.Fatalf("marshal unchanged assertions: %v", err)
	}
	if _, err := fixture.service.CorrectProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID, unchangedJSON); !errors.Is(err, ErrTwinProposalUnchanged) {
		t.Fatalf("unchanged correction error = %v, want unchanged", err)
	}

	unsafe := cloneTwinAssertion(assertions[0])
	unsafe.Text = "api_key=private-value"
	unsafeJSON, err := json.Marshal([]TwinAssertion{unsafe})
	if err != nil {
		t.Fatalf("marshal unsafe assertion: %v", err)
	}
	if _, err := fixture.service.CorrectProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID, unsafeJSON); !errors.Is(err, ErrTwinUnsafeAssertion) {
		t.Fatalf("unsafe correction error = %v, want unsafe assertion", err)
	}

	edited := make([]TwinAssertion, len(assertions))
	for index := range assertions {
		edited[index] = cloneTwinAssertion(assertions[index])
	}
	edited[0].Text += " Record a focused verification result."
	editedJSON, err := json.Marshal(edited)
	if err != nil {
		t.Fatalf("marshal edited assertions: %v", err)
	}
	corrected, err := fixture.service.CorrectProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID, editedJSON)
	if err != nil {
		t.Fatalf("correct Twin proposal: %v", err)
	}
	if !corrected.Created || corrected.Proposal.Kind != "correction" || corrected.Proposal.ReplacesProposalID != initial.Proposal.ID {
		t.Fatalf("correction = %#v", corrected)
	}
	correctedAssertions, err := twinAssertions(corrected.Proposal.Content)
	if err != nil || correctedAssertions[0].Provenance != (TwinAssertionProvenance{Kind: TwinProvenanceHumanEdit, Generator: "human-edit-v1"}) {
		t.Fatalf("corrected assertions = %#v, err = %v", correctedAssertions, err)
	}
	replayed, err := fixture.service.CorrectProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID, editedJSON)
	if err != nil || replayed.Created || replayed.Proposal.ID != corrected.Proposal.ID {
		t.Fatalf("replayed correction = %#v, err = %v", replayed, err)
	}

	alternative := append([]TwinAssertion(nil), edited...)
	alternative[0] = cloneTwinAssertion(edited[0])
	alternative[0].Text += " Preserve the audit trail."
	alternativeJSON, err := json.Marshal(alternative)
	if err != nil {
		t.Fatalf("marshal alternative correction: %v", err)
	}
	if _, err := fixture.service.CorrectProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID, alternativeJSON); !errors.Is(err, ErrTwinProposalSuperseded) {
		t.Fatalf("alternative correction error = %v, want superseded", err)
	}
	if _, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID); !errors.Is(err, ErrTwinProposalSuperseded) {
		t.Fatalf("accept predecessor error = %v, want superseded", err)
	}
	if _, err := fixture.service.RejectProposal(fixture.ctx, fixture.workspaceID, initial.Proposal.ID, fixture.actorID, "use the correction"); !errors.Is(err, ErrTwinProposalSuperseded) {
		t.Fatalf("reject predecessor error = %v, want superseded", err)
	}

	signed, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, corrected.Proposal.ID, fixture.actorID)
	if err != nil || !signed.Created || signed.Version.ProposalID != corrected.Proposal.ID {
		t.Fatalf("sign correction = %#v, err = %v", signed, err)
	}
	reloadedInitial, err := fixture.queries.GetTwinProposal(fixture.ctx, db.GetTwinProposalParams{WorkspaceID: fixture.workspaceID, ID: initial.Proposal.ID})
	if err != nil || !bytes.Equal(reloadedInitial.Content, frozen) {
		t.Fatalf("initial proposal mutated: err = %v", err)
	}
}

func TestCanonicalTwinProposalEditAllowsRemovingAllAssertions(t *testing.T) {
	assertions, err := canonicalTwinProposalEdit(json.RawMessage(`[]`))
	if err != nil || len(assertions) != 0 {
		t.Fatalf("empty correction assertions = %#v, err = %v", assertions, err)
	}
}
