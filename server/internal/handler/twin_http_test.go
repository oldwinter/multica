package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

const twinHTTPSourceID = "00000000-0000-0000-0000-000000000001"
const twinHTTPCitationKey = "issue:" + twinHTTPSourceID

func TestTwinHTTPProposalSignOffIdempotency(t *testing.T) {
	// Given
	fixture := newTwinHTTPFixture(t)

	// When
	empty := fixture.request(t, fixture.memberID, http.MethodGet, "/api/twins", nil, "", "")
	created := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
	var createdBody struct {
		Created  bool                 `json:"created"`
		Proposal twinProposalResponse `json:"proposal"`
	}
	decodeTwinTestResponse(t, created, &createdBody)
	repeated := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
	detail := fixture.request(t, fixture.memberID, http.MethodGet, "/api/twins/proposals/"+createdBody.Proposal.ID, nil, "proposalId", createdBody.Proposal.ID)
	accepted := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+createdBody.Proposal.ID+"/accept", nil, "proposalId", createdBody.Proposal.ID)
	var acceptedBody struct {
		Created bool                `json:"created"`
		Version twinVersionResponse `json:"version"`
	}
	decodeTwinTestResponse(t, accepted, &acceptedBody)
	repeatedAccept := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+createdBody.Proposal.ID+"/accept", nil, "proposalId", createdBody.Proposal.ID)
	overview := fixture.request(t, fixture.memberID, http.MethodGet, "/api/twins", nil, "", "")

	// Then
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"current_version":null`) || !strings.Contains(empty.Body.String(), `"pending_proposal":null`) {
		t.Fatalf("empty overview = %d %s", empty.Code, empty.Body.String())
	}
	if created.Code != http.StatusCreated || !createdBody.Created || createdBody.Proposal.Kind != "initial" {
		t.Fatalf("created proposal = %d %#v", created.Code, createdBody)
	}
	if repeated.Code != http.StatusOK || !strings.Contains(repeated.Body.String(), `"created":false`) || !strings.Contains(repeated.Body.String(), createdBody.Proposal.ID) {
		t.Fatalf("repeated proposal = %d %s", repeated.Code, repeated.Body.String())
	}
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"citation_key":"`+twinHTTPCitationKey+`"`) {
		t.Fatalf("proposal detail = %d %s", detail.Code, detail.Body.String())
	}
	if accepted.Code != http.StatusCreated || !acceptedBody.Created || acceptedBody.Version.VersionNumber != 1 {
		t.Fatalf("accepted proposal = %d %#v", accepted.Code, acceptedBody)
	}
	if repeatedAccept.Code != http.StatusOK || !strings.Contains(repeatedAccept.Body.String(), acceptedBody.Version.ID) || !strings.Contains(repeatedAccept.Body.String(), `"created":false`) {
		t.Fatalf("repeated acceptance = %d %s", repeatedAccept.Code, repeatedAccept.Body.String())
	}
	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), acceptedBody.Version.ID) || !strings.Contains(overview.Body.String(), `"can_manage":false`) {
		t.Fatalf("signed overview = %d %s", overview.Code, overview.Body.String())
	}
}

func TestTwinAuthorizationAndValidation(t *testing.T) {
	// Given
	fixture := newTwinHTTPFixture(t)
	created := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
	var body struct {
		Proposal twinProposalResponse `json:"proposal"`
	}
	decodeTwinTestResponse(t, created, &body)
	control := newTwinHTTPFixture(t)

	// When
	memberWrite := fixture.request(t, fixture.memberID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
	foreign := control.request(t, testUserID, http.MethodGet, "/api/twins/proposals/"+body.Proposal.ID, nil, "proposalId", body.Proposal.ID)
	malformed := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": "bad"}, "", "")
	unknown := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID), "extra": "bad"}, "", "")
	unauthenticated := fixture.request(t, "", http.MethodGet, "/api/twins", nil, "", "")
	rejected := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+body.Proposal.ID+"/reject", map[string]string{"reason": "not ready"}, "proposalId", body.Proposal.ID)
	repeatedReject := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+body.Proposal.ID+"/reject", map[string]string{"reason": "not ready"}, "proposalId", body.Proposal.ID)
	opposite := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+body.Proposal.ID+"/accept", nil, "proposalId", body.Proposal.ID)

	// Then
	assertTwinHTTPStatus(t, memberWrite, http.StatusForbidden, "")
	assertTwinHTTPStatus(t, foreign, http.StatusNotFound, "twin_not_found")
	assertTwinHTTPStatus(t, malformed, http.StatusBadRequest, "")
	assertTwinHTTPStatus(t, unknown, http.StatusBadRequest, "")
	assertTwinHTTPStatus(t, unauthenticated, http.StatusUnauthorized, "")
	assertTwinHTTPStatus(t, rejected, http.StatusOK, "")
	assertTwinHTTPStatus(t, repeatedReject, http.StatusOK, "")
	assertTwinHTTPStatus(t, opposite, http.StatusConflict, "twin_proposal_already_decided")
}

func TestTwinHTTPCorrectionMustReplaceTheReviewHead(t *testing.T) {
	fixture := newTwinHTTPFixture(t)
	created := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
	var createdBody struct {
		Proposal twinProposalResponse `json:"proposal"`
	}
	decodeTwinTestResponse(t, created, &createdBody)
	var content service.TwinProposalContent
	if err := json.Unmarshal(createdBody.Proposal.Content, &content); err != nil || len(content.Assertions) == 0 {
		t.Fatalf("decode editable proposal content = %#v, err = %v", content, err)
	}
	content.Assertions[0].Text += " Record focused evidence."
	request := map[string]any{"edited_assertions": content.Assertions}

	memberWrite := fixture.request(t, fixture.memberID, http.MethodPost, "/api/twins/proposals/"+createdBody.Proposal.ID+"/correct", request, "proposalId", createdBody.Proposal.ID)
	corrected := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+createdBody.Proposal.ID+"/correct", request, "proposalId", createdBody.Proposal.ID)
	var correctedBody struct {
		Created  bool                 `json:"created"`
		Proposal twinProposalResponse `json:"proposal"`
	}
	decodeTwinTestResponse(t, corrected, &correctedBody)
	replayed := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+createdBody.Proposal.ID+"/correct", request, "proposalId", createdBody.Proposal.ID)
	predecessor := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+createdBody.Proposal.ID+"/accept", nil, "proposalId", createdBody.Proposal.ID)
	signed := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+correctedBody.Proposal.ID+"/accept", nil, "proposalId", correctedBody.Proposal.ID)

	assertTwinHTTPStatus(t, memberWrite, http.StatusForbidden, "")
	if corrected.Code != http.StatusCreated || !correctedBody.Created || correctedBody.Proposal.Kind != "correction" || correctedBody.Proposal.ReplacesProposalID == nil || *correctedBody.Proposal.ReplacesProposalID != createdBody.Proposal.ID {
		t.Fatalf("corrected proposal = %d %#v", corrected.Code, correctedBody)
	}
	if replayed.Code != http.StatusOK || !strings.Contains(replayed.Body.String(), `"created":false`) || !strings.Contains(replayed.Body.String(), correctedBody.Proposal.ID) {
		t.Fatalf("replayed correction = %d %s", replayed.Code, replayed.Body.String())
	}
	assertTwinHTTPStatus(t, predecessor, http.StatusConflict, "twin_proposal_superseded")
	assertTwinHTTPStatus(t, signed, http.StatusCreated, "")
}

type twinHTTPFixture struct {
	workspaceID  pgtype.UUID
	revisionID   pgtype.UUID
	memberID     string
	egressPolicy service.LMWikiEgressPolicy
}

func newTwinHTTPFixture(t *testing.T) twinHTTPFixture {
	t.Helper()
	configureTwinHTTPGenerator(t)
	ctx := context.Background()
	workspaceID := createLMWikiTestWorkspace(t, ctx, "twin-http")
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, workspaceID, testUserID); err != nil {
		t.Fatalf("create Twin owner: %v", err)
	}
	memberID := createTwinHTTPMember(t, workspaceID)
	fixture := twinHTTPFixture{workspaceID: workspaceID, memberID: memberID}
	fixture.egressPolicy = configureTwinHTTPEgressPolicy(t, workspaceID)
	fixture.revisionID = createTwinHTTPRevision(t, fixture, "Test issue", true)
	return fixture
}

type twinHTTPProposalGenerator struct{}

func (twinHTTPProposalGenerator) Generate(_ context.Context, input service.TwinProposalGenerationInput) (service.TwinProposalCandidate, error) {
	assertions := make([]service.TwinAssertion, 0, len(input.BuilderInput.Citations))
	for _, citation := range input.BuilderInput.Citations {
		assertions = append(assertions, service.TwinAssertion{
			ID: "fixture:" + citation.CitationKey, Type: service.TwinAssertionProcedure, Text: citation.Label,
			Applicability:     service.TwinAssertionApplicability{IssueID: citation.SourceID},
			EvidenceCitations: []string{citation.CitationKey}, Confidence: 0.9,
			Provenance: service.TwinAssertionProvenance{Kind: service.TwinProvenanceModel, Generator: "http-fixture-model"},
		})
	}
	return service.TwinProposalCandidate{Assertions: assertions}, nil
}

func configureTwinHTTPGenerator(t *testing.T) {
	t.Helper()
	previous := testHandler.TwinService.ProposalGenerator
	testHandler.TwinService.ProposalGenerator = twinHTTPProposalGenerator{}
	t.Cleanup(func() { testHandler.TwinService.ProposalGenerator = previous })
}

func createTwinHTTPMember(t *testing.T, workspaceID pgtype.UUID) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	if err := testPool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Twin HTTP Member', 'twin-http-' || gen_random_uuid()::text || '@example.com') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("create Twin member user: %v", err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, workspaceID, userID); err != nil {
		t.Fatalf("create Twin member: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })
	return userID
}

func (f twinHTTPFixture) request(t *testing.T, userID, method, path string, body any, param, value string) *httptest.ResponseRecorder {
	t.Helper()
	request := newRequestAs(userID, method, path, body)
	request.Header.Set("X-Workspace-ID", uuidToString(f.workspaceID))
	if param != "" {
		request = withURLParam(request, param, value)
	}
	recorder := httptest.NewRecorder()
	switch {
	case method == http.MethodGet && strings.Contains(path, "/versions/"):
		testHandler.GetTwinVersion(recorder, request)
	case method == http.MethodGet && strings.Contains(path, "/proposals/"):
		testHandler.GetTwinProposal(recorder, request)
	case method == http.MethodGet:
		testHandler.GetTwins(recorder, request)
	case strings.HasSuffix(path, "/accept"):
		testHandler.AcceptTwinProposal(recorder, request)
	case strings.HasSuffix(path, "/reject"):
		testHandler.RejectTwinProposal(recorder, request)
	case strings.HasSuffix(path, "/correct"):
		testHandler.CorrectTwinProposal(recorder, request)
	default:
		testHandler.CreateTwinProposal(recorder, request)
	}
	return recorder
}

func decodeTwinTestResponse(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(recorder.Body).Decode(target); err != nil {
		t.Fatalf("decode Twin response: %v: %s", err, recorder.Body.String())
	}
}

func assertTwinHTTPStatus(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status || (code != "" && !strings.Contains(recorder.Body.String(), `"code":"`+code+`"`)) {
		t.Fatalf("Twin response = %d %s, want %d code %q", recorder.Code, recorder.Body.String(), status, code)
	}
}
