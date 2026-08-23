package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var (
	ErrInvalidWikiPageCreate           = errors.New("invalid wiki page create")
	ErrInvalidWikiPageChange           = errors.New("invalid wiki page change")
	ErrInvalidWikiProposal             = errors.New("invalid Wiki proposal")
	ErrInvalidWikiProposalEvidence     = errors.New("invalid Wiki proposal evidence")
	ErrWikiRevisionConflict            = errors.New("Wiki revision conflict")
	ErrWikiProposalIdempotencyConflict = errors.New("Wiki proposal idempotency conflict")
	ErrWikiProposalAlreadyReviewed     = errors.New("Wiki proposal already reviewed")
	ErrWikiProposalRevisionConflict    = errors.New("Wiki proposal revision conflict")
)

const maxWikiEvidenceRefs = 100

type WikiPageCreateInput struct {
	WorkspaceID pgtype.UUID
	ProjectID   pgtype.UUID
	OwnerUserID pgtype.UUID
	Scope       string
	Path        string
	Title       string
	Content     string
	ActorType   string
	ActorID     pgtype.UUID
	SourceKind  string
	SourceRefID pgtype.UUID
}

type WikiPageAccess struct {
	PageID      pgtype.UUID
	WorkspaceID pgtype.UUID
	OwnerUserID pgtype.UUID
}

type WikiRevisionAccess struct {
	RevisionID  pgtype.UUID
	WorkspaceID pgtype.UUID
	OwnerUserID pgtype.UUID
}

type WikiPageSearchInput struct {
	Scope         string
	WorkspaceID   pgtype.UUID
	ProjectID     pgtype.UUID
	OwnerUserID   pgtype.UUID
	Query         string
	Limit         int32
	AllowPersonal bool
}

type WikiPageUpdateInput struct {
	Page                   db.WikiPage
	ExpectedRevisionNumber int64
	Path                   pgtype.Text
	Title                  pgtype.Text
	Content                pgtype.Text
	ActorID                pgtype.UUID
}

type WikiPageRestoreInput struct {
	Page                   db.WikiPage
	RevisionID             pgtype.UUID
	ExpectedRevisionNumber int64
	ActorID                pgtype.UUID
}

type WikiProposalCreateInput struct {
	Page                db.WikiPage
	AgentID             pgtype.UUID
	AuthenticatedTaskID pgtype.UUID
	BaseRevisionNumber  int64
	Path                string
	Title               string
	Content             string
	Rationale           string
	EvidenceRefs        []string
	IdempotencyKey      string
}

type WikiProposalCreateResult struct {
	Proposal db.WikiPageEditProposal
	Created  bool
}

type WikiProposalAcceptInput struct {
	Page                   db.WikiPage
	Proposal               db.WikiPageEditProposal
	ExpectedRevisionNumber int64
	Path                   pgtype.Text
	Title                  pgtype.Text
	Content                pgtype.Text
	ReviewerID             pgtype.UUID
	ReviewReason           pgtype.Text
}

type WikiProposalRejectInput struct {
	Page         db.WikiPage
	Proposal     db.WikiPageEditProposal
	ReviewerID   pgtype.UUID
	ReviewReason pgtype.Text
}

// WikiKnowledge is the single domain seam used by HTTP and non-HTTP producers.
// Callers that already own a transaction use CreatePage, then invoke
// PublishCreatedPage only after that transaction commits.
type WikiKnowledge interface {
	CreatePage(context.Context, *db.Queries, WikiPageCreateInput) (db.WikiPage, error)
	CreatePageAndPublish(context.Context, *db.Queries, WikiPageCreateInput) (db.WikiPage, error)
	PublishCreatedPage(context.Context, *db.Queries, pgtype.UUID, string, pgtype.UUID) error
	GetPage(context.Context, *db.Queries, WikiPageAccess) (db.WikiPage, error)
	SearchPages(context.Context, *db.Queries, WikiPageSearchInput) ([]db.WikiPage, error)
	GetRevision(context.Context, *db.Queries, WikiRevisionAccess) (db.WikiPageRevision, error)
	GetPageRevision(context.Context, *db.Queries, db.WikiPage, pgtype.UUID) (db.WikiPageRevision, error)
	ListRevisions(context.Context, *db.Queries, db.WikiPage) ([]db.WikiPageRevision, error)
	UpdatePage(context.Context, *db.Queries, WikiPageUpdateInput) (db.WikiPage, error)
	RestorePage(context.Context, *db.Queries, WikiPageRestoreInput) (db.WikiPage, error)
	DeletePage(context.Context, *db.Queries, db.WikiPage, pgtype.UUID) error
	ListProposals(context.Context, *db.Queries, db.WikiPage) ([]db.WikiPageEditProposal, error)
	GetProposal(context.Context, *db.Queries, db.WikiPage, pgtype.UUID) (db.WikiPageEditProposal, error)
	CreateProposal(context.Context, *db.Queries, WikiProposalCreateInput) (WikiProposalCreateResult, error)
	AcceptProposal(context.Context, *db.Queries, WikiProposalAcceptInput) (db.WikiPage, db.WikiPageEditProposal, error)
	RejectProposal(context.Context, *db.Queries, WikiProposalRejectInput) (db.WikiPageEditProposal, error)
}

type WikiKnowledgeService struct {
	events *events.Bus
}

func NewWikiKnowledgeService(buses ...*events.Bus) WikiKnowledge {
	service := &WikiKnowledgeService{}
	if len(buses) > 0 {
		service.events = buses[0]
	}
	return service
}

func (s *WikiKnowledgeService) CreatePage(ctx context.Context, queries *db.Queries, input WikiPageCreateInput) (db.WikiPage, error) {
	input.Path = normalizeWikiKnowledgePath(input.Path)
	input.Title = strings.TrimSpace(input.Title)
	if queries == nil || !validWikiPageCreateInput(input) {
		return db.WikiPage{}, ErrInvalidWikiPageCreate
	}
	row, err := queries.CreateWikiPageWithProvenance(ctx, db.CreateWikiPageWithProvenanceParams{
		WorkspaceID: input.WorkspaceID, Scope: input.Scope, ProjectID: input.ProjectID,
		OwnerUserID: input.OwnerUserID, Path: input.Path, Title: input.Title,
		Content: input.Content, ActorType: input.ActorType, ActorID: input.ActorID,
		SourceKind: input.SourceKind, SourceRefID: input.SourceRefID,
	})
	if err != nil {
		return db.WikiPage{}, err
	}
	return wikiPageFromProvenanceCreate(row), nil
}

func (s *WikiKnowledgeService) CreatePageAndPublish(ctx context.Context, queries *db.Queries, input WikiPageCreateInput) (db.WikiPage, error) {
	page, err := s.CreatePage(ctx, queries, input)
	if err != nil {
		return db.WikiPage{}, err
	}
	s.publishPage(protocol.EventWikiPageCreated, page, input.ActorType, input.ActorID)
	s.publishPage(protocol.EventWikiRevisionCreated, page, input.ActorType, input.ActorID)
	return page, nil
}

func (s *WikiKnowledgeService) PublishCreatedPage(ctx context.Context, queries *db.Queries, pageID pgtype.UUID, actorType string, actorID pgtype.UUID) error {
	if queries == nil || !pageID.Valid {
		return ErrInvalidWikiPageCreate
	}
	page, err := queries.GetWikiPage(ctx, pageID)
	if err != nil {
		return err
	}
	s.publishPage(protocol.EventWikiPageCreated, page, actorType, actorID)
	s.publishPage(protocol.EventWikiRevisionCreated, page, actorType, actorID)
	return nil
}

func (*WikiKnowledgeService) GetPage(ctx context.Context, queries *db.Queries, access WikiPageAccess) (db.WikiPage, error) {
	return queries.GetWikiPageForActor(ctx, db.GetWikiPageForActorParams{
		PageID: access.PageID, WorkspaceID: access.WorkspaceID, OwnerUserID: access.OwnerUserID,
	})
}

func (*WikiKnowledgeService) SearchPages(ctx context.Context, queries *db.Queries, input WikiPageSearchInput) ([]db.WikiPage, error) {
	switch input.Scope {
	case "all":
		if input.AllowPersonal {
			return queries.SearchWikiPagesAll(ctx, db.SearchWikiPagesAllParams{WorkspaceID: input.WorkspaceID, OwnerUserID: input.OwnerUserID, SearchQuery: input.Query, ResultLimit: input.Limit})
		}
		return queries.SearchWikiPagesInWorkspace(ctx, db.SearchWikiPagesInWorkspaceParams{WorkspaceID: input.WorkspaceID, SearchQuery: input.Query, ResultLimit: input.Limit})
	case "workspace":
		return queries.SearchWikiPagesInWorkspace(ctx, db.SearchWikiPagesInWorkspaceParams{WorkspaceID: input.WorkspaceID, SearchQuery: input.Query, ResultLimit: input.Limit})
	case "project":
		return queries.SearchWikiPagesInProject(ctx, db.SearchWikiPagesInProjectParams{WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID, SearchQuery: input.Query, ResultLimit: input.Limit})
	case "user":
		if !input.AllowPersonal {
			return nil, ErrInvalidWikiPageCreate
		}
		return queries.SearchWikiPagesByOwner(ctx, db.SearchWikiPagesByOwnerParams{OwnerUserID: input.OwnerUserID, SearchQuery: input.Query, ResultLimit: input.Limit})
	default:
		return nil, ErrInvalidWikiPageCreate
	}
}

func (*WikiKnowledgeService) GetRevision(ctx context.Context, queries *db.Queries, access WikiRevisionAccess) (db.WikiPageRevision, error) {
	return queries.GetWikiPageRevisionForActor(ctx, db.GetWikiPageRevisionForActorParams{
		RevisionID: access.RevisionID, WorkspaceID: access.WorkspaceID, OwnerUserID: access.OwnerUserID,
	})
}

func (*WikiKnowledgeService) GetPageRevision(ctx context.Context, queries *db.Queries, page db.WikiPage, revisionID pgtype.UUID) (db.WikiPageRevision, error) {
	return queries.GetWikiPageRevision(ctx, db.GetWikiPageRevisionParams{PageID: page.ID, ID: revisionID})
}

func (*WikiKnowledgeService) ListRevisions(ctx context.Context, queries *db.Queries, page db.WikiPage) ([]db.WikiPageRevision, error) {
	return queries.ListWikiPageRevisions(ctx, page.ID)
}

func (s *WikiKnowledgeService) UpdatePage(ctx context.Context, queries *db.Queries, input WikiPageUpdateInput) (db.WikiPage, error) {
	if !input.Page.ID.Valid || !input.ActorID.Valid || input.ExpectedRevisionNumber <= 0 {
		return db.WikiPage{}, ErrInvalidWikiPageChange
	}
	if input.Path.Valid {
		input.Path.String = normalizeWikiKnowledgePath(input.Path.String)
		if !validWikiKnowledgePath(input.Path.String) {
			return db.WikiPage{}, ErrInvalidWikiPageChange
		}
	}
	if input.Title.Valid {
		input.Title.String = strings.TrimSpace(input.Title.String)
	}
	row, err := queries.UpdateWikiPage(ctx, db.UpdateWikiPageParams{
		PageID: input.Page.ID, ExpectedRevisionNumber: input.ExpectedRevisionNumber,
		NewPath: input.Path, NewTitle: input.Title, NewContent: input.Content, ActorID: input.ActorID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.WikiPage{}, ErrWikiRevisionConflict
	}
	if err != nil {
		return db.WikiPage{}, err
	}
	page := wikiPageFromUpdate(row)
	s.publishPage(protocol.EventWikiPageUpdated, page, "member", input.ActorID)
	s.publishPage(protocol.EventWikiRevisionCreated, page, "member", input.ActorID)
	return page, nil
}

func (s *WikiKnowledgeService) RestorePage(ctx context.Context, queries *db.Queries, input WikiPageRestoreInput) (db.WikiPage, error) {
	if !input.Page.ID.Valid || !input.RevisionID.Valid || !input.ActorID.Valid || input.ExpectedRevisionNumber <= 0 {
		return db.WikiPage{}, ErrInvalidWikiPageChange
	}
	if _, err := queries.GetWikiPageRevision(ctx, db.GetWikiPageRevisionParams{PageID: input.Page.ID, ID: input.RevisionID}); err != nil {
		return db.WikiPage{}, err
	}
	row, err := queries.RestoreWikiPageRevision(ctx, db.RestoreWikiPageRevisionParams{
		PageID: input.Page.ID, RevisionID: input.RevisionID, ActorID: input.ActorID,
		ExpectedRevisionNumber: input.ExpectedRevisionNumber,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.WikiPage{}, ErrWikiRevisionConflict
	}
	if err != nil {
		return db.WikiPage{}, err
	}
	page := wikiPageFromRestore(row)
	s.publishPage(protocol.EventWikiPageUpdated, page, "member", input.ActorID)
	s.publishPage(protocol.EventWikiRevisionRestored, page, "member", input.ActorID)
	return page, nil
}

func (s *WikiKnowledgeService) DeletePage(ctx context.Context, queries *db.Queries, page db.WikiPage, actorID pgtype.UUID) error {
	if !page.ID.Valid || !actorID.Valid {
		return ErrInvalidWikiPageChange
	}
	if err := queries.DeleteWikiPage(ctx, page.ID); err != nil {
		return err
	}
	s.publishPage(protocol.EventWikiPageDeleted, page, "member", actorID)
	return nil
}

func (*WikiKnowledgeService) ListProposals(ctx context.Context, queries *db.Queries, page db.WikiPage) ([]db.WikiPageEditProposal, error) {
	return queries.ListWikiPageEditProposals(ctx, db.ListWikiPageEditProposalsParams{WorkspaceID: page.WorkspaceID, PageID: page.ID})
}

func (*WikiKnowledgeService) GetProposal(ctx context.Context, queries *db.Queries, page db.WikiPage, proposalID pgtype.UUID) (db.WikiPageEditProposal, error) {
	return queries.GetWikiPageEditProposal(ctx, db.GetWikiPageEditProposalParams{WorkspaceID: page.WorkspaceID, PageID: page.ID, ID: proposalID})
}

func (s *WikiKnowledgeService) CreateProposal(ctx context.Context, queries *db.Queries, input WikiProposalCreateInput) (WikiProposalCreateResult, error) {
	input.Path = normalizeWikiKnowledgePath(input.Path)
	input.Title = strings.TrimSpace(input.Title)
	input.Rationale = strings.TrimSpace(input.Rationale)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validWikiProposalCreateInput(input) {
		return WikiProposalCreateResult{}, ErrInvalidWikiProposal
	}
	evidence, err := validateWikiProposalEvidence(ctx, queries, input)
	if err != nil {
		return WikiProposalCreateResult{}, err
	}
	key := input.IdempotencyKey
	existing, err := queries.GetWikiPageEditProposalByIdempotencyKey(ctx, db.GetWikiPageEditProposalByIdempotencyKeyParams{
		WorkspaceID: input.Page.WorkspaceID, AgentID: input.AgentID, IdempotencyKey: key,
	})
	if err == nil {
		if !sameWikiProposalIntent(existing, input, evidence) {
			return WikiProposalCreateResult{}, ErrWikiProposalIdempotencyConflict
		}
		return WikiProposalCreateResult{Proposal: existing}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return WikiProposalCreateResult{}, err
	}

	row, err := queries.CreateWikiPageEditProposal(ctx, db.CreateWikiPageEditProposalParams{
		WorkspaceID: input.Page.WorkspaceID, AgentID: input.AgentID, IdempotencyKey: key,
		BaseRevisionNumber: input.BaseRevisionNumber, ProposedPath: input.Path,
		ProposedTitle: input.Title, ProposedContent: input.Content,
		Rationale: input.Rationale, EvidenceRefs: evidence, PageID: input.Page.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := queries.GetWikiPageEditProposalByIdempotencyKey(ctx, db.GetWikiPageEditProposalByIdempotencyKeyParams{
			WorkspaceID: input.Page.WorkspaceID, AgentID: input.AgentID, IdempotencyKey: key,
		})
		if lookupErr == nil {
			if !sameWikiProposalIntent(existing, input, evidence) {
				return WikiProposalCreateResult{}, ErrWikiProposalIdempotencyConflict
			}
			return WikiProposalCreateResult{Proposal: existing}, nil
		}
		if errors.Is(lookupErr, pgx.ErrNoRows) {
			return WikiProposalCreateResult{}, ErrWikiProposalRevisionConflict
		}
		return WikiProposalCreateResult{}, lookupErr
	}
	if err != nil {
		return WikiProposalCreateResult{}, err
	}
	proposal := db.WikiPageEditProposal(row)
	if !sameWikiProposalIntent(proposal, input, evidence) {
		return WikiProposalCreateResult{}, ErrWikiProposalIdempotencyConflict
	}
	s.publishProposal(protocol.EventWikiProposalCreated, input.Page, proposal, "agent", input.AgentID)
	return WikiProposalCreateResult{Proposal: proposal, Created: true}, nil
}

func (s *WikiKnowledgeService) AcceptProposal(ctx context.Context, queries *db.Queries, input WikiProposalAcceptInput) (db.WikiPage, db.WikiPageEditProposal, error) {
	if !validWikiProposalReview(input.Page, input.Proposal, input.ReviewerID) || input.ExpectedRevisionNumber <= 0 || len([]rune(input.ReviewReason.String)) > 2000 {
		return db.WikiPage{}, db.WikiPageEditProposal{}, ErrInvalidWikiProposal
	}
	if input.Path.Valid {
		input.Path.String = normalizeWikiKnowledgePath(input.Path.String)
		if !validWikiKnowledgePath(input.Path.String) {
			return db.WikiPage{}, db.WikiPageEditProposal{}, ErrInvalidWikiProposal
		}
	}
	if input.Title.Valid {
		input.Title.String = strings.TrimSpace(input.Title.String)
	}
	if input.Proposal.Status != "pending" {
		return db.WikiPage{}, db.WikiPageEditProposal{}, ErrWikiProposalAlreadyReviewed
	}
	if input.Proposal.BaseRevisionNumber != input.ExpectedRevisionNumber || input.Page.CurrentRevisionNumber != input.ExpectedRevisionNumber {
		return db.WikiPage{}, db.WikiPageEditProposal{}, ErrWikiProposalRevisionConflict
	}
	row, err := queries.AcceptWikiPageEditProposal(ctx, db.AcceptWikiPageEditProposalParams{
		WorkspaceID: input.Page.WorkspaceID, PageID: input.Page.ID, ProposalID: input.Proposal.ID,
		ExpectedRevisionNumber: input.ExpectedRevisionNumber, OverridePath: input.Path,
		OverrideTitle: input.Title, OverrideContent: input.Content,
		ReviewerID: input.ReviewerID, ReviewReason: input.ReviewReason,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.WikiPage{}, db.WikiPageEditProposal{}, ErrWikiProposalRevisionConflict
	}
	if err != nil {
		return db.WikiPage{}, db.WikiPageEditProposal{}, err
	}
	page := wikiPageFromProposalAcceptance(row)
	proposal, err := queries.GetWikiPageEditProposal(ctx, db.GetWikiPageEditProposalParams{
		WorkspaceID: page.WorkspaceID, PageID: page.ID, ID: input.Proposal.ID,
	})
	if err != nil {
		return db.WikiPage{}, db.WikiPageEditProposal{}, err
	}
	s.publishProposal(protocol.EventWikiProposalReviewed, page, proposal, "member", input.ReviewerID)
	s.publishPage(protocol.EventWikiPageUpdated, page, "member", input.ReviewerID)
	s.publishPage(protocol.EventWikiRevisionCreated, page, "member", input.ReviewerID)
	return page, proposal, nil
}

func (s *WikiKnowledgeService) RejectProposal(ctx context.Context, queries *db.Queries, input WikiProposalRejectInput) (db.WikiPageEditProposal, error) {
	if !validWikiProposalReview(input.Page, input.Proposal, input.ReviewerID) || len([]rune(input.ReviewReason.String)) > 2000 {
		return db.WikiPageEditProposal{}, ErrInvalidWikiProposal
	}
	if input.Proposal.Status != "pending" {
		return db.WikiPageEditProposal{}, ErrWikiProposalAlreadyReviewed
	}
	proposal, err := queries.RejectWikiPageEditProposal(ctx, db.RejectWikiPageEditProposalParams{
		WorkspaceID: input.Page.WorkspaceID, PageID: input.Page.ID, ID: input.Proposal.ID,
		ReviewerID: input.ReviewerID, ReviewReason: input.ReviewReason,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.WikiPageEditProposal{}, ErrWikiProposalAlreadyReviewed
	}
	if err != nil {
		return db.WikiPageEditProposal{}, err
	}
	s.publishProposal(protocol.EventWikiProposalReviewed, input.Page, proposal, "member", input.ReviewerID)
	return proposal, nil
}

func validateWikiProposalEvidence(ctx context.Context, queries *db.Queries, input WikiProposalCreateInput) ([]byte, error) {
	if queries == nil || len(input.EvidenceRefs) == 0 || len(input.EvidenceRefs) > maxWikiEvidenceRefs || !input.AuthenticatedTaskID.Valid {
		return nil, ErrInvalidWikiProposalEvidence
	}
	task, err := queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{
		ID: input.AuthenticatedTaskID, WorkspaceID: input.Page.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && task.AgentID != input.AgentID) {
		return nil, ErrInvalidWikiProposalEvidence
	}
	if err != nil {
		return nil, err
	}
	canonical := make([]string, 0, len(input.EvidenceRefs))
	for _, raw := range input.EvidenceRefs {
		kind, rawID, ok := strings.Cut(strings.TrimSpace(raw), ":")
		if !ok || rawID == "" || strings.Contains(rawID, ":") {
			return nil, ErrInvalidWikiProposalEvidence
		}
		evidenceID, err := util.ParseUUID(rawID)
		if err != nil {
			return nil, ErrInvalidWikiProposalEvidence
		}
		switch kind {
		case "task":
			if evidenceID != input.AuthenticatedTaskID {
				return nil, ErrInvalidWikiProposalEvidence
			}
		case "room":
			_, err := queries.GetRoomArtifactInWorkspaceForWikiEvidence(ctx, db.GetRoomArtifactInWorkspaceForWikiEvidenceParams{
				ID: evidenceID, WorkspaceID: input.Page.WorkspaceID,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInvalidWikiProposalEvidence
			}
			if err != nil {
				return nil, err
			}
		default:
			return nil, ErrInvalidWikiProposalEvidence
		}
		canonical = append(canonical, kind+":"+util.UUIDToString(evidenceID))
	}
	return json.Marshal(canonical)
}

func sameWikiProposalIntent(proposal db.WikiPageEditProposal, input WikiProposalCreateInput, evidence []byte) bool {
	return proposal.PageID == input.Page.ID && proposal.BaseRevisionNumber == input.BaseRevisionNumber &&
		proposal.ProposedPath == input.Path && proposal.ProposedTitle == input.Title &&
		proposal.ProposedContent == input.Content && proposal.Rationale == input.Rationale &&
		bytes.Equal(proposal.EvidenceRefs, evidence)
}

func (s *WikiKnowledgeService) publishPage(eventType string, page db.WikiPage, actorType string, actorID pgtype.UUID) {
	if s.events == nil {
		return
	}
	payload := protocol.WikiEventPayload{
		PageID: util.UUIDToString(page.ID), Scope: page.Scope,
		ProjectID: optionalWikiUUID(page.ProjectID), RevisionID: util.UUIDToString(page.CurrentRevisionID),
		RevisionNumber: page.CurrentRevisionNumber, RecipientID: optionalWikiUUID(page.OwnerUserID),
	}
	s.events.Publish(events.Event{
		Type: eventType, WorkspaceID: optionalWikiUUID(page.WorkspaceID),
		ActorType: actorType, ActorID: optionalWikiUUID(actorID), Payload: payload,
	})
}

func (s *WikiKnowledgeService) publishProposal(eventType string, page db.WikiPage, proposal db.WikiPageEditProposal, actorType string, actorID pgtype.UUID) {
	if s.events == nil {
		return
	}
	payload := protocol.WikiEventPayload{
		PageID: util.UUIDToString(page.ID), Scope: page.Scope,
		ProjectID: optionalWikiUUID(page.ProjectID), ProposalID: util.UUIDToString(proposal.ID),
		BaseRevisionNumber: proposal.BaseRevisionNumber, ReviewStatus: proposal.Status,
		RecipientID: optionalWikiUUID(page.OwnerUserID), AcceptedRevisionID: optionalWikiUUID(proposal.AcceptedRevisionID),
	}
	if proposal.AcceptedRevisionID.Valid {
		payload.AcceptedRevisionNumber = page.CurrentRevisionNumber
	}
	s.events.Publish(events.Event{
		Type: eventType, WorkspaceID: optionalWikiUUID(page.WorkspaceID),
		ActorType: actorType, ActorID: optionalWikiUUID(actorID), Payload: payload,
	})
}

func optionalWikiUUID(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return util.UUIDToString(value)
}

func wikiPageFromProvenanceCreate(row db.CreateWikiPageWithProvenanceRow) db.WikiPage {
	return db.WikiPage{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Scope: row.Scope,
		ProjectID: row.ProjectID, OwnerUserID: row.OwnerUserID,
		Path: row.Path, Title: row.Title, Content: row.Content,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		CurrentRevisionNumber: row.CurrentRevisionNumber, CurrentRevisionID: row.CurrentRevisionID,
		ContentDigest: row.ContentDigest, LastSourceKind: row.LastSourceKind,
		LastActorType: row.LastActorType, LastActorID: row.LastActorID,
	}
}

func wikiPageFromUpdate(row db.UpdateWikiPageRow) db.WikiPage {
	return db.WikiPage{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Scope: row.Scope, ProjectID: row.ProjectID,
		OwnerUserID: row.OwnerUserID, Path: row.Path, Title: row.Title, Content: row.Content,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		CurrentRevisionNumber: row.CurrentRevisionNumber, CurrentRevisionID: row.CurrentRevisionID,
		ContentDigest: row.ContentDigest, LastSourceKind: row.LastSourceKind,
		LastActorType: row.LastActorType, LastActorID: row.LastActorID,
	}
}

func wikiPageFromRestore(row db.RestoreWikiPageRevisionRow) db.WikiPage {
	return db.WikiPage{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Scope: row.Scope, ProjectID: row.ProjectID,
		OwnerUserID: row.OwnerUserID, Path: row.Path, Title: row.Title, Content: row.Content,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		CurrentRevisionNumber: row.CurrentRevisionNumber, CurrentRevisionID: row.CurrentRevisionID,
		ContentDigest: row.ContentDigest, LastSourceKind: row.LastSourceKind,
		LastActorType: row.LastActorType, LastActorID: row.LastActorID,
	}
}

func wikiPageFromProposalAcceptance(row db.AcceptWikiPageEditProposalRow) db.WikiPage {
	return db.WikiPage{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Scope: row.Scope, ProjectID: row.ProjectID,
		OwnerUserID: row.OwnerUserID, Path: row.Path, Title: row.Title, Content: row.Content,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		CurrentRevisionNumber: row.CurrentRevisionNumber, CurrentRevisionID: row.CurrentRevisionID,
		ContentDigest: row.ContentDigest, LastSourceKind: row.LastSourceKind,
		LastActorType: row.LastActorType, LastActorID: row.LastActorID,
	}
}

func validWikiPageCreateInput(input WikiPageCreateInput) bool {
	if !validWikiKnowledgePath(input.Path) || strings.TrimSpace(input.Title) == "" || !input.ActorID.Valid {
		return false
	}
	if input.ActorType != "member" && input.ActorType != "agent" && input.ActorType != "system" {
		return false
	}
	switch input.SourceKind {
	case "human", "restore", "system":
	case "room_promotion", "agent_proposal":
		if !input.SourceRefID.Valid {
			return false
		}
	default:
		return false
	}
	switch input.Scope {
	case "workspace":
		return input.WorkspaceID.Valid && !input.ProjectID.Valid && !input.OwnerUserID.Valid
	case "project":
		return input.WorkspaceID.Valid && input.ProjectID.Valid && !input.OwnerUserID.Valid
	case "user":
		return !input.WorkspaceID.Valid && !input.ProjectID.Valid && input.OwnerUserID.Valid && input.SourceKind == "human" && input.ActorType == "member"
	default:
		return false
	}
}

func validWikiProposalCreateInput(input WikiProposalCreateInput) bool {
	return input.Page.ID.Valid && input.Page.WorkspaceID.Valid &&
		(input.Page.Scope == "workspace" || input.Page.Scope == "project") &&
		input.AgentID.Valid && input.AuthenticatedTaskID.Valid && input.BaseRevisionNumber > 0 &&
		validWikiKnowledgePath(input.Path) &&
		input.Rationale != "" && len([]rune(input.Rationale)) <= 8000 &&
		input.IdempotencyKey != "" && len(input.IdempotencyKey) <= 200
}

func validWikiProposalReview(page db.WikiPage, proposal db.WikiPageEditProposal, reviewerID pgtype.UUID) bool {
	return page.ID.Valid && page.WorkspaceID.Valid && reviewerID.Valid &&
		(page.Scope == "workspace" || page.Scope == "project") &&
		proposal.ID.Valid && proposal.WorkspaceID == page.WorkspaceID && proposal.PageID == page.ID
}

func validWikiKnowledgePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return false
	}
	cleaned := normalizeWikiKnowledgePath(value)
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../") &&
		!strings.Contains(cleaned, "\\") && strings.HasSuffix(strings.ToLower(cleaned), ".md")
}

func normalizeWikiKnowledgePath(value string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
}
