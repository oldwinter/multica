package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type TxStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type TaskNotifier interface {
	NotifyTaskEnqueued(context.Context, db.AgentTaskQueue)
}

type EventSink interface {
	Publish(events.Event)
}

type Service struct {
	queries *db.Queries
	tx      TxStarter
	tasks   TaskEnqueuer
	targets ArtifactTargetCreator
	events  EventSink
	now     func() time.Time
}

func NewService(queries *db.Queries, tx TxStarter, tasks TaskEnqueuer, targets ArtifactTargetCreator, eventSink EventSink) *Service {
	return &Service{queries: queries, tx: tx, tasks: tasks, targets: targets, events: eventSink, now: time.Now}
}

func (s *Service) List(ctx context.Context, workspaceID pgtype.UUID) ([]db.Room, error) {
	return s.queries.ListRooms(ctx, workspaceID)
}

func (s *Service) Get(ctx context.Context, workspaceID, roomID pgtype.UUID) (Detail, error) {
	roomRow, err := s.queries.GetRoom(ctx, db.GetRoomParams{ID: roomID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, fmt.Errorf("load room: %w", err)
	}
	return s.detail(ctx, s.queries, roomRow)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Detail, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Instructions = strings.TrimSpace(input.Instructions)
	if !input.WorkspaceID.Valid || !input.ActorUserID.Valid || input.Title == "" || len([]rune(input.Title)) > 160 || len([]rune(input.Instructions)) > 20000 {
		return Detail{}, ErrInvalidInput
	}
	if input.FacilitatorAgentID.Valid == input.FacilitatorSquadID.Valid {
		return Detail{}, fmt.Errorf("choose one facilitator: %w", ErrInvalidInput)
	}
	if input.DailyTurnLimit != nil && *input.DailyTurnLimit <= 0 {
		return Detail{}, fmt.Errorf("daily turn limit: %w", ErrInvalidInput)
	}
	if input.ScheduleIntervalMinutes != nil && (*input.ScheduleIntervalMinutes < 5 || *input.ScheduleIntervalMinutes > 10080) {
		return Detail{}, fmt.Errorf("schedule interval: %w", ErrInvalidInput)
	}

	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return Detail{}, fmt.Errorf("begin create room: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err := queries.LockRoomWorkspaceForWrite(ctx, input.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, ErrInvalidParticipant
		}
		return Detail{}, fmt.Errorf("lock room workspace: %w", err)
	}
	if _, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: input.ActorUserID, WorkspaceID: input.WorkspaceID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, ErrInvalidParticipant
		}
		return Detail{}, fmt.Errorf("validate room creator: %w", err)
	}

	facilitatorID := input.FacilitatorAgentID
	participants := append([]ParticipantInput(nil), input.Participants...)
	if input.FacilitatorSquadID.Valid {
		squad, squadErr := queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: input.FacilitatorSquadID, WorkspaceID: input.WorkspaceID})
		if squadErr != nil || squad.ArchivedAt.Valid {
			return Detail{}, ErrInvalidParticipant
		}
		facilitatorID = squad.LeaderID
		members, membersErr := queries.ListSquadMemberPreviewRowsBySquad(ctx, squad.ID)
		if membersErr != nil {
			return Detail{}, fmt.Errorf("list facilitator squad: %w", membersErr)
		}
		for _, member := range members {
			role := "participant"
			if member.MemberType == "agent" && member.MemberID == squad.LeaderID {
				role = "facilitator"
			}
			participants = append(participants, ParticipantInput{Type: member.MemberType, ID: member.MemberID, Role: role})
		}
	}
	if err := validateRoomAgent(ctx, queries, input.WorkspaceID, input.ActorUserID, facilitatorID); err != nil {
		return Detail{}, err
	}
	participants = append(participants,
		ParticipantInput{Type: "agent", ID: facilitatorID, Role: "facilitator"},
		ParticipantInput{Type: "member", ID: input.ActorUserID, Role: "participant"},
	)

	nextWake := pgtype.Timestamptz{}
	interval := pgtype.Int4{}
	if input.ScheduleIntervalMinutes != nil {
		interval = pgtype.Int4{Int32: *input.ScheduleIntervalMinutes, Valid: true}
		nextWake = pgtype.Timestamptz{Time: s.now().UTC().Add(time.Duration(*input.ScheduleIntervalMinutes) * time.Minute), Valid: true}
	}
	limit := pgtype.Int4{}
	if input.DailyTurnLimit != nil {
		limit = pgtype.Int4{Int32: *input.DailyTurnLimit, Valid: true}
	}
	roomRow, err := queries.CreateRoom(ctx, db.CreateRoomParams{
		WorkspaceID:             input.WorkspaceID,
		Title:                   input.Title,
		Instructions:            input.Instructions,
		CreatedByUserID:         input.ActorUserID,
		FacilitatorAgentID:      facilitatorID,
		FacilitatorSquadID:      input.FacilitatorSquadID,
		DailyTurnLimit:          limit,
		ScheduleIntervalMinutes: interval,
		NextWakeAt:              nextWake,
	})
	if err != nil {
		return Detail{}, fmt.Errorf("create room: %w", err)
	}
	for _, participant := range participants {
		role := participant.Role
		if role == "" {
			role = "participant"
		}
		if err := validateParticipant(ctx, queries, input.WorkspaceID, input.ActorUserID, participant); err != nil {
			return Detail{}, err
		}
		sourceSquadID := pgtype.UUID{}
		if input.FacilitatorSquadID.Valid {
			sourceSquadID = input.FacilitatorSquadID
		}
		if _, err := queries.AddRoomParticipant(ctx, db.AddRoomParticipantParams{
			WorkspaceID: input.WorkspaceID, RoomID: roomRow.ID,
			ParticipantType: participant.Type, ParticipantID: participant.ID,
			Role: role, SourceSquadID: sourceSquadID,
		}); err != nil {
			return Detail{}, fmt.Errorf("add room participant: %w", err)
		}
	}
	created, err := s.detail(ctx, queries, roomRow)
	if err != nil {
		return Detail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Detail{}, fmt.Errorf("commit create room: %w", err)
	}
	s.publish("room:created", roomRow, input.ActorUserID, map[string]any{"room": roomRow})
	return created, nil
}

func (s *Service) PostMessage(ctx context.Context, input MessageInput) (MessageResult, error) {
	input.Body = strings.TrimSpace(input.Body)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.MentionAgents = canonicalUUIDs(input.MentionAgents)
	if input.Body == "" || len([]rune(input.Body)) > 100000 || input.IdempotencyKey == "" {
		return MessageResult{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return MessageResult{}, fmt.Errorf("begin room message: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err := queries.LockRoomWorkspaceForWrite(ctx, input.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MessageResult{}, ErrNotFound
		}
		return MessageResult{}, fmt.Errorf("lock room workspace: %w", err)
	}
	roomRow, err := queries.GetRoomForUpdate(ctx, db.GetRoomForUpdateParams{ID: input.RoomID, WorkspaceID: input.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return MessageResult{}, ErrNotFound
	}
	if err != nil {
		return MessageResult{}, fmt.Errorf("lock room for message: %w", err)
	}
	if _, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: input.ActorUserID, WorkspaceID: input.WorkspaceID}); err != nil {
		return MessageResult{}, ErrInvalidParticipant
	}
	wakeKey := "message:" + input.IdempotencyKey
	mentionData, err := json.Marshal(input.MentionAgents)
	if err != nil {
		return MessageResult{}, fmt.Errorf("encode room mentions: %w", err)
	}
	if existing, existingErr := queries.GetRoomCycleByWakeKey(ctx, db.GetRoomCycleByWakeKeyParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, WakeKey: wakeKey}); existingErr == nil {
		entry, entryErr := queries.GetRoomEntry(ctx, db.GetRoomEntryParams{ID: existing.TriggeringEntryID, WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
		if entryErr != nil {
			return MessageResult{}, fmt.Errorf("load repeated room message: %w", entryErr)
		}
		if entry.Body != input.Body || !sameMentionIDs(entry.Mentions, input.MentionAgents) {
			return MessageResult{}, ErrIdempotencyConflict
		}
		result, resultErr := loadWakeResult(ctx, queries, existing)
		if resultErr != nil {
			return MessageResult{}, resultErr
		}
		targets := input.MentionAgents
		if len(targets) == 0 {
			targets = []pgtype.UUID{roomRow.FacilitatorAgentID}
		}
		if err := authorizeWakeReplay(ctx, queries, roomRow, input.ActorUserID, targets, result.Turns); err != nil {
			return MessageResult{}, err
		}
		result.replayed = true
		if err := tx.Commit(ctx); err != nil {
			return MessageResult{}, fmt.Errorf("commit repeated room message: %w", err)
		}
		return MessageResult{Entry: entry, WakeResult: result}, nil
	} else if !errors.Is(existingErr, pgx.ErrNoRows) {
		return MessageResult{}, fmt.Errorf("check repeated room message: %w", existingErr)
	}
	participants, err := queries.ListRoomParticipants(ctx, db.ListRoomParticipantsParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if err != nil {
		return MessageResult{}, fmt.Errorf("list room participants: %w", err)
	}
	participantAgents := make(map[pgtype.UUID]struct{}, len(participants))
	for _, participant := range participants {
		if participant.ParticipantType == "agent" {
			participantAgents[participant.ParticipantID] = struct{}{}
		}
	}
	for _, agentID := range input.MentionAgents {
		if _, ok := participantAgents[agentID]; !ok {
			return MessageResult{}, ErrInvalidParticipant
		}
	}
	entry, err := queries.AddRoomEntry(ctx, db.AddRoomEntryParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
		EntryType: "message", AuthorType: "member", AuthorID: input.ActorUserID,
		Body: input.Body, Mentions: mentionData,
	})
	if err != nil {
		return MessageResult{}, fmt.Errorf("add room message: %w", err)
	}
	targets := input.MentionAgents
	source := "mention"
	if len(targets) == 0 {
		targets = []pgtype.UUID{roomRow.FacilitatorAgentID}
		source = "message"
	}
	result, err := s.wakeTx(ctx, queries, roomRow, WakeInput{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID,
		ActorUserID: input.ActorUserID, Source: source, WakeKey: wakeKey,
		TriggeringEntryID: entry.ID, TargetAgentIDs: targets,
	})
	if err != nil {
		return MessageResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MessageResult{}, fmt.Errorf("commit room message: %w", err)
	}
	if !result.replayed {
		s.afterWake(ctx, roomRow, input.ActorUserID, result)
	}
	s.publish("room:entry", roomRow, input.ActorUserID, map[string]any{"room_id": util.UUIDToString(roomRow.ID), "entry": entry})
	return MessageResult{Entry: entry, WakeResult: result}, nil
}

func (s *Service) Wake(ctx context.Context, input WakeInput) (WakeResult, error) {
	input.WakeKey = strings.TrimSpace(input.WakeKey)
	if input.WakeKey == "" || !validWakeSource(input.Source) {
		return WakeResult{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return WakeResult{}, fmt.Errorf("begin room wake: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err := queries.LockRoomWorkspaceForWrite(ctx, input.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WakeResult{}, ErrNotFound
		}
		return WakeResult{}, fmt.Errorf("lock room workspace: %w", err)
	}
	roomRow, err := queries.GetRoomForUpdate(ctx, db.GetRoomForUpdateParams{ID: input.RoomID, WorkspaceID: input.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return WakeResult{}, ErrNotFound
	}
	if err != nil {
		return WakeResult{}, fmt.Errorf("lock room for wake: %w", err)
	}
	result, err := s.wakeTx(ctx, queries, roomRow, input)
	if err != nil {
		return WakeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WakeResult{}, fmt.Errorf("commit room wake: %w", err)
	}
	if !result.replayed {
		s.afterWake(ctx, roomRow, input.ActorUserID, result)
	}
	return result, nil
}

func (s *Service) SetStatus(ctx context.Context, workspaceID, roomID pgtype.UUID, status string) (db.Room, error) {
	if status != "active" && status != "paused" && status != "archived" {
		return db.Room{}, ErrInvalidInput
	}
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return db.Room{}, fmt.Errorf("begin room status update: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err := queries.LockRoomWorkspaceForWrite(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Room{}, ErrNotFound
		}
		return db.Room{}, fmt.Errorf("lock room workspace: %w", err)
	}
	updated, err := queries.SetRoomStatus(ctx, db.SetRoomStatusParams{Status: status, ID: roomID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Room{}, ErrNotFound
	}
	if err != nil {
		return db.Room{}, fmt.Errorf("set room status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Room{}, fmt.Errorf("commit room status update: %w", err)
	}
	s.publish("room:updated", updated, pgtype.UUID{}, map[string]any{"room": updated})
	return updated, nil
}

func (s *Service) wakeTx(ctx context.Context, queries *db.Queries, roomRow db.Room, input WakeInput) (WakeResult, error) {
	targetIDs := canonicalUUIDs(input.TargetAgentIDs)
	if len(targetIDs) == 0 {
		targetIDs = []pgtype.UUID{roomRow.FacilitatorAgentID}
	}
	participants, err := queries.ListRoomParticipants(ctx, db.ListRoomParticipantsParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID})
	if err != nil {
		return WakeResult{}, fmt.Errorf("list room wake participants: %w", err)
	}
	allowed := make(map[pgtype.UUID]struct{}, len(participants))
	for _, participant := range participants {
		if participant.ParticipantType == "agent" {
			allowed[participant.ParticipantID] = struct{}{}
		}
	}
	for _, targetID := range targetIDs {
		if _, ok := allowed[targetID]; !ok {
			return WakeResult{}, ErrInvalidParticipant
		}
	}

	invokerID := input.ActorUserID
	if !invokerID.Valid {
		if input.Source != "schedule" {
			return WakeResult{}, ErrInvocationNotAllowed
		}
		invokerID = roomRow.CreatedByUserID
	}
	_, memberErr := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: invokerID, WorkspaceID: roomRow.WorkspaceID})
	if memberErr != nil && !errors.Is(memberErr, pgx.ErrNoRows) {
		return WakeResult{}, fmt.Errorf("validate room invoker: %w", memberErr)
	}
	if existing, existingErr := queries.GetRoomCycleByWakeKey(ctx, db.GetRoomCycleByWakeKeyParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, WakeKey: input.WakeKey}); existingErr == nil {
		result, err := loadWakeResult(ctx, queries, existing)
		if err != nil {
			return WakeResult{}, err
		}
		if err := authorizeWakeReplay(ctx, queries, roomRow, invokerID, targetIDs, result.Turns); err != nil {
			return WakeResult{}, err
		}
		result.replayed = true
		return result, nil
	} else if !errors.Is(existingErr, pgx.ErrNoRows) {
		return WakeResult{}, fmt.Errorf("check room wake identity: %w", existingErr)
	}
	unavailable := errors.Is(memberErr, pgx.ErrNoRows)
	agents := make([]db.Agent, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		agent, agentErr := queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: targetID, WorkspaceID: roomRow.WorkspaceID})
		if agentErr != nil {
			if errors.Is(agentErr, pgx.ErrNoRows) {
				unavailable = true
				continue
			}
			return WakeResult{}, fmt.Errorf("load room target agent: %w", agentErr)
		}
		if !canMemberInvokeAgent(ctx, queries, agent, invokerID, roomRow.WorkspaceID) {
			return WakeResult{}, ErrInvocationNotAllowed
		}
		ready, readyErr := roomAgentReady(ctx, queries, agent)
		if readyErr != nil {
			return WakeResult{}, fmt.Errorf("check room target readiness: %w", readyErr)
		}
		if !ready {
			unavailable = true
		}
		agents = append(agents, agent)
	}

	reason := ""
	switch roomRow.Status {
	case "paused":
		reason = "room_paused"
	case "archived":
		reason = "room_archived"
	default:
		if roomRow.ActiveCycleID.Valid {
			reason = "cycle_active"
		}
	}
	if reason == "" && unavailable {
		reason = "no_targets"
	}
	if reason == "" && roomRow.DailyTurnLimit.Valid {
		now := s.now().UTC()
		used, err := queries.CountRoomTurnsSince(ctx, db.CountRoomTurnsSinceParams{
			WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID,
			SinceAt: pgtype.Timestamptz{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
		})
		if err != nil {
			return WakeResult{}, fmt.Errorf("count room turn budget: %w", err)
		}
		if used+int64(len(targetIDs)) > int64(roomRow.DailyTurnLimit.Int32) {
			reason = "budget_exhausted"
		}
	}
	if reason != "" {
		cycle, err := queries.CreateRoomCycle(ctx, db.CreateRoomCycleParams{
			WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, Source: input.Source,
			WakeKey: input.WakeKey, TriggeringEntryID: input.TriggeringEntryID,
			Status: "refused", RefusalReason: pgtype.Text{String: reason, Valid: true}, PlannedAt: input.PlannedAt,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			cycle, err = queries.GetRoomCycleByWakeKey(ctx, db.GetRoomCycleByWakeKeyParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, WakeKey: input.WakeKey})
			if err == nil {
				result, loadErr := loadWakeResult(ctx, queries, cycle)
				if loadErr != nil {
					return WakeResult{}, loadErr
				}
				if replayErr := authorizeWakeReplay(ctx, queries, roomRow, invokerID, targetIDs, result.Turns); replayErr != nil {
					return WakeResult{}, replayErr
				}
				result.replayed = true
				return result, nil
			}
		}
		if err != nil {
			return WakeResult{}, fmt.Errorf("persist refused room wake: %w", err)
		}
		turns := make([]db.RoomTurn, 0, len(targetIDs))
		for _, targetID := range targetIDs {
			turn, turnErr := queries.CreateRoomTurn(ctx, db.CreateRoomTurnParams{
				WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
				AgentID: targetID, SquadID: roomRow.FacilitatorSquadID, Status: "refused",
				RefusalReason: pgtype.Text{String: reason, Valid: true},
			})
			if turnErr != nil {
				return WakeResult{}, fmt.Errorf("persist refused Room turn: %w", turnErr)
			}
			turns = append(turns, turn)
		}
		return WakeResult{Cycle: cycle, Turns: turns, Tasks: []db.AgentTaskQueue{}}, nil
	}

	cycle, err := queries.CreateRoomCycle(ctx, db.CreateRoomCycleParams{
		WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, Source: input.Source,
		WakeKey: input.WakeKey, TriggeringEntryID: input.TriggeringEntryID,
		Status: "queued", PlannedAt: input.PlannedAt,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, existingErr := queries.GetRoomCycleByWakeKey(ctx, db.GetRoomCycleByWakeKeyParams{WorkspaceID: input.WorkspaceID, RoomID: input.RoomID, WakeKey: input.WakeKey})
		if existingErr != nil {
			return WakeResult{}, fmt.Errorf("reload repeated room wake: %w", existingErr)
		}
		result, loadErr := loadWakeResult(ctx, queries, existing)
		result.replayed = loadErr == nil
		return result, loadErr
	}
	if err != nil {
		return WakeResult{}, fmt.Errorf("create room cycle: %w", err)
	}
	if _, err := queries.SetRoomActiveCycle(ctx, db.SetRoomActiveCycleParams{
		ActiveCycleID: cycle.ID, ID: roomRow.ID, WorkspaceID: roomRow.WorkspaceID,
	}); err != nil {
		return WakeResult{}, fmt.Errorf("activate room cycle: %w", err)
	}
	entryRows, err := queries.ListRoomEntries(ctx, db.ListRoomEntriesParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, LimitCount: 100})
	if err != nil {
		return WakeResult{}, fmt.Errorf("load room transcript: %w", err)
	}
	entries := roomEntries(entryRows)
	result := WakeResult{Cycle: cycle, Turns: make([]db.RoomTurn, 0, len(agents)), Tasks: make([]db.AgentTaskQueue, 0, len(agents))}
	for _, agent := range agents {
		turn, err := queries.CreateRoomTurn(ctx, db.CreateRoomTurnParams{
			WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
			AgentID: agent.ID, SquadID: roomRow.FacilitatorSquadID, Status: "queued",
		})
		if err != nil {
			return WakeResult{}, fmt.Errorf("create room turn: %w", err)
		}
		previous, previousErr := queries.GetLastCompletedRoomTurnForAgent(ctx, db.GetLastCompletedRoomTurnForAgentParams{
			WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, AgentID: agent.ID,
		})
		if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
			return WakeResult{}, fmt.Errorf("load room session continuity: %w", previousErr)
		}
		contextData, err := protocol.EncodeRoomTaskContextV1(protocol.RoomTaskContextV1{
			WorkspaceID: util.UUIDToString(roomRow.WorkspaceID), RoomID: util.UUIDToString(roomRow.ID),
			CycleID: util.UUIDToString(cycle.ID), TurnID: util.UUIDToString(turn.ID), Title: roomRow.Title,
			Instructions: roomRow.Instructions, Memory: roomRow.Memory, Transcript: roomTaskTranscript(entries),
		})
		if err != nil {
			return WakeResult{}, fmt.Errorf("encode room task context: %w", err)
		}
		originator := input.ActorUserID
		accountable := input.ActorUserID
		source := "direct_human"
		if input.Source == "schedule" || !input.ActorUserID.Valid {
			originator = pgtype.UUID{}
			accountable = roomRow.CreatedByUserID
			source = "trigger_owner"
		}
		evidenceID := cycle.ID
		evidenceKind := "room_cycle"
		if input.TriggeringEntryID.Valid {
			evidenceID = input.TriggeringEntryID
			evidenceKind = "room_entry"
		}
		task, err := s.tasks.EnqueueRoomTurn(ctx, queries, RoomTaskEnqueueInput{
			Agent: agent, RoomTurnID: turn.ID, SquadID: roomRow.FacilitatorSquadID,
			Context: contextData, OriginatorUserID: originator, AccountableUserID: accountable,
			OriginatorSource: source, TriggerEvidenceKind: evidenceKind, TriggerEvidenceID: evidenceID,
			SessionID: previous.SessionID, WorkDir: previous.WorkDir,
		})
		if err != nil {
			return WakeResult{}, fmt.Errorf("enqueue room turn: %w", err)
		}
		result.Turns = append(result.Turns, turn)
		result.Tasks = append(result.Tasks, task)
	}
	return result, nil
}

func (s *Service) detail(ctx context.Context, queries *db.Queries, roomRow db.Room) (Detail, error) {
	participants, err := queries.ListRoomParticipants(ctx, db.ListRoomParticipantsParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID})
	if err != nil {
		return Detail{}, fmt.Errorf("list room participants: %w", err)
	}
	entryRows, err := queries.ListRoomEntries(ctx, db.ListRoomEntriesParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, LimitCount: 200})
	if err != nil {
		return Detail{}, fmt.Errorf("list room entries: %w", err)
	}
	entries := roomEntries(entryRows)
	cycles, err := queries.ListRoomCycles(ctx, db.ListRoomCyclesParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, LimitCount: 100})
	if err != nil {
		return Detail{}, fmt.Errorf("list room cycles: %w", err)
	}
	turns, err := queries.ListRoomTurns(ctx, db.ListRoomTurnsParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID})
	if err != nil {
		return Detail{}, fmt.Errorf("list room turns: %w", err)
	}
	artifacts, err := queries.ListRoomArtifacts(ctx, db.ListRoomArtifactsParams{WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID})
	if err != nil {
		return Detail{}, fmt.Errorf("list room artifacts: %w", err)
	}
	return Detail{Room: roomRow, Participants: participants, Entries: entries, Cycles: cycles, Turns: turns, Artifacts: artifacts}, nil
}

func validateParticipant(ctx context.Context, queries *db.Queries, workspaceID, actorUserID pgtype.UUID, participant ParticipantInput) error {
	if !participant.ID.Valid || (participant.Role != "" && participant.Role != "facilitator" && participant.Role != "participant" && participant.Role != "observer") {
		return ErrInvalidParticipant
	}
	switch participant.Type {
	case "agent":
		return validateRoomAgent(ctx, queries, workspaceID, actorUserID, participant.ID)
	case "member":
		_, err := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: participant.ID, WorkspaceID: workspaceID})
		if err != nil {
			return ErrInvalidParticipant
		}
	default:
		return ErrInvalidParticipant
	}
	return nil
}

func loadWakeResult(ctx context.Context, queries *db.Queries, cycle db.RoomCycle) (WakeResult, error) {
	turns, err := queries.ListRoomTurnsByCycle(ctx, db.ListRoomTurnsByCycleParams{WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID, CycleID: cycle.ID})
	if err != nil {
		return WakeResult{}, fmt.Errorf("list repeated room turns: %w", err)
	}
	tasks := make([]db.AgentTaskQueue, 0, len(turns))
	for _, turn := range turns {
		task, taskErr := queries.GetLatestTaskForRoomTurn(ctx, turn.ID)
		if taskErr == nil {
			tasks = append(tasks, task)
		} else if !errors.Is(taskErr, pgx.ErrNoRows) {
			return WakeResult{}, fmt.Errorf("load repeated room task: %w", taskErr)
		}
	}
	return WakeResult{Cycle: cycle, Turns: turns, Tasks: tasks}, nil
}

func validWakeSource(source string) bool {
	return source == "message" || source == "mention" || source == "manual" || source == "schedule" || source == "agent"
}

func deduplicateUUIDs(ids []pgtype.UUID) []pgtype.UUID {
	seen := make(map[pgtype.UUID]struct{}, len(ids))
	result := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func canonicalUUIDs(ids []pgtype.UUID) []pgtype.UUID {
	result := deduplicateUUIDs(ids)
	sort.Slice(result, func(i, j int) bool {
		return util.UUIDToString(result[i]) < util.UUIDToString(result[j])
	})
	return result
}

func sameUUIDs(left, right []pgtype.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func authorizeWakeReplay(ctx context.Context, queries *db.Queries, roomRow db.Room, invokerID pgtype.UUID, targetIDs []pgtype.UUID, turns []db.RoomTurn) error {
	targetIDs = canonicalUUIDs(targetIDs)
	if len(turns) > 0 {
		existingTargets := make([]pgtype.UUID, len(turns))
		for index, turn := range turns {
			existingTargets[index] = turn.AgentID
		}
		existingTargets = canonicalUUIDs(existingTargets)
		if !sameUUIDs(existingTargets, targetIDs) {
			return ErrIdempotencyConflict
		}
		targetIDs = existingTargets
	}
	for _, targetID := range targetIDs {
		agent, err := queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: targetID, WorkspaceID: roomRow.WorkspaceID})
		if err != nil || !canMemberInvokeAgent(ctx, queries, agent, invokerID, roomRow.WorkspaceID) {
			return ErrInvocationNotAllowed
		}
	}
	return nil
}

func roomEntries(rows []db.RoomEntry) []db.RoomEntry {
	entries := make([]db.RoomEntry, len(rows))
	for index := range rows {
		entries[len(rows)-1-index] = rows[index]
	}
	return entries
}

func roomTaskTranscript(entries []db.RoomEntry) []protocol.RoomTaskTranscriptEntryV1 {
	transcript := make([]protocol.RoomTaskTranscriptEntryV1, len(entries))
	for index, entry := range entries {
		var mentions []pgtype.UUID
		_ = json.Unmarshal(entry.Mentions, &mentions)
		mentionIDs := make([]string, len(mentions))
		for mentionIndex, mention := range mentions {
			mentionIDs[mentionIndex] = util.UUIDToString(mention)
		}
		transcript[index] = protocol.RoomTaskTranscriptEntryV1{
			ID: util.UUIDToString(entry.ID), Ordinal: entry.Ordinal,
			EntryType: entry.EntryType, AuthorType: entry.AuthorType,
			AuthorID: util.UUIDToString(entry.AuthorID), Body: entry.Body,
			Mentions: mentionIDs, CreatedAt: entry.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
		}
	}
	return transcript
}

func roomAgentReady(ctx context.Context, queries *db.Queries, agent db.Agent) (bool, error) {
	if agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
		return false, nil
	}
	runtimeRow, err := queries.GetAgentRuntime(ctx, agent.RuntimeID)
	if err != nil {
		return false, err
	}
	return runtimeRow.Status == "online", nil
}

func sameMentionIDs(raw json.RawMessage, want []pgtype.UUID) bool {
	var got []pgtype.UUID
	if err := json.Unmarshal(raw, &got); err != nil {
		return false
	}
	got = canonicalUUIDs(got)
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func validateRoomAgent(ctx context.Context, queries *db.Queries, workspaceID, actorUserID, agentID pgtype.UUID) error {
	agent, err := queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: workspaceID})
	if err != nil || agent.ArchivedAt.Valid {
		return ErrInvalidParticipant
	}
	if !canMemberInvokeAgent(ctx, queries, agent, actorUserID, workspaceID) {
		return ErrInvocationNotAllowed
	}
	return nil
}

func canMemberInvokeAgent(ctx context.Context, queries *db.Queries, agent db.Agent, userID, workspaceID pgtype.UUID) bool {
	if !userID.Valid {
		return false
	}
	if agent.OwnerID == userID {
		return true
	}
	if agent.PermissionMode != "public_to" {
		return false
	}
	targets, err := queries.ListAgentInvocationTargets(ctx, agent.ID)
	if err != nil {
		return false
	}
	_, memberErr := queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: userID, WorkspaceID: workspaceID})
	for _, target := range targets {
		switch target.TargetType {
		case "workspace":
			if memberErr == nil {
				return true
			}
		case "member":
			if target.TargetID == userID {
				return true
			}
		}
	}
	return false
}

func (s *Service) afterWake(ctx context.Context, roomRow db.Room, actorID pgtype.UUID, result WakeResult) {
	for _, task := range result.Tasks {
		if notifier, ok := s.tasks.(TaskNotifier); ok {
			notifier.NotifyTaskEnqueued(ctx, task)
		}
	}
	s.publish("room:cycle", roomRow, actorID, map[string]any{"room_id": util.UUIDToString(roomRow.ID), "cycle": result.Cycle, "turns": roomTurnEvents(result.Turns)})
}

type roomTurnEvent struct {
	ID            string  `json:"id"`
	CycleID       string  `json:"cycle_id"`
	Status        string  `json:"status"`
	RefusalReason *string `json:"refusal_reason,omitempty"`
	CreatedAt     string  `json:"created_at"`
	StartedAt     *string `json:"started_at,omitempty"`
	CompletedAt   *string `json:"completed_at,omitempty"`
}

func roomTurnEvents(turns []db.RoomTurn) []roomTurnEvent {
	result := make([]roomTurnEvent, len(turns))
	for index, turn := range turns {
		result[index] = roomTurnEvent{
			ID: util.UUIDToString(turn.ID), CycleID: util.UUIDToString(turn.CycleID),
			Status: turn.Status, RefusalReason: nullableText(turn.RefusalReason),
			CreatedAt: nullableTimestamp(turn.CreatedAt), StartedAt: timestampPointer(turn.StartedAt),
			CompletedAt: timestampPointer(turn.CompletedAt),
		}
	}
	return result
}

func nullableText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullableTimestamp(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func timestampPointer(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func (s *Service) publish(eventType string, roomRow db.Room, actorID pgtype.UUID, payload any) {
	if s.events == nil {
		return
	}
	actorType := "system"
	actor := ""
	if actorID.Valid {
		actorType = "member"
		actor = util.UUIDToString(actorID)
	}
	s.events.Publish(events.Event{Type: eventType, WorkspaceID: util.UUIDToString(roomRow.WorkspaceID), ActorType: actorType, ActorID: actor, Payload: payload})
}
