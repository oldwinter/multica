package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

const forkAuditConfigKey = "skill_evolution_fork"

var (
	ErrSkillForkInvalidInput      = fmt.Errorf("invalid Skill fork input: %w", ErrWorkspaceSkillInvalidInput)
	ErrSkillForkSourceUnavailable = fmt.Errorf("Skill fork source is unavailable: %w", ErrWorkspaceSkillNotFound)
	ErrSkillForkIdempotency       = fmt.Errorf("Skill fork idempotency key conflicts with an existing Skill: %w", ErrSkillNameConflict)
)

type workspaceSkillForker struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

type skillForkAudit struct {
	Version         int             `json:"version"`
	SourceSkillID   string          `json:"source_skill_id"`
	SourceOwnership OwnershipClass  `json:"source_ownership"`
	SourceReason    OwnershipReason `json:"source_reason"`
	SourceHash      Digest          `json:"source_hash"`
	Digest          Digest          `json:"digest"`
	ActorID         string          `json:"actor_id"`
	IdempotencyKey  string          `json:"idempotency_key"`
}

type skillForkAuditPayload struct {
	Version         int             `json:"version"`
	SourceSkillID   string          `json:"source_skill_id"`
	SourceOwnership OwnershipClass  `json:"source_ownership"`
	SourceReason    OwnershipReason `json:"source_reason"`
	SourceHash      Digest          `json:"source_hash"`
	ActorID         string          `json:"actor_id"`
	IdempotencyKey  string          `json:"idempotency_key"`
}

// NewWorkspaceSkillForker adapts the existing Skill and Skill-file writes to
// the SkillEvolution fork seam. CreateSkill deliberately leaves both plugin
// ownership and Agent assignments empty for the new workspace-owned copy.
func NewWorkspaceSkillForker(queries *db.Queries, pool *pgxpool.Pool) SkillForker {
	return &workspaceSkillForker{queries: queries, pool: pool}
}

func (f *workspaceSkillForker) ForkSkill(ctx context.Context, input ForkSkillInput) (ForkSkillResult, error) {
	input.NewName = strings.TrimSpace(input.NewName)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if f == nil || f.queries == nil || f.pool == nil || !validUUID(input.WorkspaceID) ||
		!validUUID(input.SourceSkillID) || !validUUID(input.ActorID) || !input.ExpectedSourceHash.Valid() ||
		!boundedToken(input.NewName, 255) || !boundedToken(input.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return ForkSkillResult{}, ErrSkillForkInvalidInput
	}

	tx, err := f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return ForkSkillResult{}, fmt.Errorf("begin Skill fork: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := f.queries.WithTx(tx)

	sourceSkill, err := queries.LockWorkspaceSkillForEvolution(ctx, db.LockWorkspaceSkillForEvolutionParams{
		WorkspaceID: input.WorkspaceID,
		SkillID:     input.SourceSkillID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ForkSkillResult{}, ErrSkillForkSourceUnavailable
	}
	if err != nil {
		return ForkSkillResult{}, forkDBError("lock fork source", err)
	}
	ownership := classifyWorkspaceSkillOwnership(sourceSkill)
	if !forkableOwnership(ownership) {
		return ForkSkillResult{}, &ForkRequiredError{Ownership: ownership}
	}
	sourceFiles, err := queries.LockWorkspaceSkillFilesForEvolution(ctx, db.LockWorkspaceSkillFilesForEvolutionParams{
		WorkspaceID: input.WorkspaceID,
		SkillID:     input.SourceSkillID,
	})
	if err != nil {
		return ForkSkillResult{}, forkDBError("lock fork source files", err)
	}
	source, err := buildWorkspaceSkillSnapshot(sourceSkill, sourceFiles)
	if err != nil {
		return ForkSkillResult{}, fmt.Errorf("validate fork source bundle: %w", err)
	}
	currentHash := Digest(source.Manifest.Hash)
	if currentHash != input.ExpectedSourceHash {
		return ForkSkillResult{}, &StaleBaseError{Expected: input.ExpectedSourceHash, Current: currentHash}
	}

	audit := newSkillForkAudit(input, ownership)
	config, err := forkedSkillConfig(sourceSkill.Config, audit)
	if err != nil {
		return ForkSkillResult{}, err
	}
	if existing, err := queries.GetSkillByWorkspaceAndName(ctx, db.GetSkillByWorkspaceAndNameParams{
		WorkspaceID: input.WorkspaceID,
		Name:        input.NewName,
	}); err == nil {
		return replaySkillFork(existing, audit)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ForkSkillResult{}, forkDBError("check Skill fork name", err)
	}

	created, err := queries.CreateSkill(ctx, db.CreateSkillParams{
		WorkspaceID: input.WorkspaceID,
		Name:        input.NewName,
		Description: source.Skill.Description,
		Content:     source.Skill.Content,
		Config:      config,
		CreatedBy:   input.ActorID,
	})
	if err != nil {
		if isForkUniqueViolation(err) {
			// A unique violation aborts this transaction, so the caller must
			// retry from a fresh snapshot. Sequential replays are resolved by
			// the lookup above without issuing another write.
			return ForkSkillResult{}, ErrSkillForkIdempotency
		}
		return ForkSkillResult{}, forkDBError("create Skill fork", err)
	}
	for _, file := range source.Files {
		if _, err := queries.CreateWorkspaceSkillFileForEvolution(ctx, db.CreateWorkspaceSkillFileForEvolutionParams{
			WorkspaceID: input.WorkspaceID,
			SkillID:     created.ID,
			Path:        file.Path,
			Content:     file.Content,
		}); err != nil {
			return ForkSkillResult{}, forkDBError("copy Skill fork file", err)
		}
	}

	forked, err := loadWorkspaceSkill(ctx, queries, input.WorkspaceID, created.ID)
	if err != nil {
		return ForkSkillResult{}, fmt.Errorf("reload Skill fork: %w", err)
	}
	expectedBundle := source.Bundle
	expectedBundle.ID = uuid.UUID(created.ID.Bytes).String()
	expectedBundle.Name = input.NewName
	expectedManifest, err := skillbundle.BuildValidatedManifest(expectedBundle)
	if err != nil {
		return ForkSkillResult{}, fmt.Errorf("validate expected Skill fork: %w", err)
	}
	if forked.Ownership.Class != OwnershipWorkspace || !forked.Ownership.DirectEvolution ||
		forked.Skill.PluginInstallationID.Valid || forked.Manifest.Hash != expectedManifest.Hash ||
		forked.Skill.CreatedBy != input.ActorID {
		return ForkSkillResult{}, ErrPostWriteHashMismatch
	}
	if err := tx.Commit(ctx); err != nil {
		return ForkSkillResult{}, forkDBError("commit Skill fork", err)
	}
	return ForkSkillResult{SkillID: created.ID, SourceAuditDigest: audit.Digest}, nil
}

func forkableOwnership(ownership Ownership) bool {
	return ownership.ForkRequired && (ownership.Class == OwnershipPlugin ||
		ownership.Class == OwnershipExternal || ownership.Class == OwnershipRuntimeLocal)
}

func newSkillForkAudit(input ForkSkillInput, ownership Ownership) skillForkAudit {
	payload := skillForkAuditPayload{
		Version: 1, SourceSkillID: uuid.UUID(input.SourceSkillID.Bytes).String(),
		SourceOwnership: ownership.Class, SourceReason: ownership.Reason,
		SourceHash: input.ExpectedSourceHash, ActorID: uuid.UUID(input.ActorID.Bytes).String(),
		IdempotencyKey: input.IdempotencyKey,
	}
	return skillForkAudit{
		Version: payload.Version, SourceSkillID: payload.SourceSkillID,
		SourceOwnership: payload.SourceOwnership, SourceReason: payload.SourceReason,
		SourceHash: payload.SourceHash, Digest: digestSafeValue("skill-fork-audit-v1", payload),
		ActorID: payload.ActorID, IdempotencyKey: payload.IdempotencyKey,
	}
}

func forkedSkillConfig(raw []byte, audit skillForkAudit) ([]byte, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil || config == nil {
		return nil, ErrSkillForkSourceUnavailable
	}
	delete(config, "origin")
	auditJSON, err := json.Marshal(audit)
	if err != nil {
		return nil, fmt.Errorf("encode Skill fork audit: %w", err)
	}
	config[forkAuditConfigKey] = auditJSON
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode Skill fork config: %w", err)
	}
	return encoded, nil
}

func replaySkillFork(existing db.Skill, want skillForkAudit) (ForkSkillResult, error) {
	if existing.PluginInstallationID.Valid || !existing.ID.Valid || classifyWorkspaceSkillOwnership(existing).Class != OwnershipWorkspace {
		return ForkSkillResult{}, ErrSkillForkIdempotency
	}
	var config map[string]json.RawMessage
	if json.Unmarshal(existing.Config, &config) != nil || config == nil {
		return ForkSkillResult{}, ErrSkillForkIdempotency
	}
	var got skillForkAudit
	if json.Unmarshal(config[forkAuditConfigKey], &got) != nil || got != want || !got.Digest.Valid() {
		return ForkSkillResult{}, ErrSkillForkIdempotency
	}
	return ForkSkillResult{SkillID: existing.ID, SourceAuditDigest: got.Digest}, nil
}

func isForkUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func forkDBError(action string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
		return ErrConcurrentBundleDrift
	}
	return fmt.Errorf("%s: %w", action, err)
}

var _ SkillForker = (*workspaceSkillForker)(nil)
