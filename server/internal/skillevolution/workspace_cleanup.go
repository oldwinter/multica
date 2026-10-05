package skillevolution

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/handler"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var _ handler.WorkspaceCleanupContributor = (*WorkspaceCleanup)(nil)

type WorkspaceCleanup struct{}

func NewWorkspaceCleanup() *WorkspaceCleanup {
	return &WorkspaceCleanup{}
}

func (*WorkspaceCleanup) DeleteWorkspace(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID) error {
	if queries == nil || !validUUID(workspaceID) {
		return ErrPersistenceInvalidInput
	}
	return queries.DeleteWorkspaceSkillEvolutionData(ctx, workspaceID)
}
