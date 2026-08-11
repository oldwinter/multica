package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type TwinAssertionResponse struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	SourceCount int64    `json:"sourceCount"`
	SourceRefs  []string `json:"sourceRefs"`
	Reviewed    bool     `json:"reviewed"`
}

type TwinTopicResponse struct {
	ID              string `json:"id"`
	IssueIdentifier string `json:"issueIdentifier"`
	Title           string `json:"title"`
	State           string `json:"state"`
	Owner           string `json:"owner"`
	UpdatedAt       string `json:"updatedAt"`
}

type TwinReviewStepResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type TwinOverviewResponse struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	State          string                   `json:"state"`
	ReviewDigest   string                   `json:"reviewDigest"`
	UpdatedAt      string                   `json:"updatedAt"`
	SourceCount    int64                    `json:"sourceCount"`
	AssertionCount int64                    `json:"assertionCount"`
	SkillCount     int64                    `json:"skillCount"`
	RuleCount      int64                    `json:"ruleCount"`
	Assertions     []TwinAssertionResponse  `json:"assertions"`
	Topics         []TwinTopicResponse      `json:"topics"`
	ReviewSteps    []TwinReviewStepResponse `json:"reviewSteps"`
}

type TwinOverviewEnvelope struct {
	Twin *TwinOverviewResponse `json:"twin"`
}

func decodeTwinJSON[T any](raw []byte, target *T) error {
	if len(raw) == 0 {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("Twin metadata cannot be null")
	}
	return json.Unmarshal(raw, target)
}

func twinProfileToResponse(profile db.TwinProfile) (TwinOverviewResponse, error) {
	response := TwinOverviewResponse{
		ID: uuidToString(profile.ID), Name: profile.Name, State: profile.State,
		ReviewDigest: profile.ReviewDigest, UpdatedAt: timestampToString(profile.UpdatedAt),
		SourceCount: profile.SourceCount, AssertionCount: profile.AssertionCount,
		SkillCount: profile.SkillCount, RuleCount: profile.RuleCount,
		Assertions: []TwinAssertionResponse{}, Topics: []TwinTopicResponse{},
		ReviewSteps: []TwinReviewStepResponse{},
	}
	if err := decodeTwinJSON(profile.Assertions, &response.Assertions); err != nil {
		return TwinOverviewResponse{}, errors.New("invalid Twin assertions metadata")
	}
	if err := decodeTwinJSON(profile.Topics, &response.Topics); err != nil {
		return TwinOverviewResponse{}, errors.New("invalid Twin topics metadata")
	}
	if err := decodeTwinJSON(profile.ReviewSteps, &response.ReviewSteps); err != nil {
		return TwinOverviewResponse{}, errors.New("invalid Twin review path metadata")
	}
	return response, nil
}

func (h *Handler) GetTwinOverview(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	profile, err := h.Queries.GetTwinProfileByWorkspace(r.Context(), workspaceUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, TwinOverviewEnvelope{Twin: nil})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load Twin profile")
		return
	}
	response, err := twinProfileToResponse(profile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, TwinOverviewEnvelope{Twin: &response})
}
