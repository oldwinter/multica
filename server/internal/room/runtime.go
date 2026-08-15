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
	changed bool
	room    db.Room
	cycle   db.RoomCycle
	entry   *db.RoomEntry
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
		s.publish("room:entry", outcome.room, pgtype.UUID{}, map[string]any{
			"room_id": util.UUIDToString(outcome.room.ID),
			"entry":   *outcome.entry,
		})
	}
	s.publish("room:cycle", outcome.room, pgtype.UUID{}, map[string]any{
		"room_id": util.UUIDToString(outcome.room.ID),
		"cycle":   outcome.cycle,
	})
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
		} else if !errors.Is(cycleErr, pgx.ErrNoRows) {
			return taskSyncOutcome{}, fmt.Errorf("mark Room cycle running: %w", cycleErr)
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
	return taskSyncOutcome{changed: true, room: roomRow, cycle: cycle, entry: entry}, nil
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
