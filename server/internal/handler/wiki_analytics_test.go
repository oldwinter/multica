package handler

import (
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWikiProposalAcceptanceEdited(t *testing.T) {
	base := int64(2)
	proposal := db.WikiPageEditProposal{
		ProposedPath:    "handbook/on-call.md",
		ProposedTitle:   "On-call",
		ProposedContent: "# On-call\n",
	}

	path := " handbook//on-call.md "
	title := " On-call "
	content := "# On-call\n"
	unchanged := acceptWikiProposalRequest{
		ExpectedRevisionNumber: &base,
		Path:                   &path,
		Title:                  &title,
		Content:                &content,
	}
	if wikiProposalAcceptanceEdited(unchanged, proposal) {
		t.Fatal("normalized values equal to the proposal must not count as an edit")
	}

	editedContent := "# On-call\nEscalate after ten minutes.\n"
	changed := unchanged
	changed.Content = &editedContent
	if !wikiProposalAcceptanceEdited(changed, proposal) {
		t.Fatal("a human content override must count as an edit")
	}

	if wikiProposalAcceptanceEdited(acceptWikiProposalRequest{}, proposal) {
		t.Fatal("acceptance without overrides must not count as an edit")
	}
}
