package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const lmWikiMaxSelectedWikiPages = 500

var (
	ErrLMWikiInvalidSourcePolicy = errors.New("invalid lm wiki source policy")
	ErrLMWikiSourceNotEligible   = errors.New("lm wiki source is not eligible")
	ErrLMWikiSourcePolicyStale   = errors.New("lm wiki source policy is stale")
)

var defaultLMWikiSourceClasses = []string{"autopilot_run", "issue", "project", "project_resource"}

var allowedLMWikiSourceClasses = map[string]struct{}{
	"autopilot_run":    {},
	"issue":            {},
	"project":          {},
	"project_resource": {},
	"wiki_page":        {},
}

type LMWikiSourcePolicy struct {
	SourceClasses           []string               `json:"source_classes"`
	WikiPages               []LMWikiSourceWikiPage `json:"wiki_pages"`
	RemoteGenerationEnabled bool                   `json:"remote_generation_enabled"`
}

type LMWikiSourceWikiPage struct {
	PageID         string `json:"page_id"`
	RevisionNumber int64  `json:"revision_number"`
}

type LMWikiSourcePolicyState struct {
	LMWikiSourcePolicy
	PolicyVersion int64                   `json:"policy_version"`
	PolicyDigest  string                  `json:"policy_digest"`
	Exclusions    []LMWikiSourceExclusion `json:"exclusions"`
}

type LMWikiSourcePolicyExpectation struct {
	PolicyVersion int64
	PolicyDigest  string
}

type LMWikiSourcePolicyStaleError struct {
	Current LMWikiSourcePolicyState
}

func (e *LMWikiSourcePolicyStaleError) Error() string { return ErrLMWikiSourcePolicyStale.Error() }
func (e *LMWikiSourcePolicyStaleError) Unwrap() error { return ErrLMWikiSourcePolicyStale }

type LMWikiPinWikiRevisionInput struct {
	PageID      pgtype.UUID
	RevisionID  pgtype.UUID
	Expectation LMWikiSourcePolicyExpectation
}

type LMWikiSourceExclusion struct {
	SourceClass string `json:"source_class"`
	State       string `json:"state"`
	Reason      string `json:"reason"`
}

type lmWikiSourceSelectionInsert struct {
	PageID         string `json:"page_id"`
	RevisionID     string `json:"revision_id"`
	RevisionNumber int64  `json:"revision_number"`
}

var permanentLMWikiSourceExclusions = []LMWikiSourceExclusion{
	{SourceClass: "personal_wiki", State: "always_excluded", Reason: "personal_scope_never_eligible"},
	{SourceClass: "local_only", State: "always_excluded", Reason: "local_only_never_leaves_owner_daemon"},
}

func normalizeLMWikiSourceClasses(input []string) ([]string, error) {
	seen := make(map[string]struct{}, len(input))
	for _, sourceClass := range input {
		if _, ok := allowedLMWikiSourceClasses[sourceClass]; !ok {
			return nil, fmt.Errorf("source class %q: %w", sourceClass, ErrLMWikiInvalidSourcePolicy)
		}
		seen[sourceClass] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for sourceClass := range seen {
		result = append(result, sourceClass)
	}
	sort.Strings(result)
	return result, nil
}

func (s *WikiService) GetSourcePolicy(ctx context.Context, workspaceID pgtype.UUID) (LMWikiSourcePolicyState, error) {
	tx, err := s.TxStarter.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("begin lm wiki source policy read: %w", err)
	}
	defer tx.Rollback(ctx)

	state, err := loadLMWikiSourcePolicyState(ctx, s.Queries.WithTx(tx), workspaceID)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("commit lm wiki source policy read: %w", err)
	}
	return state, nil
}

func (s *WikiService) UpdateSourcePolicy(ctx context.Context, workspaceID, updatedByID pgtype.UUID, input LMWikiSourcePolicy) (LMWikiSourcePolicyState, error) {
	return s.updateSourcePolicy(ctx, workspaceID, updatedByID, input, nil)
}

func (s *WikiService) UpdateSourcePolicyIfCurrent(ctx context.Context, workspaceID, updatedByID pgtype.UUID, input LMWikiSourcePolicy, expectation LMWikiSourcePolicyExpectation) (LMWikiSourcePolicyState, error) {
	return s.updateSourcePolicy(ctx, workspaceID, updatedByID, input, &expectation)
}

func (s *WikiService) updateSourcePolicy(ctx context.Context, workspaceID, updatedByID pgtype.UUID, input LMWikiSourcePolicy, expectation *LMWikiSourcePolicyExpectation) (LMWikiSourcePolicyState, error) {
	sourceClasses, err := normalizeLMWikiSourceClasses(input.SourceClasses)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	if len(input.WikiPages) > lmWikiMaxSelectedWikiPages {
		return LMWikiSourcePolicyState{}, fmt.Errorf("too many selected Wiki pages: %w", ErrLMWikiInvalidSourcePolicy)
	}
	wireSelections, err := normalizeLMWikiSourceWikiPages(input.WikiPages)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	input.SourceClasses = sourceClasses
	input.WikiPages = wireSelections

	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("begin lm wiki source policy update: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	if _, err := qtx.LockWorkspaceForWikiArtifactCreate(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LMWikiSourcePolicyState{}, ErrLMWikiNotFound
		}
		return LMWikiSourcePolicyState{}, fmt.Errorf("lock lm wiki source policy workspace: %w", err)
	}
	if err := qtx.LockLMWikiLifecycle(ctx, workspaceID); err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("lock lm wiki source policy: %w", err)
	}

	current, err := loadLMWikiSourcePolicyState(ctx, qtx, workspaceID)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	candidate, err := newLMWikiSourcePolicyState(input, current.PolicyVersion)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	if candidate.PolicyDigest == current.PolicyDigest {
		if err := tx.Commit(ctx); err != nil {
			return LMWikiSourcePolicyState{}, fmt.Errorf("commit unchanged lm wiki source policy: %w", err)
		}
		return current, nil
	}
	if expectation != nil {
		if err := checkLMWikiSourcePolicyExpectation(current, *expectation); err != nil {
			return LMWikiSourcePolicyState{}, err
		}
	}
	retained, err := qtx.ListLMWikiSourceWikiPages(ctx, workspaceID)
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("list retained lm wiki Wiki page selections: %w", err)
	}
	row, err := saveLMWikiSourcePolicy(ctx, qtx, workspaceID, updatedByID, input, retained)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("commit lm wiki source policy update: %w", err)
	}

	state, err := newLMWikiSourcePolicyState(LMWikiSourcePolicy{
		SourceClasses: sourceClasses, WikiPages: wireSelections,
		RemoteGenerationEnabled: row.RemoteGenerationEnabled,
	}, row.PolicyVersion)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	s.publishLMWikiSourcePolicyChanged(workspaceID, updatedByID, state.PolicyVersion)
	return state, nil
}

func (s *WikiService) PinWikiRevision(ctx context.Context, workspaceID, updatedByID pgtype.UUID, input LMWikiPinWikiRevisionInput) (LMWikiSourcePolicyState, error) {
	if !input.PageID.Valid || !input.RevisionID.Valid || strings.TrimSpace(input.Expectation.PolicyDigest) == "" {
		return LMWikiSourcePolicyState{}, ErrLMWikiInvalidSourcePolicy
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("begin lm wiki Wiki revision pin: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	if _, err := qtx.LockWorkspaceForWikiArtifactCreate(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LMWikiSourcePolicyState{}, ErrLMWikiNotFound
		}
		return LMWikiSourcePolicyState{}, fmt.Errorf("lock lm wiki Wiki revision pin workspace: %w", err)
	}
	if err := qtx.LockLMWikiLifecycle(ctx, workspaceID); err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("lock lm wiki Wiki revision pin: %w", err)
	}

	current, err := loadLMWikiSourcePolicyState(ctx, qtx, workspaceID)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	page, err := qtx.GetWikiPageInWorkspace(ctx, db.GetWikiPageInWorkspaceParams{ID: input.PageID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return LMWikiSourcePolicyState{}, ErrLMWikiSourceNotEligible
	}
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("load lm wiki Wiki pin page: %w", err)
	}
	if page.Scope != "workspace" && page.Scope != "project" {
		return LMWikiSourcePolicyState{}, ErrLMWikiSourceNotEligible
	}
	revision, err := qtx.GetWikiPageRevision(ctx, db.GetWikiPageRevisionParams{PageID: page.ID, ID: input.RevisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return LMWikiSourcePolicyState{}, ErrLMWikiSourceNotEligible
	}
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("load lm wiki Wiki pin revision: %w", err)
	}
	pageID := util.UUIDToString(page.ID)
	if lmWikiPolicyPinsRevision(current, pageID, revision.RevisionNumber) {
		if err := tx.Commit(ctx); err != nil {
			return LMWikiSourcePolicyState{}, fmt.Errorf("commit repeated lm wiki Wiki revision pin: %w", err)
		}
		return current, nil
	}
	if err := checkLMWikiSourcePolicyExpectation(current, input.Expectation); err != nil {
		return LMWikiSourcePolicyState{}, err
	}

	classes := append([]string(nil), current.SourceClasses...)
	classes = append(classes, "wiki_page")
	classes, err = normalizeLMWikiSourceClasses(classes)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	pages := make([]LMWikiSourceWikiPage, 0, len(current.WikiPages)+1)
	for _, selected := range current.WikiPages {
		if selected.PageID != pageID {
			pages = append(pages, selected)
		}
	}
	pages = append(pages, LMWikiSourceWikiPage{PageID: pageID, RevisionNumber: revision.RevisionNumber})
	pages, err = normalizeLMWikiSourceWikiPages(pages)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	policy := LMWikiSourcePolicy{
		SourceClasses: classes, WikiPages: pages,
		RemoteGenerationEnabled: current.RemoteGenerationEnabled,
	}
	retained, err := qtx.ListLMWikiSourceWikiPages(ctx, workspaceID)
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("list retained lm wiki Wiki page selections: %w", err)
	}
	row, err := saveLMWikiSourcePolicy(ctx, qtx, workspaceID, updatedByID, policy, retained)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("commit lm wiki Wiki revision pin: %w", err)
	}
	state, err := newLMWikiSourcePolicyState(policy, row.PolicyVersion)
	if err != nil {
		return LMWikiSourcePolicyState{}, err
	}
	s.publishLMWikiSourcePolicyChanged(workspaceID, updatedByID, state.PolicyVersion)
	return state, nil
}

func loadLMWikiSourcePolicyState(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) (LMWikiSourcePolicyState, error) {
	classes := append([]string(nil), defaultLMWikiSourceClasses...)
	version := int64(0)
	remoteGenerationEnabled := false
	row, err := queries.GetLMWikiSourcePolicy(ctx, workspaceID)
	if err == nil {
		if err := json.Unmarshal(row.SourceClasses, &classes); err != nil {
			return LMWikiSourcePolicyState{}, fmt.Errorf("decode lm wiki source classes: %w", err)
		}
		classes, err = normalizeLMWikiSourceClasses(classes)
		if err != nil {
			return LMWikiSourcePolicyState{}, err
		}
		version = row.PolicyVersion
		remoteGenerationEnabled = row.RemoteGenerationEnabled
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return LMWikiSourcePolicyState{}, fmt.Errorf("load lm wiki source policy: %w", err)
	}

	rows, err := queries.ListLMWikiSourceWikiPages(ctx, workspaceID)
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("list lm wiki Wiki page selections: %w", err)
	}
	selections := make([]LMWikiSourceWikiPage, len(rows))
	for index, selection := range rows {
		selections[index] = LMWikiSourceWikiPage{
			PageID: util.UUIDToString(selection.PageID), RevisionNumber: selection.RevisionNumber,
		}
	}
	return newLMWikiSourcePolicyState(LMWikiSourcePolicy{
		SourceClasses: classes, WikiPages: selections,
		RemoteGenerationEnabled: remoteGenerationEnabled,
	}, version)
}

func newLMWikiSourcePolicyState(policy LMWikiSourcePolicy, version int64) (LMWikiSourcePolicyState, error) {
	classes := append([]string(nil), policy.SourceClasses...)
	if classes == nil {
		classes = []string{}
	}
	pages := append([]LMWikiSourceWikiPage(nil), policy.WikiPages...)
	if pages == nil {
		pages = []LMWikiSourceWikiPage{}
	}
	sort.Strings(classes)
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].PageID == pages[j].PageID {
			return pages[i].RevisionNumber < pages[j].RevisionNumber
		}
		return pages[i].PageID < pages[j].PageID
	})
	policy.SourceClasses = classes
	policy.WikiPages = pages
	canonical, err := json.Marshal(struct {
		SourceClasses           []string               `json:"source_classes"`
		WikiPages               []LMWikiSourceWikiPage `json:"wiki_pages"`
		RemoteGenerationEnabled bool                   `json:"remote_generation_enabled"`
	}{
		SourceClasses: classes, WikiPages: pages,
		RemoteGenerationEnabled: policy.RemoteGenerationEnabled,
	})
	if err != nil {
		return LMWikiSourcePolicyState{}, fmt.Errorf("marshal canonical lm wiki source policy: %w", err)
	}
	return LMWikiSourcePolicyState{
		LMWikiSourcePolicy: policy,
		PolicyVersion:      version,
		PolicyDigest:       digestLMWiki(canonical),
		Exclusions:         append([]LMWikiSourceExclusion(nil), permanentLMWikiSourceExclusions...),
	}, nil
}

func normalizeLMWikiSourceWikiPages(input []LMWikiSourceWikiPage) ([]LMWikiSourceWikiPage, error) {
	byPage := make(map[string]LMWikiSourceWikiPage, len(input))
	for _, selection := range input {
		pageID, err := util.ParseUUID(selection.PageID)
		if err != nil || selection.RevisionNumber <= 0 {
			return nil, fmt.Errorf("invalid Wiki page selection: %w", ErrLMWikiInvalidSourcePolicy)
		}
		canonicalPageID := util.UUIDToString(pageID)
		if existing, ok := byPage[canonicalPageID]; ok && existing.RevisionNumber != selection.RevisionNumber {
			return nil, fmt.Errorf("multiple revisions selected for Wiki page %s: %w", canonicalPageID, ErrLMWikiInvalidSourcePolicy)
		}
		byPage[canonicalPageID] = LMWikiSourceWikiPage{PageID: canonicalPageID, RevisionNumber: selection.RevisionNumber}
	}

	wireSelections := make([]LMWikiSourceWikiPage, 0, len(byPage))
	for _, selection := range byPage {
		wireSelections = append(wireSelections, selection)
	}
	sort.Slice(wireSelections, func(i, j int) bool { return wireSelections[i].PageID < wireSelections[j].PageID })
	return wireSelections, nil
}

func checkLMWikiSourcePolicyExpectation(current LMWikiSourcePolicyState, expectation LMWikiSourcePolicyExpectation) error {
	if expectation.PolicyVersion < 0 || strings.TrimSpace(expectation.PolicyDigest) == "" {
		return ErrLMWikiInvalidSourcePolicy
	}
	if current.PolicyVersion != expectation.PolicyVersion || current.PolicyDigest != expectation.PolicyDigest {
		return &LMWikiSourcePolicyStaleError{Current: current}
	}
	return nil
}

func lmWikiPolicyPinsRevision(state LMWikiSourcePolicyState, pageID string, revisionNumber int64) bool {
	wikiEnabled := false
	for _, sourceClass := range state.SourceClasses {
		if sourceClass == "wiki_page" {
			wikiEnabled = true
			break
		}
	}
	if !wikiEnabled {
		return false
	}
	for _, selection := range state.WikiPages {
		if selection.PageID == pageID && selection.RevisionNumber == revisionNumber {
			return true
		}
	}
	return false
}

func saveLMWikiSourcePolicy(ctx context.Context, queries *db.Queries, workspaceID, updatedByID pgtype.UUID, policy LMWikiSourcePolicy, retained []db.LmWikiSourceWikiPage) (db.LmWikiSourcePolicy, error) {
	selections, err := validateLMWikiSourceWikiPages(ctx, queries, workspaceID, policy.WikiPages, retained)
	if err != nil {
		return db.LmWikiSourcePolicy{}, err
	}
	classesJSON, err := json.Marshal(policy.SourceClasses)
	if err != nil {
		return db.LmWikiSourcePolicy{}, fmt.Errorf("marshal lm wiki source classes: %w", err)
	}
	selectionsJSON, err := json.Marshal(selections)
	if err != nil {
		return db.LmWikiSourcePolicy{}, fmt.Errorf("marshal lm wiki Wiki page selections: %w", err)
	}
	if err := queries.DeleteLMWikiSourceWikiPages(ctx, workspaceID); err != nil {
		return db.LmWikiSourcePolicy{}, fmt.Errorf("replace lm wiki Wiki page selections: %w", err)
	}
	if len(selections) > 0 {
		if err := queries.CreateLMWikiSourceWikiPages(ctx, db.CreateLMWikiSourceWikiPagesParams{
			WorkspaceID: workspaceID, SelectedByID: updatedByID, Selections: selectionsJSON,
		}); err != nil {
			return db.LmWikiSourcePolicy{}, fmt.Errorf("save lm wiki Wiki page selections: %w", err)
		}
	}
	row, err := queries.UpsertLMWikiSourcePolicy(ctx, db.UpsertLMWikiSourcePolicyParams{
		WorkspaceID: workspaceID, SourceClasses: classesJSON,
		RemoteGenerationEnabled: policy.RemoteGenerationEnabled, UpdatedByID: updatedByID,
	})
	if err != nil {
		return db.LmWikiSourcePolicy{}, fmt.Errorf("save lm wiki source policy: %w", err)
	}
	return row, nil
}

func validateLMWikiSourceWikiPages(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, input []LMWikiSourceWikiPage, retained []db.LmWikiSourceWikiPage) ([]lmWikiSourceSelectionInsert, error) {
	retainedByPage := make(map[string]db.LmWikiSourceWikiPage, len(retained))
	for _, selection := range retained {
		retainedByPage[util.UUIDToString(selection.PageID)] = selection
	}

	rows := make([]lmWikiSourceSelectionInsert, 0, len(input))
	for _, selection := range input {
		pageID, _ := util.ParseUUID(selection.PageID)
		revision, err := queries.GetWikiPageRevisionForLMWikiPolicy(ctx, db.GetWikiPageRevisionForLMWikiPolicyParams{
			WorkspaceID: workspaceID, PageID: pageID, RevisionNumber: selection.RevisionNumber,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			previous, ok := retainedByPage[selection.PageID]
			if !ok || previous.RevisionNumber != selection.RevisionNumber {
				return nil, fmt.Errorf("Wiki page revision %s:%d is not eligible: %w", selection.PageID, selection.RevisionNumber, ErrLMWikiInvalidSourcePolicy)
			}
			rows = append(rows, lmWikiSourceSelectionInsert{
				PageID: selection.PageID, RevisionID: util.UUIDToString(previous.RevisionID), RevisionNumber: selection.RevisionNumber,
			})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("validate lm wiki Wiki page revision: %w", err)
		}
		rows = append(rows, lmWikiSourceSelectionInsert{
			PageID: selection.PageID, RevisionID: util.UUIDToString(revision.ID), RevisionNumber: selection.RevisionNumber,
		})
	}
	return rows, nil
}
