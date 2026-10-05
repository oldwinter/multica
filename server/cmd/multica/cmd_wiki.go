package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/util"
)

var wikiCmd = &cobra.Command{
	Use:   "wiki",
	Short: "Read Wiki knowledge and propose reviewed changes",
	Long: "Reads revisioned Workspace Wiki knowledge. Agents can search and cite " +
		"exact revisions, but shared changes must be submitted as human-reviewed proposals.",
}

var wikiListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Wiki pages in an eligible scope",
	Args:  cobra.NoArgs,
	RunE:  runWikiList,
}

var wikiGetCmd = &cobra.Command{
	Use:   "get <page-id>",
	Short: "Get a Wiki page with its exact revision and provenance",
	Args:  exactArgs(1),
	RunE:  runWikiGet,
}

var wikiSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Wiki titles, paths, and Markdown content",
	Args:  exactArgs(1),
	RunE:  runWikiSearch,
}

var wikiProposeCmd = &cobra.Command{
	Use:   "propose <page-id>",
	Short: "Propose a reviewed change to a shared Wiki page",
	Long: "Submits an Agent-authored change against an exact base revision. " +
		"The proposal does not mutate the page; a human must review it before it can create a revision. " +
		"Retries must reuse the same idempotency key.",
	Args: exactArgs(1),
	RunE: runWikiPropose,
}

func init() {
	wikiCmd.AddCommand(wikiListCmd)
	wikiCmd.AddCommand(wikiGetCmd)
	wikiCmd.AddCommand(wikiSearchCmd)
	wikiCmd.AddCommand(wikiProposeCmd)

	addWikiScopeFlags(wikiListCmd)
	wikiListCmd.Flags().String("output", "table", "Output format: table or json")

	wikiGetCmd.Flags().String("output", "json", "Output format: table or json")

	addWikiScopeFlags(wikiSearchCmd)
	wikiSearchCmd.Flags().String("output", "table", "Output format: table or json")

	wikiProposeCmd.Flags().Int64("base-revision", 0, "Exact current page revision number")
	wikiProposeCmd.Flags().String("path", "", "Proposed relative .md path")
	wikiProposeCmd.Flags().String("title", "", "Proposed page title")
	wikiProposeCmd.Flags().String("content", "", "Proposed Markdown content (decodes \\n, \\r, \\t, and \\\\)")
	wikiProposeCmd.Flags().String("content-file", "", "Read proposed Markdown from a UTF-8 file inside the working directory")
	wikiProposeCmd.Flags().Bool("allow-external-file", false, "Allow --content-file outside the working directory")
	wikiProposeCmd.Flags().String("rationale", "", "Why this knowledge change is needed")
	wikiProposeCmd.Flags().StringArray("evidence-ref", nil, "Evidence reference such as task:<id> (may be repeated)")
	wikiProposeCmd.Flags().String("agent-id", "", "Agent author ID (defaults to MULTICA_AGENT_ID in task context)")
	wikiProposeCmd.Flags().String("idempotency-key", "", "Stable key reused for retries")
	wikiProposeCmd.Flags().String("output", "json", "Output format: table or json")
}

func addWikiScopeFlags(cmd *cobra.Command) {
	cmd.Flags().String("scope", "workspace", "Wiki scope: workspace, project, or user")
	cmd.Flags().String("project-id", "", "Project ID (required when --scope project)")
}

type wikiPageSummary struct {
	ID                    string  `json:"id"`
	WorkspaceID           *string `json:"workspace_id"`
	Scope                 string  `json:"scope"`
	ProjectID             *string `json:"project_id"`
	OwnerUserID           *string `json:"owner_user_id"`
	Path                  string  `json:"path"`
	Title                 string  `json:"title"`
	CreatedBy             *string `json:"created_by"`
	CurrentRevisionNumber int64   `json:"current_revision_number"`
	CurrentRevisionID     string  `json:"current_revision_id"`
	ContentDigest         string  `json:"content_digest"`
	LastSourceKind        string  `json:"last_source_kind"`
	LastActorType         string  `json:"last_actor_type"`
	LastActorID           *string `json:"last_actor_id"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

type wikiPage struct {
	wikiPageSummary
	Content string `json:"content"`
}

type wikiProposal struct {
	ID                 string   `json:"id"`
	PageID             string   `json:"page_id"`
	BaseRevisionNumber int64    `json:"base_revision_number"`
	ProposedPath       string   `json:"proposed_path"`
	ProposedTitle      string   `json:"proposed_title"`
	ProposedContent    string   `json:"proposed_content"`
	ContentDigest      string   `json:"content_digest"`
	Rationale          string   `json:"rationale"`
	EvidenceRefs       []string `json:"evidence_refs"`
	AgentID            string   `json:"agent_id"`
	IdempotencyKey     string   `json:"idempotency_key"`
	Status             string   `json:"status"`
	ReviewedByID       *string  `json:"reviewed_by_id"`
	ReviewReason       *string  `json:"review_reason"`
	ReviewedAt         *string  `json:"reviewed_at"`
	AcceptedRevisionID *string  `json:"accepted_revision_id"`
	CreatedAt          string   `json:"created_at"`
}

type createWikiProposalRequest struct {
	BaseRevisionNumber int64    `json:"base_revision_number"`
	ProposedPath       string   `json:"proposed_path"`
	ProposedTitle      string   `json:"proposed_title"`
	ProposedContent    string   `json:"proposed_content"`
	Rationale          string   `json:"rationale"`
	EvidenceRefs       []string `json:"evidence_refs"`
	AgentID            string   `json:"agent_id"`
	IdempotencyKey     string   `json:"idempotency_key"`
}

func runWikiList(cmd *cobra.Command, _ []string) error {
	path, err := wikiScopedPath(cmd, "/api/wiki/pages", nil)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var pages []wikiPageSummary
	if err := client.GetJSON(ctx, path, &pages); err != nil {
		return fmt.Errorf("list wiki pages: %w", err)
	}
	return printWikiPageSummaries(cmd, pages)
}

func runWikiGet(cmd *cobra.Command, args []string) error {
	pageID := strings.TrimSpace(args[0])
	if pageID == "" {
		return fmt.Errorf("page id cannot be empty")
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var page wikiPage
	path := "/api/wiki/pages/" + url.PathEscape(pageID)
	if err := client.GetJSON(ctx, path, &page); err != nil {
		return fmt.Errorf("get wiki page: %w", err)
	}
	return printWikiPage(cmd, page)
}

func runWikiSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(args[0])
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}
	path, err := wikiScopedPath(cmd, "/api/wiki/search", url.Values{"q": []string{query}})
	if err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var pages []wikiPageSummary
	if err := client.GetJSON(ctx, path, &pages); err != nil {
		return fmt.Errorf("search wiki pages: %w", err)
	}
	return printWikiPageSummaries(cmd, pages)
}

func runWikiPropose(cmd *cobra.Command, args []string) error {
	pageID := strings.TrimSpace(args[0])
	if pageID == "" {
		return fmt.Errorf("page id cannot be empty")
	}
	body, err := buildWikiProposalRequest(cmd)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var proposal wikiProposal
	path := "/api/wiki/pages/" + url.PathEscape(pageID) + "/proposals"
	if err := client.PostJSON(ctx, path, body, &proposal); err != nil {
		return fmt.Errorf("propose wiki edit: %w", err)
	}
	return printWikiProposal(cmd, proposal)
}

func wikiScopedPath(cmd *cobra.Command, endpoint string, params url.Values) (string, error) {
	if params == nil {
		params = url.Values{}
	}
	scope, _ := cmd.Flags().GetString("scope")
	scope = strings.TrimSpace(scope)
	switch scope {
	case "workspace", "user":
	case "project":
	default:
		return "", fmt.Errorf("--scope must be workspace, project, or user")
	}
	projectID, _ := cmd.Flags().GetString("project-id")
	projectID = strings.TrimSpace(projectID)
	if scope == "project" && projectID == "" {
		return "", fmt.Errorf("--project-id is required when --scope project")
	}
	if scope != "project" && projectID != "" {
		return "", fmt.Errorf("--project-id can only be used with --scope project")
	}
	params.Set("scope", scope)
	if projectID != "" {
		params.Set("project_id", projectID)
	}
	return endpoint + "?" + params.Encode(), nil
}

func buildWikiProposalRequest(cmd *cobra.Command) (createWikiProposalRequest, error) {
	baseRevision, _ := cmd.Flags().GetInt64("base-revision")
	if baseRevision < 1 {
		return createWikiProposalRequest{}, fmt.Errorf("--base-revision must be at least 1")
	}
	proposedPath, err := requiredWikiStringFlag(cmd, "path")
	if err != nil {
		return createWikiProposalRequest{}, err
	}
	proposedTitle, err := requiredWikiStringFlag(cmd, "title")
	if err != nil {
		return createWikiProposalRequest{}, err
	}
	proposedContent, err := resolveWikiProposalContent(cmd)
	if err != nil {
		return createWikiProposalRequest{}, err
	}
	rationale, err := requiredWikiStringFlag(cmd, "rationale")
	if err != nil {
		return createWikiProposalRequest{}, err
	}
	agentID, err := resolveWikiProposalAgentID(cmd)
	if err != nil {
		return createWikiProposalRequest{}, err
	}
	idempotencyKey, err := requiredWikiStringFlag(cmd, "idempotency-key")
	if err != nil {
		return createWikiProposalRequest{}, err
	}
	evidenceRefs, _ := cmd.Flags().GetStringArray("evidence-ref")
	cleanEvidence := make([]string, 0, len(evidenceRefs))
	for _, ref := range evidenceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return createWikiProposalRequest{}, fmt.Errorf("--evidence-ref cannot be empty")
		}
		cleanEvidence = append(cleanEvidence, ref)
	}
	if len(cleanEvidence) == 0 {
		return createWikiProposalRequest{}, fmt.Errorf("at least one --evidence-ref is required")
	}

	return createWikiProposalRequest{
		BaseRevisionNumber: baseRevision,
		ProposedPath:       proposedPath,
		ProposedTitle:      proposedTitle,
		ProposedContent:    proposedContent,
		Rationale:          rationale,
		EvidenceRefs:       cleanEvidence,
		AgentID:            agentID,
		IdempotencyKey:     idempotencyKey,
	}, nil
}

func resolveWikiProposalAgentID(cmd *cobra.Command) (string, error) {
	flagAgentID, _ := cmd.Flags().GetString("agent-id")
	flagAgentID = strings.TrimSpace(flagAgentID)
	taskAgentID := strings.TrimSpace(os.Getenv("MULTICA_AGENT_ID"))
	if taskAgentID != "" {
		if flagAgentID != "" && flagAgentID != taskAgentID {
			return "", fmt.Errorf("--agent-id must match MULTICA_AGENT_ID in task context")
		}
		return taskAgentID, nil
	}
	if flagAgentID == "" {
		return "", fmt.Errorf("agent identity is required: run in an Agent task or pass --agent-id for non-task debugging")
	}
	return flagAgentID, nil
}

func requiredWikiStringFlag(cmd *cobra.Command, name string) (string, error) {
	value, _ := cmd.Flags().GetString(name)
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--%s is required", name)
	}
	return value, nil
}

func resolveWikiProposalContent(cmd *cobra.Command) (string, error) {
	inline, _ := cmd.Flags().GetString("content")
	filePath, _ := cmd.Flags().GetString("content-file")
	inlineSet := cmd.Flags().Changed("content")
	fileSet := cmd.Flags().Changed("content-file")
	if inlineSet && fileSet {
		return "", fmt.Errorf("--content and --content-file are mutually exclusive")
	}
	if !inlineSet && !fileSet {
		return "", fmt.Errorf("one of --content or --content-file is required")
	}
	if inlineSet {
		if inline == "" {
			return "", fmt.Errorf("--content cannot be empty")
		}
		return util.UnescapeBackslashEscapes(inline), nil
	}
	if err := ensureFileFlagWithinWorkdir(cmd, "content-file", "content", filePath); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file for --content-file: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("file content for --content-file is empty")
	}
	return string(data), nil
}

func wikiOutputFormat(cmd *cobra.Command) (string, error) {
	output, _ := cmd.Flags().GetString("output")
	switch output {
	case "json", "table":
		return output, nil
	default:
		return "", fmt.Errorf("--output must be table or json")
	}
}

func printWikiPageSummaries(cmd *cobra.Command, pages []wikiPageSummary) error {
	output, err := wikiOutputFormat(cmd)
	if err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(os.Stdout, pages)
	}
	rows := make([][]string, 0, len(pages))
	for _, page := range pages {
		rows = append(rows, []string{
			page.ID,
			page.Scope,
			page.Path,
			page.Title,
			strconv.FormatInt(page.CurrentRevisionNumber, 10),
			wikiCitationKey(page.CurrentRevisionID),
			page.ContentDigest,
			page.LastSourceKind,
			wikiActor(page.LastActorType, page.LastActorID),
			page.UpdatedAt,
		})
	}
	cli.PrintTable(os.Stdout, []string{"ID", "SCOPE", "PATH", "TITLE", "REVISION", "CITATION", "DIGEST", "SOURCE", "ACTOR", "UPDATED"}, rows)
	return nil
}

func printWikiPage(cmd *cobra.Command, page wikiPage) error {
	output, err := wikiOutputFormat(cmd)
	if err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(os.Stdout, page)
	}
	cli.PrintTable(os.Stdout, []string{"FIELD", "VALUE"}, [][]string{
		{"ID", page.ID},
		{"SCOPE", page.Scope},
		{"PATH", page.Path},
		{"TITLE", page.Title},
		{"REVISION", strconv.FormatInt(page.CurrentRevisionNumber, 10)},
		{"CITATION", wikiCitationKey(page.CurrentRevisionID)},
		{"DIGEST", page.ContentDigest},
		{"SOURCE", page.LastSourceKind},
		{"ACTOR", wikiActor(page.LastActorType, page.LastActorID)},
		{"UPDATED", page.UpdatedAt},
		{"CONTENT", page.Content},
	})
	return nil
}

func printWikiProposal(cmd *cobra.Command, proposal wikiProposal) error {
	output, err := wikiOutputFormat(cmd)
	if err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(os.Stdout, proposal)
	}
	cli.PrintTable(os.Stdout, []string{"ID", "PAGE", "BASE_REVISION", "STATUS", "DIGEST", "AGENT", "EVIDENCE", "CREATED"}, [][]string{{
		proposal.ID,
		proposal.PageID,
		strconv.FormatInt(proposal.BaseRevisionNumber, 10),
		proposal.Status,
		proposal.ContentDigest,
		proposal.AgentID,
		strings.Join(proposal.EvidenceRefs, ","),
		proposal.CreatedAt,
	}})
	return nil
}

func wikiActor(actorType string, actorID *string) string {
	if actorID == nil || *actorID == "" {
		return actorType
	}
	return actorType + ":" + *actorID
}

func wikiCitationKey(revisionID string) string {
	if revisionID == "" {
		return ""
	}
	return "wiki_page_revision:" + revisionID
}
