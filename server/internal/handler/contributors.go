package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// TaskDispatchSkill is the content-free identity of a skill bundle delivered
// to a daemon for one task execution.
type TaskDispatchSkill struct {
	Source  string
	SkillID string
}

// TaskDispatchEvent describes an execution only after the claim transaction
// has committed. It is offered only for claims carrying lightweight skill
// refs; legacy full-bundle claims have no durable dispatch snapshot.
type TaskDispatchEvent struct {
	WorkspaceID  string
	TaskID       string
	RuntimeID    string
	AgentID      string
	DispatchedAt time.Time
	Skills       []TaskDispatchSkill
}

// TaskCompletionEvent describes an execution only after its terminal
// completion transaction has committed.
type TaskCompletionEvent struct {
	WorkspaceID            string
	TaskID                 string
	RuntimeID              string
	AgentID                string
	DispatchedAt           time.Time
	CapabilityProven       bool
	SkillExecutionManifest json.RawMessage
}

// TaskDispatchContributor accepts durable dispatch observations without
// blocking task delivery. False means its bounded queue was full.
type TaskDispatchContributor interface {
	OfferTaskDispatch(TaskDispatchEvent) bool
}

// TaskCompletionContributor accepts durable completion observations without
// blocking terminal callback success. False means its bounded queue was full.
type TaskCompletionContributor interface {
	OfferTaskCompletion(TaskCompletionEvent) bool
}

// WorkspaceCleanupContributor removes module-owned workspace data inside the
// workspace deletion transaction. Returning an error refuses the deletion so
// a workspace cannot be committed with contributor-owned rows left behind.
type WorkspaceCleanupContributor interface {
	DeleteWorkspace(context.Context, *db.Queries, pgtype.UUID) error
}

type contributorRegistry struct {
	mu               sync.RWMutex
	taskDispatch     []TaskDispatchContributor
	taskCompletion   []TaskCompletionContributor
	workspaceCleanup []WorkspaceCleanupContributor
}

// RegisterTaskDispatchContributor adds a neutral task dispatch observer.
func (h *Handler) RegisterTaskDispatchContributor(contributor TaskDispatchContributor) {
	if h == nil || contributor == nil {
		return
	}
	registry := h.ensureContributorRegistry()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.taskDispatch = append(registry.taskDispatch, contributor)
}

// RegisterTaskCompletionContributor adds a neutral task completion observer.
func (h *Handler) RegisterTaskCompletionContributor(contributor TaskCompletionContributor) {
	if h == nil || contributor == nil {
		return
	}
	registry := h.ensureContributorRegistry()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.taskCompletion = append(registry.taskCompletion, contributor)
}

// RegisterWorkspaceCleanupContributor adds a transactional workspace cleanup
// participant. Contributors run in registration order.
func (h *Handler) RegisterWorkspaceCleanupContributor(contributor WorkspaceCleanupContributor) {
	if h == nil || contributor == nil {
		return
	}
	registry := h.ensureContributorRegistry()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.workspaceCleanup = append(registry.workspaceCleanup, contributor)
}

func (h *Handler) taskDispatchContributorSnapshot() []TaskDispatchContributor {
	if h == nil || h.contributors == nil {
		return nil
	}
	registry := h.contributors
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return append([]TaskDispatchContributor(nil), registry.taskDispatch...)
}

func (h *Handler) taskCompletionContributorSnapshot() []TaskCompletionContributor {
	if h == nil || h.contributors == nil {
		return nil
	}
	registry := h.contributors
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return append([]TaskCompletionContributor(nil), registry.taskCompletion...)
}

func (h *Handler) workspaceCleanupContributorSnapshot() []WorkspaceCleanupContributor {
	if h == nil || h.contributors == nil {
		return nil
	}
	registry := h.contributors
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return append([]WorkspaceCleanupContributor(nil), registry.workspaceCleanup...)
}

func (h *Handler) ensureContributorRegistry() *contributorRegistry {
	if h.contributors == nil {
		// Production handlers initialize this in New. The lazy path keeps zero
		// Handler values ergonomic for focused tests and narrow adapters.
		h.contributors = &contributorRegistry{}
	}
	return h.contributors
}

func (h *Handler) offerTaskDispatch(event TaskDispatchEvent) {
	for _, contributor := range h.taskDispatchContributorSnapshot() {
		callOfferTaskDispatch(contributor, cloneTaskDispatchEvent(event))
	}
}

func (h *Handler) offerTaskCompletion(event TaskCompletionEvent) {
	for _, contributor := range h.taskCompletionContributorSnapshot() {
		callOfferTaskCompletion(contributor, cloneTaskCompletionEvent(event))
	}
}

func (h *Handler) deleteWorkspaceContributorData(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) error {
	for i, contributor := range h.workspaceCleanupContributorSnapshot() {
		if err := callWorkspaceCleanupContributor(ctx, contributor, queries, workspaceID); err != nil {
			return fmt.Errorf("workspace cleanup contributor %d: %w", i+1, err)
		}
	}
	return nil
}

func callOfferTaskDispatch(contributor TaskDispatchContributor, event TaskDispatchEvent) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("task dispatch contributor panicked", "panic", recovered, "task_id", event.TaskID)
		}
	}()
	if !contributor.OfferTaskDispatch(event) {
		slog.Warn("task dispatch contributor queue full", "task_id", event.TaskID)
	}
}

func callOfferTaskCompletion(contributor TaskCompletionContributor, event TaskCompletionEvent) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("task completion contributor panicked", "panic", recovered, "task_id", event.TaskID)
		}
	}()
	if !contributor.OfferTaskCompletion(event) {
		slog.Warn("task completion contributor queue full", "task_id", event.TaskID)
	}
}

func callWorkspaceCleanupContributor(ctx context.Context, contributor WorkspaceCleanupContributor, queries *db.Queries, workspaceID pgtype.UUID) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return contributor.DeleteWorkspace(ctx, queries, workspaceID)
}

func cloneTaskDispatchEvent(event TaskDispatchEvent) TaskDispatchEvent {
	event.Skills = append([]TaskDispatchSkill(nil), event.Skills...)
	return event
}

func cloneTaskCompletionEvent(event TaskCompletionEvent) TaskCompletionEvent {
	event.SkillExecutionManifest = append(json.RawMessage(nil), event.SkillExecutionManifest...)
	return event
}

func taskDispatchEventFromResponse(taskID, runtimeID string, dispatchedAt pgtype.Timestamptz, resp AgentTaskResponse) (TaskDispatchEvent, bool) {
	event := TaskDispatchEvent{
		WorkspaceID: resp.WorkspaceID,
		TaskID:      taskID,
		RuntimeID:   runtimeID,
	}
	if dispatchedAt.Valid {
		event.DispatchedAt = dispatchedAt.Time
	}
	if resp.Agent == nil || len(resp.Agent.SkillRefs) == 0 {
		return event, false
	}

	event.AgentID = resp.Agent.ID
	event.Skills = make([]TaskDispatchSkill, 0, len(resp.Agent.SkillRefs))
	for _, skill := range resp.Agent.SkillRefs {
		event.Skills = append(event.Skills, TaskDispatchSkill{
			Source:  skill.Source,
			SkillID: skill.ID,
		})
	}
	return event, true
}
