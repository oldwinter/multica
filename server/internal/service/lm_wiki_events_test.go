package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestWikiServicePublishesTypedLMWikiLifecycleEvents(t *testing.T) {
	t.Parallel()

	workspaceID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	actorID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	revisionID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	bus := events.New()
	service := &WikiService{Events: bus}
	var got []events.Event
	bus.SubscribeAll(func(event events.Event) { got = append(got, event) })

	service.publishLMWikiSourcePolicyChanged(workspaceID, actorID, 4)
	service.publishLMWikiRevisionChanged(workspaceID, pgtype.UUID{}, db.LmWikiRevision{ID: revisionID, RevisionNumber: 7})
	service.publishLMWikiReviewChanged(workspaceID, actorID, db.LmWikiRevision{ID: revisionID, RevisionNumber: 7}, "accepted")

	if len(got) != 3 {
		t.Fatalf("published events = %#v, want 3", got)
	}
	wantTypes := []string{
		protocol.EventLMWikiSourcePolicyChanged,
		protocol.EventLMWikiRevisionChanged,
		protocol.EventLMWikiReviewChanged,
	}
	for index, event := range got {
		if event.Type != wantTypes[index] || event.WorkspaceID != util.UUIDToString(workspaceID) {
			t.Fatalf("event %d = %#v", index, event)
		}
	}
	policyPayload, ok := got[0].Payload.(protocol.LMWikiEventPayload)
	if !ok || policyPayload.PolicyVersion != 4 || got[0].ActorType != "member" || got[0].ActorID != util.UUIDToString(actorID) {
		t.Fatalf("policy event = %#v", got[0])
	}
	revisionPayload, ok := got[1].Payload.(protocol.LMWikiEventPayload)
	if !ok || revisionPayload.RevisionID != util.UUIDToString(revisionID) || revisionPayload.RevisionNumber != 7 || got[1].ActorType != "system" || got[1].ActorID != "" {
		t.Fatalf("revision event = %#v", got[1])
	}
	reviewPayload, ok := got[2].Payload.(protocol.LMWikiEventPayload)
	if !ok || reviewPayload.ReviewDecision != "accepted" || got[2].ActorType != "member" || got[2].ActorID != util.UUIDToString(actorID) {
		t.Fatalf("review event = %#v", got[2])
	}
}
