package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var _ WikiKnowledge = (*WikiKnowledgeService)(nil)

func TestValidWikiPageCreateInput(t *testing.T) {
	t.Parallel()
	uuid := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}

	room := WikiPageCreateInput{WorkspaceID: uuid, Scope: "workspace", Path: "rooms/result.md", Title: "Result", ActorType: "member", ActorID: uuid, SourceKind: "room_promotion", SourceRefID: uuid}
	if !validWikiPageCreateInput(room) {
		t.Fatal("room promotion should be a valid shared Wiki create")
	}

	personalAgent := WikiPageCreateInput{OwnerUserID: uuid, Scope: "user", Path: "private.md", Title: "Private", ActorType: "agent", ActorID: uuid, SourceKind: "agent_proposal"}
	if validWikiPageCreateInput(personalAgent) {
		t.Fatal("agent-created personal Wiki page must be rejected")
	}

	crossTenant := room
	crossTenant.OwnerUserID = uuid
	if validWikiPageCreateInput(crossTenant) {
		t.Fatal("shared Wiki page must not carry a personal owner")
	}
}

func TestWikiPageFromProvenanceCreatePreservesRevisionIdentity(t *testing.T) {
	t.Parallel()
	pageID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	revisionID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	actorID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}

	page := wikiPageFromProvenanceCreate(db.CreateWikiPageWithProvenanceRow{
		ID: pageID, Scope: "workspace", Path: "rooms/result.md", Title: "Result",
		Content: "approved", CurrentRevisionNumber: 1, CurrentRevisionID: revisionID,
		ContentDigest: "sha256:digest", LastSourceKind: "room_promotion",
		LastActorType: "member", LastActorID: actorID,
	})

	if page.ID != pageID || page.CurrentRevisionID != revisionID || page.CurrentRevisionNumber != 1 {
		t.Fatalf("revision identity was not preserved: %+v", page)
	}
	if page.LastSourceKind != "room_promotion" || page.LastActorID != actorID {
		t.Fatalf("provenance was not preserved: %+v", page)
	}
}

func TestWikiKnowledgePublishesContentFreePersonalEventsToOwner(t *testing.T) {
	t.Parallel()
	bus := events.New()
	service := NewWikiKnowledgeService(bus).(*WikiKnowledgeService)
	pageID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	revisionID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	ownerID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	actorID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	page := db.WikiPage{
		ID: pageID, Scope: "user", OwnerUserID: ownerID,
		CurrentRevisionID: revisionID, CurrentRevisionNumber: 7,
		Content: "must never enter realtime", ContentDigest: "sha256:secret",
	}

	var received events.Event
	bus.Subscribe(protocol.EventWikiPageUpdated, func(event events.Event) { received = event })
	service.publishPage(protocol.EventWikiPageUpdated, page, "member", actorID)

	if received.WorkspaceID != "" || received.ActorType != "member" || received.ActorID != optionalWikiUUID(actorID) {
		t.Fatalf("unexpected event routing: %+v", received)
	}
	payload, ok := received.Payload.(protocol.WikiEventPayload)
	if !ok {
		t.Fatalf("payload type = %T", received.Payload)
	}
	if payload.PageID != optionalWikiUUID(pageID) || payload.RecipientID != optionalWikiUUID(ownerID) || payload.RevisionNumber != 7 {
		t.Fatalf("unexpected event payload: %+v", payload)
	}
}

func TestWikiKnowledgePublishesProposalReviewIdentity(t *testing.T) {
	t.Parallel()
	bus := events.New()
	service := NewWikiKnowledgeService(bus).(*WikiKnowledgeService)
	workspaceID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	pageID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	proposalID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	revisionID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	reviewerID := pgtype.UUID{Bytes: [16]byte{5}, Valid: true}
	page := db.WikiPage{ID: pageID, WorkspaceID: workspaceID, Scope: "workspace", CurrentRevisionNumber: 3}
	proposal := db.WikiPageEditProposal{
		ID: proposalID, PageID: pageID, BaseRevisionNumber: 2,
		Status: "accepted", AcceptedRevisionID: revisionID,
	}

	var received events.Event
	bus.Subscribe(protocol.EventWikiProposalReviewed, func(event events.Event) { received = event })
	service.publishProposal(protocol.EventWikiProposalReviewed, page, proposal, "member", reviewerID)

	payload, ok := received.Payload.(protocol.WikiEventPayload)
	if !ok {
		t.Fatalf("payload type = %T", received.Payload)
	}
	if received.WorkspaceID != optionalWikiUUID(workspaceID) || payload.ProposalID != optionalWikiUUID(proposalID) ||
		payload.AcceptedRevisionID != optionalWikiUUID(revisionID) || payload.AcceptedRevisionNumber != 3 {
		t.Fatalf("unexpected review event: event=%+v payload=%+v", received, payload)
	}
}

func TestWikiKnowledgeProposalCreateValidatesDomainInput(t *testing.T) {
	t.Parallel()
	pageID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	workspaceID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	agentID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	taskID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	valid := WikiProposalCreateInput{
		Page:    db.WikiPage{ID: pageID, WorkspaceID: workspaceID, Scope: "workspace"},
		AgentID: agentID, AuthenticatedTaskID: taskID, BaseRevisionNumber: 1,
		Path: "knowledge/result.md", Title: "Result", Rationale: "Verified evidence",
		IdempotencyKey: "run-1",
	}
	if !validWikiProposalCreateInput(valid) {
		t.Fatal("expected valid proposal input")
	}

	for _, tc := range []struct {
		name   string
		mutate func(*WikiProposalCreateInput)
	}{
		{name: "blank rationale", mutate: func(input *WikiProposalCreateInput) { input.Rationale = " \t\n " }},
		{name: "blank idempotency", mutate: func(input *WikiProposalCreateInput) { input.IdempotencyKey = " \t " }},
		{name: "long idempotency", mutate: func(input *WikiProposalCreateInput) { input.IdempotencyKey = strings.Repeat("x", 201) }},
		{name: "unsafe path", mutate: func(input *WikiProposalCreateInput) { input.Path = "../secret.md" }},
		{name: "non markdown path", mutate: func(input *WikiProposalCreateInput) { input.Path = "notes.txt" }},
		{name: "personal page", mutate: func(input *WikiProposalCreateInput) { input.Page.Scope = "user" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			tc.mutate(&input)
			_, err := NewWikiKnowledgeService().CreateProposal(context.Background(), nil, input)
			if !errors.Is(err, ErrInvalidWikiProposal) {
				t.Fatalf("CreateProposal error = %v, want ErrInvalidWikiProposal", err)
			}
		})
	}
}

func TestWikiKnowledgeProposalReviewValidatesDomainIdentity(t *testing.T) {
	t.Parallel()
	pageID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	workspaceID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	proposalID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	reviewerID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	page := db.WikiPage{ID: pageID, WorkspaceID: workspaceID, Scope: "workspace", CurrentRevisionNumber: 1}
	proposal := db.WikiPageEditProposal{
		ID: proposalID, WorkspaceID: workspaceID, PageID: pageID,
		BaseRevisionNumber: 1, Status: "pending",
	}
	service := NewWikiKnowledgeService()

	for _, tc := range []struct {
		name   string
		mutate func(*WikiProposalAcceptInput)
	}{
		{name: "missing reviewer", mutate: func(input *WikiProposalAcceptInput) { input.ReviewerID = pgtype.UUID{} }},
		{name: "wrong proposal page", mutate: func(input *WikiProposalAcceptInput) { input.Proposal.PageID = reviewerID }},
		{name: "wrong proposal workspace", mutate: func(input *WikiProposalAcceptInput) { input.Proposal.WorkspaceID = reviewerID }},
		{name: "unsafe override path", mutate: func(input *WikiProposalAcceptInput) { input.Path = pgtype.Text{String: "../secret.md", Valid: true} }},
		{name: "long review reason", mutate: func(input *WikiProposalAcceptInput) {
			input.ReviewReason = pgtype.Text{String: strings.Repeat("r", 2001), Valid: true}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := WikiProposalAcceptInput{
				Page: page, Proposal: proposal, ExpectedRevisionNumber: 1, ReviewerID: reviewerID,
			}
			tc.mutate(&input)
			_, _, err := service.AcceptProposal(context.Background(), nil, input)
			if !errors.Is(err, ErrInvalidWikiProposal) {
				t.Fatalf("AcceptProposal error = %v, want ErrInvalidWikiProposal", err)
			}
		})
	}
}

func TestWikiKnowledgeAllowsBlankProposalAndUpdateTitles(t *testing.T) {
	t.Parallel()
	pageID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	workspaceID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	agentID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	taskID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	input := WikiProposalCreateInput{
		Page:    db.WikiPage{ID: pageID, WorkspaceID: workspaceID, Scope: "workspace"},
		AgentID: agentID, AuthenticatedTaskID: taskID, BaseRevisionNumber: 1,
		Path: "knowledge/result.md", Title: "", Rationale: "Verified evidence", IdempotencyKey: "run-1",
	}
	if !validWikiProposalCreateInput(input) {
		t.Fatal("blank proposal title should remain valid; clients render the path fallback")
	}
}

func TestSameWikiProposalIntentRejectsIdempotencyKeyReuse(t *testing.T) {
	t.Parallel()
	pageID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	input := WikiProposalCreateInput{
		Page: db.WikiPage{ID: pageID}, BaseRevisionNumber: 3,
		Path: "playbook/review.md", Title: "Review", Content: "# Review",
		Rationale: "evidence",
	}
	evidence := []byte(`["task:11111111-1111-1111-1111-111111111111"]`)
	proposal := db.WikiPageEditProposal{
		PageID: pageID, BaseRevisionNumber: input.BaseRevisionNumber,
		ProposedPath: input.Path, ProposedTitle: input.Title,
		ProposedContent: input.Content, Rationale: input.Rationale, EvidenceRefs: evidence,
	}
	if !sameWikiProposalIntent(proposal, input, evidence) {
		t.Fatal("the original proposal intent must be idempotent")
	}

	changed := input
	changed.Page.ID = pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	if sameWikiProposalIntent(proposal, changed, evidence) {
		t.Fatal("reusing a key for another page must conflict")
	}
	changed = input
	changed.Content = "changed"
	if sameWikiProposalIntent(proposal, changed, evidence) {
		t.Fatal("reusing a key for changed content must conflict")
	}
}
