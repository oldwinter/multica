package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

const (
	maxRoomEntryRunes          = 100000
	maxMemorySummaryRunes      = 4000
	maxMemoryContributionRunes = 6000
	maxMemoryContributions     = 10
)

type memoryContribution struct {
	AgentID string `json:"agent_id"`
	TurnID  string `json:"turn_id"`
	Body    string `json:"body"`
	At      string `json:"at"`
}

type roomMemory struct {
	Summary             string               `json:"summary"`
	Facts               []string             `json:"facts"`
	Decisions           []string             `json:"decisions"`
	OpenQuestions       []string             `json:"open_questions"`
	RecentContributions []memoryContribution `json:"recent_contributions,omitempty"`
}

type taskSyncOutcome struct {
	changed       bool
	room          db.Room
	cycle         db.RoomCycle
	entry         *db.RoomEntry
	task          *db.AgentTaskQueue
	turn          *db.RoomTurn
	revision      *db.RoomMemoryRevision
	failureReason string
	budgetRefused bool
}

func (s *Service) SyncTask(ctx context.Context, taskID pgtype.UUID) (bool, error) {
	if !taskID.Valid {
		return false, ErrInvalidInput
	}
	outcome, err := s.syncTask(ctx, taskID)
	if err != nil || !outcome.changed {
		return false, err
	}
	if outcome.entry != nil {
		s.publish(EventRoomEntry, outcome.room, pgtype.UUID{}, roomEntryEventPayload(*outcome.entry))
	}
	if outcome.task != nil && outcome.turn != nil {
		if notifier, ok := s.tasks.(TaskNotifier); ok {
			notifier.NotifyTaskEnqueued(ctx, *outcome.task)
		}
		s.publish(EventRoomTurn, outcome.room, pgtype.UUID{}, roomTurnEventPayload(*outcome.turn))
	}
	if outcome.revision != nil {
		s.publish(EventRoomMemoryRevision, outcome.room, pgtype.UUID{}, roomMemoryRevisionEventPayload(*outcome.revision))
	}
	if outcome.failureReason != "" {
		s.recordRoomCycleFailed(outcome.room, outcome.cycle, outcome.failureReason)
	}
	if outcome.budgetRefused {
		s.recordRoomSynthesisBudgetRefused(outcome.room, outcome.cycle)
	}
	s.publish(EventRoomCycle, outcome.room, pgtype.UUID{}, roomCycleEventPayload(outcome.cycle))
	return true, nil
}

func (s *Service) Reconcile(ctx context.Context, limit int32) (int, error) {
	if limit <= 0 || limit > 1000 {
		return 0, ErrInvalidInput
	}
	tasks, err := s.queries.ListUnsyncedTerminalRoomTasks(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("list unsynchronized Room tasks: %w", err)
	}
	count := 0
	for _, task := range tasks {
		changed, syncErr := s.SyncTask(ctx, task.ID)
		if syncErr != nil {
			return count, fmt.Errorf("synchronize Room task %s: %w", util.UUIDToString(task.ID), syncErr)
		}
		if changed {
			count++
		}
	}
	return count, nil
}

func (s *Service) syncTask(ctx context.Context, taskID pgtype.UUID) (taskSyncOutcome, error) {
	tx, err := s.tx.Begin(ctx)
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("begin Room task synchronization: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)

	task, err := queries.GetAgentTask(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return taskSyncOutcome{}, nil
	}
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("load Room task: %w", err)
	}
	if !task.RoomTurnID.Valid {
		return taskSyncOutcome{}, nil
	}
	turn, err := queries.GetRoomTurnByTask(ctx, task.ID)
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("load Room turn: %w", err)
	}
	if _, err := queries.LockRoomWorkspaceForWrite(ctx, turn.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return taskSyncOutcome{}, nil
		}
		return taskSyncOutcome{}, fmt.Errorf("lock Room workspace: %w", err)
	}
	roomRow, err := queries.GetRoomForUpdate(ctx, db.GetRoomForUpdateParams{ID: turn.RoomID, WorkspaceID: turn.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return taskSyncOutcome{}, nil
	}
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("lock Room for task synchronization: %w", err)
	}
	latest, err := queries.GetLatestTaskForRoomTurn(ctx, turn.ID)
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("load latest Room task attempt: %w", err)
	}
	if latest.ID != task.ID {
		return taskSyncOutcome{}, nil
	}

	if task.Status == "running" {
		changed := false
		if _, err := queries.MarkRoomTurnRunning(ctx, turn.ID); err == nil {
			changed = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return taskSyncOutcome{}, fmt.Errorf("mark Room turn running: %w", err)
		}
		cycle, cycleErr := queries.MarkRoomCycleRunning(ctx, db.MarkRoomCycleRunningParams{ID: turn.CycleID, WorkspaceID: turn.WorkspaceID})
		if cycleErr == nil {
			changed = true
		} else if errors.Is(cycleErr, pgx.ErrNoRows) {
			cycle, cycleErr = queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: turn.CycleID, WorkspaceID: turn.WorkspaceID, RoomID: turn.RoomID})
		} else {
			return taskSyncOutcome{}, fmt.Errorf("mark Room cycle running: %w", cycleErr)
		}
		if cycleErr != nil {
			return taskSyncOutcome{}, fmt.Errorf("load running Room cycle: %w", cycleErr)
		}
		if !changed {
			return taskSyncOutcome{}, nil
		}
		if err := tx.Commit(ctx); err != nil {
			return taskSyncOutcome{}, fmt.Errorf("commit Room running state: %w", err)
		}
		return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle}, nil
	}
	if task.Status != "completed" && task.Status != "failed" && task.Status != "cancelled" {
		return taskSyncOutcome{}, nil
	}

	completedTurn, err := queries.CompleteRoomTurn(ctx, db.CompleteRoomTurnParams{
		Status: task.Status, Result: task.Result, SessionID: task.SessionID,
		WorkDir: task.WorkDir, StartedAt: task.StartedAt, ID: turn.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return taskSyncOutcome{}, nil
	}
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("complete Room turn: %w", err)
	}

	var entry *db.RoomEntry
	if task.Status == "completed" {
		body := completedTaskOutput(task.Result)
		created, entryErr := queries.AddRoomEntry(ctx, db.AddRoomEntryParams{
			WorkspaceID: turn.WorkspaceID, RoomID: turn.RoomID,
			CycleID: turn.CycleID, TurnID: turn.ID, EntryType: "result",
			AuthorType: "agent", AuthorID: turn.AgentID, Body: truncateRunes(body, maxRoomEntryRunes),
			Mentions: []byte("[]"),
		})
		if entryErr != nil {
			return taskSyncOutcome{}, fmt.Errorf("append Room result: %w", entryErr)
		}
		entry = &created
	}
	if roomRow.CapabilityVersion >= 2 {
		return s.syncOutcomeV2Tx(ctx, tx, queries, task, completedTurn, roomRow, entry)
	}

	turns, err := queries.ListRoomTurnsByCycle(ctx, db.ListRoomTurnsByCycleParams{
		WorkspaceID: turn.WorkspaceID, RoomID: turn.RoomID, CycleID: turn.CycleID,
	})
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("list Room cycle turns: %w", err)
	}
	cycleStatus, terminal := roomCycleStatus(turns)
	cycle := db.RoomCycle{ID: turn.CycleID, WorkspaceID: turn.WorkspaceID, RoomID: turn.RoomID, Status: "running"}
	if terminal {
		cycle, err = queries.CompleteRoomCycle(ctx, db.CompleteRoomCycleParams{
			Status: cycleStatus, StartedAt: completedTurn.StartedAt,
			ID: turn.CycleID, WorkspaceID: turn.WorkspaceID,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return taskSyncOutcome{}, fmt.Errorf("complete Room cycle: %w", err)
		}
	} else {
		cycle, err = queries.MarkRoomCycleRunning(ctx, db.MarkRoomCycleRunningParams{ID: turn.CycleID, WorkspaceID: turn.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			cycle, err = queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: turn.CycleID, WorkspaceID: turn.WorkspaceID, RoomID: turn.RoomID})
		}
		if err != nil {
			return taskSyncOutcome{}, fmt.Errorf("keep Room cycle running: %w", err)
		}
	}

	if task.Status == "completed" {
		memory, memoryErr := reduceRoomMemory(roomRow.Memory, completedTurn, entry.Body, s.now())
		if memoryErr != nil {
			return taskSyncOutcome{}, memoryErr
		}
		completedCycleID := pgtype.UUID{}
		if terminal {
			completedCycleID = turn.CycleID
		}
		roomRow, err = queries.UpdateRoomMemory(ctx, db.UpdateRoomMemoryParams{
			Memory: memory, CompletedCycleID: completedCycleID,
			ID: roomRow.ID, WorkspaceID: roomRow.WorkspaceID,
			ExpectedMemoryVersion: roomRow.MemoryVersion,
		})
		if err != nil {
			return taskSyncOutcome{}, fmt.Errorf("advance Room memory: %w", err)
		}
	} else if terminal {
		cleared, clearErr := queries.ClearRoomActiveCycle(ctx, db.ClearRoomActiveCycleParams{
			ID: roomRow.ID, WorkspaceID: roomRow.WorkspaceID, CompletedCycleID: turn.CycleID,
		})
		if clearErr == nil {
			roomRow = cleared
		} else if !errors.Is(clearErr, pgx.ErrNoRows) {
			return taskSyncOutcome{}, fmt.Errorf("clear completed Room cycle: %w", clearErr)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return taskSyncOutcome{}, fmt.Errorf("commit Room task synchronization: %w", err)
	}
	failureReason := ""
	if cycle.Status == "failed" {
		failureReason = "participant_failed"
	}
	return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle, entry: entry, failureReason: failureReason}, nil
}

func (s *Service) syncOutcomeV2Tx(ctx context.Context, tx pgx.Tx, queries *db.Queries, task db.AgentTaskQueue, completedTurn db.RoomTurn, roomRow db.Room, entry *db.RoomEntry) (taskSyncOutcome, error) {
	cycle, err := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: completedTurn.CycleID, WorkspaceID: completedTurn.WorkspaceID, RoomID: completedTurn.RoomID})
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("load Room outcome cycle: %w", err)
	}
	if completedTurn.TurnKind == "participant" {
		turns, listErr := queries.ListRoomTurnsByCycle(ctx, db.ListRoomTurnsByCycleParams{
			WorkspaceID: completedTurn.WorkspaceID, RoomID: completedTurn.RoomID, CycleID: completedTurn.CycleID,
		})
		if listErr != nil {
			return taskSyncOutcome{}, fmt.Errorf("list Room participant turns: %w", listErr)
		}
		participantCount, successfulParticipants, participantTerminal := participantTurnsTerminal(turns)
		if !participantTerminal {
			cycle, err = keepRoomCycleRunning(ctx, queries, cycle)
			if err != nil {
				return taskSyncOutcome{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return taskSyncOutcome{}, fmt.Errorf("commit Room participant result: %w", err)
			}
			return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle, entry: entry}, nil
		}
		if successfulParticipants == 0 {
			cycle, err = keepRoomCycleRunning(ctx, queries, cycle)
			if err != nil {
				return taskSyncOutcome{}, err
			}
			errorData, _ := json.Marshal(synthesisError{Code: "participant_results_unavailable", Message: "No participant result was produced.", Retryable: false})
			cycle, err = queries.FailRoomOutcomeCycle(ctx, db.FailRoomOutcomeCycleParams{
				SynthesisError: errorData, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
			})
			if err != nil {
				return taskSyncOutcome{}, fmt.Errorf("fail Room cycle without participant results: %w", err)
			}
			roomRow, err = queries.ClearRoomActiveCycle(ctx, db.ClearRoomActiveCycleParams{
				ID: roomRow.ID, WorkspaceID: roomRow.WorkspaceID, CompletedCycleID: cycle.ID,
			})
			if err != nil {
				return taskSyncOutcome{}, fmt.Errorf("clear failed Room outcome cycle: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return taskSyncOutcome{}, fmt.Errorf("commit failed Room outcome cycle: %w", err)
			}
			return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle, failureReason: "participant_failed"}, nil
		}

		requiresSynthesis := cycle.Source == "schedule" || participantCount > 1
		if !requiresSynthesis && task.Status == "completed" && entry != nil {
			if revision, revisedCycle, ok, reviseErr := s.tryCreateRoomRevision(ctx, queries, roomRow, cycle, completedTurn, []byte(entry.Body), true); reviseErr != nil {
				return taskSyncOutcome{}, reviseErr
			} else if ok {
				if err := tx.Commit(ctx); err != nil {
					return taskSyncOutcome{}, fmt.Errorf("commit direct Room synthesis: %w", err)
				}
				return taskSyncOutcome{changed: true, room: roomRow, cycle: revisedCycle, entry: entry, revision: &revision}, nil
			}
		}

		synthesisTurn, synthesisTask, enqueueErr := s.enqueueSynthesisTx(ctx, queries, roomRow, cycle, pgtype.UUID{}, "initial:"+util.UUIDToString(cycle.ID), true)
		if enqueueErr != nil {
			if errors.Is(enqueueErr, ErrBudgetExhausted) {
				errorData, _ := json.Marshal(synthesisError{Code: "budget_exhausted", Message: "The Room budget was exhausted before synthesis could start.", Retryable: true})
				cycle, err = queries.SetRoomCycleSynthesisBlocked(ctx, db.SetRoomCycleSynthesisBlockedParams{
					SynthesisError: errorData, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
				})
				if err != nil {
					return taskSyncOutcome{}, fmt.Errorf("persist budget-blocked Room synthesis: %w", err)
				}
				if err := tx.Commit(ctx); err != nil {
					return taskSyncOutcome{}, fmt.Errorf("commit budget-blocked Room synthesis: %w", err)
				}
				return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle, entry: entry, budgetRefused: true}, nil
			}
			if errors.Is(enqueueErr, ErrSynthesisNotRetryable) || errors.Is(enqueueErr, ErrInvalidParticipant) {
				errorData, _ := json.Marshal(synthesisError{Code: "facilitator_unavailable", Message: "The facilitator is unavailable or lacks the Room outcome capability.", Retryable: true})
				cycle, err = queries.SetRoomCycleSynthesisBlocked(ctx, db.SetRoomCycleSynthesisBlockedParams{
					SynthesisError: errorData, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
				})
				if err != nil {
					return taskSyncOutcome{}, fmt.Errorf("persist blocked Room synthesis: %w", err)
				}
				if err := tx.Commit(ctx); err != nil {
					return taskSyncOutcome{}, fmt.Errorf("commit blocked Room synthesis: %w", err)
				}
				return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle, entry: entry, failureReason: "facilitator_unavailable"}, nil
			}
			return taskSyncOutcome{}, enqueueErr
		}
		cycle, err = queries.SetRoomCycleSynthesizing(ctx, db.SetRoomCycleSynthesizingParams{
			SynthesisTurnID: synthesisTurn.ID, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
		})
		if err != nil {
			return taskSyncOutcome{}, fmt.Errorf("start Room facilitator synthesis: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return taskSyncOutcome{}, fmt.Errorf("commit Room facilitator synthesis: %w", err)
		}
		return taskSyncOutcome{
			changed: true, room: roomRow, cycle: cycle, entry: entry,
			task: &synthesisTask, turn: &synthesisTurn,
		}, nil
	}

	if completedTurn.TurnKind != "synthesis" {
		return taskSyncOutcome{}, fmt.Errorf("unsupported Room turn kind %q", completedTurn.TurnKind)
	}
	if task.Status == "completed" && entry != nil {
		revision, revisedCycle, ok, reviseErr := s.tryCreateRoomRevision(ctx, queries, roomRow, cycle, completedTurn, []byte(entry.Body), false)
		if reviseErr != nil {
			return taskSyncOutcome{}, reviseErr
		}
		if ok {
			if err := tx.Commit(ctx); err != nil {
				return taskSyncOutcome{}, fmt.Errorf("commit Room memory revision: %w", err)
			}
			return taskSyncOutcome{changed: true, room: roomRow, cycle: revisedCycle, entry: entry, revision: &revision}, nil
		}
	}
	errorCode := "synthesis_failed"
	message := "The synthesis turn failed. Participant contributions were preserved."
	if task.Status == "completed" {
		errorCode = "malformed_synthesis"
		message = "The facilitator response did not match the Room synthesis contract."
	}
	errorData, _ := json.Marshal(synthesisError{Code: errorCode, Message: message, Retryable: true})
	cycle, err = queries.SetRoomCycleAwaitingReview(ctx, db.SetRoomCycleAwaitingReviewParams{
		SynthesisError: errorData, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
	})
	if err != nil {
		return taskSyncOutcome{}, fmt.Errorf("persist Room synthesis error: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return taskSyncOutcome{}, fmt.Errorf("commit Room synthesis error: %w", err)
	}
	return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle, entry: entry, failureReason: errorCode}, nil
}

func (s *Service) tryCreateRoomRevision(ctx context.Context, queries *db.Queries, roomRow db.Room, cycle db.RoomCycle, turn db.RoomTurn, raw []byte, direct bool) (db.RoomMemoryRevision, db.RoomCycle, bool, error) {
	_, canonical, digest, validateErr := validateRoomSynthesis(ctx, queries, roomRow.WorkspaceID, roomRow.ID, raw)
	if validateErr != nil {
		if errors.Is(validateErr, ErrInvalidSynthesis) {
			return db.RoomMemoryRevision{}, cycle, false, nil
		}
		return db.RoomMemoryRevision{}, cycle, false, validateErr
	}
	var err error
	if direct {
		cycle, err = queries.SetRoomCycleSynthesizing(ctx, db.SetRoomCycleSynthesizingParams{
			SynthesisTurnID: turn.ID, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
		})
		if err != nil {
			return db.RoomMemoryRevision{}, cycle, false, fmt.Errorf("advance direct Room synthesis: %w", err)
		}
	}
	revision, err := queries.CreateRoomMemoryRevision(ctx, db.CreateRoomMemoryRevisionParams{
		WorkspaceID: roomRow.WorkspaceID, RoomID: roomRow.ID, CycleID: cycle.ID,
		SynthesisTurnID: turn.ID, SchemaVersion: RoomSynthesisSchemaVersion,
		Synthesis: canonical, Digest: digest, CreatorType: "agent", CreatorID: turn.AgentID,
	})
	if err != nil {
		return db.RoomMemoryRevision{}, cycle, false, fmt.Errorf("create Room memory revision: %w", err)
	}
	cycle, err = queries.SetRoomCycleAwaitingReview(ctx, db.SetRoomCycleAwaitingReviewParams{
		MemoryRevisionID: revision.ID, ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID,
	})
	if err != nil {
		return db.RoomMemoryRevision{}, cycle, false, fmt.Errorf("link Room memory revision: %w", err)
	}
	return revision, cycle, true, nil
}

func participantTurnsTerminal(turns []db.RoomTurn) (int, int, bool) {
	count := 0
	successful := 0
	for _, turn := range turns {
		if turn.TurnKind != "participant" {
			continue
		}
		count++
		switch turn.Status {
		case "completed":
			successful++
		case "failed", "cancelled", "refused":
		default:
			return count, successful, false
		}
	}
	return count, successful, count > 0
}

func keepRoomCycleRunning(ctx context.Context, queries *db.Queries, cycle db.RoomCycle) (db.RoomCycle, error) {
	updated, err := queries.MarkRoomCycleRunning(ctx, db.MarkRoomCycleRunningParams{ID: cycle.ID, WorkspaceID: cycle.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		updated, err = queries.GetRoomCycle(ctx, db.GetRoomCycleParams{ID: cycle.ID, WorkspaceID: cycle.WorkspaceID, RoomID: cycle.RoomID})
	}
	if err != nil {
		return db.RoomCycle{}, fmt.Errorf("keep Room cycle running: %w", err)
	}
	return updated, nil
}

func completedTaskOutput(result []byte) string {
	var payload protocol.TaskCompletedPayload
	if json.Unmarshal(result, &payload) == nil {
		if output := strings.TrimSpace(util.UnescapeBackslashEscapes(payload.Output)); output != "" {
			return redact.Text(output)
		}
	}
	return "This agent completed the turn without a text response."
}

func reduceRoomMemory(raw []byte, turn db.RoomTurn, body string, now time.Time) ([]byte, error) {
	memory := roomMemory{Facts: []string{}, Decisions: []string{}, OpenQuestions: []string{}, RecentContributions: []memoryContribution{}}
	if len(raw) > 0 && json.Unmarshal(raw, &memory) != nil {
		return nil, fmt.Errorf("decode Room memory")
	}
	contribution := memoryContribution{
		AgentID: util.UUIDToString(turn.AgentID), TurnID: util.UUIDToString(turn.ID),
		Body: truncateRunes(body, maxMemoryContributionRunes), At: now.UTC().Format(time.RFC3339Nano),
	}
	memory.Summary = truncateRunes(body, maxMemorySummaryRunes)
	memory.RecentContributions = append(memory.RecentContributions, contribution)
	if len(memory.RecentContributions) > maxMemoryContributions {
		memory.RecentContributions = memory.RecentContributions[len(memory.RecentContributions)-maxMemoryContributions:]
	}
	encoded, err := json.Marshal(memory)
	if err != nil {
		return nil, fmt.Errorf("encode Room memory: %w", err)
	}
	return encoded, nil
}

func roomCycleStatus(turns []db.RoomTurn) (string, bool) {
	status := "completed"
	for _, turn := range turns {
		switch turn.Status {
		case "failed":
			status = "failed"
		case "cancelled":
			if status != "failed" {
				status = "cancelled"
			}
		case "completed":
		default:
			return "running", false
		}
	}
	return status, len(turns) > 0
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}
