package service

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeLMWikiSourceClasses(t *testing.T) {
	t.Parallel()

	got, err := normalizeLMWikiSourceClasses([]string{"wiki_page", "issue", "issue", "project_resource"})
	if err != nil {
		t.Fatalf("normalize source classes: %v", err)
	}
	want := []string{"issue", "project_resource", "wiki_page"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("source classes = %#v, want %#v", got, want)
	}

	if _, err := normalizeLMWikiSourceClasses([]string{"personal_wiki"}); !errors.Is(err, ErrLMWikiInvalidSourcePolicy) {
		t.Fatalf("unknown source class error = %v, want ErrLMWikiInvalidSourcePolicy", err)
	}
}

func TestNormalizeLMWikiSourceClassesAllowsEverySharedClass(t *testing.T) {
	t.Parallel()

	want := []string{"autopilot_run", "issue", "project", "project_resource", "wiki_page"}
	got, err := normalizeLMWikiSourceClasses([]string{"wiki_page", "project_resource", "project", "issue", "autopilot_run"})
	if err != nil {
		t.Fatalf("normalize shared source classes: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shared source classes = %#v, want %#v", got, want)
	}
}

func TestNormalizeLMWikiSourceWikiPagesCanonicalizesAndRejectsConflicts(t *testing.T) {
	t.Parallel()

	got, err := normalizeLMWikiSourceWikiPages([]LMWikiSourceWikiPage{
		{PageID: "BBBBBBBB-BBBB-BBBB-BBBB-BBBBBBBBBBBB", RevisionNumber: 3},
		{PageID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", RevisionNumber: 1},
		{PageID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", RevisionNumber: 3},
	})
	if err != nil {
		t.Fatalf("normalize Wiki page sources: %v", err)
	}
	want := []LMWikiSourceWikiPage{
		{PageID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", RevisionNumber: 1},
		{PageID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", RevisionNumber: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wiki page sources = %#v, want %#v", got, want)
	}

	_, err = normalizeLMWikiSourceWikiPages([]LMWikiSourceWikiPage{
		{PageID: want[0].PageID, RevisionNumber: 1},
		{PageID: want[0].PageID, RevisionNumber: 2},
	})
	if !errors.Is(err, ErrLMWikiInvalidSourcePolicy) {
		t.Fatalf("conflicting selection error = %v, want ErrLMWikiInvalidSourcePolicy", err)
	}
}

func TestLMWikiSourcePolicyExpectationDetectsStaleVersionOrDigest(t *testing.T) {
	t.Parallel()

	current, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{SourceClasses: []string{"issue"}}, 4)
	if err != nil {
		t.Fatalf("build current policy: %v", err)
	}
	if err := checkLMWikiSourcePolicyExpectation(current, LMWikiSourcePolicyExpectation{
		PolicyVersion: current.PolicyVersion, PolicyDigest: current.PolicyDigest,
	}); err != nil {
		t.Fatalf("matching expectation: %v", err)
	}
	for _, expectation := range []LMWikiSourcePolicyExpectation{
		{PolicyVersion: 3, PolicyDigest: current.PolicyDigest},
		{PolicyVersion: current.PolicyVersion, PolicyDigest: "sha256:stale"},
	} {
		err := checkLMWikiSourcePolicyExpectation(current, expectation)
		var stale *LMWikiSourcePolicyStaleError
		if !errors.As(err, &stale) || stale.Current.PolicyVersion != current.PolicyVersion {
			t.Fatalf("stale expectation error = %#v, want current policy", err)
		}
	}
}

func TestLMWikiPolicyPinsRevisionRequiresEnabledClassAndExactRevision(t *testing.T) {
	t.Parallel()

	state := LMWikiSourcePolicyState{LMWikiSourcePolicy: LMWikiSourcePolicy{
		SourceClasses: []string{"issue", "wiki_page"},
		WikiPages:     []LMWikiSourceWikiPage{{PageID: "page-1", RevisionNumber: 7}},
	}}
	if !lmWikiPolicyPinsRevision(state, "page-1", 7) {
		t.Fatal("exact enabled revision was not recognized")
	}
	if lmWikiPolicyPinsRevision(state, "page-1", 8) {
		t.Fatal("newer revision was treated as pinned")
	}
	state.SourceClasses = []string{"issue"}
	if lmWikiPolicyPinsRevision(state, "page-1", 7) {
		t.Fatal("disabled Wiki class was treated as pinned")
	}
}

func TestLMWikiSourcePolicyStateCanonicalizesDigestAndExclusions(t *testing.T) {
	t.Parallel()

	first, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{
		SourceClasses: []string{"wiki_page", "issue"},
		WikiPages: []LMWikiSourceWikiPage{
			{PageID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", RevisionNumber: 2},
			{PageID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", RevisionNumber: 1},
		},
		RemoteGenerationEnabled: true,
	}, 7)
	if err != nil {
		t.Fatalf("build first source policy state: %v", err)
	}
	second, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{
		SourceClasses: []string{"issue", "wiki_page"},
		WikiPages: []LMWikiSourceWikiPage{
			{PageID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", RevisionNumber: 1},
			{PageID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", RevisionNumber: 2},
		},
		RemoteGenerationEnabled: true,
	}, 7)
	if err != nil {
		t.Fatalf("build second source policy state: %v", err)
	}
	if first.PolicyDigest != second.PolicyDigest || !strings.HasPrefix(first.PolicyDigest, "sha256:") {
		t.Fatalf("canonical policy digests differ: %q, %q", first.PolicyDigest, second.PolicyDigest)
	}
	if !reflect.DeepEqual(first.SourceClasses, []string{"issue", "wiki_page"}) || first.WikiPages[0].PageID[0] != 'a' {
		t.Fatalf("policy response is not canonical: %+v", first)
	}
	if len(first.Exclusions) != 2 || first.Exclusions[0].State != "always_excluded" || first.Exclusions[1].State != "always_excluded" {
		t.Fatalf("permanent exclusions = %#v", first.Exclusions)
	}

	blocked := first
	blocked.RemoteGenerationEnabled = false
	blocked, err = newLMWikiSourcePolicyState(blocked.LMWikiSourcePolicy, blocked.PolicyVersion)
	if err != nil {
		t.Fatalf("build blocked source policy state: %v", err)
	}
	if blocked.PolicyDigest == first.PolicyDigest {
		t.Fatal("remote-generation decision did not change policy digest")
	}
}

func TestBuildLMWikiSnapshotV2EmptyWikiPagesIsDeterministic(t *testing.T) {
	t.Parallel()

	policy := LMWikiEgressPolicy{
		RemoteGenerationEnabled: false,
		PolicyVersion:           3,
		PolicyDigest:            "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	first, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{EgressPolicy: policy})
	if err != nil {
		t.Fatalf("build first snapshot: %v", err)
	}
	second, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{EgressPolicy: policy})
	if err != nil {
		t.Fatalf("build second snapshot: %v", err)
	}
	if first.SourceDigest != second.SourceDigest || !reflect.DeepEqual(first.CanonicalJSON, second.CanonicalJSON) {
		t.Fatalf("empty snapshots differ: first=%s second=%s", first.CanonicalJSON, second.CanonicalJSON)
	}
	if got := string(first.CanonicalJSON); got != `{"schema_version":2,"egress_policy":{"remote_generation_enabled":false,"policy_version":3,"policy_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"issues":[],"projects":[],"project_resources":[],"autopilot_runs":[],"wiki_pages":[]}` {
		t.Fatalf("canonical empty v2 snapshot = %s", got)
	}
}

func TestBuildLMWikiSnapshotV2EgressPolicyChangesSourceDigest(t *testing.T) {
	t.Parallel()

	base := LMWikiSourceSnapshot{EgressPolicy: LMWikiEgressPolicy{
		PolicyVersion: 1,
		PolicyDigest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	blocked, err := BuildLMWikiSnapshot(base)
	if err != nil {
		t.Fatalf("build blocked snapshot: %v", err)
	}
	base.EgressPolicy.RemoteGenerationEnabled = true
	enabled, err := BuildLMWikiSnapshot(base)
	if err != nil {
		t.Fatalf("build enabled snapshot: %v", err)
	}
	if blocked.SourceDigest == enabled.SourceDigest {
		t.Fatal("egress decision did not change revision source digest")
	}
	if blocked.Content.EgressPolicy.RemoteGenerationEnabled || !enabled.Content.EgressPolicy.RemoteGenerationEnabled {
		t.Fatalf("egress policies = blocked %#v enabled %#v", blocked.Content.EgressPolicy, enabled.Content.EgressPolicy)
	}
}

func TestBuildLMWikiSnapshotV2PinsWorkspaceWikiRevision(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 8, 23, 9, 30, 0, 0, time.UTC)
	snapshot, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{
		WikiPages: []LMWikiPageRevision{
			{
				ID: "11111111-1111-1111-1111-111111111111", PageID: "22222222-2222-2222-2222-222222222222",
				Scope: "workspace", RevisionNumber: 3, Path: "engineering/review.md", Title: "Review policy",
				Content: "Require evidence before approval.", ContentDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				CreatedAt: updatedAt,
			},
		},
	})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if snapshot.Content.SchemaVersion != 2 {
		t.Fatalf("schema version = %d, want 2", snapshot.Content.SchemaVersion)
	}
	if len(snapshot.Content.WikiPages) != 1 {
		t.Fatalf("wiki pages = %#v", snapshot.Content.WikiPages)
	}
	page := snapshot.Content.WikiPages[0]
	if page.RevisionNumber != 3 || page.CitationKey != "wiki_page_revision:11111111-1111-1111-1111-111111111111" {
		t.Fatalf("wiki page = %#v", page)
	}
	if len(snapshot.Citations) != 1 || snapshot.Citations[0].SourceType != "wiki_page_revision" || snapshot.Citations[0].SourceID != page.RevisionID {
		t.Fatalf("citations = %#v", snapshot.Citations)
	}

	var canonical map[string]any
	if err := json.Unmarshal(snapshot.CanonicalJSON, &canonical); err != nil {
		t.Fatalf("decode canonical snapshot: %v", err)
	}
	if canonical["schema_version"] != float64(2) {
		t.Fatalf("canonical schema version = %#v", canonical["schema_version"])
	}
}

func TestBuildLMWikiSnapshotV2ExcludesPersonalWikiDefenseInDepth(t *testing.T) {
	t.Parallel()

	snapshot, err := BuildLMWikiSnapshot(LMWikiSourceSnapshot{WikiPages: []LMWikiPageRevision{{
		ID: "11111111-1111-1111-1111-111111111111", PageID: "22222222-2222-2222-2222-222222222222",
		Scope: "user", RevisionNumber: 1, Path: "private.md", Title: "Private", Content: "never shared",
	}}})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if len(snapshot.Content.WikiPages) != 0 || len(snapshot.Citations) != 0 {
		t.Fatalf("personal Wiki leaked into snapshot: content=%#v citations=%#v", snapshot.Content.WikiPages, snapshot.Citations)
	}
}
