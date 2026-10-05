package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinExplicitBuildPersistsSchemaV2AndAcceptanceInheritsIt(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	revision, citationKey := fixture.schemaV2WikiRevision(t, "Use focused verification")
	model := &recordingTwinJSONModel{response: []byte(`{"assertions":[{"id":"verification.focused","type":"quality_bar","text":"Use focused verification.","applicability":{"issue_id":"` + fixture.workspaceID.String() + `"},"evidence_citations":["issue:` + fixture.workspaceID.String() + `"],"confidence":0.94,"provenance":{"kind":"model","generator":"ignored"}}]}`)}
	generator, err := NewModelTwinProposalGenerator(model, "production-v2")
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.ProposalGenerator = generator
	wiki := NewWikiService(fixture.queries, fixture.pool)

	if _, err := wiki.Review(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID, "accepted", ""); err != nil {
		t.Fatalf("accept schema-v2 Wiki: %v", err)
	}
	if model.calls != 0 || len(fixture.proposals(t)) != 0 {
		t.Fatalf("Wiki acceptance called model %d times and left %d proposals", model.calls, len(fixture.proposals(t)))
	}
	result, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("explicit schema-v2 Build: %v", err)
	}
	if !result.Created || result.Proposal.SchemaVersion != 2 || model.calls != 1 || !strings.Contains(string(model.request.Evidence), `"wiki_pages"`) || model.request.CitationKeys[0] != citationKey {
		t.Fatalf("schema-v2 result = %#v, model request = %#v, calls = %d", result, model.request, model.calls)
	}
	var proposalContent TwinProposalContent
	if err := json.Unmarshal(result.Proposal.Content, &proposalContent); err != nil {
		t.Fatalf("decode schema-v2 proposal content: %v", err)
	}
	if len(proposalContent.Topics) != 1 || proposalContent.Topics[0].IssueID != fixture.workspaceID.String() {
		t.Fatalf("schema-v2 proposal topics = %#v", proposalContent.Topics)
	}
	repeated, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID)
	if err != nil || repeated.Created || repeated.Proposal.ID != result.Proposal.ID || model.calls != 1 {
		t.Fatalf("natural-key replay = %#v, %v; model calls = %d", repeated, err, model.calls)
	}
	signed, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, result.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Version.SchemaVersion != 2 || signed.Version.ContentDigest != result.Proposal.ContentDigest {
		t.Fatalf("signed schema-v2 version = %#v", signed.Version)
	}
}

func TestTwinProductionServiceRejectsEvidenceWithoutFrozenPolicyBeforeModelEgress(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	revision := fixture.acceptedWiki(t, "Frozen policy is mandatory")
	content := []byte(`{"schema_version":2,"issues":[],"projects":[],"project_resources":[],"autopilot_runs":[],"wiki_pages":[]}`)
	if _, err := fixture.pool.Exec(fixture.ctx, `UPDATE lm_wiki_revision SET content = $1, source_digest = $2 WHERE id = $3`, content, digestLMWiki(content), revision.ID); err != nil {
		t.Fatal(err)
	}
	client := &recordingTwinStructuredLLM{
		enabled:  true,
		response: `{"assertions":[{"id":"compat.model","type":"procedure","text":"Keep schema-one evidence model driven.","applicability":{"issue_id":"` + fixture.workspaceID.String() + `"},"evidence_citations":["issue:` + fixture.workspaceID.String() + `"],"confidence":0.91,"provenance":{"kind":"model","generator":"ignored"}}]}`,
	}
	production := NewProductionTwinService(fixture.queries, fixture.pool, client)

	_, err := production.EnsureProposal(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID)
	if !errors.Is(err, ErrTwinGenerationDenied) || client.calls != 0 || len(fixture.proposals(t)) != 0 {
		t.Fatalf("unfrozen production error = %v, calls = %d, proposals = %d", err, client.calls, len(fixture.proposals(t)))
	}
}

func TestTwinEvidenceProviderRequiresMatchingPersistedEgressProof(t *testing.T) {
	policyDigest := "sha256:" + strings.Repeat("a", 64)
	canonicalContent := []byte(`{"schema_version":2,"egress_policy":{"remote_generation_enabled":true,"policy_version":7,"policy_digest":"` + policyDigest + `"},"issues":[],"projects":[],"project_resources":[],"autopilot_runs":[],"wiki_pages":[]}`)
	// JSONB returns object keys in database order, not Wiki's canonical struct order.
	content := []byte(`{"issues":[],"projects":[],"wiki_pages":[],"schema_version":2,"autopilot_runs":[],"egress_policy":{"policy_digest":"` + policyDigest + `","policy_version":7,"remote_generation_enabled":true},"project_resources":[]}`)
	valid := db.LmWikiRevision{
		SchemaVersion: 2, Content: content, SourceDigest: digestLMWiki(canonicalContent),
		SourcePolicyVersion: 7, SourcePolicyDigest: policyDigest, RemoteGenerationEnabled: true,
	}
	canonical, err := validateTwinAcceptedEvidenceRevision(valid)
	if err != nil || string(canonical) != string(canonicalContent) {
		t.Fatalf("valid evidence proof: %v", err)
	}

	tests := []struct {
		name   string
		change func(*db.LmWikiRevision)
	}{
		{name: "database schema", change: func(value *db.LmWikiRevision) { value.SchemaVersion = 1 }},
		{name: "policy version", change: func(value *db.LmWikiRevision) { value.SourcePolicyVersion++ }},
		{name: "policy digest", change: func(value *db.LmWikiRevision) { value.SourcePolicyDigest = "sha256:" + strings.Repeat("b", 64) }},
		{name: "remote decision", change: func(value *db.LmWikiRevision) { value.RemoteGenerationEnabled = false }},
		{name: "source digest", change: func(value *db.LmWikiRevision) { value.SourceDigest = "sha256:" + strings.Repeat("c", 64) }},
		{name: "canonical policy", change: func(value *db.LmWikiRevision) {
			value.Content = []byte(`{"schema_version":2,"issues":[],"projects":[],"project_resources":[],"autopilot_runs":[],"wiki_pages":[]}`)
			value.SourceDigest = digestLMWiki(value.Content)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			revision := valid
			revision.Content = append([]byte(nil), valid.Content...)
			tc.change(&revision)
			if _, err := validateTwinAcceptedEvidenceRevision(revision); !errors.Is(err, ErrTwinGenerationDenied) {
				t.Fatalf("validation error = %v, want %v", err, ErrTwinGenerationDenied)
			}
		})
	}
}

func TestTwinExplicitBuildRejectsEgressWithoutCallingModel(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	revision, _ := fixture.schemaV2WikiRevision(t, "Private accepted evidence")
	if _, err := fixture.queries.CreateLMWikiReview(fixture.ctx, db.CreateLMWikiReviewParams{WorkspaceID: fixture.workspaceID, RevisionID: revision.ID, Decision: "accepted", ReviewerID: fixture.actorID}); err != nil {
		t.Fatal(err)
	}
	model := &recordingTwinJSONModel{response: []byte(`{"assertions":[]}`)}
	generator, err := NewModelTwinProposalGenerator(model, "production-v2")
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.ProposalGenerator = generator

	_, err = fixture.service.ensureProposalWithEgress(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID, false)
	if !errors.Is(err, ErrTwinGenerationDenied) || model.calls != 0 || len(fixture.proposals(t)) != 0 {
		t.Fatalf("denied Build error = %v, calls = %d, proposals = %d", err, model.calls, len(fixture.proposals(t)))
	}
}

func TestTwinExplicitBuildRejectsSourceThatChangesDuringGeneration(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	stale := fixture.acceptedWiki(t, "Initially latest")
	fixture.service.ProposalGenerator = twinProposalGeneratorFunc(func(ctx context.Context, input TwinProposalGenerationInput) (TwinProposalCandidate, error) {
		fixture.acceptedWiki(t, "Accepted while model ran")
		return (InventoryTwinProposalGenerator{}).Generate(ctx, input)
	})

	_, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, stale.ID, fixture.actorID)
	if !errors.Is(err, ErrTwinWikiStale) || len(fixture.proposals(t)) != 0 {
		t.Fatalf("raced Build error = %v, proposals = %d", err, len(fixture.proposals(t)))
	}
}

func TestTwinDBEvidenceProviderKeepsNullSourceUpdatedAtEmpty(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	revision := fixture.acceptedWiki(t, "Source timestamp is optional")
	fixture.service.ProposalGenerator = twinProposalGeneratorFunc(func(ctx context.Context, input TwinProposalGenerationInput) (TwinProposalCandidate, error) {
		if len(input.BuilderInput.Citations) != 1 || input.BuilderInput.Citations[0].SourceUpdatedAt != "" {
			t.Fatalf("provider citations = %#v, want one empty source_updated_at", input.BuilderInput.Citations)
		}
		return (InventoryTwinProposalGenerator{}).Generate(ctx, input)
	})

	if _, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, revision.ID, fixture.actorID); err != nil {
		t.Fatalf("Build with NULL source_updated_at: %v", err)
	}
}

type twinProposalGeneratorFunc func(context.Context, TwinProposalGenerationInput) (TwinProposalCandidate, error)

func (f twinProposalGeneratorFunc) Generate(ctx context.Context, input TwinProposalGenerationInput) (TwinProposalCandidate, error) {
	return f(ctx, input)
}

func (f twinServiceFixture) schemaV2WikiRevision(t *testing.T, title string) (db.LmWikiRevision, string) {
	t.Helper()
	snapshot, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{
		EgressPolicy: f.egressPolicy,
		Issues:       []LMWikiIssue{{ID: f.workspaceID.String(), Number: 1, Title: title, Status: "todo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := f.queries.CreateLMWikiRevision(f.ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID: f.workspaceID, SourceDigest: snapshot.SourceDigest, Content: snapshot.CanonicalJSON,
		SourcePolicyVersion: f.egressPolicy.PolicyVersion, SourcePolicyDigest: f.egressPolicy.PolicyDigest,
		RemoteGenerationEnabled: true, TriggerKind: "manual", RequestedByID: f.actorID,
	})
	if err != nil {
		t.Fatal(err)
	}
	citations, err := marshalLMWikiCitations(snapshot.Citations)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.queries.CreateLMWikiCitations(f.ctx, db.CreateLMWikiCitationsParams{WorkspaceID: f.workspaceID, RevisionID: revision.ID, Citations: citations}); err != nil {
		t.Fatal(err)
	}
	return revision, snapshot.Citations[0].CitationKey
}
