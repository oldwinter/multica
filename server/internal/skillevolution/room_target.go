package skillevolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/room"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type roomProposalQueries interface {
	LockWorkspaceSkillForEvolution(context.Context, db.LockWorkspaceSkillForEvolutionParams) (db.Skill, error)
	LockWorkspaceSkillFilesForEvolution(context.Context, db.LockWorkspaceSkillFilesForEvolutionParams) ([]db.SkillFile, error)
	GetSkillEvolutionLoop(context.Context, db.GetSkillEvolutionLoopParams) (db.SkillEvolutionLoop, error)
	GetSkillEvolutionRevisionByHash(context.Context, db.GetSkillEvolutionRevisionByHashParams) (db.SkillEvolutionRevision, error)
	GetSkillEvolutionProposalByGenerationKey(context.Context, db.GetSkillEvolutionProposalByGenerationKeyParams) (db.SkillEvolutionProposal, error)
	CreateSkillEvolutionProposal(context.Context, db.CreateSkillEvolutionProposalParams) (db.SkillEvolutionProposal, error)
}

// RoomSkillProposalTarget registers only executable_procedure. Create queues a
// durable proposal in the Room promotion transaction; the post-commit callback
// runs the lifecycle. A failed callback leaves that queued proposal available
// to ProcessRoomArtifactTarget for idempotent recovery.
type RoomSkillProposalTarget struct {
	lifecycle *Lifecycle
	source    *RoomCandidateSource
	metrics   *Metrics
}

func NewRoomSkillProposalTarget(lifecycle *Lifecycle, source *RoomCandidateSource, metrics ...*Metrics) *RoomSkillProposalTarget {
	target := &RoomSkillProposalTarget{lifecycle: lifecycle, source: source}
	if len(metrics) > 0 {
		target.metrics = metrics[0]
	}
	return target
}

func (target *RoomSkillProposalTarget) CreateRoomArtifactTarget(
	ctx context.Context,
	_ pgx.Tx,
	queries *db.Queries,
	artifact db.RoomArtifact,
) (pgtype.UUID, error) {
	if queries == nil {
		return pgtype.UUID{}, ErrRoomCandidateInvalid
	}
	return target.createQueuedProposal(ctx, queries, artifact)
}

func (target *RoomSkillProposalTarget) RoomArtifactTargetCreated(ctx context.Context, artifact db.RoomArtifact) {
	_, _ = target.ProcessRoomArtifactTarget(ctx, artifact)
}

func (target *RoomSkillProposalTarget) ProcessRoomArtifactTarget(ctx context.Context, artifact db.RoomArtifact) (Generation, error) {
	if target == nil || target.lifecycle == nil || target.source == nil || !artifact.TargetID.Valid {
		return Generation{}, ErrRoomCandidateInvalid
	}
	request, _, err := target.requestForArtifact(ctx, artifact, true)
	if err != nil {
		return Generation{}, err
	}
	started := time.Now()
	result, err := target.lifecycle.CreateProposalFromRoomRecommendation(ctx, request)
	target.recordGenerationMetrics(result, err, time.Since(started))
	if err != nil {
		return result, err
	}
	if result.Proposal.ID != artifact.TargetID {
		return result, ErrPersistenceConflict
	}
	return result, nil
}

func (target *RoomSkillProposalTarget) recordGenerationMetrics(result Generation, err error, duration time.Duration) {
	if target == nil || target.metrics == nil || result.Replayed {
		return
	}
	target.metrics.RecordLatency(duration)
	if errors.Is(err, ErrEvaluationFailed) {
		target.metrics.RecordValidationFailure()
	}
	if result.Validation.CostUsdTicks.Valid {
		target.metrics.RecordCost(result.Validation.CostUsdTicks.Int64)
	}
	if result.Replay.CostUsdTicks.Valid {
		target.metrics.RecordCost(result.Replay.CostUsdTicks.Int64)
	}
	if validUUID(result.Candidate.Revision.ID) {
		target.metrics.RecordRevision()
	}
	if err != nil || ProposalState(result.Proposal.State) != ProposalStateReady {
		return
	}
	target.metrics.RecordProposalGenerated()
}

func (target *RoomSkillProposalTarget) createQueuedProposal(ctx context.Context, queries roomProposalQueries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	if target == nil || target.lifecycle == nil || target.lifecycle.skills == nil || target.source == nil || queries == nil {
		return pgtype.UUID{}, ErrRoomCandidateInvalid
	}
	request, accepted, err := target.requestForArtifact(ctx, artifact, false)
	if err != nil {
		return pgtype.UUID{}, err
	}
	lockedSkill, err := queries.LockWorkspaceSkillForEvolution(ctx, db.LockWorkspaceSkillForEvolutionParams{
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID,
	})
	if err != nil {
		return pgtype.UUID{}, err
	}
	lockedFiles, err := queries.LockWorkspaceSkillFilesForEvolution(ctx, db.LockWorkspaceSkillFilesForEvolutionParams{
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID,
	})
	if err != nil {
		return pgtype.UUID{}, err
	}
	live, err := buildWorkspaceSkillSnapshot(lockedSkill, lockedFiles)
	if err != nil {
		return pgtype.UUID{}, err
	}
	liveHash := Digest(live.Manifest.Hash)
	if live.Skill.WorkspaceID != request.WorkspaceID || live.Skill.ID != request.SkillID ||
		liveHash != accepted.ExpectedBaseHash || live.Ownership.Class != OwnershipWorkspace ||
		!live.Ownership.DirectEvolution {
		return pgtype.UUID{}, &StaleBaseError{Expected: accepted.ExpectedBaseHash, Current: liveHash}
	}
	loop, err := queries.GetSkillEvolutionLoop(ctx, db.GetSkillEvolutionLoopParams{WorkspaceID: request.WorkspaceID, SkillID: request.SkillID})
	if err != nil {
		return pgtype.UUID{}, err
	}
	if loop.WorkspaceID != request.WorkspaceID || loop.SkillID != request.SkillID || !loop.IsEnabled || LoopMode(loop.Mode) != LoopModePropose {
		return pgtype.UUID{}, ErrEvolutionObserveOnly
	}
	if loop.NextEligibleAt.Valid && time.Now().UTC().Before(loop.NextEligibleAt.Time) {
		return pgtype.UUID{}, ErrEvolutionCooldown
	}
	if err := validateAcceptedImprovement(request, accepted, int(loop.MinimumSignals), int(loop.MaxEvidenceRefs)); err != nil {
		return pgtype.UUID{}, err
	}
	base, err := queries.GetSkillEvolutionRevisionByHash(ctx, db.GetSkillEvolutionRevisionByHashParams{
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, BundleHash: string(accepted.ExpectedBaseHash),
	})
	if err != nil {
		return pgtype.UUID{}, err
	}
	if base.WorkspaceID != request.WorkspaceID || base.SkillID != request.SkillID || base.BundleHash != string(accepted.ExpectedBaseHash) ||
		base.OwnershipClass != string(OwnershipWorkspace) {
		return pgtype.UUID{}, ErrRoomCandidateInvalid
	}
	generationKey := lifecycleKey("room-recommendation", request.IdempotencyKey)
	existing, err := queries.GetSkillEvolutionProposalByGenerationKey(ctx, db.GetSkillEvolutionProposalByGenerationKeyParams{
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, GenerationIdempotencyKey: generationKey,
	})
	if err == nil {
		if !sameQueuedRoomProposal(existing, loop, base, accepted, generationKey) {
			return pgtype.UUID{}, ErrPersistenceConflict
		}
		return existing.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, err
	}
	proposal, err := queries.CreateSkillEvolutionProposal(ctx, db.CreateSkillEvolutionProposalParams{
		GenerationIdempotencyKey: generationKey, RequestedByID: accepted.AcceptedByID,
		BaseRevisionID: base.ID, BaseHash: string(accepted.ExpectedBaseHash),
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, LoopID: loop.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		proposal, err = queries.GetSkillEvolutionProposalByGenerationKey(ctx, db.GetSkillEvolutionProposalByGenerationKeyParams{
			WorkspaceID: request.WorkspaceID, SkillID: request.SkillID, GenerationIdempotencyKey: generationKey,
		})
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if !sameQueuedRoomProposal(proposal, loop, base, accepted, generationKey) {
		return pgtype.UUID{}, ErrPersistenceConflict
	}
	return proposal.ID, nil
}

func (target *RoomSkillProposalTarget) requestForArtifact(
	ctx context.Context,
	artifact db.RoomArtifact,
	requireApproval bool,
) (RoomRecommendationRequest, AcceptedImprovementRecommendation, error) {
	if target == nil || target.source == nil || artifact.Kind != string(room.RecommendationTargetExecutableProcedure) ||
		!validUUID(artifact.ID) || !validUUID(artifact.WorkspaceID) || !validUUID(artifact.RoomID) ||
		!validUUID(artifact.MemoryRevisionID) || !validUUID(artifact.CreatedByUserID) ||
		!artifact.RecommendationKey.Valid || !boundedToken(artifact.RecommendationKey.String, 160) ||
		!boundedToken(artifact.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, ErrRoomCandidateInvalid
	}
	_, artifactCandidate, artifactBaseHash, err := decodeRoomCandidate(artifact.Body)
	if err != nil {
		return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, err
	}
	skillID, err := parseUUIDText(artifactCandidate.Bundle.ID)
	if err != nil {
		return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, ErrRoomCandidateInvalid
	}
	recommendationID := improvementRecommendationID(room.AcceptedOutcomeRef{
		RoomID: artifact.RoomID, MemoryRevisionID: artifact.MemoryRevisionID,
		RecommendationKey: artifact.RecommendationKey.String,
	})
	request := RoomRecommendationRequest{
		WorkspaceID: artifact.WorkspaceID, SkillID: skillID, RecommendationID: recommendationID,
		IdempotencyKey: artifact.IdempotencyKey,
	}
	var accepted AcceptedImprovementRecommendation
	if requireApproval {
		accepted, err = target.source.LoadAcceptedImprovement(ctx, request)
	} else {
		accepted, _, err = target.source.loadAcceptedImprovement(ctx, request, artifact.CreatedByUserID)
	}
	if err != nil {
		return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, err
	}
	if accepted.ExpectedBaseHash != artifactBaseHash || !reflect.DeepEqual(accepted.Candidate, artifactCandidate) {
		return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, ErrRoomCandidateSourceDrift
	}
	if requireApproval {
		ref := room.AcceptedOutcomeRef{WorkspaceID: artifact.WorkspaceID, RoomID: artifact.RoomID,
			MemoryRevisionID: artifact.MemoryRevisionID, RecommendationKey: artifact.RecommendationKey.String}
		approved, review, approvalErr := target.source.approvedArtifact(ctx, ref)
		if approvalErr != nil {
			return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, approvalErr
		}
		if approved.ID != artifact.ID || approved.TargetID != artifact.TargetID || approved.SourceDigest != artifact.SourceDigest ||
			review.ReviewedByUserID != artifact.CreatedByUserID || accepted.AcceptedByID != review.ReviewedByUserID {
			return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, ErrRoomCandidateSourceDrift
		}
	}
	if artifact.SourceDigest != roomArtifactSourceDigest(artifact) {
		return RoomRecommendationRequest{}, AcceptedImprovementRecommendation{}, ErrRoomCandidateSourceDrift
	}
	return request, accepted, nil
}

func sameQueuedRoomProposal(
	proposal db.SkillEvolutionProposal,
	loop db.SkillEvolutionLoop,
	base db.SkillEvolutionRevision,
	accepted AcceptedImprovementRecommendation,
	generationKey string,
) bool {
	return validUUID(proposal.ID) && proposal.WorkspaceID == accepted.WorkspaceID && proposal.SkillID == accepted.SkillID &&
		proposal.LoopID == loop.ID && proposal.BaseRevisionID == base.ID && proposal.BaseHash == string(accepted.ExpectedBaseHash) &&
		proposal.GenerationIdempotencyKey == generationKey && proposal.RequestedByID == accepted.AcceptedByID &&
		ProposalState(proposal.State) == ProposalStateQueued
}

func roomArtifactSourceDigest(artifact db.RoomArtifact) string {
	rationale := ""
	if artifact.Rationale.Valid {
		rationale = artifact.Rationale.String
	}
	value := strings.Join([]string{
		artifact.Kind, artifact.Title, rationale, artifact.Body, "", "",
		uuidText(artifact.MemoryRevisionID), artifact.RecommendationKey.String,
	}, "\x00")
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
