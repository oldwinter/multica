package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinHTTPCurrentEvolutionAndVersionDetail(t *testing.T) {
	fixture := newTwinHTTPFixture(t)
	created := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
	var proposal struct {
		Proposal twinProposalResponse `json:"proposal"`
	}
	decodeTwinTestResponse(t, created, &proposal)
	accepted := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+proposal.Proposal.ID+"/accept", nil, "proposalId", proposal.Proposal.ID)
	var signed struct {
		Version twinVersionResponse `json:"version"`
	}
	decodeTwinTestResponse(t, accepted, &signed)
	alreadyCurrent := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
	versionDetail := fixture.request(t, fixture.memberID, http.MethodGet, "/api/twins/versions/"+signed.Version.ID, nil, "versionId", signed.Version.ID)
	newerWiki := createTwinHTTPRevision(t, fixture, "New issue", true)
	evolution := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(newerWiki)}, "", "")
	var evolutionBody struct {
		Proposal twinProposalResponse `json:"proposal"`
	}
	decodeTwinTestResponse(t, evolution, &evolutionBody)
	rejected := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+evolutionBody.Proposal.ID+"/reject", map[string]string{"reason": "keep current"}, "proposalId", evolutionBody.Proposal.ID)
	overview := fixture.request(t, fixture.memberID, http.MethodGet, "/api/twins", nil, "", "")

	assertTwinHTTPStatus(t, alreadyCurrent, http.StatusConflict, "twin_already_current")
	if versionDetail.Code != http.StatusOK || !strings.Contains(versionDetail.Body.String(), `"citation_key":"`+twinHTTPCitationKey+`"`) || !strings.Contains(versionDetail.Body.String(), uuidToString(fixture.revisionID)) {
		t.Fatalf("version detail = %d %s", versionDetail.Code, versionDetail.Body.String())
	}
	if evolution.Code != http.StatusCreated || evolutionBody.Proposal.Kind != "evolution" || evolutionBody.Proposal.BaseTwinVersionID == nil || *evolutionBody.Proposal.BaseTwinVersionID != signed.Version.ID {
		t.Fatalf("evolution = %d %#v", evolution.Code, evolutionBody)
	}
	assertTwinHTTPStatus(t, rejected, http.StatusOK, "")
	if !strings.Contains(overview.Body.String(), `"current_version":{"id":"`+signed.Version.ID+`"`) {
		t.Fatalf("current Twin changed after rejection: %s", overview.Body.String())
	}
}

func TestTwinHTTPConflictAndReviewValidationCodes(t *testing.T) {
	t.Run("unaccepted wiki and malformed reviews", func(t *testing.T) {
		fixture := newTwinHTTPUnacceptedFixture(t)
		unaccepted := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
		assertTwinHTTPStatus(t, unaccepted, http.StatusConflict, "wiki_revision_not_accepted")

		acceptedWiki := createTwinHTTPRevision(t, fixture, "Accepted issue", true)
		created := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(acceptedWiki)}, "", "")
		var body struct {
			Proposal twinProposalResponse `json:"proposal"`
		}
		decodeTwinTestResponse(t, created, &body)
		acceptBody := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+body.Proposal.ID+"/accept", map[string]string{"unexpected": "body"}, "proposalId", body.Proposal.ID)
		longReason := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+body.Proposal.ID+"/reject", map[string]string{"reason": strings.Repeat("x", 2001)}, "proposalId", body.Proposal.ID)
		assertTwinHTTPStatus(t, acceptBody, http.StatusBadRequest, "")
		assertTwinHTTPStatus(t, longReason, http.StatusBadRequest, "twin_review_invalid")
	})

	t.Run("stale source", func(t *testing.T) {
		fixture := newTwinHTTPFixture(t)
		created := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
		var body struct {
			Proposal twinProposalResponse `json:"proposal"`
		}
		decodeTwinTestResponse(t, created, &body)
		createTwinHTTPRevision(t, fixture, "Latest issue", true)
		stale := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+body.Proposal.ID+"/accept", nil, "proposalId", body.Proposal.ID)
		assertTwinHTTPStatus(t, stale, http.StatusConflict, "twin_wiki_stale")
	})

	t.Run("stale base", func(t *testing.T) {
		fixture := newTwinHTTPFixture(t)
		initial := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
		var initialBody struct {
			Proposal twinProposalResponse `json:"proposal"`
		}
		decodeTwinTestResponse(t, initial, &initialBody)
		fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+initialBody.Proposal.ID+"/accept", nil, "proposalId", initialBody.Proposal.ID)
		newerWiki := createTwinHTTPRevision(t, fixture, "Evolution issue", true)
		evolution := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(newerWiki)}, "", "")
		var evolutionBody struct {
			Proposal twinProposalResponse `json:"proposal"`
		}
		decodeTwinTestResponse(t, evolution, &evolutionBody)
		advanceTwinHTTPBase(t, fixture, evolutionBody.Proposal.ID)
		stale := fixture.request(t, testUserID, http.MethodPost, "/api/twins/proposals/"+evolutionBody.Proposal.ID+"/accept", nil, "proposalId", evolutionBody.Proposal.ID)
		assertTwinHTTPStatus(t, stale, http.StatusConflict, "twin_base_stale")
	})

	t.Run("admin can build", func(t *testing.T) {
		fixture := newTwinHTTPFixture(t)
		adminID := createTwinHTTPRoleMember(t, fixture.workspaceID, "admin")
		created := fixture.request(t, adminID, http.MethodPost, "/api/twins/proposals", map[string]string{"wiki_revision_id": uuidToString(fixture.revisionID)}, "", "")
		assertTwinHTTPStatus(t, created, http.StatusCreated, "")
	})
}

func advanceTwinHTTPBase(t *testing.T, fixture twinHTTPFixture, evolutionID string) {
	t.Helper()
	queries := db.New(testPool)
	proposal, err := queries.GetTwinProposal(context.Background(), db.GetTwinProposalParams{WorkspaceID: fixture.workspaceID, ID: parseUUID(evolutionID)})
	if err != nil {
		t.Fatal(err)
	}
	competing, err := queries.CreateTwinProposal(context.Background(), db.CreateTwinProposalParams{WorkspaceID: fixture.workspaceID, Kind: "initial", SourceWikiRevisionID: proposal.SourceWikiRevisionID, Content: proposal.Content, ContentDigest: proposal.ContentDigest, RequestedByID: parseUUID(testUserID)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queries.CreateTwinProposalReview(context.Background(), db.CreateTwinProposalReviewParams{WorkspaceID: fixture.workspaceID, ProposalID: competing.ID, Decision: "accepted", ReviewerID: parseUUID(testUserID)}); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.CreateTwinVersion(context.Background(), db.CreateTwinVersionParams{WorkspaceID: fixture.workspaceID, ProposalID: competing.ID, SignedOffByID: parseUUID(testUserID)}); err != nil {
		t.Fatal(err)
	}
}

func newTwinHTTPUnacceptedFixture(t *testing.T) twinHTTPFixture {
	t.Helper()
	configureTwinHTTPGenerator(t)
	ctx := context.Background()
	workspaceID := createLMWikiTestWorkspace(t, ctx, "twin-http-unaccepted")
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, workspaceID, testUserID); err != nil {
		t.Fatal(err)
	}
	fixture := twinHTTPFixture{workspaceID: workspaceID, memberID: createTwinHTTPMember(t, workspaceID)}
	fixture.egressPolicy = configureTwinHTTPEgressPolicy(t, workspaceID)
	fixture.revisionID = createTwinHTTPRevision(t, fixture, "Pending issue", false)
	return fixture
}

func configureTwinHTTPEgressPolicy(t *testing.T, workspaceID pgtype.UUID) service.LMWikiEgressPolicy {
	t.Helper()
	queries := db.New(testPool)
	state, err := service.NewWikiService(queries, testPool).UpdateSourcePolicy(
		context.Background(), workspaceID, parseUUID(testUserID),
		service.LMWikiSourcePolicy{SourceClasses: []string{"issue"}, RemoteGenerationEnabled: true},
	)
	if err != nil {
		t.Fatalf("authorize Twin HTTP evidence: %v", err)
	}
	return service.LMWikiEgressPolicy{
		RemoteGenerationEnabled: state.RemoteGenerationEnabled,
		PolicyVersion:           state.PolicyVersion, PolicyDigest: state.PolicyDigest,
	}
}

func createTwinHTTPRevision(t *testing.T, fixture twinHTTPFixture, title string, accepted bool) pgtype.UUID {
	t.Helper()
	snapshot, err := service.BuildLMWikiSnapshot(service.LMWikiSourceSnapshot{
		EgressPolicy: fixture.egressPolicy,
		Issues: []service.LMWikiIssue{{
			ID: twinHTTPSourceID, Number: 1, Title: title,
			Description: "Evidence", Status: "todo", Priority: "high",
		}},
	})
	if err != nil {
		t.Fatalf("build Twin HTTP evidence: %v", err)
	}
	queries := db.New(testPool)
	revision, err := queries.CreateLMWikiRevision(context.Background(), db.CreateLMWikiRevisionParams{
		WorkspaceID: fixture.workspaceID, SourceDigest: snapshot.SourceDigest, Content: snapshot.CanonicalJSON,
		SourcePolicyVersion: fixture.egressPolicy.PolicyVersion, SourcePolicyDigest: fixture.egressPolicy.PolicyDigest,
		RemoteGenerationEnabled: true, TriggerKind: "manual", RequestedByID: parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create Twin HTTP revision: %v", err)
	}
	if err := queries.CreateLMWikiCitations(context.Background(), db.CreateLMWikiCitationsParams{WorkspaceID: fixture.workspaceID, RevisionID: revision.ID, Citations: wikiCitationJSON(t, parseUUID(twinHTTPSourceID), twinHTTPCitationKey)}); err != nil {
		t.Fatalf("create Twin HTTP revision citation: %v", err)
	}
	if accepted {
		if _, err := service.NewWikiService(queries, testPool).Review(context.Background(), fixture.workspaceID, revision.ID, parseUUID(testUserID), "accepted", ""); err != nil {
			t.Fatalf("accept Twin HTTP revision: %v", err)
		}
	}
	return revision.ID
}

func createTwinHTTPRoleMember(t *testing.T, workspaceID pgtype.UUID, role string) string {
	t.Helper()
	var userID string
	if err := testPool.QueryRow(context.Background(), `INSERT INTO "user" (name, email) VALUES ('Twin HTTP Role', 'twin-role-' || gen_random_uuid()::text || '@example.com') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(context.Background(), `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, $3)`, workspaceID, userID, role); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })
	return userID
}
