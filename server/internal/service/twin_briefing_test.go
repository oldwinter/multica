package service

import (
	"errors"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestResolveTwinUsePolicy_usesMostSpecificApplicableBinding(t *testing.T) {
	ctx := TwinUsePolicyContext{
		WorkspaceID: "workspace-1",
		AgentID:     "agent-1",
		ProjectID:   "project-1",
		IssueID:     "issue-1",
		RunID:       "run-1",
	}
	bindings := []TwinUsePolicyBinding{
		{ID: "workspace", Scope: TwinUseScopeWorkspace, ScopeID: "workspace-1", State: TwinUseEnabled},
		{ID: "agent", Scope: TwinUseScopeAgent, ScopeID: "agent-1", State: TwinUsePreview},
		{ID: "project", Scope: TwinUseScopeProject, ScopeID: "project-1", State: TwinUseEnabled},
		{ID: "issue", Scope: TwinUseScopeIssue, ScopeID: "issue-1", State: TwinUseOff},
		{ID: "run", Scope: TwinUseScopeOneOff, ScopeID: "run-1", State: TwinUsePreview},
		{ID: "other-run", Scope: TwinUseScopeOneOff, ScopeID: "run-2", State: TwinUseEnabled},
	}

	decision, err := ResolveTwinUsePolicy(ctx, bindings)

	if err != nil {
		t.Fatalf("ResolveTwinUsePolicy() error = %v", err)
	}
	if decision.State != TwinUsePreview || decision.Scope != TwinUseScopeOneOff || decision.BindingID != "run" || !decision.Explicit {
		t.Fatalf("decision = %#v", decision)
	}
	assertPolicyExclusion(t, decision.Exclusions, "issue", TwinPolicyBindingShadowed)
	assertPolicyExclusion(t, decision.Exclusions, "other-run", TwinPolicyBindingNotApplicable)
}

func TestResolveTwinUsePolicy_precedenceMatrix(t *testing.T) {
	ctx := TwinUsePolicyContext{WorkspaceID: "w", AgentID: "a", ProjectID: "p", IssueID: "i", RunID: "r"}
	bindings := []TwinUsePolicyBinding{
		{ID: "w", Scope: TwinUseScopeWorkspace, ScopeID: "w", State: TwinUseEnabled},
		{ID: "a", Scope: TwinUseScopeAgent, ScopeID: "a", State: TwinUsePreview},
		{ID: "p", Scope: TwinUseScopeProject, ScopeID: "p", State: TwinUseOff},
		{ID: "i", Scope: TwinUseScopeIssue, ScopeID: "i", State: TwinUseEnabled},
		{ID: "r", Scope: TwinUseScopeOneOff, ScopeID: "r", State: TwinUsePreview},
	}
	for _, tc := range []struct {
		name       string
		bindings   []TwinUsePolicyBinding
		wantScope  TwinUsePolicyScope
		wantState  TwinUsePolicyState
		wantSource string
	}{
		{name: "workspace", bindings: bindings[:1], wantScope: TwinUseScopeWorkspace, wantState: TwinUseEnabled, wantSource: "w"},
		{name: "agent over workspace", bindings: bindings[:2], wantScope: TwinUseScopeAgent, wantState: TwinUsePreview, wantSource: "a"},
		{name: "project over agent", bindings: bindings[:3], wantScope: TwinUseScopeProject, wantState: TwinUseOff, wantSource: "p"},
		{name: "issue over project", bindings: bindings[:4], wantScope: TwinUseScopeIssue, wantState: TwinUseEnabled, wantSource: "i"},
		{name: "one-off over issue", bindings: bindings, wantScope: TwinUseScopeOneOff, wantState: TwinUsePreview, wantSource: "r"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTwinUsePolicy(ctx, tc.bindings)
			if err != nil {
				t.Fatal(err)
			}
			if got.Scope != tc.wantScope || got.State != tc.wantState || got.BindingID != tc.wantSource {
				t.Fatalf("decision = %#v", got)
			}
		})
	}
}

func TestResolveTwinUsePolicy_defaultsOffAndRejectsAmbiguity(t *testing.T) {
	ctx := TwinUsePolicyContext{WorkspaceID: "w", AgentID: "a"}
	decision, err := ResolveTwinUsePolicy(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != TwinUseOff || decision.Explicit || decision.Reason != TwinPolicyNoExplicitBinding {
		t.Fatalf("default decision = %#v", decision)
	}

	_, err = ResolveTwinUsePolicy(ctx, []TwinUsePolicyBinding{
		{ID: "first", Scope: TwinUseScopeAgent, ScopeID: "a", State: TwinUseEnabled},
		{ID: "second", Scope: TwinUseScopeAgent, ScopeID: "a", State: TwinUseOff},
	})
	if !errors.Is(err, ErrTwinUsePolicyInvalid) {
		t.Fatalf("duplicate binding error = %v", err)
	}
}

func TestTwinBriefingCompiler_selectsRelevantSignedAssertions(t *testing.T) {
	compiler := NewTwinBriefingCompiler()
	input := validTwinBriefingInput()
	input.Version.Assertions = []TwinBriefingAssertion{
		{ID: "general", Lifecycle: TwinAssertionSigned, Type: TwinAssertionPreference, Text: "Prefer concise status updates", CitationIDs: []string{"citation-general"}},
		{ID: "project", Lifecycle: TwinAssertionSigned, Type: TwinAssertionProcedure, Text: "Run the release checklist", CitationIDs: []string{"citation-project"}, Applicability: TwinAssertionApplicability{ProjectID: "project-1"}},
		{ID: "keyword", Lifecycle: TwinAssertionSigned, Type: TwinAssertionQualityBar, Text: "Include rollback evidence", CitationIDs: []string{"citation-release"}, Applicability: TwinAssertionApplicability{Keywords: []string{"release"}}},
		{ID: "wrong-issue", Lifecycle: TwinAssertionSigned, Type: TwinAssertionConstraint, Text: "Use a different issue rule", CitationIDs: []string{"citation-wrong"}, Applicability: TwinAssertionApplicability{IssueID: "issue-2"}},
		{ID: "wrong-keyword", Lifecycle: TwinAssertionSigned, Type: TwinAssertionPreference, Text: "Prepare an invoice", CitationIDs: []string{"citation-invoice"}, Applicability: TwinAssertionApplicability{Keywords: []string{"billing"}}},
		{ID: "proposal", Lifecycle: TwinAssertionProposal, Type: TwinAssertionConstraint, Text: "Mutable proposal must not appear", CitationIDs: []string{"citation-proposal"}},
	}

	got, err := compiler.Compile(input)

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !got.Inject || got.PreviewOnly || got.VersionID != "version-1" || got.CompilerVersion != TwinBriefingCompilerVersion {
		t.Fatalf("compiled metadata = %#v", got)
	}
	wantIDs := []string{"project", "keyword", "general"}
	if !reflect.DeepEqual(got.SelectedAssertionIDs, wantIDs) {
		t.Fatalf("selected assertion ids = %#v, want %#v", got.SelectedAssertionIDs, wantIDs)
	}
	if strings.Contains(got.Briefing, "Mutable proposal") || strings.Contains(got.Briefing, "invoice") || !strings.Contains(got.Briefing, "system safety, workspace permissions, and the current user request take precedence") {
		t.Fatalf("briefing = %q", got.Briefing)
	}
	if !reflect.DeepEqual(got.CitationIDs, []string{"citation-general", "citation-project", "citation-release"}) {
		t.Fatalf("citation ids = %#v", got.CitationIDs)
	}
	assertBriefingExclusion(t, got.Exclusions, "proposal", TwinBriefingMutableProposal)
	assertBriefingExclusion(t, got.Exclusions, "wrong-issue", TwinBriefingIrrelevant)
	assertBriefingExclusion(t, got.Exclusions, "wrong-keyword", TwinBriefingIrrelevant)
	if got.ByteCount != len([]byte(got.Briefing)) || got.TokenCount != estimateTwinBriefingTokens(got.Briefing) || got.ByteCount > TwinBriefingMaxBytes || got.TokenCount > TwinBriefingMaxTokens {
		t.Fatalf("budget metadata = bytes:%d tokens:%d", got.ByteCount, got.TokenCount)
	}
	if got.Digest != digestTwin([]byte(got.Briefing)) {
		t.Fatalf("digest = %q, want digest of exact briefing", got.Digest)
	}
	wantAuthority := []TwinInstructionAuthority{
		TwinAuthoritySystemSafety,
		TwinAuthorityWorkspacePermission,
		TwinAuthorityUserRequest,
		TwinAuthoritySignedBriefing,
	}
	if !reflect.DeepEqual(got.AuthorityOrder, wantAuthority) {
		t.Fatalf("authority order = %#v, want %#v", got.AuthorityOrder, wantAuthority)
	}
}

func TestTwinBriefingCompiler_previewCompilesButNeverInjects(t *testing.T) {
	input := validTwinBriefingInput()
	input.Policy.State = TwinUsePreview

	got, err := NewTwinBriefingCompiler().Compile(input)

	if err != nil {
		t.Fatal(err)
	}
	if got.Briefing == "" || got.Inject || !got.PreviewOnly || got.PolicyDecision.State != TwinUsePreview {
		t.Fatalf("preview result = %#v", got)
	}
	assertBriefingExclusion(t, got.Exclusions, "", TwinBriefingPreviewOnly)
}

func TestTwinBriefingCompiler_doesNotInjectWhenNothingIsRelevant(t *testing.T) {
	input := validTwinBriefingInput()
	input.Version.Assertions[0].Applicability.ProjectID = "project-2"

	got, err := NewTwinBriefingCompiler().Compile(input)

	if err != nil {
		t.Fatal(err)
	}
	if got.Inject || len(got.SelectedAssertionIDs) != 0 {
		t.Fatalf("empty selection result = %#v", got)
	}
	assertBriefingExclusion(t, got.Exclusions, "assertion-1", TwinBriefingIrrelevant)
	assertBriefingExclusion(t, got.Exclusions, "", TwinBriefingNoRelevantAssertion)
}

func TestTwinBriefingCompiler_failClosedMatrix(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*TwinBriefingInput)
		code TwinBriefingExclusionCode
	}{
		{name: "policy off", edit: func(in *TwinBriefingInput) { in.Policy.State = TwinUseOff }, code: TwinBriefingPolicyOff},
		{name: "task ineligible", edit: func(in *TwinBriefingInput) { in.Task.Eligible = false }, code: TwinBriefingTaskIneligible},
		{name: "task unauthorized", edit: func(in *TwinBriefingInput) { in.Task.Authorized = false }, code: TwinBriefingTaskUnauthorized},
		{name: "task local only", edit: func(in *TwinBriefingInput) { in.Task.LocalOnly = true }, code: TwinBriefingTaskLocalOnly},
		{name: "unsigned envelope", edit: func(in *TwinBriefingInput) { in.Version.Lifecycle = TwinVersionDraft }, code: TwinBriefingUnsignedVersion},
		{name: "mutable proposal envelope", edit: func(in *TwinBriefingInput) { in.Version.Lifecycle = TwinVersionProposal }, code: TwinBriefingMutableProposal},
		{name: "stale version", edit: func(in *TwinBriefingInput) { in.Version.Stale = true }, code: TwinBriefingStaleVersion},
		{name: "unauthorized version", edit: func(in *TwinBriefingInput) { in.Version.Authorized = false }, code: TwinBriefingVersionUnauthorized},
		{name: "local-only version", edit: func(in *TwinBriefingInput) { in.Version.LocalOnly = true }, code: TwinBriefingVersionLocalOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := validTwinBriefingInput()
			tc.edit(&input)
			got, err := NewTwinBriefingCompiler().Compile(input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Briefing != "" || got.Inject || got.PreviewOnly || got.Digest != "" || len(got.SelectedAssertionIDs) != 0 || len(got.CitationIDs) != 0 {
				t.Fatalf("fail-closed result = %#v", got)
			}
			assertBriefingExclusion(t, got.Exclusions, "", tc.code)
		})
	}
}

func TestTwinBriefingCompiler_appliesFixedBudgetsWithoutPartialAssertions(t *testing.T) {
	input := validTwinBriefingInput()
	input.Version.Assertions = []TwinBriefingAssertion{
		{ID: "issue-first", Lifecycle: TwinAssertionSigned, Type: TwinAssertionProcedure, Text: strings.Repeat("x", TwinBriefingMaxBytes), CitationIDs: []string{"citation-large"}, Applicability: TwinAssertionApplicability{IssueID: "issue-1"}},
		{ID: "small", Lifecycle: TwinAssertionSigned, Type: TwinAssertionConstraint, Text: "Keep the change atomic", CitationIDs: []string{"citation-small"}},
	}

	got, err := NewTwinBriefingCompiler().Compile(input)

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.SelectedAssertionIDs, []string{"small"}) || strings.Contains(got.Briefing, strings.Repeat("x", 100)) {
		t.Fatalf("budgeted briefing = %#v", got)
	}
	assertBriefingExclusion(t, got.Exclusions, "issue-first", TwinBriefingOverBudget)
}

func TestTwinBriefingCompiler_isDeterministicAcrossInputPermutations(t *testing.T) {
	input := validTwinBriefingInput()
	input.Version.Assertions = []TwinBriefingAssertion{
		{ID: "b", Lifecycle: TwinAssertionSigned, Type: TwinAssertionProcedure, Text: "Second", CitationIDs: []string{"c-2", "c-1"}},
		{ID: "a", Lifecycle: TwinAssertionSigned, Type: TwinAssertionConstraint, Text: "First", CitationIDs: []string{"c-3"}, Applicability: TwinAssertionApplicability{ProjectID: "project-1"}},
		{ID: "c", Lifecycle: TwinAssertionSigned, Type: TwinAssertionPreference, Text: "Not relevant", CitationIDs: []string{"c-4"}, Applicability: TwinAssertionApplicability{ProjectID: "project-2"}},
	}
	want, err := NewTwinBriefingCompiler().Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewSource(42))
	for run := 0; run < 50; run++ {
		candidate := input
		candidate.Version.Assertions = append([]TwinBriefingAssertion(nil), input.Version.Assertions...)
		random.Shuffle(len(candidate.Version.Assertions), func(i, j int) {
			candidate.Version.Assertions[i], candidate.Version.Assertions[j] = candidate.Version.Assertions[j], candidate.Version.Assertions[i]
		})
		for i := range candidate.Version.Assertions {
			random.Shuffle(len(candidate.Version.Assertions[i].CitationIDs), func(a, b int) {
				candidate.Version.Assertions[i].CitationIDs[a], candidate.Version.Assertions[i].CitationIDs[b] = candidate.Version.Assertions[i].CitationIDs[b], candidate.Version.Assertions[i].CitationIDs[a]
			})
		}
		got, err := NewTwinBriefingCompiler().Compile(candidate)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d non-deterministic: err=%v\ngot=%#v\nwant=%#v", run, err, got, want)
		}
	}
}

func TestTwinBriefingCompiler_rejectsMalformedSignedInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*TwinBriefingInput)
	}{
		{name: "missing task id", edit: func(in *TwinBriefingInput) { in.Task.TaskID = "" }},
		{name: "missing workspace id", edit: func(in *TwinBriefingInput) { in.Task.WorkspaceID = "" }},
		{name: "missing version id", edit: func(in *TwinBriefingInput) { in.Version.VersionID = "" }},
		{name: "missing signature digest", edit: func(in *TwinBriefingInput) { in.Version.SignatureDigest = "" }},
		{name: "unknown policy state", edit: func(in *TwinBriefingInput) { in.Policy.State = TwinUsePolicyState("mystery") }},
		{name: "non-explicit enabled policy", edit: func(in *TwinBriefingInput) { in.Policy.Explicit = false }},
		{name: "policy scope mismatch", edit: func(in *TwinBriefingInput) { in.Policy.ScopeID = "issue-2" }},
		{name: "duplicate assertion id", edit: func(in *TwinBriefingInput) {
			in.Version.Assertions = append(in.Version.Assertions, in.Version.Assertions[0])
		}},
		{name: "unknown assertion type", edit: func(in *TwinBriefingInput) { in.Version.Assertions[0].Type = TwinAssertionType("identity") }},
		{name: "missing citation", edit: func(in *TwinBriefingInput) { in.Version.Assertions[0].CitationIDs = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := validTwinBriefingInput()
			tc.edit(&input)
			got, err := NewTwinBriefingCompiler().Compile(input)
			if !errors.Is(err, ErrTwinBriefingInvalidInput) || got.Briefing != "" {
				t.Fatalf("Compile() = %#v, %v", got, err)
			}
		})
	}
}

func validTwinBriefingInput() TwinBriefingInput {
	return TwinBriefingInput{
		Task: TwinTaskEligibility{
			TaskID:      "task-1",
			WorkspaceID: "workspace-1",
			AgentID:     "agent-1",
			ProjectID:   "project-1",
			IssueID:     "issue-1",
			RunID:       "run-1",
			Request:     "Prepare the release notes",
			Tags:        []string{"release"},
			Eligible:    true,
			Authorized:  true,
		},
		Version: TwinSignedAssertionEnvelope{
			VersionID:       "version-1",
			SignatureDigest: "sha256:signed",
			Lifecycle:       TwinVersionSigned,
			Authorized:      true,
			Assertions: []TwinBriefingAssertion{{
				ID:          "assertion-1",
				Lifecycle:   TwinAssertionSigned,
				Type:        TwinAssertionPreference,
				Text:        "Prefer concise updates",
				CitationIDs: []string{"citation-1"},
			}},
		},
		Policy: TwinEffectiveUsePolicy{
			State:     TwinUseEnabled,
			Scope:     TwinUseScopeIssue,
			ScopeID:   "issue-1",
			BindingID: "binding-1",
			Explicit:  true,
		},
	}
}

func assertPolicyExclusion(t *testing.T, exclusions []TwinPolicyBindingExclusion, bindingID string, code TwinPolicyBindingExclusionCode) {
	t.Helper()
	for _, exclusion := range exclusions {
		if exclusion.BindingID == bindingID && exclusion.Code == code {
			return
		}
	}
	t.Fatalf("missing policy exclusion binding=%q code=%q in %#v", bindingID, code, exclusions)
}

func assertBriefingExclusion(t *testing.T, exclusions []TwinBriefingExclusion, assertionID string, code TwinBriefingExclusionCode) {
	t.Helper()
	for _, exclusion := range exclusions {
		if exclusion.AssertionID == assertionID && exclusion.Code == code {
			return
		}
	}
	t.Fatalf("missing briefing exclusion assertion=%q code=%q in %#v", assertionID, code, exclusions)
}
