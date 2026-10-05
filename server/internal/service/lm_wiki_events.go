package service

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (s *WikiService) publishLMWikiSourcePolicyChanged(workspaceID, actorID pgtype.UUID, policyVersion int64) {
	s.publishLMWikiEvent(protocol.EventLMWikiSourcePolicyChanged, workspaceID, "member", actorID, protocol.LMWikiEventPayload{PolicyVersion: policyVersion})
}

func (s *WikiService) publishLMWikiRevisionChanged(workspaceID, requestedBy pgtype.UUID, revision db.LmWikiRevision) {
	actorType := "system"
	if requestedBy.Valid {
		actorType = "member"
	}
	s.publishLMWikiEvent(protocol.EventLMWikiRevisionChanged, workspaceID, actorType, requestedBy, protocol.LMWikiEventPayload{
		RevisionID: util.UUIDToString(revision.ID), RevisionNumber: revision.RevisionNumber,
	})
}

func (s *WikiService) publishLMWikiReviewChanged(workspaceID, reviewerID pgtype.UUID, revision db.LmWikiRevision, decision string) {
	s.publishLMWikiEvent(protocol.EventLMWikiReviewChanged, workspaceID, "member", reviewerID, protocol.LMWikiEventPayload{
		RevisionID: util.UUIDToString(revision.ID), RevisionNumber: revision.RevisionNumber,
		ReviewDecision: decision,
	})
}

func (s *WikiService) publishLMWikiEvent(eventType string, workspaceID pgtype.UUID, actorType string, actorID pgtype.UUID, payload protocol.LMWikiEventPayload) {
	if s.Events == nil {
		return
	}
	s.Events.Publish(events.Event{
		Type: eventType, WorkspaceID: util.UUIDToString(workspaceID),
		ActorType: actorType, ActorID: util.UUIDToString(actorID), Payload: payload,
	})
}
