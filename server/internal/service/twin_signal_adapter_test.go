package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinFeedbackSignalAdapterListsContentFreeRefsAndRevalidatesMutableContent(t *testing.T) {
	fixture := newTwinSignalQueryFixture(t)
	authorized := 0
	adapter := newTwinSignalAdapter(fixture, TwinSignalAuthorizerFunc(func(_ context.Context, workspaceID, taskID pgtype.UUID) error {
		authorized++
		if workspaceID != fixture.workspaceID || taskID != fixture.task.ID {
			t.Fatal("authorizer received the wrong Twin signal scope")
		}
		return nil
	}))

	refs, err := adapter.ListFeedbackRefs(context.Background(), fixture.workspaceID, fixture.task.ID, 1)
	if err != nil || len(refs) != 1 {
		t.Fatalf("list Twin feedback refs = %#v, err = %v", refs, err)
	}
	assertTwinSignalRefContentFree(t, reflect.TypeOf(refs[0]))
	if refs[0].FeedbackID != fixture.feedback.ID || refs[0].AttributionID != fixture.attribution.ID || refs[0].State != "helped" {
		t.Fatalf("Twin feedback ref = %#v", refs[0])
	}
	evidence, err := adapter.LoadFeedbackEvidence(context.Background(), fixture.workspaceID, refs[0])
	if err != nil || evidence.Note == nil || *evidence.Note != fixture.feedback.Note.String || evidence.Rating != "helped" {
		t.Fatalf("load Twin feedback evidence = %#v, err = %v", evidence, err)
	}
	if authorized != 2 {
		t.Fatalf("Twin signal authorization calls = %d, want 2", authorized)
	}

	tests := []struct {
		name   string
		mutate func(*twinSignalQueryFixture)
	}{
		{name: "rating", mutate: func(f *twinSignalQueryFixture) { f.feedback.Rating = "mismatch" }},
		{name: "note", mutate: func(f *twinSignalQueryFixture) { f.feedback.Note.String = "Updated correction" }},
		{name: "updated at", mutate: func(f *twinSignalQueryFixture) {
			f.feedback.UpdatedAt.Time = f.feedback.UpdatedAt.Time.Add(time.Second)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := fixture.clone()
			test.mutate(changed)
			changedAdapter := newTwinSignalAdapter(changed, allowTwinSignalRead)
			_, err := changedAdapter.LoadFeedbackEvidence(context.Background(), changed.workspaceID, refs[0])
			if !errors.Is(err, ErrTwinSignalStale) {
				t.Fatalf("changed Twin feedback load error = %v, want ErrTwinSignalStale", err)
			}
		})
	}
}

func TestTwinFeedbackSignalAdapterFailsClosedForAuthorizationAndAttribution(t *testing.T) {
	fixture := newTwinSignalQueryFixture(t)
	denied := errors.New("private task denied")
	adapter := newTwinSignalAdapter(fixture, TwinSignalAuthorizerFunc(func(context.Context, pgtype.UUID, pgtype.UUID) error { return denied }))
	if _, err := adapter.ListFeedbackRefs(context.Background(), fixture.workspaceID, fixture.task.ID, 1); !errors.Is(err, denied) {
		t.Fatalf("unauthorized Twin feedback list error = %v", err)
	}
	if fixture.feedbackReads != 0 {
		t.Fatalf("unauthorized Twin feedback read content %d times", fixture.feedbackReads)
	}

	tests := []struct {
		name   string
		mutate func(*twinSignalQueryFixture)
	}{
		{name: "task incomplete", mutate: func(f *twinSignalQueryFixture) {
			f.task.Status = "running"
			f.task.CompletedAt = pgtype.Timestamptz{}
		}},
		{name: "duplicate attribution", mutate: func(f *twinSignalQueryFixture) {
			f.attributions = append(f.attributions, f.attribution)
		}},
		{name: "runtime mismatch", mutate: func(f *twinSignalQueryFixture) {
			f.attribution.RuntimeID = twinSignalUUID(99)
			f.attributions[0] = f.attribution
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := fixture.clone()
			test.mutate(changed)
			refs, err := newTwinSignalAdapter(changed, allowTwinSignalRead).ListFeedbackRefs(context.Background(), changed.workspaceID, changed.task.ID, 1)
			if err != nil || len(refs) != 0 {
				t.Fatalf("ineligible Twin feedback refs = %#v, err = %v", refs, err)
			}
		})
	}
}

func TestTwinAcceptedDepositionSignalAdapterBindsAcceptedHeadAndLoadsAssertionsOnlyAfterAuthorization(t *testing.T) {
	fixture := newTwinSignalQueryFixture(t)
	adapter := newTwinSignalAdapter(fixture, allowTwinSignalRead)
	refs, err := adapter.ListAcceptedDepositionRefs(context.Background(), fixture.workspaceID, fixture.task.ID, 1)
	if err != nil || len(refs) != 1 {
		t.Fatalf("list accepted Twin deposition refs = %#v, err = %v", refs, err)
	}
	assertTwinSignalRefContentFree(t, reflect.TypeOf(refs[0]))
	ref := refs[0]
	if ref.DepositionID != fixture.deposition.ID || ref.ProposalID != fixture.proposal.ID ||
		ref.AcceptedVersionID != fixture.acceptedVersion.ID || ref.SourceVersionID != fixture.baseVersion.ID {
		t.Fatalf("accepted Twin deposition ref = %#v", ref)
	}
	evidence, err := adapter.LoadAcceptedDepositionEvidence(context.Background(), fixture.workspaceID, ref)
	if err != nil || string(evidence.ProposalContent) != string(fixture.proposal.Content) {
		t.Fatalf("load accepted Twin deposition evidence = %#v, err = %v", evidence, err)
	}
	if strings.Contains(string(evidence.ProposalContent), "wiki body") {
		t.Fatal("accepted Twin deposition evidence leaked Wiki source content")
	}
	fixture.proposalReads = 0
	denied := errors.New("private task denied")
	deniedAdapter := newTwinSignalAdapter(fixture, TwinSignalAuthorizerFunc(func(context.Context, pgtype.UUID, pgtype.UUID) error { return denied }))
	if _, err := deniedAdapter.LoadAcceptedDepositionEvidence(context.Background(), fixture.workspaceID, ref); !errors.Is(err, denied) {
		t.Fatalf("unauthorized accepted Twin deposition load error = %v", err)
	}
	if fixture.proposalReads != 0 {
		t.Fatalf("unauthorized accepted Twin deposition read proposal content %d times", fixture.proposalReads)
	}

	changed := fixture.clone()
	changed.proposal.Content = []byte(`{"schema_version":2,"assertions":[{"id":"changed"}]}`)
	changed.acceptedVersion.Content = append([]byte(nil), changed.proposal.Content...)
	changed.proposal.ContentDigest = mustCanonicalTwinSignalContentDigest(t, changed.proposal.Content)
	changed.acceptedVersion.ContentDigest = changed.proposal.ContentDigest
	changed.currentVersion = changed.acceptedVersion
	if _, err := newTwinSignalAdapter(changed, allowTwinSignalRead).LoadAcceptedDepositionEvidence(context.Background(), changed.workspaceID, ref); !errors.Is(err, ErrTwinSignalStale) {
		t.Fatalf("changed accepted Twin deposition load error = %v, want ErrTwinSignalStale", err)
	}
}

func TestTwinAcceptedDepositionSignalAdapterRejectsStaleOrInvalidLifecycle(t *testing.T) {
	fixture := newTwinSignalQueryFixture(t)
	tests := []struct {
		name   string
		mutate func(*twinSignalQueryFixture)
	}{
		{name: "not current accepted head", mutate: func(f *twinSignalQueryFixture) {
			f.currentVersion.ID = twinSignalUUID(80)
		}},
		{name: "replacement exists", mutate: func(f *twinSignalQueryFixture) {
			replacement := f.deposition
			replacement.ID = twinSignalUUID(81)
			replacement.ProposalID = twinSignalUUID(82)
			replacement.ReplacesProposalID = f.proposal.ID
			replacement.State = "pending"
			f.depositions = append([]db.TwinDeposition{replacement}, f.depositions...)
		}},
		{name: "reviewer no longer manager", mutate: func(f *twinSignalQueryFixture) { f.member.Role = "member" }},
		{name: "source version drift", mutate: func(f *twinSignalQueryFixture) {
			f.proposal.SourceWikiRevisionID = twinSignalUUID(83)
		}},
		{name: "deposition evidence drift", mutate: func(f *twinSignalQueryFixture) {
			f.feedback.Rating = "mismatch"
		}},
		{name: "task incomplete", mutate: func(f *twinSignalQueryFixture) {
			f.task.Status = "failed"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := fixture.clone()
			test.mutate(changed)
			refs, err := newTwinSignalAdapter(changed, allowTwinSignalRead).ListAcceptedDepositionRefs(context.Background(), changed.workspaceID, changed.task.ID, 1)
			if err != nil || len(refs) != 0 {
				t.Fatalf("ineligible accepted Twin deposition refs = %#v, err = %v", refs, err)
			}
		})
	}
}

func TestTwinSignalListLimitsAreClosedAndBounded(t *testing.T) {
	fixture := newTwinSignalQueryFixture(t)
	adapter := newTwinSignalAdapter(fixture, allowTwinSignalRead)
	for _, limit := range []int{-1, MaxTwinSignalRefs + 1} {
		if _, err := adapter.ListFeedbackRefs(context.Background(), fixture.workspaceID, fixture.task.ID, limit); !errors.Is(err, ErrTwinSignalInvalidInput) {
			t.Fatalf("feedback limit %d error = %v", limit, err)
		}
		if _, err := adapter.ListAcceptedDepositionRefs(context.Background(), fixture.workspaceID, fixture.task.ID, limit); !errors.Is(err, ErrTwinSignalInvalidInput) {
			t.Fatalf("deposition limit %d error = %v", limit, err)
		}
	}
	refs, err := adapter.ListAcceptedDepositionRefs(context.Background(), fixture.workspaceID, fixture.task.ID, 0)
	if err != nil || refs == nil || len(refs) != 0 {
		t.Fatalf("zero-limit deposition refs = %#v, err = %v", refs, err)
	}
}

var allowTwinSignalRead = TwinSignalAuthorizerFunc(func(context.Context, pgtype.UUID, pgtype.UUID) error { return nil })

type twinSignalQueryFixture struct {
	workspaceID     pgtype.UUID
	task            db.AgentTaskQueue
	attribution     db.TwinTaskAttribution
	attributions    []db.TwinTaskAttribution
	feedback        db.TwinRunFeedback
	deposition      db.TwinDeposition
	depositions     []db.TwinDeposition
	proposal        db.TwinProposal
	review          db.TwinProposalReview
	baseVersion     db.TwinVersion
	acceptedVersion db.TwinVersion
	currentVersion  db.TwinVersion
	member          db.Member
	feedbackReads   int
	proposalReads   int
}

func newTwinSignalQueryFixture(t *testing.T) *twinSignalQueryFixture {
	t.Helper()
	workspaceID := twinSignalUUID(1)
	taskID := twinSignalUUID(2)
	attributionID := twinSignalUUID(3)
	baseVersionID := twinSignalUUID(4)
	proposalID := twinSignalUUID(5)
	acceptedVersionID := twinSignalUUID(6)
	reviewerID := twinSignalUUID(7)
	dispatchedAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	completedAt := dispatchedAt.Add(time.Minute)
	updatedAt := completedAt.Add(time.Second)
	task := db.AgentTaskQueue{
		ID: taskID, AgentID: twinSignalUUID(8), RuntimeID: twinSignalUUID(9), Status: "completed",
		DispatchedAt: twinSignalTime(dispatchedAt), CompletedAt: twinSignalTime(completedAt),
		TwinUseState: pgtype.Text{String: string(TwinUseEnabled), Valid: true}, TwinVersionID: baseVersionID,
	}
	attribution := db.TwinTaskAttribution{
		ID: attributionID, WorkspaceID: workspaceID, TaskID: taskID, AgentID: task.AgentID, RuntimeID: task.RuntimeID,
		TaskDispatchedAt: task.DispatchedAt, TwinVersionID: baseVersionID,
		Briefing: "Use the accepted Twin assertion.", AssertionIds: []byte(`["assertion.one"]`),
		CitationKeys: []byte(`["issue:one"]`), PolicyScopeType: "workspace", PolicyScopeID: workspaceID,
		PolicyState: string(TwinUseEnabled), CompilerVersion: "test-v1", CreatedAt: twinSignalTime(dispatchedAt),
	}
	attribution.BriefingDigest = TwinBriefingDigest(attribution.Briefing)
	feedback := db.TwinRunFeedback{
		ID: twinSignalUUID(10), WorkspaceID: workspaceID, TaskID: taskID, Rating: "helped",
		Note:      pgtype.Text{String: "Keep the verification focused.", Valid: true},
		CreatedAt: twinSignalTime(completedAt), UpdatedAt: twinSignalTime(updatedAt),
	}
	baseSourceRevisionID := twinSignalUUID(11)
	base := db.TwinVersion{
		ID: baseVersionID, WorkspaceID: workspaceID, VersionNumber: 1, ProposalID: twinSignalUUID(12),
		SchemaVersion:        2,
		SourceWikiRevisionID: baseSourceRevisionID, Content: []byte(`{"schema_version":2,"assertions":[{"id":"assertion.one"}]}`),
		SignedOffAt: twinSignalTime(dispatchedAt.Add(-time.Hour)), CreatedAt: twinSignalTime(dispatchedAt.Add(-time.Hour)),
	}
	base.ContentDigest = mustCanonicalTwinSignalContentDigest(t, base.Content)
	proposalContent := []byte(`{"schema_version":2,"assertions":[{"id":"assertion.one","text":"Keep verification focused."}]}`)
	proposalDigest := mustCanonicalTwinSignalContentDigest(t, proposalContent)
	proposal := db.TwinProposal{
		ID: proposalID, WorkspaceID: workspaceID, Kind: "deposition", SourceWikiRevisionID: baseSourceRevisionID,
		BaseTwinVersionID: baseVersionID, SchemaVersion: 2, Content: proposalContent, ContentDigest: proposalDigest,
		RequestedByID: reviewerID, CreatedAt: twinSignalTime(updatedAt),
	}
	review := db.TwinProposalReview{
		ID: twinSignalUUID(13), WorkspaceID: workspaceID, ProposalID: proposalID, Decision: "accepted",
		ReviewerID: reviewerID, CreatedAt: twinSignalTime(updatedAt.Add(time.Second)),
	}
	accepted := db.TwinVersion{
		ID: acceptedVersionID, WorkspaceID: workspaceID, VersionNumber: 2, ProposalID: proposalID,
		SchemaVersion:        2,
		SourceWikiRevisionID: baseSourceRevisionID, PriorVersionID: baseVersionID, Content: append([]byte(nil), proposalContent...),
		ContentDigest: proposalDigest, SignedOffByID: reviewerID,
		SignedOffAt: twinSignalTime(updatedAt.Add(2 * time.Second)), CreatedAt: twinSignalTime(updatedAt.Add(2 * time.Second)),
	}
	deposition := db.TwinDeposition{
		ID: twinSignalUUID(14), WorkspaceID: workspaceID, TaskID: taskID, BaseTwinVersionID: baseVersionID,
		ProposalID: proposalID, State: "accepted", CreatedAt: twinSignalTime(updatedAt), UpdatedAt: twinSignalTime(updatedAt.Add(2 * time.Second)),
	}
	depositionDigest, err := twinDepositionEvidenceDigest(twinDepositionEvidence{
		TaskID: taskID.String(), AttributionID: attributionID.String(), TwinVersionID: baseVersionID.String(),
		BriefingDigest: attribution.BriefingDigest, AssertionIDs: []string{"assertion.one"}, CitationKeys: []string{"issue:one"},
		PolicyScopeType: "workspace", PolicyScopeID: workspaceID.String(), FeedbackRating: feedback.Rating,
		CompletedAt: completedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	deposition.EvidenceDigest = depositionDigest
	return &twinSignalQueryFixture{
		workspaceID: workspaceID, task: task, attribution: attribution, attributions: []db.TwinTaskAttribution{attribution},
		feedback: feedback, deposition: deposition, depositions: []db.TwinDeposition{deposition},
		proposal: proposal, review: review, baseVersion: base, acceptedVersion: accepted, currentVersion: accepted,
		member: db.Member{ID: twinSignalUUID(15), WorkspaceID: workspaceID, UserID: reviewerID, Role: "owner", CreatedAt: twinSignalTime(dispatchedAt.Add(-time.Hour))},
	}
}

func (f *twinSignalQueryFixture) clone() *twinSignalQueryFixture {
	clone := *f
	clone.task = f.task
	clone.attribution = f.attribution
	clone.attributions = append([]db.TwinTaskAttribution(nil), f.attributions...)
	clone.feedback = f.feedback
	clone.deposition = f.deposition
	clone.depositions = append([]db.TwinDeposition(nil), f.depositions...)
	clone.proposal = f.proposal
	clone.proposal.Content = append([]byte(nil), f.proposal.Content...)
	clone.baseVersion = f.baseVersion
	clone.baseVersion.Content = append([]byte(nil), f.baseVersion.Content...)
	clone.acceptedVersion = f.acceptedVersion
	clone.acceptedVersion.Content = append([]byte(nil), f.acceptedVersion.Content...)
	clone.currentVersion = clone.acceptedVersion
	clone.member = f.member
	clone.feedbackReads = 0
	clone.proposalReads = 0
	return &clone
}

func (f *twinSignalQueryFixture) GetAgentTaskInWorkspace(_ context.Context, params db.GetAgentTaskInWorkspaceParams) (db.AgentTaskQueue, error) {
	if params.WorkspaceID != f.workspaceID || params.ID != f.task.ID {
		return db.AgentTaskQueue{}, pgx.ErrNoRows
	}
	return f.task, nil
}

func (f *twinSignalQueryFixture) ListTwinTaskAttributions(_ context.Context, params db.ListTwinTaskAttributionsParams) ([]db.TwinTaskAttribution, error) {
	if params.WorkspaceID != f.workspaceID || params.TaskID != f.task.ID {
		return nil, nil
	}
	return append([]db.TwinTaskAttribution(nil), f.attributions...), nil
}

func (f *twinSignalQueryFixture) GetTwinRunFeedback(_ context.Context, params db.GetTwinRunFeedbackParams) (db.TwinRunFeedback, error) {
	f.feedbackReads++
	if params.WorkspaceID != f.workspaceID || params.TaskID != f.feedback.TaskID {
		return db.TwinRunFeedback{}, pgx.ErrNoRows
	}
	return f.feedback, nil
}

func (f *twinSignalQueryFixture) ListTwinDepositionsForTask(_ context.Context, params db.ListTwinDepositionsForTaskParams) ([]db.TwinDeposition, error) {
	if params.WorkspaceID != f.workspaceID || params.TaskID != f.task.ID {
		return nil, nil
	}
	return append([]db.TwinDeposition(nil), f.depositions...), nil
}

func (f *twinSignalQueryFixture) GetTwinDeposition(_ context.Context, params db.GetTwinDepositionParams) (db.TwinDeposition, error) {
	for _, deposition := range f.depositions {
		if params.WorkspaceID == f.workspaceID && params.ID == deposition.ID {
			return deposition, nil
		}
	}
	return db.TwinDeposition{}, pgx.ErrNoRows
}

func (f *twinSignalQueryFixture) GetTwinProposal(_ context.Context, params db.GetTwinProposalParams) (db.TwinProposal, error) {
	f.proposalReads++
	if params.WorkspaceID != f.workspaceID || params.ID != f.proposal.ID {
		return db.TwinProposal{}, pgx.ErrNoRows
	}
	return f.proposal, nil
}

func (f *twinSignalQueryFixture) GetTwinProposalReview(_ context.Context, params db.GetTwinProposalReviewParams) (db.TwinProposalReview, error) {
	if params.WorkspaceID != f.workspaceID || params.ProposalID != f.review.ProposalID {
		return db.TwinProposalReview{}, pgx.ErrNoRows
	}
	return f.review, nil
}

func (f *twinSignalQueryFixture) GetTwinVersionByProposal(_ context.Context, params db.GetTwinVersionByProposalParams) (db.TwinVersion, error) {
	if params.WorkspaceID != f.workspaceID || params.ProposalID != f.acceptedVersion.ProposalID {
		return db.TwinVersion{}, pgx.ErrNoRows
	}
	return f.acceptedVersion, nil
}

func (f *twinSignalQueryFixture) GetTwinVersion(_ context.Context, params db.GetTwinVersionParams) (db.TwinVersion, error) {
	if params.WorkspaceID != f.workspaceID || params.ID != f.baseVersion.ID {
		return db.TwinVersion{}, pgx.ErrNoRows
	}
	return f.baseVersion, nil
}

func (f *twinSignalQueryFixture) GetCurrentTwinVersion(_ context.Context, workspaceID pgtype.UUID) (db.TwinVersion, error) {
	if workspaceID != f.workspaceID {
		return db.TwinVersion{}, pgx.ErrNoRows
	}
	return f.currentVersion, nil
}

func (f *twinSignalQueryFixture) GetMemberByUserAndWorkspace(_ context.Context, params db.GetMemberByUserAndWorkspaceParams) (db.Member, error) {
	if params.WorkspaceID != f.workspaceID || params.UserID != f.member.UserID {
		return db.Member{}, pgx.ErrNoRows
	}
	return f.member, nil
}

func assertTwinSignalRefContentFree(t *testing.T, typ reflect.Type) {
	t.Helper()
	for index := range typ.NumField() {
		name := strings.ToLower(typ.Field(index).Name)
		for _, forbidden := range []string{"note", "content", "assertion", "citation", "wiki", "prompt", "result", "output", "path"} {
			if strings.Contains(name, forbidden) {
				t.Fatalf("Twin signal ref field %q may carry source content", typ.Field(index).Name)
			}
		}
	}
}

func twinSignalUUID(last byte) pgtype.UUID {
	var bytes [16]byte
	bytes[6] = 0x40
	bytes[8] = 0x80
	bytes[15] = last
	return pgtype.UUID{Bytes: bytes, Valid: true}
}

func twinSignalTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func mustCanonicalTwinSignalContentDigest(t *testing.T, content []byte) string {
	t.Helper()
	digest, ok := canonicalTwinSignalContentDigest(content)
	if !ok {
		t.Fatal("test Twin content is not canonicalizable")
	}
	return digest
}

var _ twinSignalQueries = (*twinSignalQueryFixture)(nil)
