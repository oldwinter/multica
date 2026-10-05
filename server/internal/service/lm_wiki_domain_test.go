package service

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestLMWikiCanonicalSnapshot_projectsOnlyAllowlistedSafeSources(t *testing.T) {
	sources := LMWikiSourceSnapshot{
		Issues: []LMWikiIssue{{
			ID: "issue-b", Number: 2, Title: "  Fix\r\nnewline  ", Description: "line 1  \r\nline 2\t ",
			Status: "open", Priority: "high", ProjectID: "project-a", StartDate: "2026-01-02", DueDate: "2026-02-03",
			CreatedAt: time.Date(2026, 1, 1, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)), UpdatedAt: time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
		}},
		Projects: []LMWikiProject{{ID: "project-a", Title: "  Project\r\nA  ", Description: "  scope\t \r\n  details  ", Status: "active", Priority: "medium", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}},
		ProjectResources: []LMWikiProjectResource{
			{ID: "resource-local", ProjectID: "project-a", ResourceType: "local_directory", Label: "private-workdir", Position: 1, GitURL: "/Users/alice/private-workdir"},
			{ID: "resource-unknown", ProjectID: "project-a", ResourceType: "future_provider", Label: "password=do-not-leak", Position: 2, GitURL: "https://bad.example/a/b"},
			{ID: "resource-repo", ProjectID: "project-a", ResourceType: "github_repo", Label: "  Main repo  ", Position: 3, GitURL: "https://git-user:top-secret@git.example.com/acme/wiki.git?token=abc#fragment", Ref: " main ", DefaultBranchHint: " trunk "},
		},
		AutopilotRuns: []LMWikiAutopilotRun{
			{ID: "run-failed", AutopilotID: "autopilot-a", AutopilotTitle: "never include", Status: "failed", Source: "webhook"},
			{ID: "run-completed", AutopilotID: "autopilot-a", AutopilotTitle: "  Daily\r\nsummary  ", Status: "completed", Source: "webhook", IssueID: "issue-b", TriggeredAt: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), CompletedAt: time.Date(2026, 1, 3, 1, 0, 0, 0, time.UTC)},
		},
	}

	snapshot, err := BuildLMWikiSnapshot(sources)

	if err != nil {
		t.Fatalf("BuildLMWikiSnapshot() error = %v", err)
	}
	if got, want := string(snapshot.CanonicalJSON), `{"schema_version":2,"egress_policy":{"remote_generation_enabled":false,"policy_version":0,"policy_digest":""},"issues":[{"citation_key":"issue:issue-b","id":"issue-b","number":2,"title":"Fix\nnewline","description":"line 1\nline 2","status":"open","priority":"high","project_id":"project-a","start_date":"2026-01-02","due_date":"2026-02-03","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T08:00:00Z"}],"projects":[{"citation_key":"project:project-a","id":"project-a","title":"Project\nA","description":"scope\n  details","status":"active","priority":"medium","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}],"project_resources":[{"citation_key":"project_resource:resource-repo","id":"resource-repo","project_id":"project-a","label":"Main repo","position":3,"ref":{"host":"git.example.com","repository_path":"acme/wiki","ref":"main","default_branch_hint":"trunk"}}],"autopilot_runs":[{"citation_key":"autopilot_run:run-completed","id":"run-completed","autopilot_id":"autopilot-a","autopilot_title":"Daily\nsummary","source":"webhook","issue_id":"issue-b","triggered_at":"2026-01-03T00:00:00Z","completed_at":"2026-01-03T01:00:00Z","acceptance_state":"not_recorded"}],"wiki_pages":[]}`; got != want {
		t.Fatalf("CanonicalJSON = %s\nwant = %s", got, want)
	}
	if got, want := snapshot.SourceDigest, "sha256:81f84ea40d655b1ea9176e1b8943bf2bdcd8ac73e2ec2296782bc93160a7e72e"; got != want {
		t.Fatalf("SourceDigest = %q, want %q", got, want)
	}
	for _, forbidden := range []string{"local_path", "private-workdir", "future_provider", "password", "top-secret", "token=abc", "fragment", "trigger_payload", "result", "session_id", "work_dir", "/Users/alice"} {
		if strings.Contains(string(snapshot.CanonicalJSON), forbidden) {
			t.Fatalf("canonical content contains forbidden %q: %s", forbidden, snapshot.CanonicalJSON)
		}
	}
	if got, want := citationKeys(snapshot.Citations), []string{"autopilot_run:run-completed", "issue:issue-b", "project:project-a", "project_resource:resource-repo"}; !equalStrings(got, want) {
		t.Fatalf("citation keys = %v, want %v", got, want)
	}
	for _, citation := range snapshot.Citations {
		if strings.Contains(string(citation.SafeMetadata), "top-secret") {
			t.Fatalf("citation metadata leaks userinfo: %s", citation.SafeMetadata)
		}
	}
}

func TestSanitizeWikiGitRef_removesCredentialsAndTransportNoise(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want LMWikiGitRef
	}{
		{"https", "https://user:password@git.example.com/acme/repo.git?token=secret#readme", LMWikiGitRef{Host: "git.example.com", RepositoryPath: "acme/repo"}},
		{"ssh", "ssh://git@git.example.com/acme/repo.git", LMWikiGitRef{Host: "git.example.com", RepositoryPath: "acme/repo"}},
		{"scp", "git@git.example.com:acme/repo.git", LMWikiGitRef{Host: "git.example.com", RepositoryPath: "acme/repo"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SanitizeWikiGitRef(tc.raw, "", "")
			if err != nil {
				t.Fatalf("SanitizeWikiGitRef() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("SanitizeWikiGitRef() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestSanitizeWikiGitRef_rejectsUnsafeOrLocalInput(t *testing.T) {
	for _, raw := range []string{"/Users/alice/workdir", "file:///private/repo", "https://git.example.com/../private", "https://git.example.com/one", "git.example.com:one"} {
		t.Run(raw, func(t *testing.T) {
			_, err := SanitizeWikiGitRef(raw, "", "")
			if !errors.Is(err, ErrLMWikiUnsafeSource) {
				t.Fatalf("SanitizeWikiGitRef(%q) error = %v, want ErrLMWikiUnsafeSource", raw, err)
			}
		})
	}
}

func TestSanitizeWikiGitRef_rejectsScpQueryFragmentAndCredentialSuffixes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"query", "git@git.example.com:org/repo.git?token=placeholder"},
		{"fragment", "git@git.example.com:org/repo.git#private"},
		{"credential suffix", "git@git.example.com:org/repo.git@placeholder"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SanitizeWikiGitRef(tc.raw, "", "")
			if !errors.Is(err, ErrLMWikiUnsafeSource) {
				t.Fatalf("SanitizeWikiGitRef(%q) error = %v, want ErrLMWikiUnsafeSource", tc.raw, err)
			}
		})
	}
}

func TestLMWikiCanonicalPermutation_preservesBytesAndDigest(t *testing.T) {
	base := LMWikiSourceSnapshot{
		Issues:   []LMWikiIssue{{ID: "issue-z", Number: 9, Title: "Z", Status: "done"}, {ID: "issue-a", Number: 1, Title: "A", Status: "open"}, {ID: "issue-m", Number: 5, Title: "M", Status: "open"}},
		Projects: []LMWikiProject{{ID: "project-z", Title: "Z"}, {ID: "project-a", Title: "A"}, {ID: "project-m", Title: "M"}},
	}
	want, err := BuildLMWikiSnapshot(base)
	if err != nil {
		t.Fatal(err)
	}

	permutations := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		candidate := LMWikiSourceSnapshot{Issues: append([]LMWikiIssue(nil), base.Issues...), Projects: append([]LMWikiProject(nil), base.Projects...)}
		permutations.Shuffle(len(candidate.Issues), func(left, right int) {
			candidate.Issues[left], candidate.Issues[right] = candidate.Issues[right], candidate.Issues[left]
		})
		permutations.Shuffle(len(candidate.Projects), func(left, right int) {
			candidate.Projects[left], candidate.Projects[right] = candidate.Projects[right], candidate.Projects[left]
		})
		got, err := BuildLMWikiSnapshot(candidate)

		if err != nil {
			t.Fatal(err)
		}
		if string(got.CanonicalJSON) != string(want.CanonicalJSON) || got.SourceDigest != want.SourceDigest || !equalStrings(citationKeys(got.Citations), citationKeys(want.Citations)) {
			t.Fatalf("permutation %d changed canonical snapshot\ngot:  %s\nwant: %s", i, got.CanonicalJSON, want.CanonicalJSON)
		}
	}
}

func TestLMWikiStaleRemoval_omitsDeletedSourceFromCompleteDesiredSnapshot(t *testing.T) {
	withBoth, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{Issues: []LMWikiIssue{{ID: "issue-a", Number: 1, Title: "A"}, {ID: "issue-b", Number: 2, Title: "B"}}})
	if err != nil {
		t.Fatal(err)
	}

	withCurrent, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{Issues: []LMWikiIssue{{ID: "issue-a", Number: 1, Title: "A"}}})

	if err != nil {
		t.Fatal(err)
	}
	if withBoth.SourceDigest == withCurrent.SourceDigest || strings.Contains(string(withCurrent.CanonicalJSON), "issue-b") || !equalStrings(citationKeys(withCurrent.Citations), []string{"issue:issue-a"}) {
		t.Fatalf("deleted source was not removed from desired snapshot: %s", withCurrent.CanonicalJSON)
	}
}

func TestLMWikiCanonical_returnsTypedErrorsWithoutPartialOutput_whenBoundsExceeded(t *testing.T) {
	contentSources := LMWikiSourceSnapshot{}
	for i := 0; i < 33; i++ {
		contentSources.Issues = append(contentSources.Issues, LMWikiIssue{ID: "issue-" + string(rune('a'+i)), Title: strings.Repeat("x", lmWikiMaxMetadataBytes-200)})
	}
	tests := []struct {
		name    string
		sources LMWikiSourceSnapshot
		wantErr error
	}{
		{"content", contentSources, ErrLMWikiContentTooLarge},
		{"metadata", LMWikiSourceSnapshot{Issues: []LMWikiIssue{{ID: "issue-a", Title: strings.Repeat("x", lmWikiMaxMetadataBytes)}}}, ErrLMWikiMetadataTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, err := BuildLMWikiSnapshot(tc.sources)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("BuildLMWikiSnapshot() error = %v, want %v", err, tc.wantErr)
			}
			if len(snapshot.CanonicalJSON) != 0 || len(snapshot.Citations) != 0 || snapshot.SourceDigest != "" {
				t.Fatalf("oversized input returned partial snapshot: %#v", snapshot)
			}
		})
	}
}

func citationKeys(citations []LMWikiCitation) []string {
	keys := make([]string, len(citations))
	for i, citation := range citations {
		keys[i] = citation.CitationKey
	}
	return keys
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
