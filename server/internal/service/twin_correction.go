package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const twinProposalEditMaxBytes = 100_000

// CorrectProposal creates an append-only, deterministically validated
// successor. The reviewed proposal and every signed version remain immutable.
func (s *TwinService) CorrectProposal(ctx context.Context, workspaceID, proposalID, requestedBy pgtype.UUID, raw json.RawMessage) (TwinProposalResult, error) {
	edited, err := canonicalTwinProposalEdit(raw)
	if err != nil {
		return TwinProposalResult{}, err
	}

	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("begin Twin proposal correction: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.Queries.WithTx(tx)
	if err := lockTwinWorkspace(ctx, queries, workspaceID); err != nil {
		return TwinProposalResult{}, err
	}
	target, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: proposalID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalResult{}, ErrTwinNotFound
	}
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("load corrected Twin proposal: %w", err)
	}
	if target.Kind != "initial" && target.Kind != "evolution" && target.Kind != "correction" {
		return TwinProposalResult{}, ErrTwinInvalidReview
	}
	if _, err := queries.GetTwinProposalReview(ctx, db.GetTwinProposalReviewParams{WorkspaceID: workspaceID, ProposalID: proposalID}); err == nil {
		return TwinProposalResult{}, ErrTwinAlreadyDecided
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalResult{}, fmt.Errorf("load corrected Twin proposal review: %w", err)
	}

	plan, err := s.prepareProposalPlan(ctx, queries, workspaceID, target.SourceWikiRevisionID)
	if err != nil {
		return TwinProposalResult{}, err
	}
	if plan.baseTwinVersionID != target.BaseTwinVersionID {
		return TwinProposalResult{}, ErrTwinBaseStale
	}
	build, err := ValidateTwinProposal(plan.builderInput, TwinProposalCandidate{Assertions: edited})
	if err != nil {
		return TwinProposalResult{}, err
	}
	targetAssertions, err := twinAssertions(target.Content)
	if err != nil {
		return TwinProposalResult{}, err
	}
	if twinAssertionsEqualIgnoringProvenance(targetAssertions, build.Content.Assertions) {
		return TwinProposalResult{}, ErrTwinProposalUnchanged
	}

	existing, err := queries.GetTwinProposalReplacement(ctx, db.GetTwinProposalReplacementParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if err == nil {
		if existing.ContentDigest != build.ContentDigest {
			return TwinProposalResult{}, ErrTwinProposalSuperseded
		}
		if err := tx.Commit(ctx); err != nil {
			return TwinProposalResult{}, fmt.Errorf("commit repeated Twin proposal correction: %w", err)
		}
		return TwinProposalResult{Proposal: existing}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalResult{}, fmt.Errorf("load existing Twin proposal correction: %w", err)
	}
	created, err := queries.CreateTwinProposalCorrectionV2(ctx, db.CreateTwinProposalCorrectionV2Params{
		WorkspaceID: workspaceID, ReplacesProposalID: proposalID,
		Content: build.CanonicalJSON, ContentDigest: build.ContentDigest,
		RequestedByID: requestedBy,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalResult{}, ErrTwinProposalSuperseded
	}
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("create Twin proposal correction: %w", err)
	}
	proposal, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: created.ID})
	if err != nil {
		return TwinProposalResult{}, fmt.Errorf("load Twin proposal correction: %w", err)
	}
	if proposal.ContentDigest != build.ContentDigest {
		return TwinProposalResult{}, ErrTwinProposalSuperseded
	}
	if err := tx.Commit(ctx); err != nil {
		return TwinProposalResult{}, fmt.Errorf("commit Twin proposal correction: %w", err)
	}
	return TwinProposalResult{Created: true, Proposal: proposal}, nil
}

func canonicalTwinProposalEdit(raw json.RawMessage) ([]TwinAssertion, error) {
	if len(raw) == 0 || len(raw) > twinProposalEditMaxBytes {
		return nil, ErrTwinInvalidReview
	}
	var edited []TwinAssertion
	if err := json.Unmarshal(raw, &edited); err != nil || edited == nil {
		return nil, ErrTwinInvalidReview
	}
	for index := range edited {
		edited[index].Provenance = TwinAssertionProvenance{Kind: TwinProvenanceHumanEdit, Generator: "human-edit-v1"}
	}
	return edited, nil
}

func twinAssertionsEqualIgnoringProvenance(left, right []TwinAssertion) bool {
	if len(left) != len(right) {
		return false
	}
	leftByID := make(map[string]TwinAssertion, len(left))
	for _, assertion := range left {
		assertion.Provenance = TwinAssertionProvenance{}
		leftByID[assertion.ID] = assertion
	}
	for _, assertion := range right {
		previous, ok := leftByID[assertion.ID]
		if !ok {
			return false
		}
		assertion.Provenance = TwinAssertionProvenance{}
		leftJSON, leftErr := json.Marshal(previous)
		rightJSON, rightErr := json.Marshal(assertion)
		if leftErr != nil || rightErr != nil || string(leftJSON) != string(rightJSON) {
			return false
		}
	}
	return true
}
