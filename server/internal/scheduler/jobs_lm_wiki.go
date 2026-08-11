package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	JobNameLMWikiDailyReconcile = "lm_wiki_daily_reconcile"
	ScopeKindWorkspace          = "workspace"
	lmWikiWorkspacePageSize     = 1000
)

type LMWikiRefresher interface {
	Refresh(
		ctx context.Context,
		workspaceID pgtype.UUID,
		trigger string,
		requestedBy pgtype.UUID,
		planKey string,
	) (service.LMWikiRefreshResult, error)
}

func LMWikiDailyReconcileJob(queries *db.Queries, refresher LMWikiRefresher) JobSpec {
	return JobSpec{
		Name:              JobNameLMWikiDailyReconcile,
		Cadence:           24 * time.Hour,
		CatchUpMode:       CatchUpLatestOnly,
		CatchUpWindow:     48 * time.Hour,
		RunTimeout:        2 * time.Minute,
		StaleTimeout:      5 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		AllowStaleReentry: true,
		MaxAttempts:       3,
		RetryBackoff: []time.Duration{
			time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		},
		Scopes:  lmWikiWorkspaceScopes(queries),
		Handler: lmWikiDailyHandler(refresher),
	}
}

func lmWikiWorkspaceScopes(queries *db.Queries) ScopeProvider {
	return func(ctx context.Context, _ time.Time) ([]Scope, error) {
		cursor := pgtype.UUID{Valid: true}
		var scopes []Scope
		for {
			workspaceIDs, err := queries.ListLMWikiReconcileWorkspaces(ctx, db.ListLMWikiReconcileWorkspacesParams{
				WorkspaceID: cursor,
				ResultLimit: lmWikiWorkspacePageSize,
			})
			if err != nil {
				return nil, fmt.Errorf("list lm wiki reconcile workspaces: %w", err)
			}
			for _, workspaceID := range workspaceIDs {
				scopes = append(scopes, Scope{Kind: ScopeKindWorkspace, ID: util.UUIDToString(workspaceID)})
			}
			if len(workspaceIDs) < lmWikiWorkspacePageSize {
				return scopes, nil
			}
			cursor = workspaceIDs[len(workspaceIDs)-1]
		}
	}
}

func lmWikiDailyHandler(refresher LMWikiRefresher) Handler {
	return func(ctx context.Context, in HandlerInput) (HandlerResult, error) {
		if in.Scope.Kind != ScopeKindWorkspace {
			return HandlerResult{}, fmt.Errorf("lm wiki reconcile scope kind %q: expected %q", in.Scope.Kind, ScopeKindWorkspace)
		}
		workspaceID, err := util.ParseUUID(in.Scope.ID)
		if err != nil {
			return HandlerResult{}, fmt.Errorf("parse lm wiki reconcile workspace %q: %w", in.Scope.ID, err)
		}
		result, err := refresher.Refresh(
			ctx,
			workspaceID,
			"scheduled",
			pgtype.UUID{},
			in.PlanTime.UTC().Format(time.RFC3339),
		)
		if errors.Is(err, service.ErrLMWikiNotFound) {
			return HandlerResult{Result: map[string]any{"skipped_reason": "workspace_deleted"}}, nil
		}
		if err != nil {
			return HandlerResult{}, fmt.Errorf("refresh lm wiki workspace %s: %w", in.Scope.ID, err)
		}
		rowsAffected := int64(0)
		if result.Created {
			rowsAffected = 1
		}
		return HandlerResult{
			RowsAffected: rowsAffected,
			Result:       map[string]any{"created": result.Created},
		}, nil
	}
}
