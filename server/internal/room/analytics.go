package room

import (
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/analytics"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type roomAnalyticsRecorder struct {
	client  analytics.Client
	metrics *obsmetrics.BusinessMetrics
}

// SetAnalytics wires the canonical server analytics path. Call it once during
// server construction before the Room service starts handling requests.
func (s *Service) SetAnalytics(client analytics.Client, metrics *obsmetrics.BusinessMetrics) {
	s.analytics = roomAnalyticsRecorder{client: client, metrics: metrics}
}

func (s *Service) recordRoomCreated(roomRow db.Room, actorID pgtype.UUID) {
	s.recordRoomEvent(analytics.RoomCreated(
		uuidString(actorID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), roomRow.TemplateID.String,
	))
}

func (s *Service) recordRoomFirstCycleCompleted(roomRow db.Room, cycle db.RoomCycle, actorID pgtype.UUID) {
	s.recordRoomEvent(analytics.RoomFirstCycleCompleted(
		uuidString(actorID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), util.UUIDToString(cycle.ID), cycle.Source,
	))
}

func (s *Service) recordRoomSynthesisAccepted(roomRow db.Room, cycle db.RoomCycle, actorID pgtype.UUID) {
	s.recordRoomEvent(analytics.RoomSynthesisAccepted(
		uuidString(actorID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), util.UUIDToString(cycle.ID), cycle.Source,
	))
}

func (s *Service) recordRoomSynthesisRejected(roomRow db.Room, cycle db.RoomCycle, actorID pgtype.UUID) {
	s.recordRoomEvent(analytics.RoomSynthesisRejected(
		uuidString(actorID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), util.UUIDToString(cycle.ID), cycle.Source,
	))
}

func (s *Service) recordRoomSynthesisRetried(roomRow db.Room, cycle db.RoomCycle, actorID pgtype.UUID) {
	s.recordRoomEvent(analytics.RoomSynthesisRetried(
		uuidString(actorID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), util.UUIDToString(cycle.ID), cycle.Source,
	))
}

func (s *Service) recordRoomArtifactPromoted(roomRow db.Room, artifact db.RoomArtifact, actorID pgtype.UUID) {
	s.recordRoomEvent(analytics.RoomArtifactPromoted(
		uuidString(actorID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), uuidString(artifact.CycleID), artifact.Kind,
	))
}

func (s *Service) recordRoomBudgetRefused(roomRow db.Room, cycle db.RoomCycle, actorID pgtype.UUID) {
	if !cycle.RefusalReason.Valid || cycle.RefusalReason.String != "budget_exhausted" {
		return
	}
	s.recordRoomEvent(analytics.RoomBudgetRefused(
		uuidString(actorID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), util.UUIDToString(cycle.ID), cycle.Source, cycle.RefusalReason.String,
	))
}

func (s *Service) recordRoomSynthesisBudgetRefused(roomRow db.Room, cycle db.RoomCycle) {
	s.recordRoomEvent(analytics.RoomBudgetRefused(
		uuidString(roomRow.CreatedByUserID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), util.UUIDToString(cycle.ID), cycle.Source, "budget_exhausted",
	))
}

func (s *Service) recordRoomCycleFailed(roomRow db.Room, cycle db.RoomCycle, reason string) {
	s.recordRoomEvent(analytics.RoomCycleFailed(
		uuidString(roomRow.CreatedByUserID), util.UUIDToString(roomRow.WorkspaceID), util.UUIDToString(roomRow.ID), util.UUIDToString(cycle.ID), cycle.Source, reason,
	))
}

func (s *Service) recordRoomEvent(event analytics.Event) {
	obsmetrics.RecordEvent(s.analytics.client, s.analytics.metrics, event)
}

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return util.UUIDToString(value)
}
