package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"strings"
	"testing"
)

func TestModelTwinProposalGenerator_buildsValidatedSchemaV2Proposal(t *testing.T) {
	input := twinGeneratorTestInput(t)
	input.CanonicalEvidence = json.RawMessage(`{"schema_version":2,"source_digest":"` + input.SourceDigest + `","egress_policy":` + twinAuthorizedEgressPolicyJSON() + `,"items":[{"citation_key":"issue:issue-1"},{"citation_key":"project:project-1"}]}`)
	input.EvidenceSchemaVersion = 2
	model := &recordingTwinJSONModel{response: []byte(`{"assertions":[{"id":" Review.First ","type":"QUALITY_BAR","text":"  Review   every change. ","applicability":{"project_id":" PROJECT-1 ","keywords":[" Review ","review"]},"evidence_citations":["project:project-1","issue:issue-1","issue:issue-1"],"confidence":0.9,"provenance":{"kind":"deposition","generator":"untrusted"}}]}`)}
	generator, err := NewModelTwinProposalGenerator(model, "Production Model V1")
	if err != nil {
		t.Fatal(err)
	}

	build, err := GenerateTwinProposal(context.Background(), generator, TwinProposalGenerationInput{BuilderInput: input, EgressEligible: true})
	if err != nil {
		t.Fatalf("GenerateTwinProposal() error = %v", err)
	}

	if build.Content.SchemaVersion != twinProposalSchemaVersion || len(build.Content.Assertions) != 1 {
		t.Fatalf("content = %#v", build.Content)
	}
	assertion := build.Content.Assertions[0]
	if assertion.ID != "review.first" || assertion.Type != TwinAssertionQualityBar || assertion.Text != "Review every change." || assertion.Applicability.ProjectID != "project-1" || strings.Join(assertion.Applicability.Keywords, ",") != "review" {
		t.Fatalf("canonical assertion = %#v", assertion)
	}
	if len(assertion.EvidenceCitations) != 2 || assertion.EvidenceCitations[0] != "issue:issue-1" || assertion.EvidenceCitations[1] != "project:project-1" {
		t.Fatalf("canonical citations = %#v", assertion.EvidenceCitations)
	}
	if assertion.Provenance != (TwinAssertionProvenance{Kind: TwinProvenanceModel, Generator: "production-model-v1"}) {
		t.Fatalf("trusted provenance = %#v", assertion.Provenance)
	}
	if model.calls != 1 || model.request.MaxAssertions != twinMaxAssertions || len(model.request.CitationKeys) != 2 || model.request.CitationKeys[0] != "issue:issue-1" {
		t.Fatalf("model request = %#v, calls = %d", model.request, model.calls)
	}
	var evidence map[string]any
	if err := json.Unmarshal(model.request.Evidence, &evidence); err != nil || evidence["source_digest"] != input.SourceDigest {
		t.Fatalf("model evidence = %s, error = %v", model.request.Evidence, err)
	}
	if build.ContentDigest != digestTwin(build.CanonicalJSON) {
		t.Fatalf("digest = %q", build.ContentDigest)
	}
}

func TestModelTwinProposalGenerator_requiresExplicitEgressEligibility(t *testing.T) {
	model := &recordingTwinJSONModel{response: []byte(`{"assertions":[]}`)}
	generator, err := NewModelTwinProposalGenerator(model, "model-v1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = GenerateTwinProposal(context.Background(), generator, TwinProposalGenerationInput{BuilderInput: twinGeneratorTestInput(t)})

	if !errors.Is(err, ErrTwinGenerationDenied) || model.calls != 0 {
		t.Fatalf("GenerateTwinProposal() error = %v, model calls = %d", err, model.calls)
	}
}

func TestModelTwinProposalGenerator_preservesOpaqueSchemaV2Evidence(t *testing.T) {
	evidence := json.RawMessage(`{"schema_version":2,"egress_policy":` + twinAuthorizedEgressPolicyJSON() + `,"wiki_pages":[{"citation_key":"wiki_page:page-1:7","page_id":"page-1","revision":7,"markdown":"Use focused tests."}],"future_extension":{"retained":true}}`)
	input := TwinBuilderInput{
		SourceWikiRevisionID: "wiki-2", SourceDigest: "sha256:source-v2",
		CanonicalEvidence: evidence, EvidenceSchemaVersion: 2,
		Content:   LMWikiContent{SchemaVersion: 2},
		Citations: []LMWikiCitation{{CitationKey: "wiki_page:page-1:7"}},
	}
	model := &recordingTwinJSONModel{response: []byte(`{"assertions":[{"id":"test.focused","type":"procedure","text":"Use focused tests.","applicability":{"keywords":["code changes"]},"evidence_citations":["wiki_page:page-1:7"],"confidence":0.95,"provenance":{"kind":"model","generator":"ignored"}}]}`)}
	generator, err := NewModelTwinProposalGenerator(model, "model-v2")
	if err != nil {
		t.Fatal(err)
	}

	build, err := GenerateTwinProposal(context.Background(), generator, TwinProposalGenerationInput{BuilderInput: input, EgressEligible: true})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(model.request.Evidence) || string(model.request.Evidence) != string(evidence) {
		t.Fatalf("model evidence = %s, want byte-identical %s", model.request.Evidence, evidence)
	}
	if build.Content.SchemaVersion != 2 || build.Content.Assertions[0].EvidenceCitations[0] != "wiki_page:page-1:7" {
		t.Fatalf("schema-v2 build = %#v", build.Content)
	}
}

func TestModelTwinProposalGenerator_requiresFrozenEgressPolicy(t *testing.T) {
	tests := []struct {
		name     string
		evidence json.RawMessage
	}{
		{name: "schema one", evidence: json.RawMessage(`{"schema_version":1}`)},
		{name: "missing", evidence: json.RawMessage(`{"schema_version":2}`)},
		{name: "disabled", evidence: json.RawMessage(`{"schema_version":2,"egress_policy":{"remote_generation_enabled":false,"policy_version":1,"policy_digest":"sha256:` + strings.Repeat("a", 64) + `"}}`)},
		{name: "zero version", evidence: json.RawMessage(`{"schema_version":2,"egress_policy":{"remote_generation_enabled":true,"policy_version":0,"policy_digest":"sha256:` + strings.Repeat("a", 64) + `"}}`)},
		{name: "invalid digest", evidence: json.RawMessage(`{"schema_version":2,"egress_policy":{"remote_generation_enabled":true,"policy_version":1,"policy_digest":"sha256:not-a-digest"}}`)},
		{name: "unknown policy field", evidence: json.RawMessage(`{"schema_version":2,"egress_policy":{"remote_generation_enabled":true,"policy_version":1,"policy_digest":"sha256:` + strings.Repeat("a", 64) + `","implicit":true}}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := twinGeneratorTestInput(t)
			input.CanonicalEvidence = tc.evidence
			input.EvidenceSchemaVersion = 0
			model := &recordingTwinJSONModel{response: []byte(`{"assertions":[]}`)}
			generator, err := NewModelTwinProposalGenerator(model, "model-v2")
			if err != nil {
				t.Fatal(err)
			}

			_, err = GenerateTwinProposal(context.Background(), generator, TwinProposalGenerationInput{BuilderInput: input, EgressEligible: true})
			if !errors.Is(err, ErrTwinGenerationDenied) || model.calls != 0 {
				t.Fatalf("GenerateTwinProposal() error = %v, model calls = %d", err, model.calls)
			}
		})
	}
}

func TestTwinProposalValidator_rejectsUnknownCitationInOpaqueEvidence(t *testing.T) {
	input := TwinBuilderInput{
		SourceWikiRevisionID: "wiki-2", SourceDigest: "sha256:source-v2",
		CanonicalEvidence:     json.RawMessage(`{"schema_version":2,"wiki_pages":[{"citation_key":"wiki_page:invented"}]}`),
		EvidenceSchemaVersion: 2, Content: LMWikiContent{SchemaVersion: 2},
		Citations: []LMWikiCitation{{CitationKey: "wiki_page:accepted"}},
	}
	_, err := ValidateTwinProposal(input, TwinProposalCandidate{})
	if !errors.Is(err, ErrTwinCitationMissing) {
		t.Fatalf("ValidateTwinProposal() error = %v, want %v", err, ErrTwinCitationMissing)
	}
}

func TestModelTwinProposalGenerator_rejectsMalformedAndOversizedOutput(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response []byte
		want     error
	}{
		{name: "unknown field", response: []byte(`{"assertions":[],"prompt":"leak"}`), want: ErrTwinGeneratorOutput},
		{name: "trailing value", response: []byte(`{"assertions":[]} {}`), want: ErrTwinGeneratorOutput},
		{name: "null assertions", response: []byte(`{"assertions":null}`), want: ErrTwinGeneratorOutput},
		{name: "oversized", response: []byte(strings.Repeat("x", twinMaxModelResponseBytes+1)), want: ErrTwinContentTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			generator, err := NewModelTwinProposalGenerator(&recordingTwinJSONModel{response: tc.response}, "model-v1")
			if err != nil {
				t.Fatal(err)
			}
			_, err = GenerateTwinProposal(context.Background(), generator, TwinProposalGenerationInput{BuilderInput: twinGeneratorTestInput(t), EgressEligible: true})
			if !errors.Is(err, tc.want) {
				t.Fatalf("GenerateTwinProposal() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTwinProposalValidator_rejectsAdversarialAssertions(t *testing.T) {
	valid := twinGeneratorAssertion("rule.valid", TwinAssertionConstraint, "Require review before merge.", "All repository changes", "issue:issue-1")
	tooMany := make([]TwinAssertion, twinMaxAssertions+1)
	for index := range tooMany {
		tooMany[index] = twinGeneratorAssertion("rule."+string(rune('a'+index%26))+"."+strings.Repeat("x", index/26+1), TwinAssertionConstraint, "Require review.", "All changes", "issue:issue-1")
	}
	for _, tc := range []struct {
		name       string
		assertions []TwinAssertion
		want       error
	}{
		{name: "forbidden type", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Type = "persona" })}, want: ErrTwinInvalidAssertion},
		{name: "duplicate stable id", assertions: []TwinAssertion{valid, withTwinAssertion(valid, func(a *TwinAssertion) { a.ID = " RULE.VALID " })}, want: ErrTwinInvalidAssertion},
		{name: "missing citation", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.EvidenceCitations = nil })}, want: ErrTwinInvalidAssertion},
		{name: "invented citation", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.EvidenceCitations = []string{"issue:invented"} })}, want: ErrTwinCitationMissing},
		{name: "identity claim", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = "You are the workspace owner." })}, want: ErrTwinUnsafeAssertion},
		{name: "first person identity claim", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = "I am a meticulous engineer." })}, want: ErrTwinUnsafeAssertion},
		{name: "persona claim", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = "Adopt a friendly persona." })}, want: ErrTwinUnsafeAssertion},
		{name: "secret leak", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = "Use api_key=sk_1234567890abcdef" })}, want: ErrTwinUnsafeAssertion},
		{name: "unix path leak", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = "Read /home/alice/private/notes.md" })}, want: ErrTwinUnsafeAssertion},
		{name: "windows path leak", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = `Read C:\Users\Alice\notes.md` })}, want: ErrTwinUnsafeAssertion},
		{name: "profile leak", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = "Use daemon profile name=alice-private" })}, want: ErrTwinUnsafeAssertion},
		{name: "invented issue scope", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Applicability = TwinAssertionApplicability{IssueID: "issue-invented"} })}, want: ErrTwinCitationMissing},
		{name: "invented project scope", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Applicability = TwinAssertionApplicability{ProjectID: "project-invented"} })}, want: ErrTwinCitationMissing},
		{name: "model task scope", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Applicability = TwinAssertionApplicability{TaskID: "task-1"} })}, want: ErrTwinInvalidAssertion},
		{name: "unsafe applicability keyword", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) {
			a.Applicability = TwinAssertionApplicability{Keywords: []string{"api_key=sk_1234567890abcdef"}}
		})}, want: ErrTwinUnsafeAssertion},
		{name: "too many applicability keywords", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) {
			a.Applicability = TwinAssertionApplicability{Keywords: make([]string, twinMaxApplicabilityKeywords+1)}
		})}, want: ErrTwinInvalidAssertion},
		{name: "oversized text", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Text = strings.Repeat("x", twinMaxAssertionTextRunes+1) })}, want: ErrTwinInvalidAssertion},
		{name: "zero confidence", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Confidence = 0 })}, want: ErrTwinInvalidAssertion},
		{name: "nan confidence", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Confidence = math.NaN() })}, want: ErrTwinInvalidAssertion},
		{name: "untrusted provenance", assertions: []TwinAssertion{withTwinAssertion(valid, func(a *TwinAssertion) { a.Provenance.Kind = "self_reported" })}, want: ErrTwinInvalidAssertion},
		{name: "too many assertions", assertions: tooMany, want: ErrTwinInvalidAssertion},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateTwinProposal(twinGeneratorTestInput(t), TwinProposalCandidate{Assertions: tc.assertions})
			if !errors.Is(err, tc.want) {
				t.Fatalf("ValidateTwinProposal() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTwinProposalValidator_rejectsPathLikeSharedIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*TwinBuilderInput)
	}{
		{name: "revision id", change: func(input *TwinBuilderInput) { input.SourceWikiRevisionID = "/home/alice/wiki" }},
		{name: "source digest", change: func(input *TwinBuilderInput) { input.SourceDigest = "file:///tmp/digest" }},
		{name: "citation key", change: func(input *TwinBuilderInput) { input.Citations[0].CitationKey = "/home/alice/citation" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := twinGeneratorTestInput(t)
			tc.change(&input)
			_, err := ValidateTwinProposal(input, TwinProposalCandidate{})
			if !errors.Is(err, ErrTwinInvalidInput) {
				t.Fatalf("ValidateTwinProposal() error = %v, want %v", err, ErrTwinInvalidInput)
			}
		})
	}
}

func TestTwinProposalValidator_isDeterministicAndReportsChangedMeaning(t *testing.T) {
	input := twinGeneratorTestInput(t)
	current := []TwinAssertion{
		twinGeneratorAssertion("rule.b", TwinAssertionProcedure, "Run focused tests.", "before completion", "project:project-1", "issue:issue-1"),
		twinGeneratorAssertion("rule.a", TwinAssertionPreference, "Prefer concise reports.", "when reporting results", "issue:issue-1"),
	}
	input.PriorAssertions = []TwinAssertion{
		withTwinAssertion(current[0], func(a *TwinAssertion) { a.Text = "Run all tests." }),
		current[1],
		twinGeneratorAssertion("rule.removed", TwinAssertionConstraint, "Old constraint.", "All work", "issue:issue-1"),
	}
	want, err := ValidateTwinProposal(input, TwinProposalCandidate{Assertions: current})
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewSource(42))
	for run := 0; run < 100; run++ {
		candidate := append([]TwinAssertion(nil), current...)
		random.Shuffle(len(candidate), func(i, j int) { candidate[i], candidate[j] = candidate[j], candidate[i] })
		for index := range candidate {
			random.Shuffle(len(candidate[index].EvidenceCitations), func(i, j int) {
				candidate[index].EvidenceCitations[i], candidate[index].EvidenceCitations[j] = candidate[index].EvidenceCitations[j], candidate[index].EvidenceCitations[i]
			})
		}
		got, err := ValidateTwinProposal(input, TwinProposalCandidate{Assertions: candidate})
		if err != nil || string(got.CanonicalJSON) != string(want.CanonicalJSON) || got.ContentDigest != want.ContentDigest {
			t.Fatalf("run %d non-deterministic: err=%v got=%s want=%s", run, err, got.CanonicalJSON, want.CanonicalJSON)
		}
	}
	if strings.Join(want.Content.Diff.Changed, ",") != "rule.b" || strings.Join(want.Content.Diff.Unchanged, ",") != "rule.a" || strings.Join(want.Content.Diff.Removed, ",") != "rule.removed" {
		t.Fatalf("diff = %#v", want.Content.Diff)
	}
}

func TestValidatedTwinAssertionsCompileByIssueProjectAndKeyword(t *testing.T) {
	input := twinGeneratorTestInput(t)
	candidate := TwinProposalCandidate{Assertions: []TwinAssertion{
		withTwinAssertion(twinGeneratorAssertion("scope.issue", TwinAssertionProcedure, "Follow the issue checklist.", "ignored", "issue:issue-1"), func(a *TwinAssertion) {
			a.Applicability = TwinAssertionApplicability{IssueID: "issue-1"}
		}),
		withTwinAssertion(twinGeneratorAssertion("scope.project", TwinAssertionConstraint, "Keep project compatibility.", "ignored", "project:project-1"), func(a *TwinAssertion) {
			a.Applicability = TwinAssertionApplicability{ProjectID: "project-1"}
		}),
		withTwinAssertion(twinGeneratorAssertion("scope.keyword", TwinAssertionQualityBar, "Include release evidence.", "ignored", "project:project-1"), func(a *TwinAssertion) {
			a.Applicability = TwinAssertionApplicability{Keywords: []string{" Release ", "release"}}
		}),
	}}
	build, err := ValidateTwinProposal(input, candidate)
	if err != nil {
		t.Fatal(err)
	}
	signedAssertions, err := twinAssertions(build.CanonicalJSON)
	if err != nil {
		t.Fatalf("decode signed Twin content: %v", err)
	}
	assertions := make([]TwinBriefingAssertion, len(signedAssertions))
	for index, assertion := range signedAssertions {
		assertions[index] = TwinBriefingAssertion{
			ID: assertion.ID, Lifecycle: TwinAssertionSigned, Type: assertion.Type,
			Text: assertion.Text, CitationIDs: assertion.EvidenceCitations, Applicability: assertion.Applicability,
		}
	}
	compile := func(task TwinTaskEligibility) TwinCompiledBriefing {
		t.Helper()
		result, err := NewTwinBriefingCompiler().Compile(TwinBriefingInput{
			Task:    task,
			Version: TwinSignedAssertionEnvelope{VersionID: "version-1", SignatureDigest: build.ContentDigest, Lifecycle: TwinVersionSigned, Authorized: true, Assertions: assertions},
			Policy:  TwinEffectiveUsePolicy{State: TwinUseEnabled, Scope: TwinUseScopeWorkspace, ScopeID: "workspace-1", BindingID: "binding-1", Explicit: true, Reason: TwinPolicyExplicitBinding},
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	issue := compile(TwinTaskEligibility{TaskID: "task-1", WorkspaceID: "workspace-1", IssueID: "issue-1", ProjectID: "project-other", Eligible: true, Authorized: true})
	project := compile(TwinTaskEligibility{TaskID: "task-2", WorkspaceID: "workspace-1", IssueID: "issue-other", ProjectID: "project-1", Eligible: true, Authorized: true})
	keyword := compile(TwinTaskEligibility{TaskID: "task-3", WorkspaceID: "workspace-1", IssueID: "issue-other", ProjectID: "project-other", Request: "Prepare RELEASE evidence", Eligible: true, Authorized: true})
	if strings.Join(issue.SelectedAssertionIDs, ",") != "scope.issue" || strings.Join(project.SelectedAssertionIDs, ",") != "scope.project" || strings.Join(keyword.SelectedAssertionIDs, ",") != "scope.keyword" {
		t.Fatalf("compiled scopes = issue %#v project %#v keyword %#v", issue.SelectedAssertionIDs, project.SelectedAssertionIDs, keyword.SelectedAssertionIDs)
	}
}

func TestDeterministicTwinProposalGenerator_propagatesConfiguredError(t *testing.T) {
	want := errors.New("generator unavailable")
	_, err := GenerateTwinProposal(context.Background(), DeterministicTwinProposalGenerator{Err: want}, TwinProposalGenerationInput{BuilderInput: twinGeneratorTestInput(t)})
	if !errors.Is(err, want) {
		t.Fatalf("GenerateTwinProposal() error = %v, want %v", err, want)
	}
}

type recordingTwinJSONModel struct {
	response []byte
	err      error
	request  TwinModelRequest
	calls    int
}

func (m *recordingTwinJSONModel) GenerateJSON(_ context.Context, request TwinModelRequest) ([]byte, error) {
	m.calls++
	m.request = request
	return m.response, m.err
}

func twinGeneratorTestInput(t *testing.T) TwinBuilderInput {
	t.Helper()
	wiki := mustTwinWiki(t, LMWikiSourceSnapshot{
		Issues:   []LMWikiIssue{{ID: "issue-1", Number: 1, Title: "Review work", Description: "Require review", Status: "todo"}},
		Projects: []LMWikiProject{{ID: "project-1", Title: "Quality", Description: "Ship trusted work", Status: "in_progress"}},
	})
	var evidence map[string]any
	if err := json.Unmarshal(wiki.CanonicalJSON, &evidence); err != nil {
		t.Fatal(err)
	}
	evidence["schema_version"] = 2
	evidence["egress_policy"] = map[string]any{
		"remote_generation_enabled": true,
		"policy_version":            1,
		"policy_digest":             "sha256:" + strings.Repeat("a", 64),
	}
	canonical, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	return TwinBuilderInput{
		SourceWikiRevisionID: "wiki-1", SourceDigest: wiki.SourceDigest,
		CanonicalEvidence: canonical, EvidenceSchemaVersion: 2,
		Content: wiki.Content, Citations: wiki.Citations,
	}
}

func twinAuthorizedEgressPolicyJSON() string {
	return `{"remote_generation_enabled":true,"policy_version":1,"policy_digest":"sha256:` + strings.Repeat("a", 64) + `"}`
}

func twinGeneratorAssertion(id string, assertionType TwinAssertionType, text, applicability string, citations ...string) TwinAssertion {
	return TwinAssertion{
		ID:                id,
		Type:              assertionType,
		Text:              text,
		Applicability:     TwinAssertionApplicability{Keywords: []string{applicability}},
		EvidenceCitations: append([]string(nil), citations...),
		Confidence:        0.9,
		Provenance:        TwinAssertionProvenance{Kind: TwinProvenanceModel, Generator: "model-v1"},
	}
}

func withTwinAssertion(assertion TwinAssertion, change func(*TwinAssertion)) TwinAssertion {
	assertion.EvidenceCitations = append([]string(nil), assertion.EvidenceCitations...)
	assertion.Applicability.Keywords = append([]string(nil), assertion.Applicability.Keywords...)
	change(&assertion)
	return assertion
}
