package skillevolution

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/scheduler"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	JobNameSkillEvolution      = "skill_evolution"
	SkillEvolutionJobCadence   = 5 * time.Minute
	scheduledEvolutionPageSize = 100
)

var ErrScheduledLoopOutcomeInvalid = errors.New("invalid scheduled skill evolution outcome")

// EvolutionScheduleRepository lists enabled, due observe and propose loops.
// Paused and disabled loops must not be returned by the production adapter.
type EvolutionScheduleRepository interface {
	ListScheduledLoops(context.Context, time.Time, pgtype.UUID, int) ([]db.SkillEvolutionLoop, error)
}

type ScheduledLoopAction string

const (
	ScheduledLoopObserved                        ScheduledLoopAction = "observed"
	ScheduledLoopImprovementRoomQueued           ScheduledLoopAction = "improvement_room_queued"
	ScheduledLoopAcceptedRecommendationRecovered ScheduledLoopAction = "accepted_recommendation_recovered"
	ScheduledLoopSkipped                         ScheduledLoopAction = "skipped"
)

// ScheduledLoopOutcome is deliberately content-free. The runner owns domain
// behavior: observe mode calls the lifecycle observation path; propose mode
// ensures or wakes the visible Improvement Room and can recover an already
// accepted recommendation. It must never publish or synchronously run a hidden
// improver from the scheduler process.
type ScheduledLoopOutcome struct {
	Action          ScheduledLoopAction
	EligibleSignals int
}

// ScheduledLoopRunner is the composition seam between the leaf scheduler and
// Room/lifecycle orchestration. IdempotencyKey is stable across lease retries.
type ScheduledLoopRunner interface {
	RunScheduledLoop(context.Context, db.SkillEvolutionLoop, string) (ScheduledLoopOutcome, error)
}

// SkillEvolutionJob returns the one scheduler job owned by this module. The
// shared scheduler supplies distributed leases, heartbeats, stale recovery,
// and retry timing; this job only handles pagination and workspace scopes.
func SkillEvolutionJob(repository EvolutionScheduleRepository, runner ScheduledLoopRunner, metrics *Metrics) scheduler.JobSpec {
	return scheduler.JobSpec{
		Name:              JobNameSkillEvolution,
		Cadence:           SkillEvolutionJobCadence,
		CatchUpMode:       scheduler.CatchUpLatestOnly,
		CatchUpWindow:     30 * time.Minute,
		RunTimeout:        10 * time.Minute,
		StaleTimeout:      15 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		AllowStaleReentry: true,
		MaxAttempts:       3,
		RetryBackoff:      []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute},
		Scopes:            skillEvolutionWorkspaceScopes(repository),
		Handler:           skillEvolutionScheduledHandler(repository, runner, metrics),
	}
}

func skillEvolutionWorkspaceScopes(repository EvolutionScheduleRepository) scheduler.ScopeProvider {
	return func(ctx context.Context, now time.Time) ([]scheduler.Scope, error) {
		loops, err := listScheduledEvolutionLoops(ctx, repository, now)
		if err != nil {
			return nil, err
		}
		workspaceIDs := make(map[string]struct{}, len(loops))
		for _, loop := range loops {
			if scheduledLoopRunnable(loop) {
				workspaceIDs[util.UUIDToString(loop.WorkspaceID)] = struct{}{}
			}
		}
		ids := make([]string, 0, len(workspaceIDs))
		for workspaceID := range workspaceIDs {
			ids = append(ids, workspaceID)
		}
		sort.Strings(ids)
		scopes := make([]scheduler.Scope, 0, len(ids))
		for _, workspaceID := range ids {
			scopes = append(scopes, scheduler.Scope{Kind: scheduler.ScopeKindWorkspace, ID: workspaceID})
		}
		return scopes, nil
	}
}

func skillEvolutionScheduledHandler(
	repository EvolutionScheduleRepository,
	runner ScheduledLoopRunner,
	metrics *Metrics,
) scheduler.Handler {
	return func(ctx context.Context, input scheduler.HandlerInput) (scheduler.HandlerResult, error) {
		if repository == nil || runner == nil {
			return scheduler.HandlerResult{}, ErrLifecycleInvalidInput
		}
		if input.Scope.Kind != scheduler.ScopeKindWorkspace {
			return scheduler.HandlerResult{}, fmt.Errorf("skill evolution scope kind %q: expected %q", input.Scope.Kind, scheduler.ScopeKindWorkspace)
		}
		workspaceID, err := util.ParseUUID(input.Scope.ID)
		if err != nil {
			return scheduler.HandlerResult{}, fmt.Errorf("parse skill evolution workspace %q: %w", input.Scope.ID, err)
		}

		// Re-read after winning the scheduler lease. A workspace or loop can be
		// deleted, paused, or cooled down after scope discovery but before claim.
		loops, err := listScheduledEvolutionLoops(ctx, repository, time.Now().UTC())
		if err != nil {
			if scheduledEvolutionNoop(err) {
				return skippedEvolutionJobResult(), nil
			}
			return scheduler.HandlerResult{}, fmt.Errorf("refresh skill evolution loops: %w", err)
		}

		result := scheduledEvolutionResult{}
		for _, loop := range loops {
			if loop.WorkspaceID != workspaceID || !scheduledLoopRunnable(loop) {
				continue
			}
			result.LoopsSeen++
			if input.Heartbeat != nil {
				if err := input.Heartbeat(ctx); err != nil {
					return result.handlerResult(), err
				}
			}

			outcome, runErr := runner.RunScheduledLoop(ctx, loop, scheduledEvolutionKey(loop.ID, input.PlanTime))
			if runErr != nil {
				if scheduledEvolutionNoop(runErr) {
					result.Skipped++
					continue
				}
				return result.handlerResult(), fmt.Errorf("run scheduled skill evolution loop: %w", runErr)
			}
			if err := validateScheduledLoopOutcome(loop, outcome); err != nil {
				return result.handlerResult(), err
			}
			metrics.RecordEligibleSignals(outcome.EligibleSignals)
			switch outcome.Action {
			case ScheduledLoopObserved:
				result.Observed++
			case ScheduledLoopImprovementRoomQueued:
				result.ImprovementRoomsQueued++
			case ScheduledLoopAcceptedRecommendationRecovered:
				result.RecommendationsRecovered++
			case ScheduledLoopSkipped:
				result.Skipped++
			}
		}
		if result.LoopsSeen == 0 {
			return skippedEvolutionJobResult(), nil
		}
		return result.handlerResult(), nil
	}
}

type scheduledEvolutionResult struct {
	LoopsSeen                int
	Observed                 int
	ImprovementRoomsQueued   int
	RecommendationsRecovered int
	Skipped                  int
}

func (r scheduledEvolutionResult) handlerResult() scheduler.HandlerResult {
	return scheduler.HandlerResult{
		RowsAffected: int64(r.Observed + r.ImprovementRoomsQueued + r.RecommendationsRecovered),
		Result: map[string]any{
			"loops_seen":                r.LoopsSeen,
			"observed":                  r.Observed,
			"improvement_rooms_queued":  r.ImprovementRoomsQueued,
			"recommendations_recovered": r.RecommendationsRecovered,
			"skipped":                   r.Skipped,
		},
	}
}

func skippedEvolutionJobResult() scheduler.HandlerResult {
	return scheduler.HandlerResult{Result: map[string]any{
		"loops_seen":                0,
		"observed":                  0,
		"improvement_rooms_queued":  0,
		"recommendations_recovered": 0,
		"skipped":                   1,
	}}
}

func validateScheduledLoopOutcome(loop db.SkillEvolutionLoop, outcome ScheduledLoopOutcome) error {
	if outcome.EligibleSignals < 0 || outcome.EligibleSignals > int(loop.MaxEvidenceRefs) {
		return ErrScheduledLoopOutcomeInvalid
	}
	switch outcome.Action {
	case ScheduledLoopObserved:
		if LoopMode(loop.Mode) != LoopModeObserve {
			return ErrScheduledLoopOutcomeInvalid
		}
	case ScheduledLoopImprovementRoomQueued, ScheduledLoopAcceptedRecommendationRecovered:
		if LoopMode(loop.Mode) != LoopModePropose || outcome.EligibleSignals < int(loop.MinimumSignals) {
			return ErrScheduledLoopOutcomeInvalid
		}
	case ScheduledLoopSkipped:
	default:
		return ErrScheduledLoopOutcomeInvalid
	}
	return nil
}

func scheduledEvolutionNoop(err error) bool {
	return errors.Is(err, ErrEvolutionDisabled) ||
		errors.Is(err, ErrEvolutionPaused) ||
		errors.Is(err, ErrEvolutionObserveOnly) ||
		errors.Is(err, ErrEvolutionCooldown) ||
		errors.Is(err, ErrInsufficientSignals) ||
		errors.Is(err, ErrGenerationActive) ||
		errors.Is(err, ErrPersistenceNotFound) ||
		errors.Is(err, ErrWorkspaceSkillNotFound)
}

func scheduledEvolutionKey(loopID pgtype.UUID, planTime time.Time) string {
	return fmt.Sprintf("scheduled:%s:%s", planTime.UTC().Format("20060102T150405Z"), util.UUIDToString(loopID))
}

func listScheduledEvolutionLoops(
	ctx context.Context,
	repository EvolutionScheduleRepository,
	eligibleAt time.Time,
) ([]db.SkillEvolutionLoop, error) {
	if repository == nil || eligibleAt.IsZero() {
		return nil, ErrLifecycleInvalidInput
	}
	var (
		all     []db.SkillEvolutionLoop
		afterID pgtype.UUID
	)
	for {
		page, err := repository.ListScheduledLoops(ctx, eligibleAt, afterID, scheduledEvolutionPageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return all, nil
		}
		for _, loop := range page {
			if !validUUID(loop.ID) || !validUUID(loop.WorkspaceID) || !validUUID(loop.SkillID) {
				return nil, ErrLifecycleInvalidInput
			}
			mode := LoopMode(loop.Mode)
			if !mode.Valid() {
				return nil, ErrLifecycleInvalidInput
			}
		}
		all = append(all, page...)
		lastID := page[len(page)-1].ID
		if lastID == afterID {
			return nil, errors.New("skill evolution scheduler pagination did not advance")
		}
		afterID = lastID
		if len(page) < scheduledEvolutionPageSize {
			return all, nil
		}
	}
}

func scheduledLoopRunnable(loop db.SkillEvolutionLoop) bool {
	return loop.IsEnabled && (LoopMode(loop.Mode) == LoopModeObserve || LoopMode(loop.Mode) == LoopModePropose)
}
