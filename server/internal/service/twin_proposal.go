package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *TwinService) EnsureProposal(ctx context.Context, workspaceID, wikiRevisionID, requestedBy pgtype.UUID) (TwinProposalResult, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("begin twin proposal: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.Queries.WithTx(tx)
	if err := lockTwinWorkspace(ctx, queries, workspaceID); err != nil {
		return TwinProposalResult{}, err
	}
	result, err := s.ensureProposal(ctx, queries, workspaceID, wikiRevisionID, requestedBy)
	if err != nil {
		return TwinProposalResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TwinProposalResult{}, fmt.Errorf("commit twin proposal: %w", err)
	}
	return result, nil
}

func (s *TwinService) AfterLMWikiAccepted(ctx context.Context, queries *db.Queries, revision db.LmWikiRevision) error {
	if err := queries.LockTwinLifecycle(ctx, revision.WorkspaceID); err != nil {
		return fmt.Errorf("lock accepted Wiki twin lifecycle: %w", err)
	}
	_, err := s.ensureProposal(ctx, queries, revision.WorkspaceID, revision.ID, revision.RequestedByID)
	if errors.Is(err, ErrTwinAlreadyCurrent) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ensure accepted Wiki twin proposal: %w", err)
	}
	return nil
}

func (s *TwinService) ensureProposal(ctx context.Context, queries *db.Queries, workspaceID, wikiRevisionID, requestedBy pgtype.UUID) (TwinProposalResult, error) {
	revision, err := loadTwinSourceForBuild(ctx, queries, workspaceID, wikiRevisionID)
	if err != nil {
		return TwinProposalResult{}, err
	}
	current, currentErr := queries.GetCurrentTwinVersion(ctx, workspaceID)
	if currentErr == nil && current.SourceWikiRevisionID == wikiRevisionID {
		return TwinProposalResult{}, ErrTwinAlreadyCurrent
	}
	if currentErr != nil && !errors.Is(currentErr, pgx.ErrNoRows) {
		return TwinProposalResult{}, fmt.Errorf("load current twin version: %w", currentErr)
	}
	kind := "initial"
	baseID := pgtype.UUID{}
	priorAssertions := []TwinAssertion(nil)
	if currentErr == nil {
		kind = "evolution"
		baseID = current.ID
		priorAssertions, err = twinAssertions(current.Content)
		if err != nil {
			return TwinProposalResult{}, err
		}
	}
	naturalKey := db.GetTwinProposalByNaturalKeyParams{WorkspaceID: workspaceID, Kind: kind, SourceWikiRevisionID: revision.ID, BaseTwinVersionID: baseID}
	existing, err := queries.GetTwinProposalByNaturalKey(ctx, naturalKey)
	if err == nil {
		return TwinProposalResult{Proposal: existing}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalResult{}, fmt.Errorf("load twin proposal natural key: %w", err)
	}
	build, err := buildTwinFromRevision(ctx, queries, workspaceID, revision, priorAssertions)
	if err != nil {
		return TwinProposalResult{}, err
	}
	created, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{WorkspaceID: workspaceID, Kind: kind, SourceWikiRevisionID: revision.ID, BaseTwinVersionID: baseID, Content: build.CanonicalJSON, ContentDigest: build.ContentDigest, RequestedByID: requestedBy})
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("create twin proposal: %w", err)
	}
	proposal, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: created.ID})
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("load created twin proposal: %w", err)
	}
	return TwinProposalResult{Created: true, Proposal: proposal}, nil
}
