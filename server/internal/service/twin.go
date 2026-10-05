package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	TwinReviewReasonLimit        = 2000
	twinSupersededProposalReason = "Superseded by a newer signed Twin proposal."
	initialTwinReviewSteps       = `[{"id":"import","state":"complete"},{"id":"generate","state":"complete"},{"id":"topic","state":"complete"},{"id":"coordinate","state":"complete"},{"id":"accept","state":"complete"},{"id":"deposition","state":"current"}]`
	evolvedTwinReviewSteps       = `[{"id":"import","state":"complete"},{"id":"generate","state":"complete"},{"id":"topic","state":"complete"},{"id":"coordinate","state":"complete"},{"id":"accept","state":"complete"},{"id":"deposition","state":"complete"}]`
)

var (
	ErrTwinNotFound           = errors.New("twin artifact not found")
	ErrTwinWikiNotAccepted    = errors.New("twin source wiki revision is not accepted")
	ErrTwinAlreadyCurrent     = errors.New("twin source wiki revision is already current")
	ErrTwinAlreadyDecided     = errors.New("twin proposal is already decided")
	ErrTwinBaseStale          = errors.New("twin proposal base version is stale")
	ErrTwinWikiStale          = errors.New("twin proposal wiki revision is stale")
	ErrTwinInvalidReview      = errors.New("invalid twin proposal review")
	ErrTwinProposalSuperseded = errors.New("twin proposal is superseded")
	ErrTwinProposalUnchanged  = errors.New("twin proposal edit has no changes")
)

type TwinTxStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TwinService struct {
	Queries             *db.Queries
	TxStarter           TwinTxStarter
	ProposalGenerator   TwinProposalGenerator
	EvidenceProvider    TwinEvidenceProvider
	beforeVersionCreate func() error
}

type TwinProposalResult struct {
	Created  bool
	Proposal db.TwinProposal
}

type TwinVersionResult struct {
	Created bool
	Version db.TwinVersion
}

type TwinProposalReviewResult struct {
	Created  bool
	Proposal TwinProposalDetail
}

func NewTwinService(queries *db.Queries, txStarter TwinTxStarter) *TwinService {
	return &TwinService{
		Queries:           queries,
		TxStarter:         txStarter,
		ProposalGenerator: InventoryTwinProposalGenerator{},
		EvidenceProvider:  NewDBTwinEvidenceProvider(queries),
	}
}

func (s *TwinService) AcceptProposal(ctx context.Context, workspaceID, proposalID, reviewerID pgtype.UUID) (TwinVersionResult, error) {
	result, err := s.reviewProposal(ctx, workspaceID, proposalID, reviewerID, "accepted", "")
	return result.Version, err
}

func (s *TwinService) RejectProposal(ctx context.Context, workspaceID, proposalID, reviewerID pgtype.UUID, reason string) (TwinProposalReviewResult, error) {
	result, err := s.reviewProposal(ctx, workspaceID, proposalID, reviewerID, "rejected", reason)
	return TwinProposalReviewResult{Created: result.ReviewCreated, Proposal: result.Proposal}, err
}

type twinReviewResult struct {
	ReviewCreated bool
	Proposal      TwinProposalDetail
	Version       TwinVersionResult
}

func (s *TwinService) reviewProposal(ctx context.Context, workspaceID, proposalID, reviewerID pgtype.UUID, decision, reason string) (twinReviewResult, error) {
	reason = strings.TrimSpace(reason)
	if (decision != "accepted" && decision != "rejected") || len([]rune(reason)) > TwinReviewReasonLimit || (decision == "accepted" && reason != "") {
		return twinReviewResult{}, ErrTwinInvalidReview
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return twinReviewResult{}, fmt.Errorf("begin twin review: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.Queries.WithTx(tx)
	if err := lockTwinWorkspace(ctx, queries, workspaceID); err != nil {
		return twinReviewResult{}, err
	}
	proposal, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: proposalID})
	if errors.Is(err, pgx.ErrNoRows) {
		return twinReviewResult{}, ErrTwinNotFound
	}
	if err != nil {
		return twinReviewResult{}, fmt.Errorf("load reviewed twin proposal: %w", err)
	}
	existing, err := queries.GetTwinProposalReview(ctx, db.GetTwinProposalReviewParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if err == nil {
		if existing.Decision != decision {
			return twinReviewResult{}, ErrTwinAlreadyDecided
		}
		if decision == "accepted" {
			version, versionErr := queries.GetTwinVersionByProposal(ctx, db.GetTwinVersionByProposalParams{WorkspaceID: workspaceID, ProposalID: proposalID})
			if versionErr != nil {
				return twinReviewResult{}, fmt.Errorf("load repeated twin version: %w", versionErr)
			}
			currentVersion, currentErr := queries.GetCurrentTwinVersion(ctx, workspaceID)
			if currentErr != nil {
				return twinReviewResult{}, fmt.Errorf("load current twin version for profile repair: %w", currentErr)
			}
			currentProposal, currentErr := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: currentVersion.ProposalID})
			if currentErr != nil {
				return twinReviewResult{}, fmt.Errorf("load current twin proposal for profile repair: %w", currentErr)
			}
			if err := persistSignedTwinProfile(ctx, queries, workspaceID, currentProposal, currentVersion); err != nil {
				return twinReviewResult{}, err
			}
			if err := syncTwinDepositionReview(ctx, queries, workspaceID, proposal, decision); err != nil {
				return twinReviewResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return twinReviewResult{}, fmt.Errorf("commit repeated twin review: %w", err)
			}
			return twinReviewResult{Version: TwinVersionResult{Version: version}}, nil
		}
		if err := syncTwinDepositionReview(ctx, queries, workspaceID, proposal, decision); err != nil {
			return twinReviewResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return twinReviewResult{}, fmt.Errorf("commit repeated twin review: %w", err)
		}
		detail, detailErr := s.ProposalDetail(ctx, workspaceID, proposalID)
		return twinReviewResult{Proposal: detail}, detailErr
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return twinReviewResult{}, fmt.Errorf("load existing twin review: %w", err)
	}
	if err := validateTwinProposalFreshness(ctx, queries, workspaceID, proposal); err != nil {
		return twinReviewResult{}, err
	}
	if err := validateTwinProposalHead(ctx, queries, workspaceID, proposal); err != nil {
		return twinReviewResult{}, err
	}
	if err := validateTwinDepositionReview(ctx, queries, workspaceID, proposal, decision); err != nil {
		return twinReviewResult{}, err
	}
	_, err = queries.CreateTwinProposalReview(ctx, db.CreateTwinProposalReviewParams{WorkspaceID: workspaceID, ProposalID: proposalID, Decision: decision, ReviewerID: reviewerID, Reason: pgtype.Text{String: reason, Valid: reason != ""}})
	if err != nil {
		return twinReviewResult{}, fmt.Errorf("create twin proposal review: %w", err)
	}
	if decision == "rejected" {
		if err := syncTwinDepositionReview(ctx, queries, workspaceID, proposal, decision); err != nil {
			return twinReviewResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return twinReviewResult{}, fmt.Errorf("commit twin rejection: %w", err)
		}
		detail, detailErr := s.ProposalDetail(ctx, workspaceID, proposalID)
		return twinReviewResult{ReviewCreated: true, Proposal: detail}, detailErr
	}
	if s.beforeVersionCreate != nil {
		if err := s.beforeVersionCreate(); err != nil {
			return twinReviewResult{}, fmt.Errorf("before twin version create: %w", err)
		}
	}
	created, err := queries.CreateTwinVersion(ctx, db.CreateTwinVersionParams{WorkspaceID: workspaceID, ProposalID: proposalID, SignedOffByID: reviewerID})
	if err != nil {
		return twinReviewResult{}, fmt.Errorf("create twin version: %w", err)
	}
	if err := queries.RejectOtherPendingTwinProposals(ctx, db.RejectOtherPendingTwinProposalsParams{WorkspaceID: workspaceID, AcceptedProposalID: proposalID, ReviewerID: reviewerID, Reason: twinSupersededProposalReason}); err != nil {
		return twinReviewResult{}, fmt.Errorf("reject superseded twin proposals: %w", err)
	}
	if err := queries.RejectReviewedTwinDepositions(ctx, workspaceID); err != nil {
		return twinReviewResult{}, fmt.Errorf("reject superseded Twin depositions: %w", err)
	}
	if err := syncTwinDepositionReview(ctx, queries, workspaceID, proposal, decision); err != nil {
		return twinReviewResult{}, err
	}
	version, err := queries.GetTwinVersion(ctx, db.GetTwinVersionParams{WorkspaceID: workspaceID, ID: created.ID})
	if err != nil {
		return twinReviewResult{}, fmt.Errorf("load created twin version: %w", err)
	}
	if err := persistSignedTwinProfile(ctx, queries, workspaceID, proposal, version); err != nil {
		return twinReviewResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return twinReviewResult{}, fmt.Errorf("commit twin acceptance: %w", err)
	}
	return twinReviewResult{ReviewCreated: true, Version: TwinVersionResult{Created: true, Version: version}}, nil
}

func syncTwinDepositionReview(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, proposal db.TwinProposal, decision string) error {
	if proposal.Kind != "deposition" {
		return nil
	}
	deposition, err := queries.GetTwinDepositionByProposal(ctx, db.GetTwinDepositionByProposalParams{WorkspaceID: workspaceID, ProposalID: proposal.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTwinNotFound
	}
	if err != nil {
		return fmt.Errorf("load reviewed Twin deposition: %w", err)
	}
	if _, err := queries.UpdateTwinDepositionState(ctx, db.UpdateTwinDepositionStateParams{WorkspaceID: workspaceID, ID: deposition.ID, State: decision}); errors.Is(err, pgx.ErrNoRows) {
		return ErrTwinAlreadyDecided
	} else if err != nil {
		return fmt.Errorf("sync Twin deposition review: %w", err)
	}
	return nil
}

func persistSignedTwinProfile(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, proposal db.TwinProposal, version db.TwinVersion) error {
	reviewSteps := initialTwinReviewSteps
	if proposal.BaseTwinVersionID.Valid || proposal.Kind == "deposition" {
		reviewSteps = evolvedTwinReviewSteps
	}
	if _, err := queries.UpsertSignedTwinProfile(ctx, db.UpsertSignedTwinProfileParams{
		WorkspaceID:  workspaceID,
		ReviewDigest: version.ContentDigest,
		ReviewSteps:  []byte(reviewSteps),
	}); err != nil {
		return fmt.Errorf("persist signed twin review profile: %w", err)
	}
	return nil
}

func validateTwinProposalHead(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, proposal db.TwinProposal) error {
	_, err := queries.GetTwinProposalReplacement(ctx, db.GetTwinProposalReplacementParams{
		WorkspaceID: workspaceID,
		ProposalID:  proposal.ID,
	})
	if err == nil {
		return ErrTwinProposalSuperseded
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load Twin proposal replacement: %w", err)
	}
	return nil
}

func lockTwinWorkspace(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) error {
	if _, err := queries.LockWorkspaceForWikiArtifactCreate(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTwinNotFound
		}
		return fmt.Errorf("lock twin workspace: %w", err)
	}
	if err := queries.LockLMWikiLifecycle(ctx, workspaceID); err != nil {
		return fmt.Errorf("lock twin wiki lifecycle: %w", err)
	}
	if err := queries.LockTwinLifecycle(ctx, workspaceID); err != nil {
		return fmt.Errorf("lock twin lifecycle: %w", err)
	}
	return nil
}
