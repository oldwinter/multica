package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeriveWikiKnowledgeReadinessStatesAndMaintenance(t *testing.T) {
	t.Parallel()

	policy, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{
		SourceClasses: []string{"issue", "wiki_page"},
		WikiPages: []LMWikiSourceWikiPage{
			{PageID: "22222222-2222-2222-2222-222222222222", RevisionNumber: 2},
			{PageID: "33333333-3333-3333-3333-333333333333", RevisionNumber: 4},
			{PageID: "44444444-4444-4444-4444-444444444444", RevisionNumber: 1},
		},
	}, 8)
	if err != nil {
		t.Fatalf("build policy: %v", err)
	}
	result := deriveWikiKnowledgeReadiness(wikiKnowledgeReadinessInput{
		WorkspaceID: "workspace-1", Policy: policy,
		Latest: &wikiKnowledgeLMWikiSnapshot{
			RevisionID: "lm-wiki-1", PolicyVersion: policy.PolicyVersion,
			PolicyDigest: policy.PolicyDigest, Reviewed: false,
		},
		Sources: []wikiKnowledgeSourceSnapshot{
			{PageID: "11111111-1111-1111-1111-111111111111", Scope: "workspace", CurrentRevisionID: "rev-1", CurrentRevision: 1},
			{PageID: "22222222-2222-2222-2222-222222222222", Scope: "workspace", SelectedRevisionID: "rev-2", SelectedRevision: 2, CurrentRevisionID: "rev-3", CurrentRevision: 3},
			{PageID: "33333333-3333-3333-3333-333333333333", Scope: "project", ProjectID: "project-1", SelectedRevisionID: "rev-4", SelectedRevision: 4, CurrentRevisionID: "rev-4", CurrentRevision: 4},
			{PageID: "44444444-4444-4444-4444-444444444444", SelectedRevisionID: "rev-deleted", SelectedRevision: 1, Deleted: true},
		},
	})

	wantStates := []WikiKnowledgeSourceState{
		WikiKnowledgeEligibleUnpinned,
		WikiKnowledgeNewerRevisionAvailable,
		WikiKnowledgePinnedCurrent,
		WikiKnowledgeSourceDeleted,
	}
	if len(result.Sources) != len(wantStates) {
		t.Fatalf("sources = %#v", result.Sources)
	}
	for index, want := range wantStates {
		if result.Sources[index].State != want {
			t.Fatalf("source %d state = %q, want %q", index, result.Sources[index].State, want)
		}
	}
	if len(result.MaintenanceItems) != 3 {
		t.Fatalf("maintenance items = %#v, want newer, deleted, and review", result.MaintenanceItems)
	}
	kinds := make(map[string]struct{}, len(result.MaintenanceItems))
	for _, item := range result.MaintenanceItems {
		kinds[item.Kind] = struct{}{}
	}
	for _, want := range []string{"source_newer_revision", "source_deleted", "lm_wiki_review_pending"} {
		if _, ok := kinds[want]; !ok {
			t.Fatalf("maintenance kinds = %#v, missing %q", kinds, want)
		}
	}
}

func TestDeriveWikiKnowledgeReadinessExcludedAndPolicyStalePrecedence(t *testing.T) {
	t.Parallel()

	disabled, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{
		SourceClasses: []string{"issue"},
		WikiPages:     []LMWikiSourceWikiPage{{PageID: "page-1", RevisionNumber: 2}},
	}, 3)
	if err != nil {
		t.Fatalf("build disabled policy: %v", err)
	}
	disabledResult := deriveWikiKnowledgeReadiness(wikiKnowledgeReadinessInput{
		WorkspaceID: "workspace-1", Policy: disabled,
		Sources: []wikiKnowledgeSourceSnapshot{{PageID: "page-1", SelectedRevision: 2, CurrentRevision: 2, CurrentRevisionID: "revision-2"}},
	})
	if got := disabledResult.Sources[0]; got.State != WikiKnowledgeExcluded || got.NextAction.Kind != "pin_revision" {
		t.Fatalf("disabled source = %#v", got)
	}
	if len(disabledResult.MaintenanceItems) != 1 || disabledResult.MaintenanceItems[0].Kind != "source_excluded" {
		t.Fatalf("disabled maintenance = %#v", disabledResult.MaintenanceItems)
	}
	unselectedResult := deriveWikiKnowledgeReadiness(wikiKnowledgeReadinessInput{
		WorkspaceID: "workspace-1", Policy: disabled,
		Sources: []wikiKnowledgeSourceSnapshot{{PageID: "page-2", Scope: "workspace", CurrentRevision: 1, CurrentRevisionID: "revision-1"}},
	})
	if got := unselectedResult.Sources[0]; got.State != WikiKnowledgeEligibleUnpinned || got.NextAction.Kind != "pin_revision" {
		t.Fatalf("unselected shared source = %#v", got)
	}

	enabled, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{
		SourceClasses: []string{"issue", "wiki_page"},
		WikiPages:     []LMWikiSourceWikiPage{{PageID: "page-1", RevisionNumber: 2}},
	}, 4)
	if err != nil {
		t.Fatalf("build enabled policy: %v", err)
	}
	staleResult := deriveWikiKnowledgeReadiness(wikiKnowledgeReadinessInput{
		WorkspaceID: "workspace-1", Policy: enabled,
		Sources: []wikiKnowledgeSourceSnapshot{{PageID: "page-1", SelectedRevision: 2, CurrentRevision: 2, CurrentRevisionID: "revision-2"}},
		Latest:  &wikiKnowledgeLMWikiSnapshot{RevisionID: "lm-old", PolicyVersion: 3, PolicyDigest: "sha256:old"},
	})
	if got := staleResult.Sources[0]; got.State != WikiKnowledgePolicyStale || got.NextAction.Kind != "refresh_lm_wiki" {
		t.Fatalf("stale source = %#v", got)
	}
	if len(staleResult.MaintenanceItems) != 1 || staleResult.MaintenanceItems[0].Kind != "policy_stale" {
		t.Fatalf("stale maintenance = %#v", staleResult.MaintenanceItems)
	}
}

func TestWikiKnowledgeReadinessPayloadContainsNoWikiBodyOrPath(t *testing.T) {
	t.Parallel()

	policy, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{SourceClasses: []string{"wiki_page"}}, 1)
	if err != nil {
		t.Fatalf("build policy: %v", err)
	}
	result := deriveWikiKnowledgeReadiness(wikiKnowledgeReadinessInput{
		WorkspaceID: "workspace-1", Policy: policy,
		Sources: []wikiKnowledgeSourceSnapshot{{PageID: "page-1", Scope: "workspace", CurrentRevisionID: "revision-1", CurrentRevision: 1}},
	})
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal readiness: %v", err)
	}
	serialized := string(payload)
	for _, forbidden := range []string{"content", "path", "prompt", "citation", "credential", "local_path"} {
		if strings.Contains(serialized, `"`+forbidden+`"`) {
			t.Fatalf("readiness payload contains forbidden field %q: %s", forbidden, serialized)
		}
	}
}
