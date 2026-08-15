package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type roomParticipantRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Role string `json:"role,omitempty"`
}

type createRoomRequest struct {
	Title                   string                   `json:"title"`
	Instructions            string                   `json:"instructions,omitempty"`
	FacilitatorAgentID      *string                  `json:"facilitator_agent_id,omitempty"`
	FacilitatorSquadID      *string                  `json:"facilitator_squad_id,omitempty"`
	Participants            []roomParticipantRequest `json:"participants,omitempty"`
	DailyTurnLimit          *int32                   `json:"daily_turn_limit,omitempty"`
	ScheduleIntervalMinutes *int32                   `json:"schedule_interval_minutes,omitempty"`
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

type roomPromotionRequest struct {
	Kind           string  `json:"kind"`
	EntryID        *string `json:"entry_id,omitempty"`
	CycleID        *string `json:"cycle_id,omitempty"`
	IdempotencyKey string  `json:"idempotency_key"`
	Title          string  `json:"title"`
	Rationale      string  `json:"rationale,omitempty"`
}

type roomResponse struct {
	ID                      string          `json:"id"`
	WorkspaceID             string          `json:"workspace_id"`
	Title                   string          `json:"title"`
	Instructions            string          `json:"instructions"`
	CreatedByUserID         string          `json:"created_by_user_id"`
	FacilitatorAgentID      string          `json:"facilitator_agent_id"`
	FacilitatorSquadID      *string         `json:"facilitator_squad_id"`
	Status                  string          `json:"status"`
	DailyTurnLimit          *int32          `json:"daily_turn_limit"`
	ScheduleIntervalMinutes *int32          `json:"schedule_interval_minutes"`
	NextWakeAt              *string         `json:"next_wake_at"`
	ActiveCycleID           *string         `json:"active_cycle_id"`
	Memory                  json.RawMessage `json:"memory"`
	MemoryVersion           int64           `json:"memory_version"`
	CreatedAt               string          `json:"created_at"`
	UpdatedAt               string          `json:"updated_at"`
}

type roomDetailResponse struct {
	Room         roomResponse              `json:"room"`
	Participants []roomParticipantResponse `json:"participants"`
	Entries      []roomEntryResponse       `json:"entries"`
	Cycles       []roomCycleResponse       `json:"cycles"`
	Turns        []roomTurnResponse        `json:"turns"`
	Artifacts    []roomArtifactResponse    `json:"artifacts"`
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
	ID                string  `json:"id"`
	Sequence          int64   `json:"sequence"`
	Source            string  `json:"source"`
	WakeKey           string  `json:"wake_key"`
	TriggeringEntryID *string `json:"triggering_entry_id"`
	Status            string  `json:"status"`
	RefusalReason     *string `json:"refusal_reason"`
	PlannedAt         *string `json:"planned_at"`
	CreatedAt         string  `json:"created_at"`
	StartedAt         *string `json:"started_at"`
	CompletedAt       *string `json:"completed_at"`
}

type roomTurnResponse struct {
	ID            string  `json:"id"`
	CycleID       string  `json:"cycle_id"`
	AgentID       string  `json:"agent_id"`
	SquadID       *string `json:"squad_id"`
	Status        string  `json:"status"`
	RefusalReason *string `json:"refusal_reason"`
	CreatedAt     string  `json:"created_at"`
	StartedAt     *string `json:"started_at"`
	CompletedAt   *string `json:"completed_at"`
}

type roomArtifactResponse struct {
	ID              string  `json:"id"`
	CycleID         *string `json:"cycle_id"`
	TurnID          *string `json:"turn_id"`
	EntryID         *string `json:"entry_id"`
	Kind            string  `json:"kind"`
	TargetID        *string `json:"target_id"`
	Title           string  `json:"title"`
	Body            string  `json:"body"`
	Rationale       *string `json:"rationale"`
	CreatedByUserID string  `json:"created_by_user_id"`
	CreatedAt       string  `json:"created_at"`
}

type roomWakeResponse struct {
	Cycle roomCycleResponse  `json:"cycle"`
	Turns []roomTurnResponse `json:"turns"`
	Tasks []string           `json:"tasks"`
}

type roomMessageResponse struct {
	Entry roomEntryResponse `json:"entry"`
	roomWakeResponse
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
	response := make([]roomResponse, len(rooms))
	for index, roomRow := range rooms {
		response[index] = roomToResponse(roomRow)
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
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
		Instructions: request.Instructions, FacilitatorAgentID: facilitatorAgentID,
		FacilitatorSquadID: facilitatorSquadID, Participants: participants,
		DailyTurnLimit: request.DailyTurnLimit, ScheduleIntervalMinutes: request.ScheduleIntervalMinutes,
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
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
	writeJSON(w, status, roomWakeToResponse(result))
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
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	roomRow, err := h.Rooms.SetStatus(r.Context(), workspaceID, roomID, request.Status)
	if err != nil {
		h.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roomToResponse(roomRow))
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
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
	result, err := h.Rooms.Promote(r.Context(), roomdomain.PromotionInput{
		WorkspaceID: workspaceID, RoomID: roomID, ActorUserID: actorID,
		EntryID: entryID, CycleID: cycleID, Kind: request.Kind,
		IdempotencyKey: request.IdempotencyKey, Title: request.Title, Rationale: request.Rationale,
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
		Title: roomRow.Title, Instructions: roomRow.Instructions,
		CreatedByUserID: util.UUIDToString(roomRow.CreatedByUserID), FacilitatorAgentID: util.UUIDToString(roomRow.FacilitatorAgentID),
		FacilitatorSquadID: uuidPtrString(roomRow.FacilitatorSquadID), Status: roomRow.Status,
		DailyTurnLimit: int4Ptr(roomRow.DailyTurnLimit), ScheduleIntervalMinutes: int4Ptr(roomRow.ScheduleIntervalMinutes),
		NextWakeAt: timestampPtr(roomRow.NextWakeAt), ActiveCycleID: uuidPtrString(roomRow.ActiveCycleID),
		Memory: roomRow.Memory, MemoryVersion: roomRow.MemoryVersion,
		CreatedAt: timestampToString(roomRow.CreatedAt), UpdatedAt: timestampToString(roomRow.UpdatedAt),
	}
}

func roomDetailToResponse(detail roomdomain.Detail) roomDetailResponse {
	response := roomDetailResponse{
		Room:         roomToResponse(detail.Room),
		Participants: make([]roomParticipantResponse, len(detail.Participants)),
		Entries:      make([]roomEntryResponse, len(detail.Entries)),
		Cycles:       make([]roomCycleResponse, len(detail.Cycles)),
		Turns:        make([]roomTurnResponse, len(detail.Turns)),
		Artifacts:    make([]roomArtifactResponse, len(detail.Artifacts)),
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
		response.Cycles[index] = roomCycleResponse{
			ID: util.UUIDToString(cycle.ID), Sequence: cycle.Sequence, Source: cycle.Source,
			WakeKey: cycle.WakeKey, TriggeringEntryID: uuidPtrString(cycle.TriggeringEntryID),
			Status: cycle.Status, RefusalReason: textPtr(cycle.RefusalReason), PlannedAt: timestampPtr(cycle.PlannedAt),
			CreatedAt: timestampToString(cycle.CreatedAt), StartedAt: timestampPtr(cycle.StartedAt), CompletedAt: timestampPtr(cycle.CompletedAt),
		}
	}
	for index, turn := range detail.Turns {
		response.Turns[index] = roomTurnResponse{
			ID: util.UUIDToString(turn.ID), CycleID: util.UUIDToString(turn.CycleID),
			AgentID: util.UUIDToString(turn.AgentID), SquadID: uuidPtrString(turn.SquadID),
			Status: turn.Status, RefusalReason: textPtr(turn.RefusalReason),
			CreatedAt: timestampToString(turn.CreatedAt), StartedAt: timestampPtr(turn.StartedAt), CompletedAt: timestampPtr(turn.CompletedAt),
		}
	}
	for index, artifact := range detail.Artifacts {
		response.Artifacts[index] = roomArtifactToResponse(artifact)
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
	cycle := result.Cycle
	response := roomWakeResponse{
		Cycle: roomCycleResponse{
			ID: util.UUIDToString(cycle.ID), Sequence: cycle.Sequence, Source: cycle.Source,
			WakeKey: cycle.WakeKey, TriggeringEntryID: uuidPtrString(cycle.TriggeringEntryID),
			Status: cycle.Status, RefusalReason: textPtr(cycle.RefusalReason), PlannedAt: timestampPtr(cycle.PlannedAt),
			CreatedAt: timestampToString(cycle.CreatedAt), StartedAt: timestampPtr(cycle.StartedAt), CompletedAt: timestampPtr(cycle.CompletedAt),
		},
		Turns: make([]roomTurnResponse, len(result.Turns)),
		Tasks: roomTaskIDs(result.Tasks),
	}
	for index, turn := range result.Turns {
		response.Turns[index] = roomTurnResponse{
			ID: util.UUIDToString(turn.ID), CycleID: util.UUIDToString(turn.CycleID),
			AgentID: util.UUIDToString(turn.AgentID), SquadID: uuidPtrString(turn.SquadID),
			Status: turn.Status, RefusalReason: textPtr(turn.RefusalReason),
			CreatedAt: timestampToString(turn.CreatedAt), StartedAt: timestampPtr(turn.StartedAt), CompletedAt: timestampPtr(turn.CompletedAt),
		}
	}
	return response
}

func roomArtifactToResponse(artifact db.RoomArtifact) roomArtifactResponse {
	return roomArtifactResponse{
		ID: util.UUIDToString(artifact.ID), CycleID: uuidPtrString(artifact.CycleID),
		TurnID: uuidPtrString(artifact.TurnID), EntryID: uuidPtrString(artifact.EntryID),
		Kind: artifact.Kind, TargetID: uuidPtrString(artifact.TargetID), Title: artifact.Title,
		Body: artifact.Body, Rationale: textPtr(artifact.Rationale),
		CreatedByUserID: util.UUIDToString(artifact.CreatedByUserID), CreatedAt: timestampToString(artifact.CreatedAt),
	}
}

func int4Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	return &value.Int32
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

func (h *Handler) writeRoomError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, roomdomain.ErrNotFound):
		writeError(w, http.StatusNotFound, "room not found")
	case errors.Is(err, roomdomain.ErrInvalidParticipant):
		writeError(w, http.StatusBadRequest, "invalid room participant")
	case errors.Is(err, roomdomain.ErrInvocationNotAllowed):
		h.writeDispatchBlocked(w, http.StatusForbidden, ReasonInvocationNotAllowed)
	case errors.Is(err, roomdomain.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency key conflicts with an earlier request")
	case errors.Is(err, roomdomain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid room input")
	default:
		writeError(w, http.StatusInternalServerError, "room operation failed")
	}
}
