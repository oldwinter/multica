package room

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type evaluatedPreflight struct {
	result PreflightResult
	agents []db.Agent
}

func (s *Service) Preflight(ctx context.Context, input PreflightInput) (PreflightResult, error) {
	if !validWakeSource(input.Source) {
		return PreflightResult{}, ErrInvalidInput
	}
	roomRow, err := s.queries.GetRoom(ctx, db.GetRoomParams{ID: input.RoomID, WorkspaceID: input.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return PreflightResult{}, ErrNotFound
	}
	if err != nil {
		return PreflightResult{}, fmt.Errorf("load Room for preflight: %w", err)
	}
	if _, err := s.queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: input.ActorUserID, WorkspaceID: input.WorkspaceID}); err != nil {
		return PreflightResult{}, ErrInvalidParticipant
	}
	targets, err := resolveWakeTargets(ctx, s.queries, roomRow, input.Source, input.TargetAgentIDs)
	if err != nil {
		return PreflightResult{}, err
	}
	evaluated, err := s.evaluatePreflight(ctx, s.queries, roomRow, input.ActorUserID, input.Source, targets)
	return evaluated.result, err
}

func (s *Service) Usage(ctx context.Context, workspaceID, roomID pgtype.UUID) (UsageSummary, error) {
	if _, err := s.queries.GetRoom(ctx, db.GetRoomParams{ID: roomID, WorkspaceID: workspaceID}); errors.Is(err, pgx.ErrNoRows) {
		return UsageSummary{}, ErrNotFound
	} else if err != nil {
		return UsageSummary{}, fmt.Errorf("load Room for usage: %w", err)
	}
	usage, err := s.queries.GetRoomUsageSummary(ctx, db.GetRoomUsageSummaryParams{WorkspaceID: workspaceID, RoomID: roomID})
	if err != nil {
		return UsageSummary{}, fmt.Errorf("load Room usage: %w", err)
	}
	return UsageSummary{
		TurnsTotal: usage.TurnsTotal, CostTicks: usage.CostTicks, UncostedTurns: usage.UncostedTurns,
		Failures: usage.Failures, AcceptedSyntheses: usage.AcceptedSyntheses, PromotedArtifacts: usage.PromotedArtifacts,
		RepeatRunCount: usage.RepeatRunCount, ActiveWeeks: usage.ActiveWeeks,
		MedianReviewLatencySeconds:    usage.MedianReviewLatencySeconds,
		AcceptedOutcomesPerActiveWeek: usage.AcceptedOutcomesPerActiveWeek,
		PromotionRate:                 usage.PromotionRate, FailedCycles: usage.FailedCycles,
		RefusedCycles:               usage.RefusedCycles,
		CostTicksPerAcceptedOutcome: usage.CostTicksPerAcceptedOutcome,
	}, nil
}

func resolveWakeTargets(ctx context.Context, queries *db.Queries, roomRow db.Room, source string, requested []pgtype.UUID) ([]pgtype.UUID, error) {
	requested = canonicalUUIDs(requested)
	participants, err := queries.ListRoomParticipants(ctx, db.ListRoomParticipantsParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID})
	if err != nil {
		return nil, fmt.Errorf("list Room wake participants: %w", err)
	}
	allowed := make(map[pgtype.UUID]struct{}, len(participants))
	allAgents := make([]pgtype.UUID, 0, len(participants))
	for _, participant := range participants {
		if participant.ParticipantType == "agent" {
			allowed[participant.ParticipantID] = struct{}{}
			allAgents = append(allAgents, participant.ParticipantID)
		}
	}
	if len(requested) == 0 {
		if source == "manual" || source == "schedule" {
			return canonicalUUIDs(allAgents), nil
		}
		return []pgtype.UUID{roomRow.FacilitatorAgentID}, nil
	}
	for _, targetID := range requested {
		if _, ok := allowed[targetID]; !ok {
			return nil, ErrInvalidParticipant
		}
	}
	return requested, nil
}

func (s *Service) evaluatePreflight(ctx context.Context, queries *db.Queries, roomRow db.Room, invokerID pgtype.UUID, source string, targetIDs []pgtype.UUID) (evaluatedPreflight, error) {
	expectedMaxTurns, synthesisRequired := roomCyclePlan(roomRow.CapabilityVersion, source, len(targetIDs))
	result := PreflightResult{
		Source:  source,
		Allowed: true, TargetAgents: make([]PreflightAgent, 0, len(targetIDs)),
		ExpectedMaxTurns:  expectedMaxTurns,
		SynthesisRequired: synthesisRequired,
		CapabilityVersion: roomRow.CapabilityVersion, CapabilityReady: true,
		SpendLimitSupported: true,
	}
	result.RequiredDaemonCapability = protocol.DaemonCapabilityRoomTasksV1
	if roomRow.CapabilityVersion >= 2 {
		result.RequiredDaemonCapability = protocol.DaemonCapabilityRoomOutcomesV2
	}
	if roomRow.MaxCostTicks.Valid {
		result.RequiredCostCapability = protocol.DaemonCapabilityRoomCostLimitsV1
	}
	if roomRow.DailyTurnLimit.Valid {
		limit := roomRow.DailyTurnLimit.Int32
		result.Budget.DailyTurnLimit = &limit
	}
	if roomRow.MaxCostTicks.Valid {
		limit := roomRow.MaxCostTicks.Int64
		result.Budget.MaxCostTicks = &limit
	}
	usage, err := queries.GetRoomUsageSummary(ctx, db.GetRoomUsageSummaryParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID})
	if err != nil {
		return evaluatedPreflight{}, fmt.Errorf("load Room preflight usage: %w", err)
	}
	result.Budget.UsedCostTicks = usage.CostTicks
	result.Budget.UncostedTurns = usage.UncostedTurns
	if roomRow.MaxCostTicks.Valid {
		remaining := roomRow.MaxCostTicks.Int64 - usage.CostTicks
		if remaining < 0 {
			remaining = 0
		}
		result.Budget.RemainingCostTicks = &remaining
	}
	if roomRow.ActiveCycleID.Valid {
		activeCycle, cycleErr := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{
			ID: roomRow.ActiveCycleID, WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
		})
		if cycleErr != nil && !errors.Is(cycleErr, pgx.ErrNoRows) {
			return evaluatedPreflight{}, fmt.Errorf("load active Room budget reservation: %w", cycleErr)
		}
		if cycleErr == nil && activeCycle.CostLimitTicks.Valid {
			cycleUsage, usageErr := queries.GetRoomCycleUsageSummary(ctx, db.GetRoomCycleUsageSummaryParams{
				WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: activeCycle.ID,
			})
			if usageErr != nil {
				return evaluatedPreflight{}, fmt.Errorf("load active Room reservation usage: %w", usageErr)
			}
			result.Budget.ReservedCostTicks = max(int64(0), activeCycle.CostLimitTicks.Int64-cycleUsage.CostTicks)
		}
	}
	now := s.now().UTC()
	usedToday, err := queries.CountRoomTurnsSince(ctx, db.CountRoomTurnsSinceParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
		SinceAt: pgtype.Timestamptz{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		return evaluatedPreflight{}, fmt.Errorf("count Room daily turns: %w", err)
	}
	result.Budget.UsedTurns = usedToday

	agents := make([]db.Agent, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		status := PreflightAgent{AgentID: targetID}
		agent, agentErr := queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: targetID, WorkspaceID: roomRow.WorkspaceID})
		if errors.Is(agentErr, pgx.ErrNoRows) {
			status.Reason = "agent_unavailable"
			result.Allowed = false
			result.TargetAgents = append(result.TargetAgents, status)
			continue
		}
		if agentErr != nil {
			return evaluatedPreflight{}, fmt.Errorf("load Room preflight Agent: %w", agentErr)
		}
		status.InvocationAllowed = canMemberInvokeAgent(ctx, queries, agent, invokerID, roomRow.WorkspaceID)
		if !status.InvocationAllowed {
			status.Reason = "invocation_not_allowed"
			result.Allowed = false
		}
		if roomRow.CapabilityVersion >= 2 {
			status.Ready, err = s.roomAgentReadyForCapability(ctx, queries, agent, result.RequiredDaemonCapability)
		} else {
			status.Ready, err = s.roomAgentReady(ctx, queries, agent)
		}
		if err != nil {
			return evaluatedPreflight{}, fmt.Errorf("check Room Agent readiness: %w", err)
		}
		if !status.Ready && status.Reason == "" {
			status.Reason = "daemon_capability_unavailable"
			result.Allowed = false
			result.CapabilityReady = false
		}
		if status.Ready && status.Reason == "" && roomRow.MaxCostTicks.Valid {
			costReady, costErr := s.roomAgentReadyForCapability(ctx, queries, agent, protocol.DaemonCapabilityRoomCostLimitsV1)
			if costErr != nil {
				return evaluatedPreflight{}, fmt.Errorf("check Room Agent spend-limit support: %w", costErr)
			}
			if !costReady {
				status.Ready = false
				status.Reason = "spend_limit_unsupported"
				result.Allowed = false
				result.SpendLimitSupported = false
			}
		}
		result.TargetAgents = append(result.TargetAgents, status)
		agents = append(agents, agent)
	}

	switch roomRow.Status {
	case "paused":
		result.RefusalReason = "room_paused"
	case "archived":
		result.RefusalReason = "room_archived"
	default:
		if roomRow.ActiveCycleID.Valid {
			result.RefusalReason = "cycle_active"
		}
	}
	if result.RefusalReason == "" && !result.Allowed {
		result.RefusalReason = "agent_unavailable"
		for _, agent := range result.TargetAgents {
			if agent.Reason == "invocation_not_allowed" {
				result.RefusalReason = "invocation_not_allowed"
				break
			}
			if agent.Reason == "spend_limit_unsupported" {
				result.RefusalReason = "spend_limit_unsupported"
			}
		}
	}
	if result.RefusalReason == "" && roomRow.DailyTurnLimit.Valid && usedToday+int64(result.ExpectedMaxTurns) > int64(roomRow.DailyTurnLimit.Int32) {
		result.RefusalReason = "budget_exhausted"
	}
	if result.RefusalReason == "" && roomRow.MaxCostTicks.Valid {
		remaining := roomRow.MaxCostTicks.Int64 - usage.CostTicks
		if usage.UncostedTurns > 0 || remaining < int64(result.ExpectedMaxTurns) {
			result.RefusalReason = "budget_exhausted"
		}
	}
	result.Allowed = result.RefusalReason == ""
	return evaluatedPreflight{result: result, agents: agents}, nil
}

func roomCyclePlan(capabilityVersion int32, source string, targetCount int) (int32, bool) {
	expectedMaxTurns := int32(targetCount)
	if capabilityVersion < 2 {
		return expectedMaxTurns, false
	}
	return expectedMaxTurns + 1, source == "schedule" || targetCount > 1
}

func roomTurnCostLimit(total pgtype.Int8, expectedMaxTurns, ordinal int32) *int64 {
	if !total.Valid || expectedMaxTurns <= 0 || ordinal < 0 || ordinal >= expectedMaxTurns || total.Int64 < int64(expectedMaxTurns) {
		return nil
	}
	limit := total.Int64 / int64(expectedMaxTurns)
	if int64(ordinal) < total.Int64%int64(expectedMaxTurns) {
		limit++
	}
	return &limit
}
