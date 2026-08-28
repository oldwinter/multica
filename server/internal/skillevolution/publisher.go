package skillevolution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

var (
	ErrPublisherTransactionsRequired = errors.New("Skill publisher requires transactions")
	ErrStaleBase                     = errors.New("workspace Skill base is stale")
	ErrSkillNameConflict             = errors.New("workspace Skill name conflicts with another Skill")
	ErrPostWriteHashMismatch         = errors.New("published Skill bundle hash does not match reviewed bundle")
	ErrConcurrentBundleDrift         = errors.New("workspace Skill changed concurrently during publication")
	ErrPublicationUnknown            = errors.New("Skill publication outcome is unknown")
)

type StaleBaseError struct {
	Expected Digest
	Current  Digest
}

func (e *StaleBaseError) Error() string { return ErrStaleBase.Error() }
func (e *StaleBaseError) Unwrap() error { return ErrStaleBase }

type PublicationUnknownError struct {
	ExpectedPostHash Digest
	ObservedPostHash Digest
	Cause            error
}

func (e *PublicationUnknownError) Error() string { return ErrPublicationUnknown.Error() }

func (e *PublicationUnknownError) Unwrap() []error {
	if e.Cause == nil {
		return []error{ErrPublicationUnknown}
	}
	return []error{ErrPublicationUnknown, e.Cause}
}

type PublishSkillRequest struct {
	WorkspaceID      pgtype.UUID
	SkillID          pgtype.UUID
	ExpectedBaseHash Digest
	Bundle           skillbundle.Skill
}

type PublishSkillResult struct {
	Snapshot   WorkspaceSkillSnapshot
	PreHash    Digest
	PostHash   Digest
	Idempotent bool
}

// SkillPublisher is the only mutation seam used for both publication and
// rollback. Callers own authorization and append-only lifecycle records.
type SkillPublisher interface {
	Publish(context.Context, PublishSkillRequest) (PublishSkillResult, error)
}

type PublisherTxStarter interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

type WorkspaceSkillPublisher struct {
	queries   *db.Queries
	txStarter PublisherTxStarter
}

func NewWorkspaceSkillPublisher(queries *db.Queries, txStarter PublisherTxStarter) *WorkspaceSkillPublisher {
	return &WorkspaceSkillPublisher{queries: queries, txStarter: txStarter}
}

func (p *WorkspaceSkillPublisher) Publish(ctx context.Context, input PublishSkillRequest) (PublishSkillResult, error) {
	candidateManifest, candidateFiles, err := validatePublishRequest(input)
	if err != nil {
		return PublishSkillResult{}, err
	}
	if p == nil || p.queries == nil || p.txStarter == nil {
		return PublishSkillResult{}, ErrPublisherTransactionsRequired
	}

	tx, err := p.txStarter.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PublishSkillResult{}, fmt.Errorf("begin Skill publication: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := p.queries.WithTx(tx)

	lockedSkill, err := queries.LockWorkspaceSkillForEvolution(ctx, db.LockWorkspaceSkillForEvolutionParams{
		WorkspaceID: input.WorkspaceID,
		SkillID:     input.SkillID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublishSkillResult{}, ErrWorkspaceSkillNotFound
		}
		return PublishSkillResult{}, publisherDBError("lock workspace Skill", err)
	}
	ownership := classifyWorkspaceSkillOwnership(lockedSkill)
	if !ownership.DirectEvolution || ownership.Class != OwnershipWorkspace {
		return PublishSkillResult{}, &ForkRequiredError{Ownership: ownership}
	}
	lockedFiles, err := queries.LockWorkspaceSkillFilesForEvolution(ctx, db.LockWorkspaceSkillFilesForEvolutionParams{
		WorkspaceID: input.WorkspaceID,
		SkillID:     input.SkillID,
	})
	if err != nil {
		return PublishSkillResult{}, publisherDBError("lock workspace Skill files", err)
	}
	live, err := buildWorkspaceSkillSnapshot(lockedSkill, lockedFiles)
	if err != nil {
		return PublishSkillResult{}, err
	}

	currentHash := Digest(live.Manifest.Hash)
	candidateHash := Digest(candidateManifest.Hash)
	if currentHash == candidateHash {
		return PublishSkillResult{
			Snapshot: live, PreHash: currentHash, PostHash: currentHash, Idempotent: true,
		}, nil
	}
	if currentHash != input.ExpectedBaseHash {
		return PublishSkillResult{}, &StaleBaseError{Expected: input.ExpectedBaseHash, Current: currentHash}
	}

	before := live.Skill
	if _, err := queries.UpdateWorkspaceSkillBundleForEvolution(ctx, db.UpdateWorkspaceSkillBundleForEvolutionParams{
		WorkspaceID: input.WorkspaceID,
		SkillID:     input.SkillID,
		Name:        input.Bundle.Name,
		Description: input.Bundle.Description,
		Content:     input.Bundle.Content,
	}); err != nil {
		return PublishSkillResult{}, publishSkillUpdateError(err)
	}
	if err := queries.DeleteWorkspaceSkillFilesForEvolution(ctx, db.DeleteWorkspaceSkillFilesForEvolutionParams{
		WorkspaceID: input.WorkspaceID,
		SkillID:     input.SkillID,
	}); err != nil {
		return PublishSkillResult{}, publisherDBError("replace workspace Skill files", err)
	}
	for _, file := range candidateFiles {
		if _, err := queries.CreateWorkspaceSkillFileForEvolution(ctx, db.CreateWorkspaceSkillFileForEvolutionParams{
			WorkspaceID: input.WorkspaceID,
			SkillID:     input.SkillID,
			Path:        file.Path,
			Content:     file.Content,
		}); err != nil {
			if isUniqueViolation(err) {
				return PublishSkillResult{}, ErrConcurrentBundleDrift
			}
			return PublishSkillResult{}, publisherDBError("create workspace Skill file", err)
		}
	}

	persisted, err := loadWorkspaceSkill(ctx, queries, input.WorkspaceID, input.SkillID)
	if err != nil {
		if errors.Is(err, ErrPostWriteHashMismatch) {
			return PublishSkillResult{}, err
		}
		return PublishSkillResult{}, publisherDBError("reload published workspace Skill", err)
	}
	if Digest(persisted.Manifest.Hash) != candidateHash || !publisherMetadataPreserved(before, persisted.Skill) {
		return PublishSkillResult{}, ErrPostWriteHashMismatch
	}
	if err := tx.Commit(ctx); err != nil {
		if commitDefinitelyRolledBack(err) {
			if isSerializationFailure(err) {
				return PublishSkillResult{}, ErrConcurrentBundleDrift
			}
			return PublishSkillResult{}, fmt.Errorf("commit Skill publication: %w", err)
		}
		return PublishSkillResult{}, &PublicationUnknownError{ExpectedPostHash: candidateHash, Cause: err}
	}

	// A second reload detects a support-file write that did not participate in
	// the parent-row lock but completed before the publisher returned.
	committed, err := NewWorkspaceSkillRepository(p.queries).Load(ctx, input.WorkspaceID, input.SkillID)
	if err != nil {
		return PublishSkillResult{}, &PublicationUnknownError{ExpectedPostHash: candidateHash, Cause: err}
	}
	observedHash := Digest(committed.Manifest.Hash)
	if observedHash != candidateHash {
		return PublishSkillResult{}, &PublicationUnknownError{
			ExpectedPostHash: candidateHash,
			ObservedPostHash: observedHash,
			Cause:            ErrConcurrentBundleDrift,
		}
	}
	return PublishSkillResult{
		Snapshot: committed, PreHash: currentHash, PostHash: observedHash,
	}, nil
}

func validatePublishRequest(input PublishSkillRequest) (skillbundle.Manifest, []skillbundle.File, error) {
	if !validUUID(input.WorkspaceID) || !validUUID(input.SkillID) || !input.ExpectedBaseHash.Valid() ||
		input.Bundle.ID != uuid.UUID(input.SkillID.Bytes).String() || input.Bundle.Source != skillbundle.SourceWorkspace ||
		input.Bundle.Name == "" {
		return skillbundle.Manifest{}, nil, ErrWorkspaceSkillInvalidInput
	}
	manifest, err := skillbundle.BuildValidatedManifest(input.Bundle)
	if err != nil {
		return skillbundle.Manifest{}, nil, err
	}
	files := append([]skillbundle.File(nil), input.Bundle.Files...)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Path != files[j].Path {
			return files[i].Path < files[j].Path
		}
		return files[i].Content < files[j].Content
	})
	return manifest, files, nil
}

func loadWorkspaceSkill(ctx context.Context, queries *db.Queries, workspaceID, skillID pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	skill, err := queries.GetWorkspaceSkillForEvolution(ctx, db.GetWorkspaceSkillForEvolutionParams{
		WorkspaceID: workspaceID,
		SkillID:     skillID,
	})
	if err != nil {
		return WorkspaceSkillSnapshot{}, workspaceSkillError(err)
	}
	files, err := queries.ListWorkspaceSkillFilesForEvolution(ctx, db.ListWorkspaceSkillFilesForEvolutionParams{
		WorkspaceID: workspaceID,
		SkillID:     skillID,
	})
	if err != nil {
		return WorkspaceSkillSnapshot{}, fmt.Errorf("reload workspace Skill files: %w", err)
	}
	snapshot, err := buildWorkspaceSkillSnapshot(skill, files)
	if err != nil {
		return WorkspaceSkillSnapshot{}, fmt.Errorf("%w: %v", ErrPostWriteHashMismatch, err)
	}
	return snapshot, nil
}

func publisherMetadataPreserved(before, after db.Skill) bool {
	return before.ID == after.ID && before.WorkspaceID == after.WorkspaceID &&
		before.CreatedBy == after.CreatedBy && before.CreatedAt == after.CreatedAt &&
		before.UpdatedAt == after.UpdatedAt && before.PluginInstallationID == after.PluginInstallationID &&
		bytes.Equal(before.Config, after.Config)
}

func publishSkillUpdateError(err error) error {
	if isUniqueViolation(err) {
		return ErrSkillNameConflict
	}
	return publisherDBError("update workspace Skill bundle", err)
}

func publisherDBError(action string, err error) error {
	if isSerializationFailure(err) {
		return ErrConcurrentBundleDrift
	}
	return fmt.Errorf("%s: %w", action, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func commitDefinitelyRolledBack(err error) bool {
	if errors.Is(err, pgx.ErrTxCommitRollback) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr)
}

func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01")
}
