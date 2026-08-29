package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	roomMutationBodyLimit      = 1 << 20
	maxRoomParticipantRequests = 100
	maxRoomAgentTargets        = 100
)

type roomParticipantRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Role string `json:"role,omitempty"`
}

type createRoomRequest struct {
	Title                   string                   `json:"title"`
	Instructions            string                   `json:"instructions,omitempty"`
	Objective               string                   `json:"objective"`
	SuccessCriteria         []string                 `json:"success_criteria,omitempty"`
	StopConditions          []string                 `json:"stop_conditions,omitempty"`
	TemplateID              string                   `json:"template_id,omitempty"`
	FacilitatorAgentID      *string                  `json:"facilitator_agent_id,omitempty"`
	FacilitatorSquadID      *string                  `json:"facilitator_squad_id,omitempty"`
	Participants            []roomParticipantRequest `json:"participants,omitempty"`
	DailyTurnLimit          *int32                   `json:"daily_turn_limit,omitempty"`
	MaxCostTicks            *int64                   `json:"max_cost_ticks,omitempty"`
	ScheduleIntervalMinutes *int32                   `json:"schedule_interval_minutes,omitempty"`
	StartPaused             bool                     `json:"start_paused,omitempty"`
}

type roomMessageRequest struct {
	Body           string   `json:"body"`
	MentionAgents  []string `json:"mention_agent_ids,omitempty"`
	IdempotencyKey string   `json:"idempotency_key"`
}

type roomWakeRequest struct {
	IdempotencyKey string   `json:"idempotency_key"`
	TargetAgents   []string `json:"target_agent_ids,omitempty"`
}

type roomStatusRequest struct {
	Status string `json:"status"`
}

type roomBudgetRequest struct {
	DailyTurnLimit json.RawMessage `json:"daily_turn_limit"`
	MaxCostTicks   json.RawMessage `json:"max_cost_ticks"`
}

type roomPromotionRequest struct {
	Kind              string   `json:"kind"`
	EntryID           *string  `json:"entry_id,omitempty"`
	CycleID           *string  `json:"cycle_id,omitempty"`
	MemoryRevisionID  *string  `json:"memory_revision_id,omitempty"`
	RecommendationKey string   `json:"recommendation_key,omitempty"`
	CitationEntryIDs  []string `json:"citation_entry_ids,omitempty"`
	IdempotencyKey    string   `json:"idempotency_key"`
	Title             string   `json:"title"`
	Body              string   `json:"body,omitempty"`
	Rationale         string   `json:"rationale,omitempty"`
}

type roomSynthesisRetryRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type roomCycleReviewRequest struct {
	Action                string          `json:"action"`
	ExpectedMemoryVersion int64           `json:"expected_memory_version"`
	Correction            json.RawMessage `json:"correction,omitempty"`
	IdempotencyKey        string          `json:"idempotency_key"`
}

type roomCycleCancelRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type roomRecommendationReviewRequest struct {
	Action         string `json:"action"`
	IdempotencyKey string `json:"idempotency_key"`
}

type roomResponse struct {
	ID                       string                   `json:"id"`
	WorkspaceID              string                   `json:"workspace_id"`
	Title                    string                   `json:"title"`
	Instructions             string                   `json:"instructions"`
	Objective                string                   `json:"objective"`
	SuccessCriteria          json.RawMessage          `json:"success_criteria"`
	StopConditions           json.RawMessage          `json:"stop_conditions"`
	TemplateID               *string                  `json:"template_id"`
	CreatedByUserID          string                   `json:"created_by_user_id"`
	FacilitatorAgentID       string                   `json:"facilitator_agent_id"`
	FacilitatorSquadID       *string                  `json:"facilitator_squad_id"`
	Status                   string                   `json:"status"`
	DailyTurnLimit           *int32                   `json:"daily_turn_limit"`
	MaxCostTicks             *int64                   `json:"max_cost_ticks"`
	ScheduleIntervalMinutes  *int32                   `json:"schedule_interval_minutes"`
	NextWakeAt               *string                  `json:"next_wake_at"`
	ActiveCycleID            *string                  `json:"active_cycle_id"`
	Memory                   json.RawMessage          `json:"memory"`
	MemoryVersion            int64                    `json:"memory_version"`
	AcceptedMemoryRevisionID *string                  `json:"accepted_memory_revision_id"`
	CapabilityVersion        int32                    `json:"capability_version"`
	CreatedAt                string                   `json:"created_at"`
	UpdatedAt                string                   `json:"updated_at"`
	Value                    *roomValueSignalResponse `json:"value,omitempty"`
}

type roomValueSignalResponse struct {
	LastAcceptedRevisionID        *string `json:"last_accepted_revision_id"`
	LastAcceptedAt                *string `json:"last_accepted_at"`
	LastCycleID                   *string `json:"last_cycle_id"`
	LastRunStatus                 *string `json:"last_run_status"`
	LastRunPhase                  *string `json:"last_run_phase"`
	LastRunReason                 *string `json:"last_run_reason"`
	LastRunAt                     *string `json:"last_run_at"`
	LastRunCostTicks              int64   `json:"last_run_cost_ticks"`
	RepeatRunCount                int64   `json:"repeat_run_count"`
	AcceptedOutcomes              int64   `json:"accepted_outcomes"`
	ActiveWeeks                   int64   `json:"active_weeks"`
	AcceptedOutcomesPerActiveWeek float64 `json:"accepted_outcomes_per_active_week"`
	MedianReviewLatencySeconds    float64 `json:"median_review_latency_seconds"`
	PromotionRate                 float64 `json:"promotion_rate"`
	FailedCycles                  int64   `json:"failed_cycles"`
	RefusedCycles                 int64   `json:"refused_cycles"`
}

type roomDetailResponse struct {
	Room                  roomResponse                       `json:"room"`
	Participants          []roomParticipantResponse          `json:"participants"`
	Entries               []roomEntryResponse                `json:"entries"`
	Cycles                []roomCycleResponse                `json:"cycles"`
	Turns                 []roomTurnResponse                 `json:"turns"`
	Artifacts             []roomArtifactResponse             `json:"artifacts"`
	MemoryRevisions       []roomMemoryRevisionResponse       `json:"memory_revisions"`
	RecommendationReviews []roomRecommendationReviewResponse `json:"recommendation_reviews"`
}

type roomParticipantResponse struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"`
	ParticipantID string  `json:"participant_id"`
	Role          string  `json:"role"`
	SourceSquadID *string `json:"source_squad_id"`
	JoinedAt      string  `json:"joined_at"`
}

type roomEntryResponse struct {
	ID         string          `json:"id"`
	CycleID    *string         `json:"cycle_id"`
	TurnID     *string         `json:"turn_id"`
	Ordinal    int64           `json:"ordinal"`
	Type       string          `json:"type"`
	AuthorType string          `json:"author_type"`
	AuthorID   *string         `json:"author_id"`
	Body       string          `json:"body"`
	Mentions   json.RawMessage `json:"mentions"`
	CreatedAt  string          `json:"created_at"`
}

type roomCycleResponse struct {
	ID                string          `json:"id"`
	Sequence          int64           `json:"sequence"`
	Source            string          `json:"source"`
	WakeKey           string          `json:"wake_key"`
	TriggeringEntryID *string         `json:"triggering_entry_id"`
	Status            string          `json:"status"`
	Phase             string          `json:"phase"`
	RefusalReason     *string         `json:"refusal_reason"`
	SynthesisError    json.RawMessage `json:"synthesis_error"`
	SynthesisTurnID   *string         `json:"synthesis_turn_id"`
	MemoryRevisionID  *string         `json:"memory_revision_id"`
	ExpectedMaxTurns  int32           `json:"expected_max_turns"`
	CostLimitTicks    *int64          `json:"cost_limit_ticks"`
	PlannedAt         *string         `json:"planned_at"`
	CreatedAt         string          `json:"created_at"`
	StartedAt         *string         `json:"started_at"`
	CompletedAt       *string         `json:"completed_at"`
}

type roomTurnResponse struct {
	ID            string  `json:"id"`
	CycleID       string  `json:"cycle_id"`
	AgentID       string  `json:"agent_id"`
	SquadID       *string `json:"squad_id"`
	Status        string  `json:"status"`
	TurnKind      string  `json:"turn_kind"`
	Attempt       int32   `json:"attempt"`
	RefusalReason *string `json:"refusal_reason"`
	CreatedAt     string  `json:"created_at"`
	StartedAt     *string `json:"started_at"`
	CompletedAt   *string `json:"completed_at"`
}

type roomArtifactResponse struct {
	ID                string          `json:"id"`
	CycleID           *string         `json:"cycle_id"`
	TurnID            *string         `json:"turn_id"`
	EntryID           *string         `json:"entry_id"`
	MemoryRevisionID  *string         `json:"memory_revision_id"`
	RecommendationKey *string         `json:"recommendation_key"`
	Kind              string          `json:"kind"`
	TargetID          *string         `json:"target_id"`
	Title             string          `json:"title"`
	Body              string          `json:"body"`
	Rationale         *string         `json:"rationale"`
	CitationEntryIDs  json.RawMessage `json:"citation_entry_ids"`
	CreatedByUserID   string          `json:"created_by_user_id"`
	CreatedAt         string          `json:"created_at"`
}

type roomWakeResponse struct {
	Cycle   roomCycleResponse  `json:"cycle"`
	Turns   []roomTurnResponse `json:"turns"`
	Tasks   []string           `json:"tasks"`
	Code    string             `json:"code,omitempty"`
	Message string             `json:"message,omitempty"`
}

type roomMessageResponse struct {
	Entry roomEntryResponse `json:"entry"`
	roomWakeResponse
}

type roomMemoryRevisionResponse struct {
	ID                      string          `json:"id"`
	RoomID                  string          `json:"room_id"`
	CycleID                 string          `json:"cycle_id"`
	SynthesisTurnID         string          `json:"synthesis_turn_id"`
	Version                 int64           `json:"version"`
	SchemaVersion           int32           `json:"schema_version"`
	Synthesis               json.RawMessage `json:"synthesis"`
	Digest                  string          `json:"digest"`
	CreatorType             string          `json:"creator_type"`
	CreatorID               string          `json:"creator_id"`
	ReviewStatus            string          `json:"review_status"`
	ReviewedByUserID        *string         `json:"reviewed_by_user_id"`
	ReviewedAt              *string         `json:"reviewed_at"`
	CorrectedFromRevisionID *string         `json:"corrected_from_revision_id"`
	CreatedAt               string          `json:"created_at"`
}

type roomRecommendationReviewResponse struct {
	ID                string  `json:"id"`
	RoomID            string  `json:"room_id"`
	MemoryRevisionID  string  `json:"memory_revision_id"`
	RecommendationKey string  `json:"recommendation_key"`
	Status            string  `json:"status"`
	ArtifactID        *string `json:"artifact_id"`
	ReviewedByUserID  string  `json:"reviewed_by_user_id"`
	ReviewedAt        string  `json:"reviewed_at"`
}

type roomPreflightAgentResponse struct {
	AgentID           string  `json:"agent_id"`
	Ready             bool    `json:"ready"`
	InvocationAllowed bool    `json:"invocation_allowed"`
	Reason            *string `json:"reason"`
}

type roomBudgetResponse struct {
	DailyTurnLimit     *int32 `json:"daily_turn_limit"`
	UsedTurns          int64  `json:"used_turns"`
	MaxCostTicks       *int64 `json:"max_cost_ticks"`
	UsedCostTicks      int64  `json:"used_cost_ticks"`
	RemainingCostTicks *int64 `json:"remaining_cost_ticks"`
	ReservedCostTicks  int64  `json:"reserved_cost_ticks"`
	UncostedTurns      int64  `json:"uncosted_turns"`
}

type roomPreflightResponse struct {
	Source                   string                       `json:"source"`
	Allowed                  bool                         `json:"allowed"`
	RefusalReason            *string                      `json:"refusal_reason"`
	CapabilityVersion        int32                        `json:"capability_version"`
	CapabilityReady          bool                         `json:"capability_ready"`
	SpendLimitSupported      bool                         `json:"spend_limit_supported"`
	RequiredDaemonCapability string                       `json:"required_daemon_capability"`
	RequiredCostCapability   string                       `json:"required_cost_capability,omitempty"`
	TargetAgents             []roomPreflightAgentResponse `json:"target_agents"`
	ExpectedMaxTurns         int32                        `json:"expected_max_turns"`
	SynthesisRequired        bool                         `json:"synthesis_required"`
	Budget                   roomBudgetResponse           `json:"budget"`
}

type roomUsageResponse struct {
	TurnsTotal                    int64   `json:"turns_total"`
	CostTicks                     int64   `json:"cost_ticks"`
	UncostedTurns                 int64   `json:"uncosted_turns"`
	Failures                      int64   `json:"failures"`
	AcceptedSyntheses             int64   `json:"accepted_syntheses"`
	PromotedArtifacts             int64   `json:"promoted_artifacts"`
	RepeatRunCount                int64   `json:"repeat_run_count"`
	ActiveWeeks                   int64   `json:"active_weeks"`
	MedianReviewLatencySeconds    float64 `json:"median_review_latency_seconds"`
	AcceptedOutcomesPerActiveWeek float64 `json:"accepted_outcomes_per_active_week"`
	PromotionRate                 float64 `json:"promotion_rate"`
	FailedCycles                  int64   `json:"failed_cycles"`
	RefusedCycles                 int64   `json:"refused_cycles"`
	CostTicksPerAcceptedOutcome   float64 `json:"cost_ticks_per_accepted_outcome"`
}

type roomSynthesisRetryResponse struct {
	Cycle  roomCycleResponse `json:"cycle"`
	Turn   roomTurnResponse  `json:"turn"`
	TaskID string            `json:"task_id"`
}

type roomCycleReviewResponse struct {
	Room           roomResponse               `json:"room"`
	MemoryRevision roomMemoryRevisionResponse `json:"memory_revision"`
}

type roomCycleCancelResponse struct {
	Cycle roomCycleResponse `json:"cycle"`
}

type roomRecommendationReviewResultResponse struct {
	RecommendationReview roomRecommendationReviewResponse `json:"recommendation_review"`
}

func (h *Handler) ListRooms(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.roomWorkspaceID(w, r)
	if !ok {
		return
	}
	rooms, err := h.Rooms.List(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rooms")
		return
	}
	signals, err := h.Rooms.ListValueSignals(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list Room value signals")
		return
	}
	signalByRoom := make(map[string]roomValueSignalResponse, len(signals))
	for _, signal := range signals {
		signalByRoom[uuidToString(signal.RoomID)] = roomValueSignalToResponse(signal)
	}
	response := make([]roomResponse, len(rooms))
	for index, roomRow := range rooms {
		response[index] = roomToResponse(roomRow)
		if signal, exists := signalByRoom[uuidToString(roomRow.ID)]; exists {
			response[index].Value = &signal
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetRoom(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	detail, err := h.Rooms.Get(r.Context(), workspaceID, roomID)
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomDetailToResponse(detail))
}

func (h *Handler) GetRoomPreflight(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	targetValues := r.URL.Query()["target_agent_id"]
	if len(targetValues) > maxRoomAgentTargets {
		writeError(w, http.StatusBadRequest, "too many target agents")
		return
	}
	targets, ok := parseRoomUUIDs(w, targetValues, "target_agent_id")
	if !ok {
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "manual"
	}
	if source != "manual" && source != "schedule" {
		writeError(w, http.StatusBadRequest, "invalid room preflight source")
		return
	}
	result, err := h.Rooms.Preflight(r.Context(), roomdomain.PreflightInput{
		WorkspaceID: workspaceID, RoomID: roomID, ActorUserID: actorID,
		Source: source, TargetAgentIDs: targets,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomPreflightToResponse(result))
}

func (h *Handler) GetRoomUsage(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	result, err := h.Rooms.Usage(r.Context(), workspaceID, roomID)
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomUsageResponse{
		TurnsTotal: result.TurnsTotal, CostTicks: result.CostTicks,
		UncostedTurns: result.UncostedTurns, Failures: result.Failures,
		AcceptedSyntheses: result.AcceptedSyntheses, PromotedArtifacts: result.PromotedArtifacts,
		RepeatRunCount: result.RepeatRunCount, ActiveWeeks: result.ActiveWeeks,
		MedianReviewLatencySeconds:    result.MedianReviewLatencySeconds,
		AcceptedOutcomesPerActiveWeek: result.AcceptedOutcomesPerActiveWeek,
		PromotionRate:                 result.PromotionRate, FailedCycles: result.FailedCycles,
		RefusedCycles:               result.RefusedCycles,
		CostTicksPerAcceptedOutcome: result.CostTicksPerAcceptedOutcome,
	})
}

func roomValueSignalToResponse(signal roomdomain.ValueSignal) roomValueSignalResponse {
	return roomValueSignalResponse{
		LastAcceptedRevisionID: uuidToPtr(signal.LastAcceptedRevisionID),
		LastAcceptedAt:         timestampToPtr(signal.LastAcceptedAt),
		LastCycleID:            uuidToPtr(signal.LastCycleID),
		LastRunStatus:          optionalString(signal.LastRunStatus),
		LastRunPhase:           optionalString(signal.LastRunPhase),
		LastRunReason:          textToPtr(signal.LastRunReason),
		LastRunAt:              timestampToPtr(signal.LastRunAt),
		LastRunCostTicks:       signal.LastRunCostTicks, RepeatRunCount: signal.RepeatRunCount,
		AcceptedOutcomes: signal.AcceptedOutcomes, ActiveWeeks: signal.ActiveWeeks,
		AcceptedOutcomesPerActiveWeek: signal.AcceptedOutcomesPerActiveWeek,
		MedianReviewLatencySeconds:    signal.MedianReviewLatencySeconds,
		PromotionRate:                 signal.PromotionRate, FailedCycles: signal.FailedCycles,
		RefusedCycles: signal.RefusedCycles,
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.roomWorkspaceID(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request createRoomRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	if len(request.Participants) > maxRoomParticipantRequests {
		writeError(w, http.StatusBadRequest, "too many room participants")
		return
	}
	facilitatorAgentID, ok := optionalRoomUUID(w, request.FacilitatorAgentID, "facilitator_agent_id")
	if !ok {
		return
	}
	facilitatorSquadID, ok := optionalRoomUUID(w, request.FacilitatorSquadID, "facilitator_squad_id")
	if !ok {
		return
	}
	participants := make([]roomdomain.ParticipantInput, 0, len(request.Participants))
	for _, participant := range request.Participants {
		participantID, valid := parseUUIDOrBadRequest(w, participant.ID, "participant id")
		if !valid {
			return
		}
		participants = append(participants, roomdomain.ParticipantInput{Type: participant.Type, ID: participantID, Role: participant.Role})
	}
	detail, err := h.Rooms.Create(r.Context(), roomdomain.CreateInput{
		WorkspaceID: workspaceID, ActorUserID: actorID, Title: request.Title,
		Instructions: request.Instructions, Objective: request.Objective,
		SuccessCriteria: request.SuccessCriteria, StopConditions: request.StopConditions, TemplateID: request.TemplateID,
		FacilitatorAgentID: facilitatorAgentID,
		FacilitatorSquadID: facilitatorSquadID, Participants: participants,
		DailyTurnLimit: request.DailyTurnLimit, MaxCostTicks: request.MaxCostTicks,
		ScheduleIntervalMinutes: request.ScheduleIntervalMinutes,
		StartPaused:             request.StartPaused,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, roomDetailToResponse(detail))
}

func (h *Handler) PostRoomMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomMessageRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	if len(request.MentionAgents) > maxRoomAgentTargets {
		writeError(w, http.StatusBadRequest, "too many mentioned agents")
		return
	}
	mentions, valid := parseRoomUUIDs(w, request.MentionAgents, "mention_agent_ids")
	if !valid {
		return
	}
	result, err := h.Rooms.PostMessage(r.Context(), roomdomain.MessageInput{
		WorkspaceID: workspaceID, RoomID: roomID, ActorUserID: actorID,
		Body: request.Body, MentionAgents: mentions, IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Cycle.Status == "refused" {
		status = http.StatusConflict
	}
	wake := roomWakeToResponse(result.WakeResult)
	if status == http.StatusConflict {
		wake.Code = roomConflictCode(result.Cycle.RefusalReason.String)
		wake.Message = roomConflictMessage(wake.Code)
	}
	writeJSON(w, status, roomMessageResponse{Entry: roomEntryToResponse(result.Entry), roomWakeResponse: wake})
}

func (h *Handler) WakeRoom(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomWakeRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	if len(request.TargetAgents) > maxRoomAgentTargets {
		writeError(w, http.StatusBadRequest, "too many target agents")
		return
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		writeError(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}
	targets, valid := parseRoomUUIDs(w, request.TargetAgents, "target_agent_ids")
	if !valid {
		return
	}
	result, err := h.Rooms.Wake(r.Context(), roomdomain.WakeInput{
		WorkspaceID: workspaceID, RoomID: roomID, ActorUserID: actorID,
		Source: "manual", WakeKey: "manual:" + strings.TrimSpace(request.IdempotencyKey), TargetAgentIDs: targets,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	status := http.StatusAccepted
	if result.Cycle.Status == "refused" {
		status = http.StatusConflict
	}
	response := roomWakeToResponse(result)
	if status == http.StatusConflict {
		response.Code = roomConflictCode(result.Cycle.RefusalReason.String)
		response.Message = roomConflictMessage(response.Code)
	}
	writeJSON(w, status, response)
}

func (h *Handler) SetRoomStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	if _, ok := roomActorID(w, r); !ok {
		return
	}
	var request roomStatusRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	roomRow, err := h.Rooms.SetStatus(r.Context(), workspaceID, roomID, request.Status)
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomToResponse(roomRow))
}

func (h *Handler) UpdateRoomBudget(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomBudgetRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	dailyTurnLimit, ok := nullableRoomInt32(w, request.DailyTurnLimit, "daily_turn_limit")
	if !ok {
		return
	}
	maxCostTicks, ok := nullableRoomInt64(w, request.MaxCostTicks, "max_cost_ticks")
	if !ok {
		return
	}
	roomRow, err := h.Rooms.UpdateBudget(r.Context(), roomdomain.UpdateBudgetInput{
		WorkspaceID: workspaceID, RoomID: roomID, ActorUserID: actorID,
		DailyTurnLimit: dailyTurnLimit, MaxCostTicks: maxCostTicks,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomToResponse(roomRow))
}

func nullableRoomInt32(w http.ResponseWriter, raw json.RawMessage, field string) (pgtype.Int4, bool) {
	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, field+" is required")
		return pgtype.Int4{}, false
	}
	if string(raw) == "null" {
		return pgtype.Int4{}, true
	}
	var value int32
	if err := json.Unmarshal(raw, &value); err != nil || value <= 0 {
		writeError(w, http.StatusBadRequest, field+" must be a positive integer or null")
		return pgtype.Int4{}, false
	}
	return pgtype.Int4{Int32: value, Valid: true}, true
}

func nullableRoomInt64(w http.ResponseWriter, raw json.RawMessage, field string) (pgtype.Int8, bool) {
	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, field+" is required")
		return pgtype.Int8{}, false
	}
	if string(raw) == "null" {
		return pgtype.Int8{}, true
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil || value <= 0 {
		writeError(w, http.StatusBadRequest, field+" must be a positive integer or null")
		return pgtype.Int8{}, false
	}
	return pgtype.Int8{Int64: value, Valid: true}, true
}

func (h *Handler) RetryRoomSynthesis(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, cycleID, ok := h.roomCycleIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomSynthesisRetryRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	result, err := h.Rooms.RetrySynthesis(r.Context(), roomdomain.RetrySynthesisInput{
		WorkspaceID: workspaceID, RoomID: roomID, CycleID: cycleID,
		ActorUserID: actorID, IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, roomSynthesisRetryResponse{
		Cycle: roomCycleToResponse(result.Cycle), Turn: roomTurnToResponse(result.Turn),
		TaskID: util.UUIDToString(result.Task.ID),
	})
}

func (h *Handler) ReviewRoomCycle(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, cycleID, ok := h.roomCycleIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomCycleReviewRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	result, err := h.Rooms.Review(r.Context(), roomdomain.ReviewInput{
		WorkspaceID: workspaceID, RoomID: roomID, CycleID: cycleID,
		ActorUserID: actorID, Action: request.Action,
		ExpectedMemoryVersion: request.ExpectedMemoryVersion, Correction: request.Correction,
		IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomCycleReviewResponse{
		Room: roomToResponse(result.Room), MemoryRevision: roomMemoryRevisionToResponse(result.MemoryRevision),
	})
}

func (h *Handler) CancelRoomCycle(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, cycleID, ok := h.roomCycleIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomCycleCancelRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	cycle, err := h.Rooms.Cancel(r.Context(), roomdomain.CancelInput{
		WorkspaceID: workspaceID, RoomID: roomID, CycleID: cycleID,
		ActorUserID: actorID, IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomCycleCancelResponse{Cycle: roomCycleToResponse(cycle)})
}

func (h *Handler) ReviewRoomRecommendation(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	revisionID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "revisionId"), "memory revision id")
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomRecommendationReviewRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	review, err := h.Rooms.ReviewRecommendation(r.Context(), roomdomain.RecommendationReviewInput{
		WorkspaceID: workspaceID, RoomID: roomID, MemoryRevisionID: revisionID,
		RecommendationKey: chi.URLParam(r, "key"), ActorUserID: actorID,
		Action: request.Action, IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomRecommendationReviewResultResponse{
		RecommendationReview: roomRecommendationReviewToResponse(review),
	})
}

func (h *Handler) PromoteRoomArtifact(w http.ResponseWriter, r *http.Request) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return
	}
	actorID, ok := roomActorID(w, r)
	if !ok {
		return
	}
	var request roomPromotionRequest
	if !decodeRoomMutationRequest(w, r, &request) {
		return
	}
	entryID, valid := optionalRoomUUID(w, request.EntryID, "entry_id")
	if !valid {
		return
	}
	cycleID, valid := optionalRoomUUID(w, request.CycleID, "cycle_id")
	if !valid {
		return
	}
	memoryRevisionID, valid := optionalRoomUUID(w, request.MemoryRevisionID, "memory_revision_id")
	if !valid {
		return
	}
	citationEntryIDs, valid := parseRoomUUIDs(w, request.CitationEntryIDs, "citation_entry_ids")
	if !valid {
		return
	}
	legacySource := entryID.Valid != cycleID.Valid && !memoryRevisionID.Valid && strings.TrimSpace(request.RecommendationKey) == ""
	recommendationSource := !entryID.Valid && !cycleID.Valid && memoryRevisionID.Valid && strings.TrimSpace(request.RecommendationKey) != ""
	if !legacySource && !recommendationSource {
		writeErrorCode(w, http.StatusConflict, "promotion_source_mismatch", "promotion source fields do not identify exactly one Room result")
		return
	}
	result, err := h.Rooms.Promote(r.Context(), roomdomain.PromotionInput{
		WorkspaceID: workspaceID, RoomID: roomID, ActorUserID: actorID,
		EntryID: entryID, CycleID: cycleID, MemoryRevisionID: memoryRevisionID,
		RecommendationKey: request.RecommendationKey, CitationEntryIDs: citationEntryIDs,
		Kind: request.Kind, IdempotencyKey: request.IdempotencyKey,
		Title: request.Title, Body: request.Body, Rationale: request.Rationale,
	})
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeJSON(w, status, roomArtifactToResponse(result.Artifact))
}

func (h *Handler) roomWorkspaceID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	return parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
}

func (h *Handler) roomIDs(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	workspaceID, ok := h.roomWorkspaceID(w, r)
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	roomID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "room id")
	return workspaceID, roomID, ok
}

func (h *Handler) roomCycleIDs(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, pgtype.UUID, bool) {
	workspaceID, roomID, ok := h.roomIDs(w, r)
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, false
	}
	cycleID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "cycleId"), "cycle id")
	return workspaceID, roomID, cycleID, ok
}

func roomActorID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return pgtype.UUID{}, false
	}
	return parseUUIDOrBadRequest(w, userID, "user id")
}

func optionalRoomUUID(w http.ResponseWriter, value *string, field string) (pgtype.UUID, bool) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return pgtype.UUID{}, true
	}
	return parseUUIDOrBadRequest(w, strings.TrimSpace(*value), field)
}

func parseRoomUUIDs(w http.ResponseWriter, values []string, field string) ([]pgtype.UUID, bool) {
	result := make([]pgtype.UUID, 0, len(values))
	for _, value := range values {
		parsed, ok := parseUUIDOrBadRequest(w, value, field)
		if !ok {
			return nil, false
		}
		result = append(result, parsed)
	}
	return result, true
}

func decodeRoomMutationRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, roomMutationBodyLimit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(destination); err != nil {
		writeRoomMutationBodyError(w, err)
		return false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeRoomMutationBodyError(w, err)
		return false
	}
	return true
}

func writeRoomMutationBodyError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid request body")
}

func roomTaskIDs(tasks []db.AgentTaskQueue) []string {
	result := make([]string, len(tasks))
	for index, task := range tasks {
		result[index] = util.UUIDToString(task.ID)
	}
	return result
}

func roomToResponse(roomRow db.Room) roomResponse {
	return roomResponse{
		ID: util.UUIDToString(roomRow.ID), WorkspaceID: util.UUIDToString(roomRow.WorkspaceID),
		Title: roomRow.Title, Instructions: roomRow.Instructions, Objective: roomRow.Objective,
		SuccessCriteria: roomJSONOr(roomRow.SuccessCriteria, "[]"), StopConditions: roomJSONOr(roomRow.StopConditions, "[]"),
		TemplateID:      textPtr(roomRow.TemplateID),
		CreatedByUserID: util.UUIDToString(roomRow.CreatedByUserID), FacilitatorAgentID: util.UUIDToString(roomRow.FacilitatorAgentID),
		FacilitatorSquadID: uuidPtrString(roomRow.FacilitatorSquadID), Status: roomRow.Status,
		DailyTurnLimit: int4Ptr(roomRow.DailyTurnLimit), MaxCostTicks: int8Ptr(roomRow.MaxCostTicks),
		ScheduleIntervalMinutes: int4Ptr(roomRow.ScheduleIntervalMinutes),
		NextWakeAt:              timestampPtr(roomRow.NextWakeAt), ActiveCycleID: uuidPtrString(roomRow.ActiveCycleID),
		Memory: roomMemoryToResponse(roomRow.Memory), MemoryVersion: roomRow.MemoryVersion,
		AcceptedMemoryRevisionID: uuidPtrString(roomRow.AcceptedMemoryRevisionID), CapabilityVersion: roomRow.CapabilityVersion,
		CreatedAt: timestampToString(roomRow.CreatedAt), UpdatedAt: timestampToString(roomRow.UpdatedAt),
	}
}

func roomDetailToResponse(detail roomdomain.Detail) roomDetailResponse {
	response := roomDetailResponse{
		Room:                  roomToResponse(detail.Room),
		Participants:          make([]roomParticipantResponse, len(detail.Participants)),
		Entries:               make([]roomEntryResponse, len(detail.Entries)),
		Cycles:                make([]roomCycleResponse, len(detail.Cycles)),
		Turns:                 make([]roomTurnResponse, len(detail.Turns)),
		Artifacts:             make([]roomArtifactResponse, len(detail.Artifacts)),
		MemoryRevisions:       make([]roomMemoryRevisionResponse, len(detail.MemoryRevisions)),
		RecommendationReviews: make([]roomRecommendationReviewResponse, len(detail.RecommendationReviews)),
	}
	for index, participant := range detail.Participants {
		response.Participants[index] = roomParticipantResponse{
			ID: util.UUIDToString(participant.ID), Type: participant.ParticipantType,
			ParticipantID: util.UUIDToString(participant.ParticipantID), Role: participant.Role,
			SourceSquadID: uuidPtrString(participant.SourceSquadID), JoinedAt: timestampToString(participant.JoinedAt),
		}
	}
	for index, entry := range detail.Entries {
		response.Entries[index] = roomEntryToResponse(entry)
	}
	for index, cycle := range detail.Cycles {
		response.Cycles[index] = roomCycleToResponse(cycle)
	}
	for index, turn := range detail.Turns {
		response.Turns[index] = roomTurnToResponse(turn)
	}
	for index, artifact := range detail.Artifacts {
		response.Artifacts[index] = roomArtifactToResponse(artifact)
	}
	for index, revision := range detail.MemoryRevisions {
		response.MemoryRevisions[index] = roomMemoryRevisionToResponse(revision)
	}
	for index, review := range detail.RecommendationReviews {
		response.RecommendationReviews[index] = roomRecommendationReviewToResponse(review)
	}
	return response
}

func roomEntryToResponse(entry db.RoomEntry) roomEntryResponse {
	return roomEntryResponse{
		ID: util.UUIDToString(entry.ID), CycleID: uuidPtrString(entry.CycleID), TurnID: uuidPtrString(entry.TurnID),
		Ordinal: entry.Ordinal, Type: entry.EntryType, AuthorType: entry.AuthorType,
		AuthorID: uuidPtrString(entry.AuthorID), Body: entry.Body, Mentions: entry.Mentions,
		CreatedAt: timestampToString(entry.CreatedAt),
	}
}

func roomWakeToResponse(result roomdomain.WakeResult) roomWakeResponse {
	response := roomWakeResponse{
		Cycle: roomCycleToResponse(result.Cycle),
		Turns: make([]roomTurnResponse, len(result.Turns)),
		Tasks: roomTaskIDs(result.Tasks),
	}
	for index, turn := range result.Turns {
		response.Turns[index] = roomTurnToResponse(turn)
	}
	return response
}

func roomArtifactToResponse(artifact db.RoomArtifact) roomArtifactResponse {
	return roomArtifactResponse{
		ID: util.UUIDToString(artifact.ID), CycleID: uuidPtrString(artifact.CycleID),
		TurnID: uuidPtrString(artifact.TurnID), EntryID: uuidPtrString(artifact.EntryID),
		MemoryRevisionID: uuidPtrString(artifact.MemoryRevisionID), RecommendationKey: textPtr(artifact.RecommendationKey),
		Kind: artifact.Kind, TargetID: uuidPtrString(artifact.TargetID), Title: artifact.Title,
		Body: artifact.Body, Rationale: textPtr(artifact.Rationale), CitationEntryIDs: roomJSONOr(artifact.CitationEntryIds, "[]"),
		CreatedByUserID: util.UUIDToString(artifact.CreatedByUserID), CreatedAt: timestampToString(artifact.CreatedAt),
	}
}

func roomCycleToResponse(cycle db.RoomCycle) roomCycleResponse {
	return roomCycleResponse{
		ID: util.UUIDToString(cycle.ID), Sequence: cycle.Sequence, Source: cycle.Source,
		WakeKey: cycle.WakeKey, TriggeringEntryID: uuidPtrString(cycle.TriggeringEntryID),
		Status: cycle.Status, Phase: cycle.Phase, RefusalReason: externalRoomReasonPtr(cycle.RefusalReason),
		SynthesisError: roomNullableJSON(cycle.SynthesisError), SynthesisTurnID: uuidPtrString(cycle.SynthesisTurnID),
		MemoryRevisionID: uuidPtrString(cycle.MemoryRevisionID), ExpectedMaxTurns: cycle.ExpectedMaxTurns,
		CostLimitTicks: int8Ptr(cycle.CostLimitTicks),
		PlannedAt:      timestampPtr(cycle.PlannedAt), CreatedAt: timestampToString(cycle.CreatedAt),
		StartedAt: timestampPtr(cycle.StartedAt), CompletedAt: timestampPtr(cycle.CompletedAt),
	}
}

func roomTurnToResponse(turn db.RoomTurn) roomTurnResponse {
	return roomTurnResponse{
		ID: util.UUIDToString(turn.ID), CycleID: util.UUIDToString(turn.CycleID),
		AgentID: util.UUIDToString(turn.AgentID), SquadID: uuidPtrString(turn.SquadID),
		Status: turn.Status, TurnKind: turn.TurnKind, Attempt: turn.Attempt,
		RefusalReason: externalRoomReasonPtr(turn.RefusalReason), CreatedAt: timestampToString(turn.CreatedAt),
		StartedAt: timestampPtr(turn.StartedAt), CompletedAt: timestampPtr(turn.CompletedAt),
	}
}

func roomMemoryRevisionToResponse(revision db.RoomMemoryRevision) roomMemoryRevisionResponse {
	return roomMemoryRevisionResponse{
		ID: util.UUIDToString(revision.ID), RoomID: util.UUIDToString(revision.RoomID),
		CycleID: util.UUIDToString(revision.CycleID), SynthesisTurnID: util.UUIDToString(revision.SynthesisTurnID),
		Version: revision.Version, SchemaVersion: revision.SchemaVersion,
		Synthesis: roomJSONOr(revision.Synthesis, "{}"), Digest: revision.Digest,
		CreatorType: revision.CreatorType, CreatorID: util.UUIDToString(revision.CreatorID), ReviewStatus: revision.ReviewStatus,
		ReviewedByUserID: uuidPtrString(revision.ReviewedByUserID), ReviewedAt: timestampPtr(revision.ReviewedAt),
		CorrectedFromRevisionID: uuidPtrString(revision.CorrectedFromRevisionID), CreatedAt: timestampToString(revision.CreatedAt),
	}
}

func roomRecommendationReviewToResponse(review db.RoomRecommendationReview) roomRecommendationReviewResponse {
	return roomRecommendationReviewResponse{
		ID: util.UUIDToString(review.ID), RoomID: util.UUIDToString(review.RoomID),
		MemoryRevisionID: util.UUIDToString(review.MemoryRevisionID), RecommendationKey: review.RecommendationKey,
		Status: review.Status, ArtifactID: uuidPtrString(review.ArtifactID),
		ReviewedByUserID: util.UUIDToString(review.ReviewedByUserID), ReviewedAt: timestampToString(review.ReviewedAt),
	}
}

func roomPreflightToResponse(result roomdomain.PreflightResult) roomPreflightResponse {
	response := roomPreflightResponse{
		Source: result.Source, Allowed: result.Allowed, RefusalReason: externalRoomReason(result.RefusalReason),
		CapabilityVersion: result.CapabilityVersion, CapabilityReady: result.CapabilityReady,
		SpendLimitSupported:      result.SpendLimitSupported,
		RequiredDaemonCapability: result.RequiredDaemonCapability,
		RequiredCostCapability:   result.RequiredCostCapability,
		TargetAgents:             make([]roomPreflightAgentResponse, len(result.TargetAgents)),
		ExpectedMaxTurns:         result.ExpectedMaxTurns, SynthesisRequired: result.SynthesisRequired,
		Budget: roomBudgetResponse{
			DailyTurnLimit: result.Budget.DailyTurnLimit, UsedTurns: result.Budget.UsedTurns,
			MaxCostTicks: result.Budget.MaxCostTicks, UsedCostTicks: result.Budget.UsedCostTicks,
			RemainingCostTicks: result.Budget.RemainingCostTicks, ReservedCostTicks: result.Budget.ReservedCostTicks,
			UncostedTurns: result.Budget.UncostedTurns,
		},
	}
	for index, agent := range result.TargetAgents {
		response.TargetAgents[index] = roomPreflightAgentResponse{
			AgentID: util.UUIDToString(agent.AgentID), Ready: agent.Ready,
			InvocationAllowed: agent.InvocationAllowed, Reason: externalRoomReason(agent.Reason),
		}
	}
	return response
}

func roomJSONOr(value []byte, fallback string) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(value)
}

func roomMemoryToResponse(value []byte) json.RawMessage {
	var legacy struct {
		Summary             string            `json:"summary"`
		Facts               []string          `json:"facts"`
		Decisions           []string          `json:"decisions"`
		OpenQuestions       []string          `json:"open_questions"`
		RecentContributions []json.RawMessage `json:"recent_contributions"`
	}
	legacy.Facts = []string{}
	legacy.Decisions = []string{}
	legacy.OpenQuestions = []string{}
	legacy.RecentContributions = []json.RawMessage{}
	if err := json.Unmarshal(value, &legacy); err == nil {
		encoded, marshalErr := json.Marshal(legacy)
		if marshalErr == nil {
			return encoded
		}
	}
	var synthesis roomdomain.Synthesis
	if err := json.Unmarshal(value, &synthesis); err != nil || synthesis.SchemaVersion != roomdomain.RoomSynthesisSchemaVersion {
		return json.RawMessage(`{"summary":"","facts":[],"decisions":[],"open_questions":[],"recent_contributions":[]}`)
	}
	projected := struct {
		Summary             string            `json:"summary"`
		Facts               []string          `json:"facts"`
		Decisions           []string          `json:"decisions"`
		OpenQuestions       []string          `json:"open_questions"`
		RecentContributions []json.RawMessage `json:"recent_contributions"`
	}{
		Summary: synthesis.Summary, Facts: roomSynthesisTexts(synthesis.Facts),
		Decisions: roomSynthesisTexts(synthesis.Decisions), OpenQuestions: roomSynthesisTexts(synthesis.OpenQuestions),
		RecentContributions: []json.RawMessage{},
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		return json.RawMessage(`{"summary":"","facts":[],"decisions":[],"open_questions":[],"recent_contributions":[]}`)
	}
	return encoded
}

func roomSynthesisTexts(items []roomdomain.SynthesisItem) []string {
	result := make([]string, len(items))
	for index, item := range items {
		result[index] = item.Text
	}
	return result
}

func roomNullableJSON(value []byte) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage("null")
	}
	return json.RawMessage(value)
}

func int4Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	return &value.Int32
}

func int8Ptr(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func timestampPtr(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	formatted := timestampToString(value)
	return &formatted
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func externalRoomReason(reason string) *string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil
	}
	value := roomConflictCode(reason)
	return &value
}

func externalRoomReasonPtr(reason pgtype.Text) *string {
	if !reason.Valid {
		return nil
	}
	return externalRoomReason(reason.String)
}

func roomConflictCode(reason string) string {
	switch strings.TrimSpace(reason) {
	case "cycle_active", "active_cycle":
		return "active_cycle"
	case "room_paused":
		return "room_paused"
	case "room_archived":
		return "room_archived"
	case "budget_exhausted":
		return "budget_exhausted"
	case "invocation_not_allowed":
		return "invocation_not_allowed"
	case "spend_limit_unsupported":
		return "spend_limit_unsupported"
	case "agent_unavailable", "daemon_capability_unavailable", "no_targets":
		return "agent_unavailable"
	default:
		return "agent_unavailable"
	}
}

func roomConflictMessage(code string) string {
	switch code {
	case "room_paused":
		return "message saved, but the Room is paused"
	case "room_archived":
		return "message saved, but the Room is archived"
	case "active_cycle":
		return "message saved, but another Room cycle is active"
	case "budget_exhausted":
		return "message saved, but the Room budget is exhausted"
	case "spend_limit_unsupported":
		return "message saved, but no connected runtime can enforce the Room spend limit"
	case "invocation_not_allowed":
		return "message saved, but an Agent cannot be invoked by this member"
	default:
		return "message saved, but the Room could not run"
	}
}

func (h *Handler) writeRoomError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, roomdomain.ErrNotFound):
		writeError(w, http.StatusNotFound, "room not found")
	case errors.Is(err, roomdomain.ErrInvalidParticipant):
		writeError(w, http.StatusBadRequest, "invalid room participant")
	case errors.Is(err, roomdomain.ErrInvocationNotAllowed):
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":       "Room Agent invocation is not allowed",
			"code":        "invocation_not_allowed",
			"reason_code": "invocation_not_allowed",
		})
	case errors.Is(err, roomdomain.ErrIdempotencyConflict):
		writeErrorCode(w, http.StatusConflict, "idempotency_conflict", "idempotency key conflicts with an earlier request")
	case errors.Is(err, roomdomain.ErrStaleReview):
		writeErrorCode(w, http.StatusConflict, "stale_review", "Room memory review is stale")
	case errors.Is(err, roomdomain.ErrSynthesisNotRetryable):
		writeErrorCode(w, http.StatusConflict, "synthesis_not_retryable", "Room synthesis cannot be retried")
	case errors.Is(err, roomdomain.ErrBudgetExhausted):
		writeErrorCode(w, http.StatusConflict, "budget_exhausted", "Room budget is exhausted")
	case errors.Is(err, roomdomain.ErrBudgetPermissionDenied):
		writeErrorCode(w, http.StatusForbidden, "budget_permission_denied", "Room budget updates require workspace owner or admin access")
	case errors.Is(err, roomdomain.ErrBudgetBelowCommitted):
		writeErrorCode(w, http.StatusConflict, "budget_below_committed", "Room budget is below active or completed usage")
	case errors.Is(err, roomdomain.ErrBudgetHasUncostedUsage):
		writeErrorCode(w, http.StatusConflict, "budget_has_uncosted_usage", "Room cost budget requires fully costed usage")
	case errors.Is(err, roomdomain.ErrSpendLimitUnsupported):
		writeErrorCode(w, http.StatusConflict, "spend_limit_unsupported", "Room spend limits require a cost-bound execution backend")
	case errors.Is(err, roomdomain.ErrRecommendationReviewed):
		writeErrorCode(w, http.StatusConflict, "recommendation_already_reviewed", "Room recommendation was already reviewed")
	case errors.Is(err, roomdomain.ErrPromotionSourceMismatch):
		writeErrorCode(w, http.StatusConflict, "promotion_source_mismatch", "Room promotion source does not match the accepted outcome")
	case errors.Is(err, roomdomain.ErrInvalidSynthesis):
		writeError(w, http.StatusBadRequest, "invalid Room synthesis")
	case errors.Is(err, roomdomain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid room input")
	default:
		writeError(w, http.StatusInternalServerError, "room operation failed")
	}
}
