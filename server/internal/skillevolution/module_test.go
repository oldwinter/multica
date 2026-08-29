package skillevolution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/room"
	internaltestutil "github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/llm"
)

type attributionSinkStub struct {
	accepted bool
}

type productionSkillLoaderStub struct {
	snapshot WorkspaceSkillSnapshot
}

func (stub productionSkillLoaderStub) Load(context.Context, pgtype.UUID, pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	return stub.snapshot, nil
}

type productionRoomsStub struct {
	room.Rooms
	t        *testing.T
	fixture  *internaltestutil.Fixture
	roomID   pgtype.UUID
	creates  int
	messages int
	bodies   []string
}

func (stub *productionRoomsStub) Create(_ context.Context, input room.CreateInput) (room.Detail, error) {
	stub.creates++
	stub.fixture.Insert(stub.t, "room", internaltestutil.Cols{
		"id": stub.roomID, "workspace_id": input.WorkspaceID, "template_id": input.TemplateID,
		"objective": input.Objective, "status": "active", "created_at": internaltestutil.Raw("now()"),
	})
	return room.Detail{Room: db.Room{ID: stub.roomID, WorkspaceID: input.WorkspaceID, TemplateID: pgtype.Text{String: input.TemplateID, Valid: true}}}, nil
}

func (stub *productionRoomsStub) PostMessage(_ context.Context, input room.MessageInput) (room.MessageResult, error) {
	stub.messages++
	stub.bodies = append(stub.bodies, input.Body)
	entryID, cycleID := publisherUUID(), publisherUUID()
	stub.fixture.Insert(stub.t, "room_entry", internaltestutil.Cols{
		"id": entryID, "workspace_id": input.WorkspaceID, "room_id": input.RoomID,
		"ordinal": 1, "entry_type": "message", "author_type": "member",
		"author_id": input.ActorUserID, "body": input.Body,
		"mentions": internaltestutil.Raw("'[]'::jsonb"), "created_at": internaltestutil.Raw("now()"),
	})
	stub.fixture.Insert(stub.t, "room_cycle", internaltestutil.Cols{
		"id": cycleID, "workspace_id": input.WorkspaceID, "room_id": input.RoomID,
		"sequence": 1, "source": "message", "wake_key": "message:" + input.IdempotencyKey,
		"triggering_entry_id": entryID, "status": "queued", "phase": "discussion",
		"expected_max_turns": 1, "created_at": internaltestutil.Raw("now()"),
	})
	return room.MessageResult{}, nil
}

func (stub attributionSinkStub) OfferTaskDispatch(handler.TaskDispatchEvent) bool {
	return stub.accepted
}

func (stub attributionSinkStub) OfferTaskCompletion(handler.TaskCompletionEvent) bool {
	return stub.accepted
}

type replayJSONClientStub struct {
	enabled   bool
	responses []llm.JSONGeneration
	prompts   []string
	generate  func(context.Context) (llm.JSONGeneration, error)
}

func (stub *replayJSONClientStub) Enabled() bool { return stub != nil && stub.enabled }

func (stub *replayJSONClientStub) GenerateJSONWithUsage(
	ctx context.Context, _, _, prompt string, _ float64, _ int64,
) (llm.JSONGeneration, error) {
	stub.prompts = append(stub.prompts, prompt)
	if stub.generate != nil {
		return stub.generate(ctx)
	}
	if len(stub.responses) == 0 {
		return llm.JSONGeneration{}, ErrBehavioralReplayUnavailable
	}
	response := stub.responses[0]
	stub.responses = stub.responses[1:]
	return response, nil
}

func TestProductionBehavioralEvaluatorFailsClosedAtEveryBound(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	candidate := fixture.base
	candidate.Content = strings.Replace(candidate.Content, "Original.", "Updated with the focused check.", 1)
	evidence := []ResolvedEvidence{{
		Ref:     fixture.signalRef,
		Payload: []byte(`{"outcome":"needs_correction","correction":"add the focused check","reason":"bounded"}`),
	}}
	generation := func(content string, tokens int64) llm.JSONGeneration {
		return llm.JSONGeneration{Content: content, PromptTokens: tokens, CompletionTokens: tokens}
	}
	tests := []struct {
		name       string
		client     *replayJSONClientStub
		evidence   []ResolvedEvidence
		limits     ReplayLimits
		wantResult EvaluationResult
		wantReason string
		wantError  bool
	}{
		{
			name: "zero samples", client: &replayJSONClientStub{enabled: true}, evidence: nil,
			limits:    ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 2_000_000, PolicyVersion: "v1"},
			wantError: true,
		},
		{
			name: "cost limit", client: &replayJSONClientStub{enabled: true, responses: []llm.JSONGeneration{
				generation(`{"response":"base"}`, 1), generation(`{"response":"candidate"}`, 1),
				generation(`{"winner":"candidate","base_pass":false,"candidate_pass":true}`, 1),
				generation(`{"winner":"candidate","base_pass":false,"candidate_pass":true}`, 1),
			}}, evidence: evidence,
			limits:     ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 1, PolicyVersion: "v1"},
			wantResult: EvaluationResultFailed, wantReason: "configured_limit_exceeded",
		},
		{
			name: "missing usage", client: &replayJSONClientStub{enabled: true, responses: []llm.JSONGeneration{
				{Content: `{"response":"base"}`},
			}}, evidence: evidence,
			limits:     ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 2_000_000, PolicyVersion: "v1"},
			wantResult: EvaluationResultUnknown, wantReason: "adapter_error",
		},
		{
			name: "nondeterminism", client: &replayJSONClientStub{enabled: true, responses: []llm.JSONGeneration{
				generation(`{"response":"base"}`, 1), generation(`{"response":"candidate"}`, 1),
				generation(`{"winner":"candidate","base_pass":false,"candidate_pass":true}`, 1),
				generation(`{"winner":"base","base_pass":true,"candidate_pass":false}`, 1),
			}}, evidence: evidence,
			limits:     ReplayLimits{Timeout: time.Second, MaxSamples: 1, MaxCostUSDTicks: 2_000_000, PolicyVersion: "v1"},
			wantResult: EvaluationResultInconclusive, wantReason: "nondeterministic",
		},
		{
			name: "timeout", client: &replayJSONClientStub{enabled: true, generate: func(ctx context.Context) (llm.JSONGeneration, error) {
				<-ctx.Done()
				return llm.JSONGeneration{}, ctx.Err()
			}}, evidence: evidence,
			limits:     ReplayLimits{Timeout: time.Millisecond, MaxSamples: 1, MaxCostUSDTicks: 2_000_000, PolicyVersion: "v1"},
			wantResult: EvaluationResultUnknown, wantReason: "timeout",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := newProductionBehavioralEvaluator(test.client).Evaluate(context.Background(), ReplayRequest{
				Base: fixture.base, Candidate: candidate, Evidence: test.evidence, Limits: test.limits,
			})
			if test.wantError {
				if err == nil || outcome.Result == EvaluationResultPassed {
					t.Fatalf("bounded production replay = (%+v, %v), want fail-closed error", outcome, err)
				}
				return
			}
			if err != nil || outcome.Result != test.wantResult || outcome.ReasonCode != test.wantReason {
				t.Fatalf("bounded production replay = (%+v, %v), want %s/%s", outcome, err, test.wantResult, test.wantReason)
			}
			if test.name == "cost limit" && len(test.client.prompts) != 1 {
				t.Fatalf("cost-limited replay calls = %d, want immediate stop after 1", len(test.client.prompts))
			}
		})
	}
}

func TestProductionBehavioralEvaluatorRunsPairedCasesAndKeepsResultContentFree(t *testing.T) {
	fixture := newRoomCandidateFixture(t)
	secretMarker := "review-payload-marker-must-not-persist"
	client := &replayJSONClientStub{enabled: true, responses: []llm.JSONGeneration{
		{Content: `{"response":"base missed the check"}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"response":"candidate performed the check"}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"winner":"candidate","base_pass":false,"candidate_pass":true}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"winner":"candidate","base_pass":false,"candidate_pass":true}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"response":"base missed the check again"}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"response":"candidate performed the check again"}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"winner":"candidate","base_pass":false,"candidate_pass":true}`, PromptTokens: 1, CompletionTokens: 1},
		{Content: `{"winner":"candidate","base_pass":false,"candidate_pass":true}`, PromptTokens: 1, CompletionTokens: 1},
	}}
	evaluator, ok := newProductionBehavioralEvaluator(client).(*ProductionReplayEvaluator)
	if !ok {
		t.Fatalf("production evaluator type = %T", newProductionBehavioralEvaluator(client))
	}
	if _, ok := evaluator.engine.(*BoundedModelReplay); !ok || evaluator.adapter != productionReplayAdapter || evaluator.version != productionReplayVersion {
		t.Fatalf("production evaluator = %+v", evaluator)
	}
	candidate := fixture.base
	candidate.Content = strings.Replace(candidate.Content, "Original.", "Updated with the focused check.", 1)
	outcome, err := evaluator.Evaluate(context.Background(), ReplayRequest{
		Base: fixture.base, Candidate: candidate,
		Evidence: []ResolvedEvidence{
			{Ref: fixture.signalRef, Payload: []byte(`{"outcome":"needs_correction","correction":"` + secretMarker + `","reason":"bounded"}`)},
			{Ref: fixture.secondSignalRef, Payload: []byte(`{"outcome":"needs_correction","correction":"independent case","reason":"bounded"}`)},
		},
		Limits: ReplayLimits{Timeout: time.Second, MaxSamples: 2, MaxCostUSDTicks: 2_000_000, PolicyVersion: "v1"},
	})
	if err != nil || outcome.Result != EvaluationResultPassed || outcome.SampleCount != 2 || outcome.FailureCount != 0 {
		t.Fatalf("paired replay = (%+v, %v)", outcome, err)
	}
	raw, err := json.Marshal(struct {
		Result  ReplayOutcome   `json:"result"`
		Metrics json.RawMessage `json:"metrics"`
	}{outcome, outcome.SafeMetrics()})
	if err != nil || strings.Contains(string(raw), secretMarker) {
		t.Fatalf("content-free replay result leaked source payload: %s (err=%v)", raw, err)
	}
	if len(client.prompts) != 8 || !strings.Contains(client.prompts[0], secretMarker) {
		t.Fatalf("authorized case was not loaded transiently: calls=%d", len(client.prompts))
	}
}

func TestProductionMetricsAdaptersRecordRepresentativeOperations(t *testing.T) {
	metrics := NewMetrics()
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(metrics.Collectors()...)

	contributor := &attributionMetricsContributor{delegate: attributionSinkStub{accepted: true}, metrics: metrics}
	accepted := contributor.OfferTaskCompletion(handler.TaskCompletionEvent{
		CapabilityProven:       true,
		SkillExecutionManifest: []byte(`{"malformed-for-attribution":true}`),
	})
	if !accepted {
		t.Fatal("completion was not delegated")
	}
	if got := promtest.ToFloat64(metrics.ManifestEligibleRuns); got != 1 {
		t.Fatalf("manifest eligible = %v, want 1", got)
	}
	if got := promtest.ToFloat64(metrics.ManifestAttributedRuns); got != 0 {
		t.Fatalf("manifest attributed before persistence = %v, want 0", got)
	}
	if got := promtest.ToFloat64(metrics.FeedbackEligibleRuns); got != 0 {
		t.Fatalf("feedback eligible before persistence = %v, want 0", got)
	}

	store := &metricsAttributionStore{delegate: newFakeAttributionWorkerStore(), metrics: metrics}
	if _, err := store.recordAttributionBatch(context.Background(), []TaskAttributionInput{{}}); err != nil {
		t.Fatalf("record attributed batch: %v", err)
	}
	if got := promtest.ToFloat64(metrics.ManifestAttributedRuns); got != 1 {
		t.Fatalf("manifest attributed after persistence = %v, want 1", got)
	}
	if got := promtest.ToFloat64(metrics.FeedbackEligibleRuns); got != 1 {
		t.Fatalf("feedback eligible after persistence = %v, want 1", got)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather production metrics: %v", err)
	}
	want := map[string]float64{
		"multica_skill_evolution_feedback_covered_runs_total":    0,
		"multica_skill_evolution_feedback_eligible_runs_total":   1,
		"multica_skill_evolution_manifest_attributed_runs_total": 1,
		"multica_skill_evolution_manifest_eligible_runs_total":   1,
	}
	for _, family := range families {
		expected, ok := want[family.GetName()]
		if !ok || len(family.Metric) != 1 || family.Metric[0].Counter == nil {
			continue
		}
		if got := family.Metric[0].Counter.GetValue(); got != expected {
			t.Fatalf("gathered %s = %v, want %v", family.GetName(), got, expected)
		}
		delete(want, family.GetName())
	}
	if len(want) != 0 {
		t.Fatalf("missing gathered metrics: %v", want)
	}
}

func TestProductionTaskReviewRouteRegistration(t *testing.T) {
	router := chi.NewRouter()
	handlerFor := func(marker string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Route", marker)
			w.WriteHeader(http.StatusNoContent)
		}
	}
	registerTaskReviewRoutes(router, taskReviewRouteHandlers{
		create: handlerFor("create"), list: handlerFor("list"), get: handlerFor("get"),
		listManualReruns: handlerFor("manual-list"), getManualRerun: handlerFor("manual-get"),
	})
	for _, test := range []struct {
		method string
		path   string
		marker string
	}{
		{http.MethodPost, "/api/tasks/task-id/review", "create"},
		{http.MethodGet, "/api/task-run-reviews", "list"},
		{http.MethodGet, "/api/task-run-reviews/review-id", "get"},
		{http.MethodGet, "/api/task-run-reviews/manual-reruns", "manual-list"},
		{http.MethodGet, "/api/task-run-reviews/manual-reruns/task-id", "manual-get"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != http.StatusNoContent || recorder.Header().Get("X-Route") != test.marker {
			t.Fatalf("%s %s status=%d marker=%q", test.method, test.path, recorder.Code, recorder.Header().Get("X-Route"))
		}
	}
}

func TestProductionImprovementRoomQueuerCreatesVisibleRoomAndReplaysWake(t *testing.T) {
	pool := workspaceSkillPublisherTestPool(t)
	setupProductionRoomQueueSchema(t, pool)
	workspaceID, skillID, actorID, agentID := publisherUUID(), publisherUUID(), publisherUUID(), publisherUUID()
	seedPublisherSkill(t, pool, publisherSkillSeed{
		ID: skillID, WorkspaceID: workspaceID, CreatorID: actorID, Name: "deploy",
		Description: "Deploy safely", Content: "---\nname: deploy\ndescription: Deploy safely\n---\n\nVerify the release.", Config: `{}`,
	})
	fixture := internaltestutil.New(pool, uuidText(workspaceID), uuidText(actorID))
	fixture.Insert(t, "agent", internaltestutil.Cols{"id": agentID, "workspace_id": workspaceID})
	fixture.InsertNoID(t, "agent_skill", internaltestutil.Cols{
		"agent_id": agentID, "skill_id": skillID, "enabled": true,
	}, "agent_id = $1 AND skill_id = $2", agentID, skillID)
	fixture.Insert(t, "skill_evolution_loop", internaltestutil.Cols{
		"workspace_id": workspaceID, "skill_id": skillID, "is_enabled": true, "mode": "propose",
		"cooldown_seconds": 60, "minimum_signals": 1, "max_evidence_refs": 5,
		"max_replay_samples": 1, "max_cost_usd_ticks": 0, "policy_version": "v1",
	})

	snapshot := mustLoadPublisherSkill(t, NewWorkspaceSkillRepository(db.New(pool)), workspaceID, skillID)
	ref := lifecycleEvidenceRef(workspaceID, skillID, "production-room")
	source := NewSignalAdapter(EvidenceKindTaskReview,
		func(context.Context, SignalQuery) ([]EvidenceRef, error) { return []EvidenceRef{ref}, nil },
		func(_ context.Context, _ SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			return ResolvedEvidence{Ref: expected, Payload: []byte(`{"outcome":"needs_correction","correction":"Verify the release.","reason":"A check was skipped."}`)}, nil
		},
	)
	signals, err := newSignalSet([]SignalSource{source})
	if err != nil {
		t.Fatal(err)
	}
	rooms := &productionRoomsStub{
		t:       t,
		fixture: fixture,
		roomID:  publisherUUID(),
	}
	queuer := &productionImprovementRoomQueuer{
		pool: pool, queries: db.New(pool), rooms: rooms, store: NewStore(db.New(pool), pool),
		skills: productionSkillLoaderStub{snapshot: snapshot}, signals: signals,
	}

	first, err := queuer.EnsureImprovementRoom(context.Background(), workspaceID, skillID, actorID, "request-1")
	if err != nil {
		t.Fatalf("first EnsureImprovementRoom: %v", err)
	}
	second, err := queuer.EnsureImprovementRoom(context.Background(), workspaceID, skillID, actorID, "request-1")
	if err != nil {
		t.Fatalf("repeated EnsureImprovementRoom: %v", err)
	}
	if first != second || first.RoomID != rooms.roomID || first.EligibleSignals != 1 || rooms.creates != 1 || rooms.messages != 1 {
		t.Fatalf("queue results first=%+v second=%+v creates=%d messages=%d", first, second, rooms.creates, rooms.messages)
	}
	var templateID, objective, status string
	if err := pool.QueryRow(context.Background(), `SELECT template_id, objective, status FROM room WHERE id = $1`, rooms.roomID).
		Scan(&templateID, &objective, &status); err != nil {
		t.Fatalf("load visible Improvement Room: %v", err)
	}
	if templateID != improvementRoomTemplateID || objective != "Improve workspace Skill "+uuidText(skillID) || status != "active" {
		t.Fatalf("visible Improvement Room = template %q objective %q status %q", templateID, objective, status)
	}

	mustExecPublisher(t, pool, `UPDATE skill_evolution_loop SET mode = 'observe', next_eligible_at = NULL WHERE workspace_id = $1 AND skill_id = $2`, workspaceID, skillID)
	var observedActor pgtype.UUID
	observeSource := NewSignalAdapter(EvidenceKindTaskReview,
		func(_ context.Context, query SignalQuery) ([]EvidenceRef, error) {
			observedActor = query.ActorID
			return []EvidenceRef{ref}, nil
		},
		func(context.Context, SignalQuery, EvidenceRef) (ResolvedEvidence, error) {
			return ResolvedEvidence{}, nil
		},
	)
	skills := &memorySkillLoader{current: snapshot}
	lifecycle, err := NewLifecycle(
		NewStore(db.New(pool), pool), skills, &memoryPublisher{skills: skills},
		&DeterministicImprover{}, &DeterministicReplayEvaluator{}, observeSource,
	)
	if err != nil {
		t.Fatalf("compose observe lifecycle: %v", err)
	}
	loop, err := NewStore(db.New(pool), pool).GetLoop(context.Background(), workspaceID, skillID)
	if err != nil {
		t.Fatalf("load scheduled loop: %v", err)
	}
	outcome, err := (&Module{lifecycle: lifecycle, requester: &RoomProposalRequester{}}).
		RunScheduledLoop(context.Background(), loop, "scheduled:observe")
	if err != nil {
		t.Fatalf("run scheduled observe: %v", err)
	}
	if observedActor != actorID || outcome.Action != ScheduledLoopObserved || outcome.EligibleSignals != 1 {
		t.Fatalf("scheduled observe actor=%s outcome=%+v", uuidText(observedActor), outcome)
	}
}

func setupProductionRoomQueueSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	mustExecPublisher(t, pool, `
CREATE TABLE agent (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    archived_at TIMESTAMPTZ NULL
);
CREATE TABLE workspace (id UUID PRIMARY KEY);
CREATE TABLE member (workspace_id UUID NOT NULL, user_id UUID NOT NULL);
CREATE TABLE skill_evolution_loop (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
	    is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    mode TEXT NOT NULL DEFAULT 'observe',
    cooldown_seconds INTEGER NOT NULL DEFAULT 86400,
    minimum_signals INTEGER NOT NULL DEFAULT 3,
    max_evidence_refs INTEGER NOT NULL DEFAULT 20,
    max_replay_samples INTEGER NOT NULL DEFAULT 5,
    max_cost_usd_ticks BIGINT NOT NULL DEFAULT 0,
    policy_version TEXT NOT NULL DEFAULT 'v1',
    last_observed_at TIMESTAMPTZ NULL,
    last_proposal_at TIMESTAMPTZ NULL,
    next_eligible_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX skill_evolution_loop_workspace_skill_uidx
    ON skill_evolution_loop (workspace_id, skill_id);
CREATE TABLE skill_evolution_revision (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    kind TEXT NOT NULL,
    ownership_class TEXT NOT NULL,
    source TEXT NOT NULL,
    bundle_hash TEXT NOT NULL,
    metadata_digest TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    primary_content TEXT NOT NULL,
    byte_count BIGINT NOT NULL,
    support_file_count INTEGER NOT NULL,
    created_by_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX skill_evolution_revision_workspace_skill_hash_uidx
    ON skill_evolution_revision (workspace_id, skill_id, bundle_hash);
CREATE TABLE skill_evolution_revision_file (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    revision_id UUID NOT NULL,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    digest TEXT NOT NULL,
    byte_count INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX skill_evolution_revision_file_workspace_path_uidx
    ON skill_evolution_revision_file (workspace_id, revision_id, path);
CREATE TABLE skill_evolution_proposal (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    loop_id UUID NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued',
    base_revision_id UUID NOT NULL,
    candidate_revision_id UUID NULL,
    base_hash TEXT NOT NULL,
    candidate_hash TEXT NULL,
    rationale_digest TEXT NULL,
    failure_reason TEXT NULL,
    stale_reason TEXT NULL,
    generation_idempotency_key TEXT NOT NULL,
    requested_by_id UUID NULL,
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    observed_pattern TEXT NULL,
    expected_benefit TEXT NULL,
    regression_risk TEXT NULL
);
CREATE UNIQUE INDEX skill_evolution_proposal_workspace_generation_uidx
    ON skill_evolution_proposal (workspace_id, skill_id, generation_idempotency_key);
CREATE UNIQUE INDEX skill_evolution_proposal_active_uidx
    ON skill_evolution_proposal (workspace_id, skill_id)
    WHERE state IN ('queued', 'running', 'ready', 'publishing');
CREATE TABLE skill_evolution_release (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    proposal_id UUID NULL,
    source_release_id UUID NULL,
    revision_id UUID NOT NULL,
    kind TEXT NOT NULL,
    expected_base_hash TEXT NOT NULL,
    pre_hash TEXT NULL,
    post_hash TEXT NULL,
    outcome TEXT NOT NULL DEFAULT 'pending',
    actor_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL,
    error_code TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX skill_evolution_release_workspace_key_uidx
    ON skill_evolution_release (workspace_id, idempotency_key);
CREATE TABLE room (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    template_id TEXT NOT NULL,
    objective TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE room_entry (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    cycle_id UUID NULL,
    turn_id UUID NULL,
    ordinal BIGINT NOT NULL,
    entry_type TEXT NOT NULL,
    author_type TEXT NOT NULL,
    author_id UUID NULL,
    body TEXT NOT NULL,
    mentions JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE room_cycle (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    sequence BIGINT NOT NULL,
    source TEXT NOT NULL,
    wake_key TEXT NOT NULL,
    triggering_entry_id UUID NULL,
    status TEXT NOT NULL,
    refusal_reason TEXT NULL,
    planned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    phase TEXT NOT NULL DEFAULT 'discussion',
    synthesis_error JSONB NULL,
    synthesis_turn_id UUID NULL,
    memory_revision_id UUID NULL,
    expected_max_turns INTEGER NOT NULL DEFAULT 1,
    cancel_idempotency_key TEXT NULL,
    cost_limit_ticks BIGINT NULL
);
CREATE UNIQUE INDEX room_cycle_wake_key_uidx ON room_cycle (room_id, wake_key);
`)
}

var _ WorkspaceSkillLoader = productionSkillLoaderStub{}
var _ room.Rooms = (*productionRoomsStub)(nil)
