package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	lmWikiMaxContentBytes  = 2 * 1024 * 1024
	lmWikiMaxMetadataBytes = 64 * 1024
)

var (
	ErrLMWikiUnsafeSource     = errors.New("unsafe lm wiki source")
	ErrLMWikiContentTooLarge  = errors.New("lm wiki content exceeds size limit")
	ErrLMWikiMetadataTooLarge = errors.New("lm wiki citation metadata exceeds size limit")
)

type LMWikiUnsafeSourceError struct{ Reason string }

func (e *LMWikiUnsafeSourceError) Error() string { return "unsafe lm wiki source: " + e.Reason }
func (e *LMWikiUnsafeSourceError) Unwrap() error { return ErrLMWikiUnsafeSource }

type LMWikiSizeError struct {
	Target        string
	Limit, Actual int
	Cause         error
}

func (e *LMWikiSizeError) Error() string {
	return fmt.Sprintf("lm wiki %s is %d bytes; limit is %d", e.Target, e.Actual, e.Limit)
}
func (e *LMWikiSizeError) Unwrap() error { return e.Cause }

type LMWikiSourceSnapshot struct {
	EgressPolicy     LMWikiEgressPolicy
	Issues           []LMWikiIssue
	Projects         []LMWikiProject
	ProjectResources []LMWikiProjectResource
	AutopilotRuns    []LMWikiAutopilotRun
	WikiPages        []LMWikiPageRevision
}

type LMWikiEgressPolicy struct {
	RemoteGenerationEnabled bool   `json:"remote_generation_enabled"`
	PolicyVersion           int64  `json:"policy_version"`
	PolicyDigest            string `json:"policy_digest"`
}

type LMWikiIssue struct {
	ID                                                                  string
	Number                                                              int32
	Title, Description, Status, Priority, ProjectID, StartDate, DueDate string
	CreatedAt, UpdatedAt                                                time.Time
}

type LMWikiProject struct {
	ID, Title, Description, Status, Priority, StartDate, DueDate string
	CreatedAt, UpdatedAt                                         time.Time
}

type LMWikiProjectResource struct {
	ID, ProjectID, ResourceType, Label, GitURL, Ref, DefaultBranchHint string
	Position                                                           int32
}

type LMWikiAutopilotRun struct {
	ID, AutopilotID, AutopilotTitle, Status, Source, IssueID string
	TriggeredAt, CompletedAt                                 time.Time
}

type LMWikiPageRevision struct {
	ID, PageID, Scope, ProjectID, Path, Title, Content, ContentDigest string
	RevisionNumber                                                    int64
	CreatedAt                                                         time.Time
}

type LMWikiGitRef struct {
	Host              string `json:"host"`
	RepositoryPath    string `json:"repository_path"`
	Ref               string `json:"ref,omitempty"`
	DefaultBranchHint string `json:"default_branch_hint,omitempty"`
}

type LMWikiContent struct {
	SchemaVersion    int                     `json:"schema_version"`
	EgressPolicy     LMWikiEgressPolicy      `json:"egress_policy"`
	Issues           []lmWikiIssueContent    `json:"issues"`
	Projects         []lmWikiProjectContent  `json:"projects"`
	ProjectResources []lmWikiResourceContent `json:"project_resources"`
	AutopilotRuns    []lmWikiRunContent      `json:"autopilot_runs"`
	WikiPages        []lmWikiPageContent     `json:"wiki_pages"`
}

type LMWikiCanonicalSnapshot struct {
	Content       LMWikiContent
	Citations     []LMWikiCitation
	CanonicalJSON []byte
	SourceDigest  string
}

type LMWikiCitation struct {
	CitationKey     string          `json:"citation_key"`
	SourceType      string          `json:"source_type"`
	SourceID        string          `json:"source_id"`
	SourceUpdatedAt string          `json:"source_updated_at,omitempty"`
	Locator         string          `json:"locator"`
	Label           string          `json:"label"`
	SafeMetadata    json.RawMessage `json:"safe_metadata"`
	SourceDigest    string          `json:"source_digest"`
}

type lmWikiIssueContent struct {
	CitationKey string `json:"citation_key"`
	ID          string `json:"id"`
	Number      int32  `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	ProjectID   string `json:"project_id,omitempty"`
	StartDate   string `json:"start_date,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type lmWikiProjectContent struct {
	CitationKey string `json:"citation_key"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	StartDate   string `json:"start_date,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type lmWikiResourceContent struct {
	CitationKey string       `json:"citation_key"`
	ID          string       `json:"id"`
	ProjectID   string       `json:"project_id"`
	Label       string       `json:"label"`
	Position    int32        `json:"position"`
	Ref         LMWikiGitRef `json:"ref"`
}

type lmWikiRunContent struct {
	CitationKey     string `json:"citation_key"`
	ID              string `json:"id"`
	AutopilotID     string `json:"autopilot_id"`
	AutopilotTitle  string `json:"autopilot_title"`
	Source          string `json:"source"`
	IssueID         string `json:"issue_id,omitempty"`
	TriggeredAt     string `json:"triggered_at"`
	CompletedAt     string `json:"completed_at"`
	AcceptanceState string `json:"acceptance_state"`
}

type lmWikiPageContent struct {
	CitationKey    string `json:"citation_key"`
	RevisionID     string `json:"revision_id"`
	PageID         string `json:"page_id"`
	Scope          string `json:"scope"`
	ProjectID      string `json:"project_id,omitempty"`
	RevisionNumber int64  `json:"revision_number"`
	Path           string `json:"path"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	ContentDigest  string `json:"content_digest"`
	CreatedAt      string `json:"created_at,omitempty"`
}

func BuildLMWikiSnapshot(sources LMWikiSourceSnapshot) (LMWikiCanonicalSnapshot, error) {
	content := LMWikiContent{
		SchemaVersion: 2, EgressPolicy: sources.EgressPolicy,
		Issues: []lmWikiIssueContent{}, Projects: []lmWikiProjectContent{},
		ProjectResources: []lmWikiResourceContent{}, AutopilotRuns: []lmWikiRunContent{},
		WikiPages: []lmWikiPageContent{},
	}
	citations := make([]LMWikiCitation, 0, len(sources.Issues)+len(sources.Projects)+len(sources.ProjectResources)+len(sources.AutopilotRuns)+len(sources.WikiPages))
	for _, source := range sources.Issues {
		item := lmWikiIssueContent{CitationKey: citationKey("issue", source.ID), ID: source.ID, Number: source.Number, Title: normalizeLMWikiText(source.Title), Description: normalizeLMWikiText(source.Description), Status: normalizeLMWikiText(source.Status), Priority: normalizeLMWikiText(source.Priority), ProjectID: source.ProjectID, StartDate: source.StartDate, DueDate: source.DueDate, CreatedAt: canonicalTime(source.CreatedAt), UpdatedAt: canonicalTime(source.UpdatedAt)}
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("marshal issue citation item: %w", err)
		}
		citation, err := newLMWikiCitation("issue", source.ID, item.UpdatedAt, "issues/"+source.ID, "Issue #"+fmt.Sprint(source.Number)+": "+item.Title, itemJSON)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, err
		}
		content.Issues, citations = append(content.Issues, item), append(citations, citation)
	}
	for _, source := range sources.Projects {
		item := lmWikiProjectContent{CitationKey: citationKey("project", source.ID), ID: source.ID, Title: normalizeLMWikiText(source.Title), Description: normalizeLMWikiText(source.Description), Status: normalizeLMWikiText(source.Status), Priority: normalizeLMWikiText(source.Priority), StartDate: source.StartDate, DueDate: source.DueDate, CreatedAt: canonicalTime(source.CreatedAt), UpdatedAt: canonicalTime(source.UpdatedAt)}
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("marshal project citation item: %w", err)
		}
		citation, err := newLMWikiCitation("project", source.ID, item.UpdatedAt, "projects/"+source.ID, "Project: "+item.Title, itemJSON)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, err
		}
		content.Projects, citations = append(content.Projects, item), append(citations, citation)
	}
	for _, source := range sources.ProjectResources {
		if source.ResourceType != "github_repo" {
			continue
		}
		ref, err := SanitizeWikiGitRef(source.GitURL, source.Ref, source.DefaultBranchHint)
		if err != nil {
			continue
		}
		item := lmWikiResourceContent{CitationKey: citationKey("project_resource", source.ID), ID: source.ID, ProjectID: source.ProjectID, Label: normalizeLMWikiText(source.Label), Position: source.Position, Ref: ref}
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("marshal project resource citation item: %w", err)
		}
		citation, err := newLMWikiCitation("project_resource", source.ID, "", "project-resources/"+source.ID, "Repository: "+ref.Host+"/"+ref.RepositoryPath, itemJSON)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, err
		}
		content.ProjectResources, citations = append(content.ProjectResources, item), append(citations, citation)
	}
	for _, source := range sources.AutopilotRuns {
		if source.Status != "completed" || !isWikiRunSource(source.Source) {
			continue
		}
		item := lmWikiRunContent{CitationKey: citationKey("autopilot_run", source.ID), ID: source.ID, AutopilotID: source.AutopilotID, AutopilotTitle: normalizeLMWikiText(source.AutopilotTitle), Source: source.Source, IssueID: source.IssueID, TriggeredAt: canonicalTime(source.TriggeredAt), CompletedAt: canonicalTime(source.CompletedAt), AcceptanceState: "not_recorded"}
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("marshal autopilot run citation item: %w", err)
		}
		citation, err := newLMWikiCitation("autopilot_run", source.ID, item.CompletedAt, "autopilot-runs/"+source.ID, "Autopilot "+item.AutopilotTitle+" completed", itemJSON)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, err
		}
		content.AutopilotRuns, citations = append(content.AutopilotRuns, item), append(citations, citation)
	}
	for _, source := range sources.WikiPages {
		if source.Scope != "workspace" && source.Scope != "project" {
			continue
		}
		item := lmWikiPageContent{
			CitationKey: citationKey("wiki_page_revision", source.ID), RevisionID: source.ID,
			PageID: source.PageID, Scope: source.Scope, ProjectID: source.ProjectID,
			RevisionNumber: source.RevisionNumber, Path: source.Path, Title: source.Title,
			Content: source.Content, ContentDigest: source.ContentDigest,
			CreatedAt: canonicalTime(source.CreatedAt),
		}
		itemJSON, err := json.Marshal(item)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, fmt.Errorf("marshal Wiki page revision citation item: %w", err)
		}
		citation, err := newLMWikiCitation(
			"wiki_page_revision", source.ID, item.CreatedAt,
			"wiki-pages/"+source.PageID+"/revisions/"+source.ID,
			"Wiki: "+normalizeLMWikiText(source.Title), itemJSON,
		)
		if err != nil {
			return LMWikiCanonicalSnapshot{}, err
		}
		content.WikiPages, citations = append(content.WikiPages, item), append(citations, citation)
	}
	sort.Slice(content.Issues, func(i, j int) bool { return content.Issues[i].CitationKey < content.Issues[j].CitationKey })
	sort.Slice(content.Projects, func(i, j int) bool { return content.Projects[i].CitationKey < content.Projects[j].CitationKey })
	sort.Slice(content.ProjectResources, func(i, j int) bool {
		return content.ProjectResources[i].CitationKey < content.ProjectResources[j].CitationKey
	})
	sort.Slice(content.AutopilotRuns, func(i, j int) bool {
		return content.AutopilotRuns[i].CitationKey < content.AutopilotRuns[j].CitationKey
	})
	sort.Slice(content.WikiPages, func(i, j int) bool {
		return content.WikiPages[i].CitationKey < content.WikiPages[j].CitationKey
	})
	sort.Slice(citations, func(i, j int) bool { return citations[i].CitationKey < citations[j].CitationKey })
	canonical, err := json.Marshal(content)
	if err != nil {
		return LMWikiCanonicalSnapshot{}, fmt.Errorf("marshal canonical lm wiki content: %w", err)
	}
	if len(canonical) > lmWikiMaxContentBytes {
		return LMWikiCanonicalSnapshot{}, &LMWikiSizeError{Target: "content", Limit: lmWikiMaxContentBytes, Actual: len(canonical), Cause: ErrLMWikiContentTooLarge}
	}
	return LMWikiCanonicalSnapshot{Content: content, Citations: citations, CanonicalJSON: canonical, SourceDigest: digestLMWiki(canonical)}, nil
}

func newLMWikiCitation(sourceType, id, updatedAt, locator, label string, itemJSON []byte) (LMWikiCitation, error) {
	metadata, err := json.Marshal(struct {
		CitationKey string `json:"citation_key"`
		SourceType  string `json:"source_type"`
		SourceID    string `json:"source_id"`
		Label       string `json:"label"`
	}{citationKey(sourceType, id), sourceType, id, label})
	if err != nil {
		return LMWikiCitation{}, fmt.Errorf("marshal %s citation metadata: %w", sourceType, err)
	}
	if len(metadata) > lmWikiMaxMetadataBytes {
		return LMWikiCitation{}, &LMWikiSizeError{Target: "citation metadata", Limit: lmWikiMaxMetadataBytes, Actual: len(metadata), Cause: ErrLMWikiMetadataTooLarge}
	}
	return LMWikiCitation{CitationKey: citationKey(sourceType, id), SourceType: sourceType, SourceID: id, SourceUpdatedAt: updatedAt, Locator: locator, Label: label, SafeMetadata: metadata, SourceDigest: digestLMWiki(itemJSON)}, nil
}

func citationKey(sourceType, id string) string { return sourceType + ":" + id }
func digestLMWiki(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func canonicalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
func isWikiRunSource(source string) bool {
	return source == "schedule" || source == "manual" || source == "webhook" || source == "api"
}

func normalizeLMWikiText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = strings.TrimRightFunc(lines[i], unicode.IsSpace)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
