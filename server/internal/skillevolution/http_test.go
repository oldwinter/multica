package skillevolution

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

type fakeEvolutionHTTP struct {
	overview  EvolutionOverview
	proposal  ProposalView
	configure func(LoopConfig) (db.SkillEvolutionLoop, error)
	publish   func(PublishRequest) (Publication, error)
	rollback  func(RollbackRequest) (Publication, error)
}

func (f *fakeEvolutionHTTP) Overview(context.Context, pgtype.UUID, pgtype.UUID, int) (EvolutionOverview, error) {
	return f.overview, nil
}
func (f *fakeEvolutionHTTP) ReadProposal(context.Context, pgtype.UUID, pgtype.UUID) (ProposalView, error) {
	return f.proposal, nil
}
func (f *fakeEvolutionHTTP) Configure(_ context.Context, _ DecisionActor, input LoopConfig) (db.SkillEvolutionLoop, error) {
	if f.configure != nil {
		return f.configure(input)
	}
	return db.SkillEvolutionLoop{}, nil
}
func (*fakeEvolutionHTTP) Enable(context.Context, DecisionActor, LoopConfig) (db.SkillEvolutionLoop, error) {
	return db.SkillEvolutionLoop{}, nil
}
func (*fakeEvolutionHTTP) Pause(context.Context, DecisionActor, pgtype.UUID, pgtype.UUID) (db.SkillEvolutionLoop, error) {
	return db.SkillEvolutionLoop{}, nil
}
func (*fakeEvolutionHTTP) Observe(context.Context, ObserveRequest) (Observation, error) {
	return Observation{}, nil
}
func (*fakeEvolutionHTTP) Generate(context.Context, GenerateRequest) (Generation, error) {
	panic("HTTP must not invoke generic generation")
}
func (*fakeEvolutionHTTP) CreateProposalFromRoomRecommendation(context.Context, RoomRecommendationRequest) (Generation, error) {
	return Generation{}, nil
}
func (*fakeEvolutionHTTP) Reject(context.Context, RejectRequest) (db.SkillEvolutionProposal, error) {
	return db.SkillEvolutionProposal{}, nil
}

func (f *fakeEvolutionHTTP) Publish(_ context.Context, input PublishRequest) (Publication, error) {
	if f.publish != nil {
		return f.publish(input)
	}
	return Publication{}, nil
}
func (f *fakeEvolutionHTTP) Rollback(_ context.Context, input RollbackRequest) (Publication, error) {
	if f.rollback != nil {
		return f.rollback(input)
	}
	return Publication{}, nil
}
func (*fakeEvolutionHTTP) Fork(context.Context, ForkRequest) (Fork, error) { return Fork{}, nil }

type fakeSkillLoader struct{ snapshot WorkspaceSkillSnapshot }

func (f fakeSkillLoader) Load(context.Context, pgtype.UUID, pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	return f.snapshot, nil
}

type fakeProposalRequester struct{ called bool }

func (f *fakeProposalRequester) RequestProposal(context.Context, pgtype.UUID, pgtype.UUID, pgtype.UUID, string) (ProposalRequestResult, error) {
	f.called = true
	return ProposalRequestResult{State: "improvement_room_queued", RoomID: testEvolutionUUID("room"), EligibleSignals: 2}, nil
}

func TestEvolutionHTTPOverviewWithoutLoopIsBoundedAndContentFree(t *testing.T) {
	workspaceID, skillID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("skill"), testEvolutionUUID("user")
	skill := WorkspaceSkillSnapshot{Skill: db.Skill{ID: skillID, WorkspaceID: workspaceID, Name: "review", Content: "secret primary", CreatedBy: userID},
		Ownership: Ownership{Class: OwnershipWorkspace, Reason: OwnershipReasonManual, DirectEvolution: true}, Manifest: skillbundle.Manifest{Hash: testEvolutionDigest()}}
	evolution := &fakeEvolutionHTTP{overview: EvolutionOverview{Skill: skill}}
	httpLeaf := NewHTTP(evolution, fakeSkillLoader{snapshot: skill}, &fakeProposalRequester{})

	recorder := httptest.NewRecorder()
	httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(http.MethodGet, "/skills/"+uuidString(skillID), nil, workspaceID, userID, "member"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"secret primary", "idempotency", "primary_content", "files"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("overview leaked %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"loop":null`) || !strings.Contains(body, testEvolutionDigest()) {
		t.Fatalf("missing disabled-default contract: %s", body)
	}
}

func TestEvolutionHTTPProposalDetailUsesExactNoTrailingSlashRoute(t *testing.T) {
	workspaceID, proposalID, skillID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("proposal"), testEvolutionUUID("skill"), testEvolutionUUID("user")
	baseID := testEvolutionUUID("base")
	evolution := &fakeEvolutionHTTP{proposal: ProposalView{
		Detail: ProposalDetail{Proposal: db.SkillEvolutionProposal{
			ID: proposalID, WorkspaceID: workspaceID, SkillID: skillID, BaseRevisionID: baseID,
			State: "ready", BaseHash: testEvolutionDigest(), CreatedAt: testEvolutionTime(), UpdatedAt: testEvolutionTime(),
		}},
		Base: RevisionSnapshot{Revision: db.SkillEvolutionRevision{ID: baseID, WorkspaceID: workspaceID, SkillID: skillID}},
	}}
	httpLeaf := NewHTTP(evolution, fakeSkillLoader{}, &fakeProposalRequester{})
	recorder := httptest.NewRecorder()
	httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(http.MethodGet, "/proposals/"+uuidString(proposalID), nil, workspaceID, userID, "member"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d location=%q body=%s", recorder.Code, recorder.Header().Get("Location"), recorder.Body.String())
	}
}

func TestEvolutionHTTPStrictBodyAndHumanAuthorization(t *testing.T) {
	workspaceID, skillID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("skill"), testEvolutionUUID("user")
	skill := WorkspaceSkillSnapshot{Skill: db.Skill{ID: skillID, WorkspaceID: workspaceID, Name: "review", CreatedBy: userID}}
	calls := 0
	evolution := &fakeEvolutionHTTP{configure: func(LoopConfig) (db.SkillEvolutionLoop, error) { calls++; return db.SkillEvolutionLoop{}, nil }}
	httpLeaf := NewHTTP(evolution, fakeSkillLoader{snapshot: skill}, &fakeProposalRequester{})
	body := []byte(`{"enabled":true,"mode":"observe","cooldown_seconds":3600,"minimum_signals":2,"max_evidence_refs":20,"max_replay_samples":8,"max_cost_usd_ticks":10000,"policy_version":"v1","unknown":true}`)

	request := evolutionRequest(http.MethodPut, "/skills/"+uuidString(skillID)+"/loop", body, workspaceID, userID, "member")
	recorder := httptest.NewRecorder()
	httpLeaf.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || calls != 0 {
		t.Fatalf("unknown field status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}

	validBody := []byte(`{"enabled":true,"mode":"observe","cooldown_seconds":3600,"minimum_signals":2,"max_evidence_refs":20,"max_replay_samples":8,"max_cost_usd_ticks":10000,"policy_version":"v1"}`)
	request = evolutionRequest(http.MethodPut, "/skills/"+uuidString(skillID)+"/loop", validBody, workspaceID, userID, "owner")
	request.Header.Set("X-Actor-Source", "task_token")
	recorder = httptest.NewRecorder()
	httpLeaf.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("machine status=%d calls=%d body=%s", recorder.Code, calls, recorder.Body.String())
	}

	for _, invalid := range []struct {
		body   []byte
		status int
	}{
		{append(append([]byte(nil), validBody...), []byte(` {}`)...), http.StatusBadRequest},
		{[]byte(`{"enabled":true,"mode":"observe","cooldown_seconds":3600,"minimum_signals":2,"max_evidence_refs":20,"max_replay_samples":8,"max_cost_usd_ticks":10000,"policy_version":"` + strings.Repeat("x", skillEvolutionBodyLimit) + `"}`), http.StatusRequestEntityTooLarge},
	} {
		recorder = httptest.NewRecorder()
		httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(http.MethodPut, "/skills/"+uuidString(skillID)+"/loop", invalid.body, workspaceID, userID, "owner"))
		if recorder.Code != invalid.status {
			t.Fatalf("strict body status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
}

func TestEvolutionHTTPAdminOnlyMutationMatrix(t *testing.T) {
	workspaceID, skillID, proposalID, releaseID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("skill"), testEvolutionUUID("proposal"), testEvolutionUUID("release"), testEvolutionUUID("user")
	httpLeaf := NewHTTP(&fakeEvolutionHTTP{}, fakeSkillLoader{}, &fakeProposalRequester{})
	for _, test := range []struct {
		path string
		body string
	}{
		{"/proposals/" + uuidString(proposalID) + "/publish", `{"idempotency_key":"decision-1"}`},
		{"/skills/" + uuidString(skillID) + "/releases/" + uuidString(releaseID) + "/rollback", `{"idempotency_key":"rollback-1"}`},
		{"/skills/" + uuidString(skillID) + "/fork", `{"name":"forked","idempotency_key":"fork-1"}`},
	} {
		recorder := httptest.NewRecorder()
		httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(http.MethodPost, test.path, []byte(test.body), workspaceID, userID, "member"))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestEvolutionHTTPProposalRequestUsesRoomBoundary(t *testing.T) {
	workspaceID, skillID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("skill"), testEvolutionUUID("user")
	skill := WorkspaceSkillSnapshot{Skill: db.Skill{ID: skillID, WorkspaceID: workspaceID, Name: "review", CreatedBy: userID}}
	requester := &fakeProposalRequester{}
	httpLeaf := NewHTTP(&fakeEvolutionHTTP{}, fakeSkillLoader{snapshot: skill}, requester)
	recorder := httptest.NewRecorder()
	httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(http.MethodPost, "/skills/"+uuidString(skillID)+"/proposals", []byte(`{"idempotency_key":"request-1"}`), workspaceID, userID, "member"))
	if recorder.Code != http.StatusAccepted || !requester.called {
		t.Fatalf("status=%d called=%v body=%s", recorder.Code, requester.called, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"state":"improvement_room_queued"`) || !strings.Contains(recorder.Body.String(), `"room_id":"`) ||
		strings.Contains(recorder.Body.String(), `"proposal"`) {
		t.Fatalf("unexpected proposal workflow response: %s", recorder.Body.String())
	}
	repeated := httptest.NewRecorder()
	httpLeaf.Routes().ServeHTTP(repeated, evolutionRequest(http.MethodPost, "/skills/"+uuidString(skillID)+"/proposals", []byte(`{"idempotency_key":"request-1"}`), workspaceID, userID, "member"))
	if repeated.Code != http.StatusAccepted || repeated.Body.String() != recorder.Body.String() {
		t.Fatalf("repeated status=%d body=%s, want %s", repeated.Code, repeated.Body.String(), recorder.Body.String())
	}
}

func TestEvolutionHTTPRejectsMissingIdempotencyKeyBeforeCallingBoundary(t *testing.T) {
	workspaceID, skillID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("skill"), testEvolutionUUID("user")
	skill := WorkspaceSkillSnapshot{Skill: db.Skill{ID: skillID, WorkspaceID: workspaceID, Name: "review", CreatedBy: userID}}
	requester := &fakeProposalRequester{}
	httpLeaf := NewHTTP(&fakeEvolutionHTTP{}, fakeSkillLoader{snapshot: skill}, requester)

	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/skills/" + uuidString(skillID) + "/pause", `{"idempotency_key":""}`},
		{http.MethodPost, "/skills/" + uuidString(skillID) + "/proposals", `{}`},
	} {
		recorder := httptest.NewRecorder()
		httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(test.method, test.path, []byte(test.body), workspaceID, userID, "owner"))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
	}
	if requester.called {
		t.Fatal("invalid proposal request crossed the Room boundary")
	}
}

func TestEvolutionHTTPPreservesUnknownPublicationIdentity(t *testing.T) {
	workspaceID, skillID, proposalID, releaseID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("skill"), testEvolutionUUID("proposal"), testEvolutionUUID("release"), testEvolutionUUID("user")
	baseID := testEvolutionUUID("base")
	unknown := db.SkillEvolutionRelease{
		ID: releaseID, WorkspaceID: workspaceID, SkillID: skillID, RevisionID: baseID,
		Kind: "publish", ExpectedBaseHash: testEvolutionDigest(), Outcome: "publication_unknown", CreatedAt: testEvolutionTime(),
	}
	proposal := db.SkillEvolutionProposal{
		ID: proposalID, WorkspaceID: workspaceID, SkillID: skillID, BaseRevisionID: baseID,
		State: "publishing", BaseHash: testEvolutionDigest(), CreatedAt: testEvolutionTime(), UpdatedAt: testEvolutionTime(),
	}
	evolution := &fakeEvolutionHTTP{
		publish: func(PublishRequest) (Publication, error) {
			return Publication{Proposal: proposal, Release: unknown}, ErrPublicationUnknown
		},
		rollback: func(RollbackRequest) (Publication, error) {
			value := unknown
			value.Kind = "rollback"
			return Publication{Release: value}, ErrPublicationUnknown
		},
	}
	httpLeaf := NewHTTP(evolution, fakeSkillLoader{}, &fakeProposalRequester{})
	for _, test := range []struct {
		path string
	}{
		{"/proposals/" + uuidString(proposalID) + "/publish"},
		{"/skills/" + uuidString(skillID) + "/releases/" + uuidString(releaseID) + "/rollback"},
	} {
		recorder := httptest.NewRecorder()
		httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(http.MethodPost, test.path, []byte(`{"idempotency_key":"decision-1"}`), workspaceID, userID, "owner"))
		if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), uuidString(releaseID)) ||
			!strings.Contains(recorder.Body.String(), `"outcome":"publication_unknown"`) {
			t.Fatalf("%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestEvolutionHTTPRecordsSuccessfulPublicationMetrics(t *testing.T) {
	workspaceID, skillID, proposalID, releaseID, userID := testEvolutionUUID("workspace"), testEvolutionUUID("skill"), testEvolutionUUID("proposal"), testEvolutionUUID("release"), testEvolutionUUID("user")
	release := db.SkillEvolutionRelease{ID: releaseID, WorkspaceID: workspaceID, SkillID: skillID,
		RevisionID: testEvolutionUUID("revision"), Kind: "publish", ExpectedBaseHash: testEvolutionDigest(),
		Outcome: string(ReleaseOutcomeSucceeded), CreatedAt: testEvolutionTime()}
	proposal := db.SkillEvolutionProposal{ID: proposalID, WorkspaceID: workspaceID, SkillID: skillID,
		BaseRevisionID: testEvolutionUUID("base"), State: string(ProposalStatePublished), BaseHash: testEvolutionDigest(),
		CreatedAt: testEvolutionTime(), UpdatedAt: testEvolutionTime()}
	evolution := &fakeEvolutionHTTP{
		publish: func(PublishRequest) (Publication, error) {
			return Publication{Proposal: proposal, Release: release}, nil
		},
		rollback: func(RollbackRequest) (Publication, error) {
			value := release
			value.Kind = string(ReleaseKindRollback)
			return Publication{Release: value}, nil
		},
	}
	metrics := NewMetrics()
	httpLeaf := NewHTTP(evolution, fakeSkillLoader{}, &fakeProposalRequester{}, metrics)
	for _, path := range []string{
		"/proposals/" + uuidString(proposalID) + "/publish",
		"/skills/" + uuidString(skillID) + "/releases/" + uuidString(releaseID) + "/rollback",
	} {
		recorder := httptest.NewRecorder()
		httpLeaf.Routes().ServeHTTP(recorder, evolutionRequest(http.MethodPost, path, []byte(`{"idempotency_key":"decision-1"}`), workspaceID, userID, "owner"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
	if got := testutil.ToFloat64(metrics.ProposalsAccepted); got != 1 {
		t.Fatalf("proposals accepted = %v", got)
	}
	if got := testutil.ToFloat64(metrics.Publications); got != 1 {
		t.Fatalf("publications = %v", got)
	}
	if got := testutil.ToFloat64(metrics.Rollbacks); got != 1 {
		t.Fatalf("rollbacks = %v", got)
	}
}

func evolutionRequest(method, path string, body []byte, workspaceID, userID pgtype.UUID, role string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	member := db.Member{ID: testEvolutionUUID("member"), WorkspaceID: workspaceID, UserID: userID, Role: role}
	request = request.WithContext(middleware.SetMemberContext(request.Context(), uuidString(workspaceID), member))
	return request
}

func testEvolutionUUID(seed string) pgtype.UUID {
	value := uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed))
	return pgtype.UUID{Bytes: value, Valid: true}
}

func uuidString(value pgtype.UUID) string { return uuid.UUID(value.Bytes).String() }
func testEvolutionDigest() string         { return "sha256:" + strings.Repeat("1", 64) }
func testEvolutionTime() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Unix(100, 0).UTC(), Valid: true}
}
