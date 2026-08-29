package skillevolution

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/scheduler"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestSkillEvolutionJobUsesOneStableWorkspaceScopedSchedulerSpec(t *testing.T) {
	workspaceA := scheduledUUID("10000000-0000-4000-8000-000000000001")
	workspaceB := scheduledUUID("10000000-0000-4000-8000-000000000002")
	repository := &scheduledLoopRepository{loops: []db.SkillEvolutionLoop{
		scheduledLoop(workspaceB, scheduledUUID("20000000-0000-4000-8000-000000000001"), scheduledUUID("30000000-0000-4000-8000-000000000001"), LoopModePropose, true),
		scheduledLoop(workspaceA, scheduledUUID("20000000-0000-4000-8000-000000000002"), scheduledUUID("30000000-0000-4000-8000-000000000002"), LoopModeObserve, true),
		scheduledLoop(workspaceA, scheduledUUID("20000000-0000-4000-8000-000000000003"), scheduledUUID("30000000-0000-4000-8000-000000000003"), LoopModePaused, true),
		scheduledLoop(workspaceB, scheduledUUID("20000000-0000-4000-8000-000000000004"), scheduledUUID("30000000-0000-4000-8000-000000000004"), LoopModePropose, false),
	}}
	job := SkillEvolutionJob(repository, &scheduledLoopRunnerFake{}, nil)

	if job.Name != JobNameSkillEvolution || job.Cadence != SkillEvolutionJobCadence ||
		!job.AllowStaleReentry || job.MaxAttempts != 3 || job.HeartbeatInterval >= job.StaleTimeout {
		t.Fatalf("unexpected scheduler spec: %+v", job)
	}
	scopes, err := job.Scopes(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	want := []scheduler.Scope{
		{Kind: scheduler.ScopeKindWorkspace, ID: uuid.UUID(workspaceA.Bytes).String()},
		{Kind: scheduler.ScopeKindWorkspace, ID: uuid.UUID(workspaceB.Bytes).String()},
	}
	if !reflect.DeepEqual(scopes, want) {
		t.Fatalf("scopes = %#v, want %#v", scopes, want)
	}

	manager := scheduler.NewManager(nil, scheduler.Options{})
	if err := manager.Register(job); err != nil {
		t.Fatalf("register job: %v", err)
	}
	if err := manager.Register(job); err == nil {
		t.Fatal("duplicate job registration succeeded")
	}
}

func TestSkillEvolutionScheduledHandlerDelegatesObserveAndProposeWithoutGenerating(t *testing.T) {
	workspaceID := scheduledUUID("10000000-0000-4000-8000-000000000011")
	observe := scheduledLoop(workspaceID,
		scheduledUUID("20000000-0000-4000-8000-000000000011"),
		scheduledUUID("30000000-0000-4000-8000-000000000011"), LoopModeObserve, true,
	)
	propose := scheduledLoop(workspaceID,
		scheduledUUID("20000000-0000-4000-8000-000000000012"),
		scheduledUUID("30000000-0000-4000-8000-000000000012"), LoopModePropose, true,
	)
	repository := &scheduledLoopRepository{loops: []db.SkillEvolutionLoop{observe, propose}}
	var calls []scheduledLoopCall
	runner := &scheduledLoopRunnerFake{run: func(_ context.Context, loop db.SkillEvolutionLoop, key string) (ScheduledLoopOutcome, error) {
		calls = append(calls, scheduledLoopCall{loop: loop, key: key})
		if LoopMode(loop.Mode) == LoopModeObserve {
			return ScheduledLoopOutcome{Action: ScheduledLoopObserved, EligibleSignals: 3}, nil
		}
		return ScheduledLoopOutcome{Action: ScheduledLoopImprovementRoomQueued, EligibleSignals: 3}, nil
	}}
	job := SkillEvolutionJob(repository, runner, NewMetrics())
	planTime := time.Date(2026, 8, 28, 12, 5, 0, 0, time.UTC)
	heartbeats := 0
	result, err := job.Handler(context.Background(), scheduler.HandlerInput{
		Scope:    scheduler.Scope{Kind: scheduler.ScopeKindWorkspace, ID: uuid.UUID(workspaceID.Bytes).String()},
		PlanTime: planTime,
		Heartbeat: func(context.Context) error {
			heartbeats++
			return nil
		},
	})
	if err != nil || result.RowsAffected != 2 {
		t.Fatalf("handler = (%+v, %v)", result, err)
	}
	if len(calls) != 2 || heartbeats != 2 {
		t.Fatalf("calls = %+v, heartbeats = %d", calls, heartbeats)
	}
	for _, call := range calls {
		if call.key != scheduledEvolutionKey(call.loop.ID, planTime) {
			t.Fatalf("runner key = %q", call.key)
		}
	}
	if result.Result["observed"] != 1 || result.Result["improvement_rooms_queued"] != 1 {
		t.Fatalf("result = %#v", result.Result)
	}
}

func TestSkillEvolutionScheduledHandlerReusesKeyAndRecoversAcceptedRecommendation(t *testing.T) {
	workspaceID := scheduledUUID("10000000-0000-4000-8000-000000000021")
	loop := scheduledLoop(workspaceID,
		scheduledUUID("20000000-0000-4000-8000-000000000021"),
		scheduledUUID("30000000-0000-4000-8000-000000000021"), LoopModePropose, true,
	)
	var keys []string
	runner := &scheduledLoopRunnerFake{run: func(_ context.Context, _ db.SkillEvolutionLoop, key string) (ScheduledLoopOutcome, error) {
		keys = append(keys, key)
		if len(keys) == 1 {
			return ScheduledLoopOutcome{Action: ScheduledLoopImprovementRoomQueued, EligibleSignals: 1}, nil
		}
		return ScheduledLoopOutcome{Action: ScheduledLoopAcceptedRecommendationRecovered, EligibleSignals: 1}, nil
	}}
	job := SkillEvolutionJob(&scheduledLoopRepository{loops: []db.SkillEvolutionLoop{loop}}, runner, nil)
	input := scheduler.HandlerInput{
		Scope:    scheduler.Scope{Kind: scheduler.ScopeKindWorkspace, ID: uuid.UUID(workspaceID.Bytes).String()},
		PlanTime: time.Date(2026, 8, 28, 12, 10, 0, 0, time.UTC),
	}
	first, err := job.Handler(context.Background(), input)
	if err != nil || first.Result["improvement_rooms_queued"] != 1 {
		t.Fatalf("first handler = (%+v, %v)", first, err)
	}
	second, err := job.Handler(context.Background(), input)
	if err != nil || second.Result["recommendations_recovered"] != 1 {
		t.Fatalf("retry handler = (%+v, %v)", second, err)
	}
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("retry keys = %#v", keys)
	}
}

func TestSkillEvolutionScheduledHandlerSkipsPausedDisabledAndDeletedLoops(t *testing.T) {
	workspaceID := scheduledUUID("10000000-0000-4000-8000-000000000031")
	paused := scheduledLoop(workspaceID,
		scheduledUUID("20000000-0000-4000-8000-000000000031"),
		scheduledUUID("30000000-0000-4000-8000-000000000031"), LoopModePaused, true,
	)
	disabled := scheduledLoop(workspaceID,
		scheduledUUID("20000000-0000-4000-8000-000000000032"),
		scheduledUUID("30000000-0000-4000-8000-000000000032"), LoopModePropose, false,
	)
	calls := 0
	repository := &scheduledLoopRepository{loops: []db.SkillEvolutionLoop{paused, disabled}}
	job := SkillEvolutionJob(repository, &scheduledLoopRunnerFake{run: func(context.Context, db.SkillEvolutionLoop, string) (ScheduledLoopOutcome, error) {
		calls++
		return ScheduledLoopOutcome{}, nil
	}}, nil)
	scopes, err := job.Scopes(context.Background(), time.Now().UTC())
	if err != nil || len(scopes) != 0 {
		t.Fatalf("paused/disabled scopes = (%+v, %v)", scopes, err)
	}

	// An already discovered scope can race with workspace deletion. The
	// post-lease read is authoritative and returns a successful no-op.
	result, err := job.Handler(context.Background(), scheduler.HandlerInput{
		Scope: scheduler.Scope{Kind: scheduler.ScopeKindWorkspace, ID: uuid.UUID(workspaceID.Bytes).String()},
	})
	if err != nil || result.Result["loops_seen"] != 0 || result.Result["skipped"] != 1 || calls != 0 {
		t.Fatalf("handler = (%+v, %v), calls = %d", result, err, calls)
	}
}

func TestSkillEvolutionScheduledHandlerValidatesRunnerOutcomeAndRetriesInfrastructureFailure(t *testing.T) {
	workspaceID := scheduledUUID("10000000-0000-4000-8000-000000000041")
	loop := scheduledLoop(workspaceID,
		scheduledUUID("20000000-0000-4000-8000-000000000041"),
		scheduledUUID("30000000-0000-4000-8000-000000000041"), LoopModePropose, true,
	)
	repository := &scheduledLoopRepository{loops: []db.SkillEvolutionLoop{loop}}
	input := scheduler.HandlerInput{Scope: scheduler.Scope{Kind: scheduler.ScopeKindWorkspace, ID: uuid.UUID(workspaceID.Bytes).String()}}

	invalid := SkillEvolutionJob(repository, &scheduledLoopRunnerFake{run: func(context.Context, db.SkillEvolutionLoop, string) (ScheduledLoopOutcome, error) {
		return ScheduledLoopOutcome{Action: ScheduledLoopObserved, EligibleSignals: 1}, nil
	}}, nil)
	if _, err := invalid.Handler(context.Background(), input); !errors.Is(err, ErrScheduledLoopOutcomeInvalid) {
		t.Fatalf("invalid outcome error = %v", err)
	}
	loop.MinimumSignals = 2
	repository.loops = []db.SkillEvolutionLoop{loop}
	belowMinimum := SkillEvolutionJob(repository, &scheduledLoopRunnerFake{run: func(context.Context, db.SkillEvolutionLoop, string) (ScheduledLoopOutcome, error) {
		return ScheduledLoopOutcome{Action: ScheduledLoopImprovementRoomQueued, EligibleSignals: 1}, nil
	}}, nil)
	if _, err := belowMinimum.Handler(context.Background(), input); !errors.Is(err, ErrScheduledLoopOutcomeInvalid) {
		t.Fatalf("below-minimum outcome error = %v", err)
	}

	wantErr := errors.New("temporary Room transport failure")
	retry := SkillEvolutionJob(repository, &scheduledLoopRunnerFake{run: func(context.Context, db.SkillEvolutionLoop, string) (ScheduledLoopOutcome, error) {
		return ScheduledLoopOutcome{}, wantErr
	}}, nil)
	if _, err := retry.Handler(context.Background(), input); !errors.Is(err, wantErr) {
		t.Fatalf("handler error = %v, want %v", err, wantErr)
	}
}

type scheduledLoopRepository struct {
	loops []db.SkillEvolutionLoop
	err   error
}

func (r *scheduledLoopRepository) ListScheduledLoops(_ context.Context, _ time.Time, afterID pgtype.UUID, _ int) ([]db.SkillEvolutionLoop, error) {
	if r.err != nil {
		return nil, r.err
	}
	if afterID.Valid {
		return nil, nil
	}
	return append([]db.SkillEvolutionLoop(nil), r.loops...), nil
}

type scheduledLoopCall struct {
	loop db.SkillEvolutionLoop
	key  string
}

type scheduledLoopRunnerFake struct {
	run func(context.Context, db.SkillEvolutionLoop, string) (ScheduledLoopOutcome, error)
}

func (r *scheduledLoopRunnerFake) RunScheduledLoop(ctx context.Context, loop db.SkillEvolutionLoop, key string) (ScheduledLoopOutcome, error) {
	if r.run != nil {
		return r.run(ctx, loop, key)
	}
	return ScheduledLoopOutcome{Action: ScheduledLoopSkipped}, nil
}

func scheduledLoop(workspaceID, skillID, loopID pgtype.UUID, mode LoopMode, enabled bool) db.SkillEvolutionLoop {
	return db.SkillEvolutionLoop{
		ID: loopID, WorkspaceID: workspaceID, SkillID: skillID,
		IsEnabled: enabled, Mode: string(mode), MinimumSignals: 1, MaxEvidenceRefs: 8,
	}
}

func scheduledUUID(value string) pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.MustParse(value), Valid: true}
}
