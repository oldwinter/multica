package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/issueposition"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type RoomArtifactTargets struct {
	issues *IssueService
}

func NewRoomArtifactTargets(issues *IssueService) *RoomArtifactTargets {
	return &RoomArtifactTargets{issues: issues}
}

func (targets *RoomArtifactTargets) CreateRoomArtifactTarget(ctx context.Context, tx pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	switch artifact.Kind {
	case "issue":
		if targets == nil || targets.issues == nil {
			return pgtype.UUID{}, fmt.Errorf("issue service is unavailable")
		}
		issue, err := targets.issues.createRoomIssueTarget(ctx, tx, artifact)
		if err != nil {
			return pgtype.UUID{}, err
		}
		return issue.ID, nil
	case "wiki":
		wikiPath := path.Join("rooms", util.UUIDToString(artifact.RoomID), util.UUIDToString(artifact.ID)+".md")
		page, err := queries.CreateWikiPage(ctx, db.CreateWikiPageParams{
			WorkspaceID: artifact.WorkspaceID, Scope: "workspace", Path: wikiPath,
			Title: artifact.Title, Content: artifact.Body, CreatedBy: artifact.CreatedByUserID,
		})
		if err != nil {
			return pgtype.UUID{}, err
		}
		return page.ID, nil
	default:
		return pgtype.UUID{}, fmt.Errorf("unsupported Room artifact target kind %q", artifact.Kind)
	}
}

func (s *IssueService) createRoomIssueTarget(ctx context.Context, tx pgx.Tx, artifact db.RoomArtifact) (db.Issue, error) {
	queries := s.Queries.WithTx(tx)
	number, err := queries.IncrementIssueCounter(ctx, artifact.WorkspaceID)
	if err != nil {
		return db.Issue{}, fmt.Errorf("increment Room issue counter: %w", err)
	}
	position, err := issueposition.NextTopPosition(ctx, tx, artifact.WorkspaceID, "todo")
	if err != nil {
		return db.Issue{}, fmt.Errorf("position Room issue: %w", err)
	}
	issue, err := queries.CreateIssue(ctx, db.CreateIssueParams{
		ID:          dbid.NewV7(),
		WorkspaceID: artifact.WorkspaceID,
		Title:       artifact.Title,
		Description: pgtype.Text{String: artifact.Body, Valid: true},
		Status:      "todo",
		Priority:    "none",
		CreatorType: "member",
		CreatorID:   artifact.CreatedByUserID,
		Position:    position,
		Number:      number,
	})
	if err != nil {
		return db.Issue{}, fmt.Errorf("create Room issue: %w", err)
	}
	artifactID, err := json.Marshal(util.UUIDToString(artifact.ID))
	if err != nil {
		return db.Issue{}, fmt.Errorf("encode Room artifact metadata: %w", err)
	}
	issue, err = queries.SetIssueMetadataKey(ctx, db.SetIssueMetadataKeyParams{
		Key: "room_artifact_id", Value: artifactID, ID: issue.ID, WorkspaceID: artifact.WorkspaceID,
	})
	if err != nil {
		return db.Issue{}, fmt.Errorf("link Room artifact issue: %w", err)
	}
	return issue, nil
}

func (targets *RoomArtifactTargets) RoomArtifactTargetCreated(ctx context.Context, artifact db.RoomArtifact) {
	if artifact.Kind != "issue" || targets == nil || targets.issues == nil || !artifact.TargetID.Valid {
		return
	}
	issue, err := targets.issues.Queries.GetIssue(ctx, artifact.TargetID)
	if err != nil {
		return
	}
	actorID := util.UUIDToString(artifact.CreatedByUserID)
	targets.issues.publishIssueCreated(issue, nil, nil, "member", actorID, IssueCreateOpts{Platform: "room"})
	targets.issues.captureCreatedAnalytics(issue, "member", actorID, IssueCreateOpts{Platform: "room"})
}
