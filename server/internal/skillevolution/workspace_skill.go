package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

var (
	ErrWorkspaceSkillInvalidInput = errors.New("invalid workspace Skill input")
	ErrWorkspaceSkillNotFound     = errors.New("workspace Skill not found")
	ErrForkRequired               = errors.New("workspace Skill must be forked before publication")
)

type ForkRequiredError struct {
	Ownership Ownership
}

func (e *ForkRequiredError) Error() string { return ErrForkRequired.Error() }
func (e *ForkRequiredError) Unwrap() error { return ErrForkRequired }

type WorkspaceSkillSnapshot struct {
	Skill     db.Skill
	Files     []db.SkillFile
	Ownership Ownership
	Bundle    skillbundle.Skill
	Manifest  skillbundle.Manifest
}

// WorkspaceSkillRepository rebuilds the live canonical bundle from the
// upstream Skill tables without making them evolution-aware.
type WorkspaceSkillRepository struct {
	queries *db.Queries
}

func NewWorkspaceSkillRepository(queries *db.Queries) *WorkspaceSkillRepository {
	return &WorkspaceSkillRepository{queries: queries}
}

func (r *WorkspaceSkillRepository) Load(ctx context.Context, workspaceID, skillID pgtype.UUID) (WorkspaceSkillSnapshot, error) {
	if r == nil || r.queries == nil || !validUUID(workspaceID) || !validUUID(skillID) {
		return WorkspaceSkillSnapshot{}, ErrWorkspaceSkillInvalidInput
	}
	skill, err := r.queries.GetWorkspaceSkillForEvolution(ctx, db.GetWorkspaceSkillForEvolutionParams{
		WorkspaceID: workspaceID,
		SkillID:     skillID,
	})
	if err != nil {
		return WorkspaceSkillSnapshot{}, workspaceSkillError(err)
	}
	files, err := r.queries.ListWorkspaceSkillFilesForEvolution(ctx, db.ListWorkspaceSkillFilesForEvolutionParams{
		WorkspaceID: workspaceID,
		SkillID:     skillID,
	})
	if err != nil {
		return WorkspaceSkillSnapshot{}, fmt.Errorf("list workspace Skill files: %w", err)
	}
	return buildWorkspaceSkillSnapshot(skill, files)
}

func buildWorkspaceSkillSnapshot(skill db.Skill, files []db.SkillFile) (WorkspaceSkillSnapshot, error) {
	if !validUUID(skill.ID) || !validUUID(skill.WorkspaceID) {
		return WorkspaceSkillSnapshot{}, ErrWorkspaceSkillInvalidInput
	}
	bundle := skillbundle.Skill{
		ID:          uuid.UUID(skill.ID.Bytes).String(),
		Source:      skillbundle.SourceWorkspace,
		Name:        skill.Name,
		Description: skill.Description,
		Content:     skill.Content,
		Files:       make([]skillbundle.File, len(files)),
	}
	for i, file := range files {
		if file.SkillID != skill.ID {
			return WorkspaceSkillSnapshot{}, ErrWorkspaceSkillInvalidInput
		}
		bundle.Files[i] = skillbundle.File{Path: file.Path, Content: file.Content}
	}
	manifest, err := skillbundle.BuildValidatedManifest(bundle)
	if err != nil {
		return WorkspaceSkillSnapshot{}, fmt.Errorf("validate live workspace Skill bundle: %w", err)
	}
	return WorkspaceSkillSnapshot{
		Skill:     skill,
		Files:     append([]db.SkillFile(nil), files...),
		Ownership: classifyWorkspaceSkillOwnership(skill),
		Bundle:    bundle,
		Manifest:  manifest,
	}, nil
}

func classifyWorkspaceSkillOwnership(skill db.Skill) Ownership {
	return ClassifyOwnership(OwnershipInput{
		PluginInstallationPresent: skill.PluginInstallationID.Valid,
		Config:                    json.RawMessage(skill.Config),
	})
}

func workspaceSkillError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWorkspaceSkillNotFound
	}
	return fmt.Errorf("load workspace Skill: %w", err)
}
