package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

const (
	wikiTestWorkspaceID = "11111111-1111-1111-1111-111111111111"
	wikiTestProjectID   = "22222222-2222-2222-2222-222222222222"
	wikiTestPageID      = "33333333-3333-3333-3333-333333333333"
	wikiTestAgentID     = "44444444-4444-4444-4444-444444444444"
	wikiTestRevisionID  = "55555555-5555-5555-5555-555555555555"
)

func newWikiTestCmd(serverURL string) *cobra.Command {
	cmd := &cobra.Command{Use: "wiki-test"}
	cmd.Flags().String("server-url", "", "")
	cmd.Flags().String("workspace-id", "", "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().String("scope", "workspace", "")
	cmd.Flags().String("project-id", "", "")
	cmd.Flags().String("output", "json", "")
	cmd.Flags().Int64("base-revision", 0, "")
	cmd.Flags().String("path", "", "")
	cmd.Flags().String("title", "", "")
	cmd.Flags().String("content", "", "")
	cmd.Flags().String("content-file", "", "")
	cmd.Flags().Bool("allow-external-file", false, "")
	cmd.Flags().String("rationale", "", "")
	cmd.Flags().StringArray("evidence-ref", nil, "")
	cmd.Flags().String("agent-id", "", "")
	cmd.Flags().String("idempotency-key", "", "")
	_ = cmd.Flags().Set("server-url", serverURL)
	_ = cmd.Flags().Set("workspace-id", wikiTestWorkspaceID)
	return cmd
}

func setWikiTestEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MULTICA_TOKEN", "test-token")
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv("MULTICA_DAEMON_PORT", "")
}

func wikiSummaryFixture() wikiPageSummary {
	workspaceID := wikiTestWorkspaceID
	actorID := wikiTestAgentID
	return wikiPageSummary{
		ID:                    wikiTestPageID,
		WorkspaceID:           &workspaceID,
		Scope:                 "workspace",
		Path:                  "concepts/agents.md",
		Title:                 "Agents",
		CurrentRevisionNumber: 7,
		CurrentRevisionID:     wikiTestRevisionID,
		ContentDigest:         "sha256:page-digest",
		LastSourceKind:        "agent_proposal",
		LastActorType:         "agent",
		LastActorID:           &actorID,
		CreatedAt:             "2026-08-23T08:00:00Z",
		UpdatedAt:             "2026-08-23T09:00:00Z",
	}
}

func TestWikiCommandExposesReadAndProposalContractOnly(t *testing.T) {
	got := make([]string, 0, len(wikiCmd.Commands()))
	for _, cmd := range wikiCmd.Commands() {
		got = append(got, cmd.Name())
	}
	slices.Sort(got)
	want := []string{"get", "list", "propose", "search"}
	if !slices.Equal(got, want) {
		t.Fatalf("wiki subcommands = %v, want %v", got, want)
	}

	for _, name := range []string{
		"base-revision", "path", "title", "content", "content-file", "rationale",
		"evidence-ref", "agent-id", "idempotency-key",
	} {
		if wikiProposeCmd.Flags().Lookup(name) == nil {
			t.Errorf("wiki propose missing --%s", name)
		}
	}
}

func TestRunWikiListUsesTypedScopedRouteAndPrintsProvenance(t *testing.T) {
	setWikiTestEnvironment(t)
	page := wikiSummaryFixture()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/wiki/pages" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("scope"); got != "project" {
			t.Fatalf("scope = %q, want project", got)
		}
		if got := r.URL.Query().Get("project_id"); got != wikiTestProjectID {
			t.Fatalf("project_id = %q, want %q", got, wikiTestProjectID)
		}
		if got := r.Header.Get("X-Workspace-ID"); got != wikiTestWorkspaceID {
			t.Fatalf("X-Workspace-ID = %q, want %q", got, wikiTestWorkspaceID)
		}
		_ = json.NewEncoder(w).Encode([]wikiPageSummary{page})
	}))
	defer srv.Close()

	cmd := newWikiTestCmd(srv.URL)
	_ = cmd.Flags().Set("scope", "project")
	_ = cmd.Flags().Set("project-id", wikiTestProjectID)
	out, err := captureStdout(t, func() error { return runWikiList(cmd, nil) })
	if err != nil {
		t.Fatalf("runWikiList: %v", err)
	}
	var got []wikiPageSummary
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0].CurrentRevisionNumber != 7 || got[0].CurrentRevisionID != wikiTestRevisionID || got[0].ContentDigest != "sha256:page-digest" {
		t.Fatalf("output lost revision/digest: %+v", got)
	}
	if got[0].LastSourceKind != "agent_proposal" || got[0].LastActorID == nil || *got[0].LastActorID != wikiTestAgentID {
		t.Fatalf("output lost provenance: %+v", got[0])
	}
}

func TestRunWikiGetPrintsExactPageContract(t *testing.T) {
	setWikiTestEnvironment(t)
	page := wikiPage{wikiPageSummary: wikiSummaryFixture(), Content: "# Agents\n\nUse citations."}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/wiki/pages/"+wikiTestPageID {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	cmd := newWikiTestCmd(srv.URL)
	out, err := captureStdout(t, func() error { return runWikiGet(cmd, []string{wikiTestPageID}) })
	if err != nil {
		t.Fatalf("runWikiGet: %v", err)
	}
	var got wikiPage
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if got.Content != page.Content || got.CurrentRevisionNumber != 7 || got.CurrentRevisionID != wikiTestRevisionID || got.ContentDigest != "sha256:page-digest" {
		t.Fatalf("page output = %+v", got)
	}
}

func TestRunWikiSearchEncodesQueryAndScope(t *testing.T) {
	setWikiTestEnvironment(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/wiki/search" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "quality bar + CJK 中文" {
			t.Fatalf("q = %q", got)
		}
		if got := r.URL.Query().Get("scope"); got != "workspace" {
			t.Fatalf("scope = %q", got)
		}
		_ = json.NewEncoder(w).Encode([]wikiPageSummary{wikiSummaryFixture()})
	}))
	defer srv.Close()

	cmd := newWikiTestCmd(srv.URL)
	out, err := captureStdout(t, func() error {
		return runWikiSearch(cmd, []string{"  quality bar + CJK 中文  "})
	})
	if err != nil {
		t.Fatalf("runWikiSearch: %v", err)
	}
	if !strings.Contains(out, "sha256:page-digest") || !strings.Contains(out, "agent_proposal") {
		t.Fatalf("search output missing revision provenance: %s", out)
	}
}

func TestWikiTableOutputIncludesExactRevisionCitation(t *testing.T) {
	setWikiTestEnvironment(t)
	cmd := newWikiTestCmd("http://127.0.0.1:0")
	_ = cmd.Flags().Set("output", "table")

	out, err := captureStdout(t, func() error {
		return printWikiPageSummaries(cmd, []wikiPageSummary{wikiSummaryFixture()})
	})
	if err != nil {
		t.Fatalf("print wiki summaries: %v", err)
	}
	if !strings.Contains(out, "CITATION") || !strings.Contains(out, "wiki_page_revision:"+wikiTestRevisionID) {
		t.Fatalf("table output missing exact revision citation: %s", out)
	}
}

func TestRunWikiProposePostsReviewableAgentContract(t *testing.T) {
	setWikiTestEnvironment(t)
	t.Setenv("MULTICA_TOKEN", "mat_test-token")
	t.Setenv("MULTICA_AGENT_ID", wikiTestAgentID)
	t.Setenv("MULTICA_TASK_ID", "task-9")
	var got createWikiProposalRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/wiki/pages/"+wikiTestPageID+"/proposals" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if headerAgentID := r.Header.Get("X-Agent-ID"); headerAgentID != wikiTestAgentID {
			t.Fatalf("X-Agent-ID = %q, want %q", headerAgentID, wikiTestAgentID)
		}
		if taskID := r.Header.Get("X-Task-ID"); taskID != "task-9" {
			t.Fatalf("X-Task-ID = %q, want task-9", taskID)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(wikiProposal{
			ID:                 "proposal-1",
			PageID:             wikiTestPageID,
			BaseRevisionNumber: 7,
			ProposedPath:       got.ProposedPath,
			ProposedTitle:      got.ProposedTitle,
			ProposedContent:    got.ProposedContent,
			ContentDigest:      "sha256:proposal-digest",
			Rationale:          got.Rationale,
			EvidenceRefs:       got.EvidenceRefs,
			AgentID:            got.AgentID,
			IdempotencyKey:     got.IdempotencyKey,
			Status:             "pending",
			CreatedAt:          "2026-08-23T10:00:00Z",
		})
	}))
	defer srv.Close()

	cmd := newWikiTestCmd(srv.URL)
	flags := map[string]string{
		"base-revision":   "7",
		"path":            "concepts/agents.md",
		"title":           "Agents",
		"content":         `# Agents\n\nUse exact citations.`,
		"rationale":       "Preserve the reviewed operating rule",
		"idempotency-key": "task-9:wiki-proposal",
	}
	for name, value := range flags {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}
	_ = cmd.Flags().Set("evidence-ref", "task:task-9")
	_ = cmd.Flags().Set("evidence-ref", "room:room-2")

	out, err := captureStdout(t, func() error { return runWikiPropose(cmd, []string{wikiTestPageID}) })
	if err != nil {
		t.Fatalf("runWikiPropose: %v", err)
	}
	if got.BaseRevisionNumber != 7 || got.ProposedContent != "# Agents\n\nUse exact citations." {
		t.Fatalf("proposal body = %+v", got)
	}
	if !slices.Equal(got.EvidenceRefs, []string{"task:task-9", "room:room-2"}) {
		t.Fatalf("evidence_refs = %v", got.EvidenceRefs)
	}
	if got.AgentID != wikiTestAgentID || got.IdempotencyKey != "task-9:wiki-proposal" {
		t.Fatalf("agent/idempotency contract = %+v", got)
	}
	var proposal wikiProposal
	if err := json.Unmarshal([]byte(out), &proposal); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if proposal.ContentDigest != "sha256:proposal-digest" || proposal.Status != "pending" {
		t.Fatalf("proposal output lost digest/status: %+v", proposal)
	}
}

func TestResolveWikiProposalContentFilePreservesExactBytes(t *testing.T) {
	setWikiTestEnvironment(t)
	file, err := os.CreateTemp(".", ".wiki-proposal-*.md")
	if err != nil {
		t.Fatalf("create content file: %v", err)
	}
	name := file.Name()
	t.Cleanup(func() { _ = os.Remove(name) })
	want := "# 标题\n\nKeep the final newline.\n"
	if _, err := file.WriteString(want); err != nil {
		t.Fatalf("write content file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close content file: %v", err)
	}

	cmd := newWikiTestCmd("http://127.0.0.1:0")
	if err := cmd.Flags().Set("content-file", name); err != nil {
		t.Fatal(err)
	}
	got, err := resolveWikiProposalContent(cmd)
	if err != nil {
		t.Fatalf("resolveWikiProposalContent: %v", err)
	}
	if got != want {
		t.Fatalf("content = %q, want exact %q", got, want)
	}
}

func TestRunWikiProposePreservesConflictCode(t *testing.T) {
	setWikiTestEnvironment(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"page changed","code":"wiki_revision_conflict","current_revision_number":8}`))
	}))
	defer srv.Close()

	cmd := newWikiTestCmd(srv.URL)
	for name, value := range map[string]string{
		"base-revision": "7", "path": "index.md", "title": "Home", "content": "# Home",
		"rationale": "Update guidance", "evidence-ref": "task:task-9", "agent-id": wikiTestAgentID,
		"idempotency-key": "stable-key",
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}

	_, err := captureStdout(t, func() error { return runWikiPropose(cmd, []string{wikiTestPageID}) })
	if err == nil {
		t.Fatal("expected conflict")
	}
	var httpErr *cli.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error chain lost *cli.HTTPError: %v", err)
	}
	if httpErr.StatusCode != http.StatusConflict || !strings.Contains(httpErr.Body, "wiki_revision_conflict") {
		t.Fatalf("HTTP conflict = %+v", httpErr)
	}
}

func TestRunWikiListPreservesPersonalScopeRejection(t *testing.T) {
	setWikiTestEnvironment(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != "user" {
			t.Fatalf("scope = %q, want user", r.URL.Query().Get("scope"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"machine actors cannot access personal wiki","code":"wiki_personal_scope_forbidden"}`))
	}))
	defer srv.Close()

	cmd := newWikiTestCmd(srv.URL)
	_ = cmd.Flags().Set("scope", "user")
	_, err := captureStdout(t, func() error { return runWikiList(cmd, nil) })
	if err == nil {
		t.Fatal("expected personal scope rejection")
	}
	var httpErr *cli.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error chain lost *cli.HTTPError: %v", err)
	}
	if httpErr.StatusCode != http.StatusForbidden || !strings.Contains(httpErr.Body, "wiki_personal_scope_forbidden") {
		t.Fatalf("personal scope error = %+v", httpErr)
	}
}

func TestBuildWikiProposalRequestRequiresAuthoritativeAgentIdentity(t *testing.T) {
	baseCommand := func() *cobra.Command {
		cmd := newWikiTestCmd("http://127.0.0.1:0")
		for name, value := range map[string]string{
			"base-revision": "7", "path": "index.md", "title": "Home", "content": "# Home",
			"rationale": "Update guidance", "evidence-ref": "task:task-9", "idempotency-key": "stable-key",
		} {
			if err := cmd.Flags().Set(name, value); err != nil {
				t.Fatalf("set --%s: %v", name, err)
			}
		}
		return cmd
	}

	t.Run("missing identity fails", func(t *testing.T) {
		t.Setenv("MULTICA_AGENT_ID", "")
		_, err := buildWikiProposalRequest(baseCommand())
		if err == nil || !strings.Contains(err.Error(), "agent identity is required") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("task identity cannot be spoofed by flag", func(t *testing.T) {
		t.Setenv("MULTICA_AGENT_ID", wikiTestAgentID)
		cmd := baseCommand()
		_ = cmd.Flags().Set("agent-id", "55555555-5555-5555-5555-555555555555")
		_, err := buildWikiProposalRequest(cmd)
		if err == nil || !strings.Contains(err.Error(), "must match MULTICA_AGENT_ID") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("task identity is the proposal author", func(t *testing.T) {
		t.Setenv("MULTICA_AGENT_ID", wikiTestAgentID)
		body, err := buildWikiProposalRequest(baseCommand())
		if err != nil {
			t.Fatalf("buildWikiProposalRequest: %v", err)
		}
		if body.AgentID != wikiTestAgentID {
			t.Fatalf("agent_id = %q, want task actor %q", body.AgentID, wikiTestAgentID)
		}
	})
}

func TestWikiScopeAndProposalValidationFailBeforeHTTP(t *testing.T) {
	t.Run("project scope needs project id", func(t *testing.T) {
		cmd := newWikiTestCmd("http://127.0.0.1:0")
		_ = cmd.Flags().Set("scope", "project")
		if err := runWikiList(cmd, nil); err == nil || !strings.Contains(err.Error(), "--project-id") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("project id cannot leak into another scope", func(t *testing.T) {
		cmd := newWikiTestCmd("http://127.0.0.1:0")
		_ = cmd.Flags().Set("scope", "user")
		_ = cmd.Flags().Set("project-id", wikiTestProjectID)
		if err := runWikiList(cmd, nil); err == nil || !strings.Contains(err.Error(), "only be used") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("proposal needs exact base and evidence", func(t *testing.T) {
		cmd := newWikiTestCmd("http://127.0.0.1:0")
		if _, err := buildWikiProposalRequest(cmd); err == nil || !strings.Contains(err.Error(), "base-revision") {
			t.Fatalf("error = %v", err)
		}
	})

}
