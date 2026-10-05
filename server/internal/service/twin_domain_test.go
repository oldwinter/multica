package service

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
)

func TestTwinBuilder_buildsEvidenceBackedInitialProposal(t *testing.T) {
	// Given
	wiki := mustTwinWiki(t, LMWikiSourceSnapshot{
		Issues: []LMWikiIssue{
			{ID: "issue-1", Number: 7, Title: "Ship it", Description: "Release plan", Status: "todo", Priority: "high"},
			{ID: "issue-2", Number: 8, Title: "Done", Status: "done"},
		},
		Projects:         []LMWikiProject{{ID: "project-1", Title: "Roadmap", Description: "Plan work", Status: "in_progress", Priority: "medium"}},
		ProjectResources: []LMWikiProjectResource{{ID: "repo-1", ProjectID: "project-1", ResourceType: "github_repo", Label: "Main", Position: 1, GitURL: "https://github.com/acme/repo.git"}},
		AutopilotRuns:    []LMWikiAutopilotRun{{ID: "run-1", AutopilotID: "autopilot-1", AutopilotTitle: "Daily sync", Status: "completed", Source: "manual"}},
	})

	// When
	build, err := BuildTwinProposal(TwinBuilderInput{SourceWikiRevisionID: "wiki-1", SourceDigest: wiki.SourceDigest, Content: wiki.Content, Citations: wiki.Citations})

	// Then
	if err != nil {
		t.Fatalf("BuildTwinProposal() error = %v", err)
	}
	if build.Content.SchemaVersion != 2 || build.Content.SourceWikiRevisionID != "wiki-1" || build.Content.SourceDigest != wiki.SourceDigest || build.Content.Name != "Workspace Twin" {
		t.Fatalf("proposal identity = %#v", build.Content)
	}
	assertTwinAssertion(t, build.Content.Assertions, "issue:issue-1", "sha256:14010093cc1fe311bc74f9a928684b6104c150b95be6c3ed6b52c7b8776dcc06", "Issue 7: Ship it", TwinAssertionApplicability{IssueID: "issue-1"})
	assertTwinAssertion(t, build.Content.Assertions, "project:project-1", "sha256:e090d40a9c209a93abee361597fd98837767598f346e99129b3084ea91fa5011", "Project: Roadmap", TwinAssertionApplicability{ProjectID: "project-1"})
	assertTwinAssertion(t, build.Content.Assertions, "project_resource:repo-1", "sha256:4bc85d793a72c2f33a3d5eda379e99c656382516e8bcef706c39627f10f0b106", "Repository: github.com/acme/repo", TwinAssertionApplicability{ProjectID: "project-1"})
	assertTwinAssertion(t, build.Content.Assertions, "autopilot_run:run-1", "sha256:dbbd54428d855662e073f070798c281ae6f72c78bbcd914852985114b3e80001", "Autopilot Daily sync completed", TwinAssertionApplicability{Keywords: []string{"autopilot"}})
	if len(build.Content.Topics) != 1 || build.Content.Topics[0].IssueID != "issue-1" || build.Content.Topics[0].IssueNumber != 7 || build.Content.Topics[0].CitationKeys[0] != "issue:issue-1" {
		t.Fatalf("topics = %#v, want only the nonterminal issue", build.Content.Topics)
	}
	if len(build.Content.Diff.Added) != 5 || len(build.Content.Diff.Removed) != 0 || len(build.Content.Diff.Unchanged) != 0 || !sortedTwinIDs(build.Content.Diff.Added) {
		t.Fatalf("initial diff = %#v", build.Content.Diff)
	}
	for _, assertion := range build.Content.Assertions {
		if !containsTwinCitation(wiki.Citations, assertion.EvidenceCitations[0]) {
			t.Fatalf("assertion %q references missing citation %q", assertion.ID, assertion.EvidenceCitations[0])
		}
	}
}

func TestTwinBuilder_diffsAgainstPriorAssertions(t *testing.T) {
	// Given
	wiki := mustTwinWiki(t, LMWikiSourceSnapshot{Issues: []LMWikiIssue{{ID: "issue-1", Number: 7, Title: "Ship it", Description: "Release plan", Status: "todo", Priority: "high"}}})
	priorID := "sha256:14010093cc1fe311bc74f9a928684b6104c150b95be6c3ed6b52c7b8776dcc06"
	input := TwinBuilderInput{SourceWikiRevisionID: "wiki-2", SourceDigest: wiki.SourceDigest, Content: wiki.Content, Citations: wiki.Citations, PriorAssertions: []TwinAssertion{{ID: "sha256:obsolete"}, {ID: priorID}}}

	// When
	build, err := BuildTwinProposal(input)

	// Then
	if err != nil {
		t.Fatalf("BuildTwinProposal() error = %v", err)
	}
	if len(build.Content.Diff.Added) != 0 || len(build.Content.Diff.Removed) != 1 || build.Content.Diff.Removed[0] != "sha256:obsolete" || len(build.Content.Diff.Changed) != 1 || build.Content.Diff.Changed[0] != priorID {
		t.Fatalf("evolution diff = %#v", build.Content.Diff)
	}
}

func TestTwinBuilder_isDeterministicAcrossInputPermutations(t *testing.T) {
	// Given
	sources := LMWikiSourceSnapshot{Issues: []LMWikiIssue{{ID: "issue-a", Number: 1, Title: "A", Status: "todo"}, {ID: "issue-b", Number: 2, Title: "B", Status: "blocked"}}, Projects: []LMWikiProject{{ID: "project-a", Title: "A", Status: "planned"}, {ID: "project-b", Title: "B", Status: "completed"}}}
	wantWiki := mustTwinWiki(t, sources)
	want, err := BuildTwinProposal(TwinBuilderInput{SourceWikiRevisionID: "wiki-1", SourceDigest: wantWiki.SourceDigest, Content: wantWiki.Content, Citations: wantWiki.Citations})
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewSource(42))

	for run := 0; run < 100; run++ {
		// When
		candidate := LMWikiSourceSnapshot{Issues: append([]LMWikiIssue(nil), sources.Issues...), Projects: append([]LMWikiProject(nil), sources.Projects...)}
		random.Shuffle(len(candidate.Issues), func(i, j int) { candidate.Issues[i], candidate.Issues[j] = candidate.Issues[j], candidate.Issues[i] })
		random.Shuffle(len(candidate.Projects), func(i, j int) {
			candidate.Projects[i], candidate.Projects[j] = candidate.Projects[j], candidate.Projects[i]
		})
		wiki := mustTwinWiki(t, candidate)
		got, err := BuildTwinProposal(TwinBuilderInput{SourceWikiRevisionID: "wiki-1", SourceDigest: wiki.SourceDigest, Content: wiki.Content, Citations: wiki.Citations})

		// Then
		if err != nil || string(got.CanonicalJSON) != string(want.CanonicalJSON) || got.ContentDigest != want.ContentDigest {
			t.Fatalf("run %d produced a non-deterministic proposal: err=%v got=%s want=%s", run, err, got.CanonicalJSON, want.CanonicalJSON)
		}
	}
}

func TestTwinBuilder_returnsTypedErrorsForMissingCitationAndOversizeContent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input TwinBuilderInput
		want  error
	}{
		{"invalid input", TwinBuilderInput{}, ErrTwinInvalidInput},
		{"missing citation", TwinBuilderInput{SourceWikiRevisionID: "wiki-1", SourceDigest: "sha256:source", Content: LMWikiContent{SchemaVersion: 1, Issues: []lmWikiIssueContent{{CitationKey: "issue:missing", ID: "missing"}}}}, ErrTwinCitationMissing},
		{"oversize assertion", TwinBuilderInput{SourceWikiRevisionID: "wiki-1", SourceDigest: "sha256:source", Content: LMWikiContent{SchemaVersion: 1, Issues: []lmWikiIssueContent{{CitationKey: "issue:large", ID: "large", Title: strings.Repeat("x", twinMaxAssertionTextRunes+1)}}}, Citations: []LMWikiCitation{{CitationKey: "issue:large", SourceType: "issue", SourceID: "large"}}}, ErrTwinInvalidAssertion},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			// When
			build, err := BuildTwinProposal(tc.input)
			// Then
			if !errors.Is(err, tc.want) || len(build.CanonicalJSON) != 0 || build.ContentDigest != "" {
				t.Fatalf("BuildTwinProposal() = %#v, %v; want empty build and %v", build, err, tc.want)
			}
		})
	}
}

func mustTwinWiki(t *testing.T, sources LMWikiSourceSnapshot) LMWikiCanonicalSnapshot {
	t.Helper()
	wiki, err := BuildLMWikiSnapshot(sources)
	if err != nil {
		t.Fatal(err)
	}
	return wiki
}

func assertTwinAssertion(t *testing.T, assertions []TwinAssertion, citationKey, id, text string, applicability TwinAssertionApplicability) {
	t.Helper()
	for _, assertion := range assertions {
		if assertion.EvidenceCitations[0] == citationKey {
			if assertion.ID != id || assertion.Type != TwinAssertionProcedure || assertion.Text != text || !equalTwinApplicability(assertion.Applicability, applicability) || assertion.Confidence != 1 || assertion.Provenance.Kind != TwinProvenanceDeterministicInventory {
				t.Fatalf("assertion for %q = %#v", citationKey, assertion)
			}
			return
		}
	}
	t.Fatalf("missing assertion for %q", citationKey)
}

func equalTwinApplicability(left, right TwinAssertionApplicability) bool {
	return left.TaskID == right.TaskID && left.WorkspaceID == right.WorkspaceID && left.AgentID == right.AgentID && left.ProjectID == right.ProjectID && left.IssueID == right.IssueID && strings.Join(left.Keywords, "\x00") == strings.Join(right.Keywords, "\x00")
}

func containsTwinCitation(citations []LMWikiCitation, key string) bool {
	for _, citation := range citations {
		if citation.CitationKey == key {
			return true
		}
	}
	return false
}

func sortedTwinIDs(ids []string) bool {
	for i := 1; i < len(ids); i++ {
		if ids[i-1] > ids[i] {
			return false
		}
	}
	return true
}
