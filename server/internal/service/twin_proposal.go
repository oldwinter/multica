package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type twinProposalPlan struct {
	kind              string
	baseTwinVersionID pgtype.UUID
	evidence          TwinAcceptedEvidence
	builderInput      TwinBuilderInput
	existing          *db.TwinProposal
}

// EnsureProposal is the explicit Twin Build operation. Its production caller
// is the owner/admin-gated HTTP endpoint; automatic Wiki acceptance never calls
// it. The accepted evidence may therefore leave storage only on this path.
func (s *TwinService) EnsureProposal(ctx context.Context, workspaceID, wikiRevisionID, requestedBy pgtype.UUID) (TwinProposalResult, error) {
	return s.ensureProposalWithEgress(ctx, workspaceID, wikiRevisionID, requestedBy, true)
}

func (s *TwinService) ensureProposalWithEgress(ctx context.Context, workspaceID, wikiRevisionID, requestedBy pgtype.UUID, egressEligible bool) (TwinProposalResult, error) {
	plan, err := s.prepareProposalPlan(ctx, s.Queries, workspaceID, wikiRevisionID)
	if err != nil {
		return TwinProposalResult{}, err
	}
	if plan.existing != nil {
		return TwinProposalResult{Proposal: *plan.existing}, nil
	}
	build, err := GenerateTwinProposal(ctx, s.ProposalGenerator, TwinProposalGenerationInput{BuilderInput: plan.builderInput, EgressEligible: egressEligible})
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("generate twin proposal: %w", err)
	}

	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("begin twin proposal persistence: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.Queries.WithTx(tx)
	if err := lockTwinWorkspace(ctx, queries, workspaceID); err != nil {
		return TwinProposalResult{}, err
	}
	fresh, err := s.prepareProposalPlan(ctx, queries, workspaceID, wikiRevisionID)
	if err != nil {
		return TwinProposalResult{}, err
	}
	if fresh.kind != plan.kind || fresh.baseTwinVersionID != plan.baseTwinVersionID {
		return TwinProposalResult{}, ErrTwinBaseStale
	}
	if fresh.evidence.RevisionID != plan.evidence.RevisionID || fresh.evidence.SourceDigest != plan.evidence.SourceDigest || !bytes.Equal(fresh.evidence.CanonicalContent, plan.evidence.CanonicalContent) {
		return TwinProposalResult{}, ErrTwinWikiStale
	}
	if fresh.existing != nil {
		if err := tx.Commit(ctx); err != nil {
			return TwinProposalResult{}, fmt.Errorf("commit existing twin proposal: %w", err)
		}
		return TwinProposalResult{Proposal: *fresh.existing}, nil
	}
	created, err := queries.CreateTwinProposalV2(ctx, db.CreateTwinProposalV2Params{
		WorkspaceID: workspaceID, Kind: plan.kind, SourceWikiRevisionID: wikiRevisionID,
		BaseTwinVersionID: plan.baseTwinVersionID, Content: build.CanonicalJSON,
		ContentDigest: build.ContentDigest, RequestedByID: requestedBy,
	})
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("create schema-v2 twin proposal: %w", err)
	}
	proposal, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: created.ID})
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("load created twin proposal: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TwinProposalResult{}, fmt.Errorf("commit twin proposal: %w", err)
	}
	return TwinProposalResult{Created: true, Proposal: proposal}, nil
}

func (s *TwinService) prepareProposalPlan(ctx context.Context, queries *db.Queries, workspaceID, wikiRevisionID pgtype.UUID) (twinProposalPlan, error) {
	provider := s.EvidenceProvider
	if transactional, ok := provider.(transactionalTwinEvidenceProvider); ok {
		provider = transactional.withQueries(queries)
	}
	evidence, err := provider.LoadAcceptedEvidence(ctx, workspaceID, wikiRevisionID)
	if err != nil {
		return twinProposalPlan{}, err
	}
	if evidence.RevisionID != wikiRevisionID.String() {
		return twinProposalPlan{}, ErrTwinWikiStale
	}
	evidenceSchemaVersion, _, err := canonicalTwinEvidence(TwinBuilderInput{CanonicalEvidence: evidence.CanonicalContent, Citations: evidence.Citations})
	if err != nil {
		return twinProposalPlan{}, err
	}
	content := LMWikiContent{SchemaVersion: evidenceSchemaVersion}
	if err := json.Unmarshal(evidence.CanonicalContent, &content); err != nil {
		return twinProposalPlan{}, fmt.Errorf("decode twin evidence compatibility view: %w", err)
	}
	current, currentErr := queries.GetCurrentTwinVersion(ctx, workspaceID)
	if currentErr == nil && current.SourceWikiRevisionID == wikiRevisionID {
		return twinProposalPlan{}, ErrTwinAlreadyCurrent
	}
	if currentErr != nil && !errors.Is(currentErr, pgx.ErrNoRows) {
		return twinProposalPlan{}, fmt.Errorf("load current twin version: %w", currentErr)
	}
	kind := "initial"
	baseID := pgtype.UUID{}
	priorAssertions := []TwinAssertion(nil)
	if currentErr == nil {
		kind = "evolution"
		baseID = current.ID
		priorAssertions, err = twinAssertions(current.Content)
		if err != nil {
			return twinProposalPlan{}, err
		}
	}

	plan := twinProposalPlan{
		kind: kind, baseTwinVersionID: baseID, evidence: evidence,
		builderInput: TwinBuilderInput{
			SourceWikiRevisionID: evidence.RevisionID, SourceDigest: evidence.SourceDigest,
			CanonicalEvidence: append(json.RawMessage(nil), evidence.CanonicalContent...), EvidenceSchemaVersion: evidenceSchemaVersion,
			Content: content, Citations: append([]LMWikiCitation(nil), evidence.Citations...), PriorAssertions: priorAssertions,
		},
	}
	existing, err := queries.GetTwinProposalByNaturalKey(ctx, db.GetTwinProposalByNaturalKeyParams{
		WorkspaceID: workspaceID, Kind: kind, SourceWikiRevisionID: wikiRevisionID, BaseTwinVersionID: baseID,
	})
	if err == nil {
		plan.existing = &existing
		return plan, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return twinProposalPlan{}, fmt.Errorf("load twin proposal natural key: %w", err)
	}
	return plan, nil
}
