package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	roomContractWorkspaceID = "10000000-0000-0000-0000-000000000001"
	roomContractUserID      = "10000000-0000-0000-0000-000000000002"
	roomContractRoomID      = "10000000-0000-0000-0000-000000000003"
	roomContractCycleID     = "10000000-0000-0000-0000-000000000004"
	roomContractTurnID      = "10000000-0000-0000-0000-000000000005"
	roomContractAgentID     = "10000000-0000-0000-0000-000000000006"
	roomContractRevisionID  = "10000000-0000-0000-0000-000000000007"
	roomContractArtifactID  = "10000000-0000-0000-0000-000000000008"
	roomContractEntryID     = "10000000-0000-0000-0000-000000000009"
	roomContractTaskID      = "10000000-0000-0000-0000-00000000000a"
)

type roomHandlerFake struct {
	createInput               roomdomain.CreateInput
	createResult              roomdomain.Detail
	messageResult             roomdomain.MessageResult
	preflightTargets          []pgtype.UUID
	preflightSource           string
	preflightResult           roomdomain.PreflightResult
	budgetInput               roomdomain.UpdateBudgetInput
	usageResult               roomdomain.UsageSummary
	valueSignals              []roomdomain.ValueSignal
	retryInput                roomdomain.RetrySynthesisInput
	retryResult               roomdomain.RetrySynthesisResult
	reviewInput               roomdomain.ReviewInput
	reviewResult              roomdomain.ReviewResult
	cancelInput               roomdomain.CancelInput
	cancelResult              db.RoomCycle
	recommendationReviewInput roomdomain.RecommendationReviewInput
	recommendationReview      db.RoomRecommendationReview
	promotionInput            roomdomain.PromotionInput
	promotionResult           roomdomain.PromotionResult
	err                       error
}

func (f *roomHandlerFake) List(context.Context, pgtype.UUID) ([]db.Room, error) {
	return nil, f.err
}
func (f *roomHandlerFake) ListValueSignals(context.Context, pgtype.UUID) ([]roomdomain.ValueSignal, error) {
	return f.valueSignals, f.err
}
func (f *roomHandlerFake) Get(context.Context, pgtype.UUID, pgtype.UUID) (roomdomain.Detail, error) {
	return f.createResult, f.err
}
func (f *roomHandlerFake) Create(_ context.Context, input roomdomain.CreateInput) (roomdomain.Detail, error) {
	f.createInput = input
	return f.createResult, f.err
}
func (f *roomHandlerFake) PostMessage(context.Context, roomdomain.MessageInput) (roomdomain.MessageResult, error) {
	return f.messageResult, f.err
}
func (f *roomHandlerFake) Wake(context.Context, roomdomain.WakeInput) (roomdomain.WakeResult, error) {
	return f.messageResult.WakeResult, f.err
}
func (f *roomHandlerFake) SetStatus(context.Context, pgtype.UUID, pgtype.UUID, string) (db.Room, error) {
	return f.createResult.Room, f.err
}
func (f *roomHandlerFake) UpdateBudget(_ context.Context, input roomdomain.UpdateBudgetInput) (db.Room, error) {
	f.budgetInput = input
	return f.createResult.Room, f.err
}
func (f *roomHandlerFake) Promote(_ context.Context, input roomdomain.PromotionInput) (roomdomain.PromotionResult, error) {
	f.promotionInput = input
	return f.promotionResult, f.err
}
func (f *roomHandlerFake) Preflight(_ context.Context, input roomdomain.PreflightInput) (roomdomain.PreflightResult, error) {
	f.preflightTargets = input.TargetAgentIDs
	f.preflightSource = input.Source
	return f.preflightResult, f.err
}
func (f *roomHandlerFake) Usage(context.Context, pgtype.UUID, pgtype.UUID) (roomdomain.UsageSummary, error) {
	return f.usageResult, f.err
}
func (f *roomHandlerFake) RetrySynthesis(_ context.Context, input roomdomain.RetrySynthesisInput) (roomdomain.RetrySynthesisResult, error) {
	f.retryInput = input
	return f.retryResult, f.err
}
func (f *roomHandlerFake) Review(_ context.Context, input roomdomain.ReviewInput) (roomdomain.ReviewResult, error) {
	f.reviewInput = input
	return f.reviewResult, f.err
}
func (f *roomHandlerFake) Cancel(_ context.Context, input roomdomain.CancelInput) (db.RoomCycle, error) {
	f.cancelInput = input
	return f.cancelResult, f.err
}
func (f *roomHandlerFake) ReviewRecommendation(_ context.Context, input roomdomain.RecommendationReviewInput) (db.RoomRecommendationReview, error) {
	f.recommendationReviewInput = input
	return f.recommendationReview, f.err
}

func TestRoomOutcomeCreateAndDetailWireContract(t *testing.T) {
	fixture := roomContractFixture(t)
	fake := &roomHandlerFake{createResult: fixture}
	h := &Handler{Rooms: fake}
	recorder := httptest.NewRecorder()
	request := roomContractRequest(http.MethodPost, "/api/rooms", map[string]any{
		"title": "Incident review", "instructions": "Stay cited", "objective": "Choose a recovery plan",
		"success_criteria": []string{"Owner named"}, "stop_conditions": []string{"Budget reached"},
		"template_id": "incident", "facilitator_agent_id": roomContractAgentID,
		"daily_turn_limit": 12, "max_cost_ticks": 44, "schedule_interval_minutes": 60,
	})

	h.CreateRoom(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if fake.createInput.Objective != "Choose a recovery plan" || fake.createInput.TemplateID != "incident" || fake.createInput.MaxCostTicks == nil || *fake.createInput.MaxCostTicks != 44 {
		t.Fatalf("create input did not preserve outcome fields: %+v", fake.createInput)
	}
	if len(fake.createInput.SuccessCriteria) != 1 || len(fake.createInput.StopConditions) != 1 {
		t.Fatalf("create criteria = %#v / %#v", fake.createInput.SuccessCriteria, fake.createInput.StopConditions)
	}
	assertRoomDetailContract(t, recorder.Body.Bytes())
}

func TestRoomOutcomeLifecycleHTTPContract(t *testing.T) {
	fixture := roomContractFixture(t)
	fake := &roomHandlerFake{
		createResult: fixture,
		preflightResult: roomdomain.PreflightResult{
			Source: "schedule", Allowed: false, RefusalReason: "cycle_active", CapabilityVersion: 2,
			CapabilityReady: true, RequiredDaemonCapability: "room-outcomes-v2",
			ExpectedMaxTurns: 2, SynthesisRequired: true,
			TargetAgents: []roomdomain.PreflightAgent{{AgentID: roomUUID(t, roomContractAgentID), Ready: true, InvocationAllowed: true}},
			Budget:       roomdomain.BudgetSummary{DailyTurnLimit: int32Pointer(12), UsedTurns: 2, MaxCostTicks: int64Pointer(44), UsedCostTicks: 5, RemainingCostTicks: int64Pointer(39), ReservedCostTicks: 12},
		},
		usageResult:  roomdomain.UsageSummary{TurnsTotal: 8, CostTicks: 21, UncostedTurns: 1, Failures: 2, AcceptedSyntheses: 3, PromotedArtifacts: 4},
		retryResult:  roomdomain.RetrySynthesisResult{Cycle: fixture.Cycles[0], Turn: fixture.Turns[0], Task: db.AgentTaskQueue{ID: roomUUID(t, roomContractTaskID)}},
		reviewResult: roomdomain.ReviewResult{Room: fixture.Room, MemoryRevision: fixture.MemoryRevisions[0]},
		cancelResult: fixture.Cycles[0], recommendationReview: fixture.RecommendationReviews[0],
		promotionResult: roomdomain.PromotionResult{Artifact: fixture.Artifacts[0], Created: true},
	}
	h := &Handler{Rooms: fake}

	t.Run("preflight", func(t *testing.T) {
		recorder := callRoomHandler(h.GetRoomPreflight, roomContractRequest(http.MethodGet, "/api/rooms/"+roomContractRoomID+"/preflight?source=schedule&target_agent_id="+roomContractAgentID, nil), "id", roomContractRoomID)
		if recorder.Code != http.StatusOK || len(fake.preflightTargets) != 1 || fake.preflightSource != "schedule" {
			t.Fatalf("preflight = %d targets=%d: %s", recorder.Code, len(fake.preflightTargets), recorder.Body.String())
		}
		var body map[string]any
		decodeRoomContract(t, recorder.Body.Bytes(), &body)
		if body["source"] != "schedule" || body["refusal_reason"] != "active_cycle" || body["required_daemon_capability"] != "room-outcomes-v2" {
			t.Fatalf("preflight body = %#v", body)
		}
		budget := body["budget"].(map[string]any)
		if budget["reserved_cost_ticks"] != float64(12) {
			t.Fatalf("preflight budget = %#v", budget)
		}
	})

	t.Run("preflight rejects an unknown source", func(t *testing.T) {
		recorder := callRoomHandler(
			h.GetRoomPreflight,
			roomContractRequest(
				http.MethodGet,
				"/api/rooms/"+roomContractRoomID+"/preflight?source=webhook",
				nil,
			),
			"id",
			roomContractRoomID,
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid preflight source = %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("usage", func(t *testing.T) {
		recorder := callRoomHandler(h.GetRoomUsage, roomContractRequest(http.MethodGet, "/api/rooms/"+roomContractRoomID+"/usage", nil), "id", roomContractRoomID)
		if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"accepted_syntheses":3`)) {
			t.Fatalf("usage = %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("budget update requires both nullable limits", func(t *testing.T) {
		recorder := callRoomHandler(h.UpdateRoomBudget, roomContractRequest(http.MethodPut, "/budget", map[string]any{
			"daily_turn_limit": 18, "max_cost_ticks": nil,
		}), "id", roomContractRoomID)
		if recorder.Code != http.StatusOK || !fake.budgetInput.DailyTurnLimit.Valid || fake.budgetInput.DailyTurnLimit.Int32 != 18 || fake.budgetInput.MaxCostTicks.Valid {
			t.Fatalf("budget update = %d input=%+v: %s", recorder.Code, fake.budgetInput, recorder.Body.String())
		}
		missing := callRoomHandler(h.UpdateRoomBudget, roomContractRequest(http.MethodPut, "/budget", map[string]any{
			"daily_turn_limit": 18,
		}), "id", roomContractRoomID)
		if missing.Code != http.StatusBadRequest {
			t.Fatalf("missing max cost = %d: %s", missing.Code, missing.Body.String())
		}
	})

	t.Run("retry", func(t *testing.T) {
		recorder := callRoomHandler(h.RetryRoomSynthesis, roomContractRequest(http.MethodPost, "/retry", map[string]any{"idempotency_key": "retry-1"}), "id", roomContractRoomID, "cycleId", roomContractCycleID)
		if recorder.Code != http.StatusAccepted || fake.retryInput.IdempotencyKey != "retry-1" || !bytes.Contains(recorder.Body.Bytes(), []byte(roomContractTaskID)) {
			t.Fatalf("retry = %d input=%+v: %s", recorder.Code, fake.retryInput, recorder.Body.String())
		}
	})

	t.Run("review", func(t *testing.T) {
		correction := map[string]any{"schema_version": 1, "summary": "Corrected", "facts": []any{}, "decisions": []any{}, "open_questions": []any{}, "disagreements": []any{}, "action_items": []any{}, "recommendations": []any{}, "confidence": 0.8}
		recorder := callRoomHandler(h.ReviewRoomCycle, roomContractRequest(http.MethodPost, "/review", map[string]any{"action": "correct", "expected_memory_version": 7, "correction": correction, "idempotency_key": "review-1"}), "id", roomContractRoomID, "cycleId", roomContractCycleID)
		if recorder.Code != http.StatusOK || fake.reviewInput.ExpectedMemoryVersion != 7 || !json.Valid(fake.reviewInput.Correction) {
			t.Fatalf("review = %d input=%+v: %s", recorder.Code, fake.reviewInput, recorder.Body.String())
		}
		assertRoomDetailObjectFields(t, recorder.Body.Bytes(), "room")
	})

	t.Run("cancel", func(t *testing.T) {
		recorder := callRoomHandler(h.CancelRoomCycle, roomContractRequest(http.MethodPost, "/cancel", map[string]any{"idempotency_key": "cancel-1"}), "id", roomContractRoomID, "cycleId", roomContractCycleID)
		if recorder.Code != http.StatusOK || fake.cancelInput.IdempotencyKey != "cancel-1" || !bytes.Contains(recorder.Body.Bytes(), []byte(`"phase":"awaiting_review"`)) {
			t.Fatalf("cancel = %d input=%+v: %s", recorder.Code, fake.cancelInput, recorder.Body.String())
		}
	})

	t.Run("recommendation reject", func(t *testing.T) {
		recorder := callRoomHandler(h.ReviewRoomRecommendation, roomContractRequest(http.MethodPost, "/recommendation", map[string]any{"action": "reject", "idempotency_key": "reject-1"}), "id", roomContractRoomID, "revisionId", roomContractRevisionID, "key", "next-step")
		if recorder.Code != http.StatusOK || fake.recommendationReviewInput.RecommendationKey != "next-step" || !bytes.Contains(recorder.Body.Bytes(), []byte(`"status":"rejected"`)) {
			t.Fatalf("recommendation = %d input=%+v: %s", recorder.Code, fake.recommendationReviewInput, recorder.Body.String())
		}
	})

	t.Run("promotion", func(t *testing.T) {
		recorder := callRoomHandler(h.PromoteRoomArtifact, roomContractRequest(http.MethodPost, "/promotion", map[string]any{
			"kind": "decision", "memory_revision_id": roomContractRevisionID, "recommendation_key": "next-step",
			"citation_entry_ids": []string{roomContractEntryID}, "idempotency_key": "promote-1", "title": "Ship it", "body": "Edited body",
		}), "id", roomContractRoomID)
		if recorder.Code != http.StatusCreated || fake.promotionInput.RecommendationKey != "next-step" || fake.promotionInput.Body != "Edited body" || len(fake.promotionInput.CitationEntryIDs) != 1 {
			t.Fatalf("promotion = %d input=%+v: %s", recorder.Code, fake.promotionInput, recorder.Body.String())
		}
		if !bytes.Contains(recorder.Body.Bytes(), []byte(`"citation_entry_ids":["`+roomContractEntryID+`"]`)) {
			t.Fatalf("promotion response lost provenance: %s", recorder.Body.String())
		}
	})
}

func TestRoomRefusalConflictPreservesSavedResult(t *testing.T) {
	tests := []struct {
		name       string
		reason     string
		code       string
		useMessage bool
	}{
		{name: "active cycle", reason: "cycle_active", code: "active_cycle", useMessage: true},
		{name: "paused", reason: "room_paused", code: "room_paused", useMessage: true},
		{name: "budget", reason: "budget_exhausted", code: "budget_exhausted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := roomContractFixture(t)
			fixture.Cycles[0].Status = "refused"
			fixture.Cycles[0].Phase = "refused"
			fixture.Cycles[0].RefusalReason = pgtype.Text{String: test.reason, Valid: true}
			fake := &roomHandlerFake{messageResult: roomdomain.MessageResult{
				Entry: fixture.Entries[0], WakeResult: roomdomain.WakeResult{
					Cycle: fixture.Cycles[0], Turns: fixture.Turns,
					Tasks: []db.AgentTaskQueue{{ID: roomUUID(t, roomContractTaskID)}},
				},
			}}
			h := &Handler{Rooms: fake}
			call := h.WakeRoom
			request := roomContractRequest(http.MethodPost, "/wake", map[string]any{"idempotency_key": "wake-1"})
			if test.useMessage {
				call = h.PostRoomMessage
				request = roomContractRequest(http.MethodPost, "/messages", map[string]any{"body": "Keep this", "idempotency_key": "message-1"})
			}
			recorder := callRoomHandler(call, request, "id", roomContractRoomID)
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]any
			decodeRoomContract(t, recorder.Body.Bytes(), &body)
			if body["code"] != test.code || body["message"] == "" || body["cycle"] == nil || body["turns"] == nil || body["tasks"] == nil {
				t.Fatalf("saved-without-run response = %#v", body)
			}
			if test.useMessage && body["entry"] == nil {
				t.Fatalf("saved message response omitted entry: %#v", body)
			}
		})
	}
}

func TestRoomConflictCodesAndMalformedInputs(t *testing.T) {
	errorsToCodes := []struct {
		err  error
		code string
	}{
		{roomdomain.ErrInvocationNotAllowed, "invocation_not_allowed"},
		{roomdomain.ErrIdempotencyConflict, "idempotency_conflict"},
		{roomdomain.ErrStaleReview, "stale_review"},
		{roomdomain.ErrSynthesisNotRetryable, "synthesis_not_retryable"},
		{roomdomain.ErrBudgetExhausted, "budget_exhausted"},
		{roomdomain.ErrRecommendationReviewed, "recommendation_already_reviewed"},
		{roomdomain.ErrPromotionSourceMismatch, "promotion_source_mismatch"},
	}
	invalidSynthesis := httptest.NewRecorder()
	(&Handler{}).writeRoomError(invalidSynthesis, roomdomain.ErrInvalidSynthesis)
	if invalidSynthesis.Code != http.StatusBadRequest {
		t.Fatalf("invalid synthesis status = %d: %s", invalidSynthesis.Code, invalidSynthesis.Body.String())
	}
	for _, test := range errorsToCodes {
		recorder := httptest.NewRecorder()
		(&Handler{}).writeRoomError(recorder, test.err)
		wantStatus := http.StatusConflict
		if errors.Is(test.err, roomdomain.ErrInvocationNotAllowed) {
			wantStatus = http.StatusForbidden
		}
		if recorder.Code != wantStatus || !bytes.Contains(recorder.Body.Bytes(), []byte(`"code":"`+test.code+`"`)) {
			t.Fatalf("error %v = %d: %s", test.err, recorder.Code, recorder.Body.String())
		}
		if errors.Is(test.err, roomdomain.ErrInvocationNotAllowed) &&
			!bytes.Contains(recorder.Body.Bytes(), []byte(`"reason_code":"invocation_not_allowed"`)) {
			t.Fatalf("invocation denial omitted compatibility reason_code: %s", recorder.Body.String())
		}
	}

	fake := &roomHandlerFake{err: errors.New("must not be called")}
	h := &Handler{Rooms: fake}
	badCycle := callRoomHandler(h.CancelRoomCycle, roomContractRequest(http.MethodPost, "/cancel", map[string]any{"idempotency_key": "x"}), "id", roomContractRoomID, "cycleId", "not-a-uuid")
	if badCycle.Code != http.StatusBadRequest {
		t.Fatalf("malformed cycle status = %d: %s", badCycle.Code, badCycle.Body.String())
	}
	badPromotion := callRoomHandler(h.PromoteRoomArtifact, roomContractRequest(http.MethodPost, "/promotion", map[string]any{
		"kind": "decision", "entry_id": roomContractEntryID, "cycle_id": roomContractCycleID, "idempotency_key": "x", "title": "bad",
	}), "id", roomContractRoomID)
	if badPromotion.Code != http.StatusConflict || !bytes.Contains(badPromotion.Body.Bytes(), []byte(`"code":"promotion_source_mismatch"`)) {
		t.Fatalf("promotion mismatch = %d: %s", badPromotion.Code, badPromotion.Body.String())
	}
}

func TestRoomOutcomeMutationsRejectMachineActors(t *testing.T) {
	h := &Handler{Rooms: &roomHandlerFake{}}
	tests := []struct {
		name string
		call http.HandlerFunc
	}{
		{name: "review", call: h.ReviewRoomCycle},
		{name: "cancel", call: h.CancelRoomCycle},
		{name: "recommendation", call: h.ReviewRoomRecommendation},
		{name: "promotion", call: h.PromoteRoomArtifact},
		{name: "budget", call: h.UpdateRoomBudget},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := roomContractRequest(http.MethodPost, "/rooms", map[string]any{})
			request.Header.Set("X-Actor-Source", "task_token")
			recorder := httptest.NewRecorder()
			RequireHumanActor(test.call).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("machine status = %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func roomContractFixture(t *testing.T) roomdomain.Detail {
	t.Helper()
	now := pgtype.Timestamptz{Time: time.Date(2026, time.August, 23, 10, 30, 0, 0, time.UTC), Valid: true}
	roomID := roomUUID(t, roomContractRoomID)
	cycleID := roomUUID(t, roomContractCycleID)
	turnID := roomUUID(t, roomContractTurnID)
	revisionID := roomUUID(t, roomContractRevisionID)
	entryID := roomUUID(t, roomContractEntryID)
	artifactID := roomUUID(t, roomContractArtifactID)
	userID := roomUUID(t, roomContractUserID)
	synthesis := json.RawMessage(`{"schema_version":1,"summary":"Cited outcome","facts":[{"text":"A fact","citation_entry_ids":["` + roomContractEntryID + `"],"confidence":0.9}],"decisions":[],"open_questions":[],"disagreements":[],"action_items":[],"recommendations":[{"key":"next-step","kind":"decision","title":"Ship it","body":"Body","rationale":"Why","citation_entry_ids":["` + roomContractEntryID + `"],"confidence":0.8}],"confidence":0.85}`)
	roomRow := db.Room{
		ID: roomID, WorkspaceID: roomUUID(t, roomContractWorkspaceID), Title: "Outcome Room", Instructions: "Cite it",
		Objective: "Decide", SuccessCriteria: []byte(`["Decision recorded"]`), StopConditions: []byte(`["Budget reached"]`),
		TemplateID: pgtype.Text{String: "decision", Valid: true}, CreatedByUserID: userID,
		FacilitatorAgentID: roomUUID(t, roomContractAgentID), Status: "active",
		DailyTurnLimit: pgtype.Int4{Int32: 12, Valid: true}, MaxCostTicks: pgtype.Int8{Int64: 44, Valid: true},
		Memory: synthesis, MemoryVersion: 7, AcceptedMemoryRevisionID: revisionID, CapabilityVersion: 2,
		CreatedAt: now, UpdatedAt: now,
	}
	cycle := db.RoomCycle{
		ID: cycleID, WorkspaceID: roomRow.WorkspaceID, RoomID: roomID, Sequence: 2, Source: "manual", WakeKey: "manual:one",
		Status: "running", Phase: "awaiting_review", SynthesisError: []byte("null"), SynthesisTurnID: turnID,
		MemoryRevisionID: revisionID, ExpectedMaxTurns: 2, CreatedAt: now, StartedAt: now,
	}
	turn := db.RoomTurn{
		ID: turnID, WorkspaceID: roomRow.WorkspaceID, RoomID: roomID, CycleID: cycleID,
		AgentID: roomRow.FacilitatorAgentID, Status: "completed", TurnKind: "synthesis", Attempt: 2,
		CreatedAt: now, StartedAt: now, CompletedAt: now,
	}
	revision := db.RoomMemoryRevision{
		ID: revisionID, WorkspaceID: roomRow.WorkspaceID, RoomID: roomID, CycleID: cycleID, SynthesisTurnID: turnID,
		Version: 3, SchemaVersion: 1, Synthesis: synthesis, Digest: "sha256:test", ReviewStatus: "accepted",
		CreatorType: "agent", CreatorID: roomRow.FacilitatorAgentID,
		ReviewedByUserID: userID, ReviewedAt: now, CreatedAt: now,
	}
	artifact := db.RoomArtifact{
		ID: artifactID, WorkspaceID: roomRow.WorkspaceID, RoomID: roomID, CycleID: cycleID, TurnID: turnID,
		Kind: "decision", TargetID: artifactID, Title: "Ship it", Body: "Edited body", CreatedByUserID: userID,
		MemoryRevisionID: revisionID, RecommendationKey: pgtype.Text{String: "next-step", Valid: true},
		CitationEntryIds: []byte(`["` + roomContractEntryID + `"]`), CreatedAt: now,
	}
	return roomdomain.Detail{
		Room:         roomRow,
		Participants: []db.RoomParticipant{{ID: roomUUID(t, "10000000-0000-0000-0000-00000000000b"), WorkspaceID: roomRow.WorkspaceID, RoomID: roomID, ParticipantType: "agent", ParticipantID: roomRow.FacilitatorAgentID, Role: "facilitator", JoinedAt: now}},
		Entries:      []db.RoomEntry{{ID: entryID, WorkspaceID: roomRow.WorkspaceID, RoomID: roomID, CycleID: cycleID, TurnID: turnID, Ordinal: 1, EntryType: "result", AuthorType: "agent", AuthorID: roomRow.FacilitatorAgentID, Body: "Evidence", Mentions: []byte("[]"), CreatedAt: now}},
		Cycles:       []db.RoomCycle{cycle}, Turns: []db.RoomTurn{turn}, Artifacts: []db.RoomArtifact{artifact},
		MemoryRevisions:       []db.RoomMemoryRevision{revision},
		RecommendationReviews: []db.RoomRecommendationReview{{ID: roomUUID(t, "10000000-0000-0000-0000-00000000000c"), WorkspaceID: roomRow.WorkspaceID, RoomID: roomID, MemoryRevisionID: revisionID, RecommendationKey: "next-step", Status: "rejected", ReviewedByUserID: userID, ReviewedAt: now}},
	}
}

func assertRoomDetailContract(t *testing.T, raw []byte) {
	t.Helper()
	var body map[string]any
	decodeRoomContract(t, raw, &body)
	assertRoomDetailObjectFields(t, raw, "room")
	for _, field := range []string{"participants", "entries", "cycles", "turns", "artifacts", "memory_revisions", "recommendation_reviews"} {
		if _, ok := body[field].([]any); !ok {
			t.Fatalf("detail field %q is not an array: %#v", field, body[field])
		}
	}
	revisions := body["memory_revisions"].([]any)
	revision := revisions[0].(map[string]any)
	if _, ok := revision["synthesis"].(map[string]any); !ok {
		t.Fatalf("revision synthesis is not JSON: %#v", revision["synthesis"])
	}
	if revision["creator_type"] != "agent" || revision["creator_id"] != roomContractAgentID {
		t.Fatalf("revision creator provenance = %#v", revision)
	}
}

func assertRoomDetailObjectFields(t *testing.T, raw []byte, field string) {
	t.Helper()
	var body map[string]any
	decodeRoomContract(t, raw, &body)
	roomObject, ok := body[field].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", field, body[field])
	}
	for _, key := range []string{"objective", "success_criteria", "stop_conditions", "template_id", "max_cost_ticks", "accepted_memory_revision_id", "capability_version"} {
		if _, ok := roomObject[key]; !ok {
			t.Fatalf("Room response omitted %q: %#v", key, roomObject)
		}
	}
	memory, ok := roomObject["memory"].(map[string]any)
	if !ok {
		t.Fatalf("Room memory is not an object: %#v", roomObject["memory"])
	}
	facts, ok := memory["facts"].([]any)
	if !ok || len(facts) == 0 {
		t.Fatalf("Room memory facts are not a populated array: %#v", memory["facts"])
	}
	if _, ok := facts[0].(string); !ok {
		t.Fatalf("Room memory must project synthesis items to installed-client strings: %#v", facts[0])
	}
}

func roomContractRequest(method, path string, body any) *http.Request {
	var buffer bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buffer).Encode(body)
	}
	request := httptest.NewRequest(method, path, &buffer)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", roomContractUserID)
	request.Header.Set("X-Workspace-ID", roomContractWorkspaceID)
	return request
}

func callRoomHandler(call http.HandlerFunc, request *http.Request, params ...string) *httptest.ResponseRecorder {
	routeContext := chi.NewRouteContext()
	for index := 0; index < len(params); index += 2 {
		routeContext.URLParams.Add(params[index], params[index+1])
	}
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()
	call(recorder, request)
	return recorder
}

func decodeRoomContract(t *testing.T, raw []byte, destination any) {
	t.Helper()
	if err := json.Unmarshal(raw, destination); err != nil {
		t.Fatalf("decode response: %v: %s", err, raw)
	}
}

func roomUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	parsed, err := util.ParseUUID(value)
	if err != nil {
		t.Fatalf("parse UUID %q: %v", value, err)
	}
	return parsed
}

func int32Pointer(value int32) *int32 { return &value }
func int64Pointer(value int64) *int64 { return &value }
