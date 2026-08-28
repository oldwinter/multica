package skillevolution

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These point reads keep generated query details out of lifecycle orchestration
// without expanding the public Store surface used by other callers.
func (s *Store) getRevisionByHash(ctx context.Context, workspaceID, skillID pgtype.UUID, hash Digest) (RevisionSnapshot, error) {
	if s == nil || s.queries == nil || !validUUID(workspaceID) || !validUUID(skillID) || !hash.Valid() {
		return RevisionSnapshot{}, ErrPersistenceInvalidInput
	}
	revision, err := s.queries.GetSkillEvolutionRevisionByHash(ctx, db.GetSkillEvolutionRevisionByHashParams{
		WorkspaceID: workspaceID, SkillID: skillID, BundleHash: string(hash),
	})
	if err != nil {
		return RevisionSnapshot{}, persistenceError(err)
	}
	return s.GetRevisionSnapshot(ctx, workspaceID, revision.ID)
}

func (s *Store) getProposalByKey(ctx context.Context, workspaceID, skillID pgtype.UUID, key string) (db.SkillEvolutionProposal, error) {
	if s == nil || s.queries == nil || !validUUID(workspaceID) || !validUUID(skillID) ||
		!boundedToken(key, MaxIdempotencyKeyBytes) {
		return db.SkillEvolutionProposal{}, ErrPersistenceInvalidInput
	}
	proposal, err := s.queries.GetSkillEvolutionProposalByGenerationKey(ctx, db.GetSkillEvolutionProposalByGenerationKeyParams{
		WorkspaceID: workspaceID, SkillID: skillID, GenerationIdempotencyKey: key,
	})
	return proposal, persistenceError(err)
}

func (s *Store) getRelease(ctx context.Context, workspaceID, releaseID pgtype.UUID) (db.SkillEvolutionRelease, error) {
	if s == nil || s.queries == nil || !validUUID(workspaceID) || !validUUID(releaseID) {
		return db.SkillEvolutionRelease{}, ErrPersistenceInvalidInput
	}
	release, err := s.queries.GetSkillEvolutionRelease(ctx, db.GetSkillEvolutionReleaseParams{
		WorkspaceID: workspaceID, ID: releaseID,
	})
	return release, persistenceError(err)
}

func (s *Store) getReleaseByKey(ctx context.Context, workspaceID pgtype.UUID, key string) (db.SkillEvolutionRelease, error) {
	if s == nil || s.queries == nil || !validUUID(workspaceID) || !boundedToken(key, MaxIdempotencyKeyBytes) {
		return db.SkillEvolutionRelease{}, ErrPersistenceInvalidInput
	}
	release, err := s.queries.GetSkillEvolutionReleaseByIdempotencyKey(ctx, db.GetSkillEvolutionReleaseByIdempotencyKeyParams{
		WorkspaceID: workspaceID, IdempotencyKey: key,
	})
	return release, persistenceError(err)
}
