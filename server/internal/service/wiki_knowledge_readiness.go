package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	wikiKnowledgeReadinessSchemaVersion = 1
	wikiKnowledgeReadinessPageLimit     = 500
)

type WikiKnowledgeSourceState string

const (
	WikiKnowledgeEligibleUnpinned       WikiKnowledgeSourceState = "eligible_unpinned"
	WikiKnowledgePinnedCurrent          WikiKnowledgeSourceState = "pinned_current"
	WikiKnowledgeNewerRevisionAvailable WikiKnowledgeSourceState = "newer_revision_available"
	WikiKnowledgeSourceDeleted          WikiKnowledgeSourceState = "source_deleted"
	WikiKnowledgeExcluded               WikiKnowledgeSourceState = "excluded"
	WikiKnowledgePolicyStale            WikiKnowledgeSourceState = "policy_stale"
)

type WikiKnowledgeNextAction struct {
	Kind             string `json:"kind"`
	PageID           string `json:"page_id,omitempty"`
	RevisionID       string `json:"revision_id,omitempty"`
	RevisionNumber   int64  `json:"revision_number,omitempty"`
	LMWikiRevisionID string `json:"lm_wiki_revision_id,omitempty"`
}

type WikiKnowledgeSourceReadiness struct {
	PageID                 string                   `json:"page_id"`
	Scope                  string                   `json:"scope,omitempty"`
	ProjectID              string                   `json:"project_id,omitempty"`
	State                  WikiKnowledgeSourceState `json:"state"`
	ReasonCode             string                   `json:"reason_code"`
	ResponsibleRole        string                   `json:"responsible_role"`
	SelectedRevisionID     string                   `json:"selected_revision_id,omitempty"`
	SelectedRevisionNumber int64                    `json:"selected_revision_number,omitempty"`
	CurrentRevisionID      string                   `json:"current_revision_id,omitempty"`
	CurrentRevisionNumber  int64                    `json:"current_revision_number,omitempty"`
	PolicyVersion          int64                    `json:"policy_version"`
	NextAction             WikiKnowledgeNextAction  `json:"next_action"`
}

type WikiKnowledgeMaintenanceItem struct {
	ID                     string                  `json:"id"`
	Kind                   string                  `json:"kind"`
	Severity               string                  `json:"severity"`
	ReasonCode             string                  `json:"reason_code"`
	ResponsibleRole        string                  `json:"responsible_role"`
	PageID                 string                  `json:"page_id,omitempty"`
	SelectedRevisionNumber int64                   `json:"selected_revision_number,omitempty"`
	PolicyVersion          int64                   `json:"policy_version"`
	NextAction             WikiKnowledgeNextAction `json:"next_action"`
}

type WikiKnowledgeReadiness struct {
	SchemaVersion    int32                          `json:"schema_version"`
	Policy           LMWikiSourcePolicyState        `json:"policy"`
	Sources          []WikiKnowledgeSourceReadiness `json:"sources"`
	MaintenanceItems []WikiKnowledgeMaintenanceItem `json:"maintenance_items"`
	Truncated        bool                           `json:"truncated"`
}

type wikiKnowledgeSourceSnapshot struct {
	PageID             string
	Scope              string
	ProjectID          string
	SelectedRevisionID string
	SelectedRevision   int64
	CurrentRevisionID  string
	CurrentRevision    int64
	Deleted            bool
}

type wikiKnowledgeLMWikiSnapshot struct {
	RevisionID              string
	PolicyVersion           int64
	PolicyDigest            string
	RemoteGenerationEnabled bool
	Reviewed                bool
}

type wikiKnowledgeReadinessInput struct {
	WorkspaceID string
	Policy      LMWikiSourcePolicyState
	Sources     []wikiKnowledgeSourceSnapshot
	Latest      *wikiKnowledgeLMWikiSnapshot
	Truncated   bool
}

func (s *WikiService) KnowledgeReadiness(ctx context.Context, workspaceID pgtype.UUID) (WikiKnowledgeReadiness, error) {
	tx, err := s.TxStarter.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return WikiKnowledgeReadiness{}, fmt.Errorf("begin Wiki knowledge readiness read: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)

	policy, err := loadLMWikiSourcePolicyState(ctx, qtx, workspaceID)
	if err != nil {
		return WikiKnowledgeReadiness{}, err
	}
	selected, err := qtx.ListLMWikiSourceWikiPages(ctx, workspaceID)
	if err != nil {
		return WikiKnowledgeReadiness{}, fmt.Errorf("list Wiki knowledge selections: %w", err)
	}
	pages, err := qtx.SearchWikiPagesInWorkspace(ctx, db.SearchWikiPagesInWorkspaceParams{
		WorkspaceID: workspaceID, SearchQuery: "", ResultLimit: wikiKnowledgeReadinessPageLimit + 1,
	})
	if err != nil {
		return WikiKnowledgeReadiness{}, fmt.Errorf("list Wiki knowledge sources: %w", err)
	}
	truncated := len(pages) > wikiKnowledgeReadinessPageLimit
	if truncated {
		pages = pages[:wikiKnowledgeReadinessPageLimit]
	}

	selectedByPage := make(map[string]db.LmWikiSourceWikiPage, len(selected))
	for _, selection := range selected {
		selectedByPage[util.UUIDToString(selection.PageID)] = selection
	}
	sources := make([]wikiKnowledgeSourceSnapshot, 0, len(pages)+len(selected))
	seen := make(map[string]struct{}, len(pages))
	for _, page := range pages {
		pageID := util.UUIDToString(page.ID)
		seen[pageID] = struct{}{}
		sources = append(sources, wikiKnowledgeSnapshotFromPage(page, selectedByPage[pageID]))
	}
	for _, selection := range selected {
		pageID := util.UUIDToString(selection.PageID)
		if _, ok := seen[pageID]; ok {
			continue
		}
		page, pageErr := qtx.GetWikiPageInWorkspace(ctx, db.GetWikiPageInWorkspaceParams{ID: selection.PageID, WorkspaceID: workspaceID})
		if pageErr == nil && (page.Scope == "workspace" || page.Scope == "project") {
			sources = append(sources, wikiKnowledgeSnapshotFromPage(page, selection))
			continue
		}
		if pageErr != nil && !errors.Is(pageErr, pgx.ErrNoRows) {
			return WikiKnowledgeReadiness{}, fmt.Errorf("resolve selected Wiki knowledge source: %w", pageErr)
		}
		sources = append(sources, wikiKnowledgeSourceSnapshot{
			PageID: pageID, SelectedRevisionID: util.UUIDToString(selection.RevisionID),
			SelectedRevision: selection.RevisionNumber, Deleted: true,
		})
	}

	var latest *wikiKnowledgeLMWikiSnapshot
	latestRow, err := qtx.GetLatestLMWikiRevision(ctx, workspaceID)
	if err == nil {
		reviewed := false
		if _, reviewErr := qtx.GetLMWikiReview(ctx, db.GetLMWikiReviewParams{WorkspaceID: workspaceID, RevisionID: latestRow.ID}); reviewErr == nil {
			reviewed = true
		} else if !errors.Is(reviewErr, pgx.ErrNoRows) {
			return WikiKnowledgeReadiness{}, fmt.Errorf("load latest LM Wiki review state: %w", reviewErr)
		}
		latest = &wikiKnowledgeLMWikiSnapshot{
			RevisionID: util.UUIDToString(latestRow.ID), PolicyVersion: latestRow.SourcePolicyVersion,
			PolicyDigest:            latestRow.SourcePolicyDigest,
			RemoteGenerationEnabled: latestRow.RemoteGenerationEnabled, Reviewed: reviewed,
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return WikiKnowledgeReadiness{}, fmt.Errorf("load latest LM Wiki readiness revision: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return WikiKnowledgeReadiness{}, fmt.Errorf("commit Wiki knowledge readiness read: %w", err)
	}
	return deriveWikiKnowledgeReadiness(wikiKnowledgeReadinessInput{
		WorkspaceID: util.UUIDToString(workspaceID), Policy: policy, Sources: sources,
		Latest: latest, Truncated: truncated,
	}), nil
}

func wikiKnowledgeSnapshotFromPage(page db.WikiPage, selection db.LmWikiSourceWikiPage) wikiKnowledgeSourceSnapshot {
	snapshot := wikiKnowledgeSourceSnapshot{
		PageID: util.UUIDToString(page.ID), Scope: page.Scope,
		CurrentRevisionID: util.UUIDToString(page.CurrentRevisionID), CurrentRevision: page.CurrentRevisionNumber,
	}
	if page.ProjectID.Valid {
		snapshot.ProjectID = util.UUIDToString(page.ProjectID)
	}
	if selection.PageID.Valid {
		snapshot.SelectedRevisionID = util.UUIDToString(selection.RevisionID)
		snapshot.SelectedRevision = selection.RevisionNumber
	}
	return snapshot
}

func deriveWikiKnowledgeReadiness(input wikiKnowledgeReadinessInput) WikiKnowledgeReadiness {
	wikiEnabled := false
	for _, sourceClass := range input.Policy.SourceClasses {
		if sourceClass == "wiki_page" {
			wikiEnabled = true
			break
		}
	}
	policyCurrent := input.Latest != nil &&
		input.Latest.PolicyVersion == input.Policy.PolicyVersion &&
		input.Latest.PolicyDigest == input.Policy.PolicyDigest &&
		input.Latest.RemoteGenerationEnabled == input.Policy.RemoteGenerationEnabled

	result := WikiKnowledgeReadiness{
		SchemaVersion: wikiKnowledgeReadinessSchemaVersion, Policy: input.Policy,
		Sources:          make([]WikiKnowledgeSourceReadiness, 0, len(input.Sources)),
		MaintenanceItems: []WikiKnowledgeMaintenanceItem{}, Truncated: input.Truncated,
	}
	for _, source := range input.Sources {
		readiness := WikiKnowledgeSourceReadiness{
			PageID: source.PageID, Scope: source.Scope, ProjectID: source.ProjectID,
			SelectedRevisionID: source.SelectedRevisionID, SelectedRevisionNumber: source.SelectedRevision,
			CurrentRevisionID: source.CurrentRevisionID, CurrentRevisionNumber: source.CurrentRevision,
			PolicyVersion: input.Policy.PolicyVersion, ResponsibleRole: "owner_admin",
			NextAction: WikiKnowledgeNextAction{Kind: "none"},
		}
		switch {
		case source.Deleted:
			readiness.State = WikiKnowledgeSourceDeleted
			readiness.ReasonCode = "page_deleted"
			readiness.NextAction = WikiKnowledgeNextAction{Kind: "remove_source", PageID: source.PageID}
		case !wikiEnabled && source.SelectedRevision > 0:
			readiness.State = WikiKnowledgeExcluded
			readiness.ReasonCode = "wiki_page_class_disabled"
			readiness.NextAction = WikiKnowledgeNextAction{Kind: "pin_revision", PageID: source.PageID, RevisionID: source.CurrentRevisionID, RevisionNumber: source.CurrentRevision}
		case source.SelectedRevision == 0:
			readiness.State = WikiKnowledgeEligibleUnpinned
			readiness.ReasonCode = "no_selection"
			readiness.NextAction = WikiKnowledgeNextAction{Kind: "pin_revision", PageID: source.PageID, RevisionID: source.CurrentRevisionID, RevisionNumber: source.CurrentRevision}
		case source.SelectedRevision < source.CurrentRevision:
			readiness.State = WikiKnowledgeNewerRevisionAvailable
			readiness.ReasonCode = "newer_revision_available"
			readiness.NextAction = WikiKnowledgeNextAction{Kind: "pin_revision", PageID: source.PageID, RevisionID: source.CurrentRevisionID, RevisionNumber: source.CurrentRevision}
		case !policyCurrent:
			readiness.State = WikiKnowledgePolicyStale
			readiness.ReasonCode = "lm_wiki_snapshot_outdated"
			readiness.NextAction = WikiKnowledgeNextAction{Kind: "refresh_lm_wiki"}
		default:
			readiness.State = WikiKnowledgePinnedCurrent
			readiness.ReasonCode = "revision_current"
		}
		result.Sources = append(result.Sources, readiness)
		if item, ok := wikiKnowledgeMaintenanceForSource(input.WorkspaceID, readiness); ok {
			result.MaintenanceItems = append(result.MaintenanceItems, item)
		}
	}
	if input.Latest != nil && policyCurrent && !input.Latest.Reviewed {
		result.MaintenanceItems = append(result.MaintenanceItems, WikiKnowledgeMaintenanceItem{
			ID:   fmt.Sprintf("%s:lm_wiki_review_pending:%s:%d", input.WorkspaceID, input.Latest.RevisionID, input.Policy.PolicyVersion),
			Kind: "lm_wiki_review_pending", Severity: "warning", ReasonCode: "latest_revision_unreviewed",
			ResponsibleRole: "owner_admin", PolicyVersion: input.Policy.PolicyVersion,
			NextAction: WikiKnowledgeNextAction{Kind: "review_lm_wiki", LMWikiRevisionID: input.Latest.RevisionID},
		})
	}
	sort.Slice(result.Sources, func(i, j int) bool { return result.Sources[i].PageID < result.Sources[j].PageID })
	sort.Slice(result.MaintenanceItems, func(i, j int) bool { return result.MaintenanceItems[i].ID < result.MaintenanceItems[j].ID })
	return result
}

func wikiKnowledgeMaintenanceForSource(workspaceID string, source WikiKnowledgeSourceReadiness) (WikiKnowledgeMaintenanceItem, bool) {
	kind, severity := "", ""
	switch source.State {
	case WikiKnowledgeNewerRevisionAvailable:
		kind, severity = "source_newer_revision", "warning"
	case WikiKnowledgeSourceDeleted:
		kind, severity = "source_deleted", "high"
	case WikiKnowledgeExcluded:
		if source.SelectedRevisionNumber > 0 {
			kind, severity = "source_excluded", "high"
		}
	case WikiKnowledgePolicyStale:
		kind, severity = "policy_stale", "warning"
	}
	if kind == "" {
		return WikiKnowledgeMaintenanceItem{}, false
	}
	return WikiKnowledgeMaintenanceItem{
		ID:   fmt.Sprintf("%s:%s:%s:%d:%d", workspaceID, kind, source.PageID, source.SelectedRevisionNumber, source.PolicyVersion),
		Kind: kind, Severity: severity, ReasonCode: source.ReasonCode,
		ResponsibleRole: source.ResponsibleRole, PageID: source.PageID,
		SelectedRevisionNumber: source.SelectedRevisionNumber, PolicyVersion: source.PolicyVersion,
		NextAction: source.NextAction,
	}, true
}
