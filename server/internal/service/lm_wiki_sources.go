package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type lmWikiCitationInsert struct {
	Ordinal         int             `json:"ordinal"`
	CitationKey     string          `json:"citation_key"`
	SourceType      string          `json:"source_type"`
	SourceID        string          `json:"source_id"`
	SourceUpdatedAt *string         `json:"source_updated_at"`
	Locator         string          `json:"locator"`
	Label           string          `json:"label"`
	SafeMetadata    json.RawMessage `json:"safe_metadata"`
	SourceDigest    string          `json:"source_digest"`
}

func marshalLMWikiCitations(citations []LMWikiCitation) ([]byte, error) {
	rows := make([]lmWikiCitationInsert, len(citations))
	for index, citation := range citations {
		row := lmWikiCitationInsert{
			Ordinal: index, CitationKey: citation.CitationKey, SourceType: citation.SourceType,
			SourceID: citation.SourceID, Locator: citation.Locator, Label: citation.Label,
			SafeMetadata: citation.SafeMetadata, SourceDigest: citation.SourceDigest,
		}
		if citation.SourceUpdatedAt != "" {
			row.SourceUpdatedAt = &citation.SourceUpdatedAt
		}
		rows[index] = row
	}
	value, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("marshal lm wiki citations: %w", err)
	}
	return value, nil
}

func loadLMWikiSnapshot(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) (LMWikiCanonicalSnapshot, error) {
	policy, err := loadLMWikiSourcePolicyState(ctx, queries, workspaceID)
	if err != nil {
		return LMWikiCanonicalSnapshot{}, err
	}
	enabled := make(map[string]bool, len(policy.SourceClasses))
	for _, sourceClass := range policy.SourceClasses {
		enabled[sourceClass] = true
	}
	sources := LMWikiSourceSnapshot{EgressPolicy: LMWikiEgressPolicy{
		RemoteGenerationEnabled: policy.RemoteGenerationEnabled,
		PolicyVersion:           policy.PolicyVersion,
		PolicyDigest:            policy.PolicyDigest,
	}}
	if enabled["issue"] {
		issues, err := queries.ListLMWikiSourceIssues(ctx, workspaceID)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("list lm wiki issues: %w", err)
		}
		for _, row := range issues {
			sources.Issues = append(sources.Issues, LMWikiIssue{ID: row.ID, Number: row.Number, Title: row.Title, Description: row.Description, Status: row.Status, Priority: row.Priority, ProjectID: row.ProjectID, StartDate: row.StartDate, DueDate: row.DueDate, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time})
		}
	}
	if enabled["project"] {
		projects, err := queries.ListLMWikiSourceProjects(ctx, workspaceID)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("list lm wiki projects: %w", err)
		}
		for _, row := range projects {
			sources.Projects = append(sources.Projects, LMWikiProject{ID: row.ID, Title: row.Title, Description: row.Description, Status: row.Status, Priority: row.Priority, StartDate: row.StartDate, DueDate: row.DueDate, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time})
		}
	}
	if enabled["project_resource"] {
		resources, err := queries.ListLMWikiSourceProjectResources(ctx, workspaceID)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("list lm wiki resources: %w", err)
		}
		for _, row := range resources {
			sources.ProjectResources = append(sources.ProjectResources, LMWikiProjectResource{ID: row.ID, ProjectID: row.ProjectID, ResourceType: row.ResourceType, Label: row.Label, Position: row.Position, GitURL: row.GitUrl, Ref: row.Ref, DefaultBranchHint: row.DefaultBranchHint})
		}
	}
	if enabled["autopilot_run"] {
		runs, err := queries.ListLMWikiSourceAutopilotRuns(ctx, workspaceID)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("list lm wiki autopilot runs: %w", err)
		}
		for _, row := range runs {
			sources.AutopilotRuns = append(sources.AutopilotRuns, LMWikiAutopilotRun{ID: row.ID, AutopilotID: row.AutopilotID, AutopilotTitle: row.AutopilotTitle, Status: row.Status, Source: row.Source, IssueID: row.IssueID, TriggeredAt: row.TriggeredAt.Time, CompletedAt: row.CompletedAt.Time})
		}
	}
	if enabled["wiki_page"] {
		pages, err := queries.ListLMWikiSourceWikiPageRevisions(ctx, workspaceID)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("list lm wiki Wiki page revisions: %w", err)
		}
		for _, row := range pages {
			sources.WikiPages = append(sources.WikiPages, LMWikiPageRevision{
				ID: row.RevisionID, PageID: row.PageID, Scope: row.Scope, ProjectID: row.ProjectID,
				RevisionNumber: row.RevisionNumber, Path: row.Path, Title: row.Title,
				Content: row.Content, ContentDigest: row.ContentDigest, CreatedAt: row.CreatedAt.Time,
			})
		}
	}
	snapshot, err := BuildLMWikiSnapshot(sources)
	if err != nil {
		return LMWikiCanonicalSnapshot{}, fmt.Errorf("build lm wiki snapshot: %w", err)
	}
	return snapshot, nil
}
