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
	issues      *IssueService
	wiki        RoomWikiPageCreator
	wikiPublish RoomWikiPagePublisher
}

// RoomWikiPageCreateInput deliberately mirrors WikiPageCreateInput without
// coupling the Rooms leaf to the Wiki feature's generated schema. The handler
// composes this through WikiKnowledge.CreatePage once that feature is enabled.
type RoomWikiPageCreateInput struct {
	WorkspaceID pgtype.UUID
	ProjectID   pgtype.UUID
	OwnerUserID pgtype.UUID
	Scope       string
	Path        string
	Title       string
	Content     string
	ActorType   string
	ActorID     pgtype.UUID
	SourceKind  string
	SourceRefID pgtype.UUID
}

type RoomWikiPageCreator func(context.Context, *db.Queries, RoomWikiPageCreateInput) (db.WikiPage, error)
type RoomWikiPagePublisher func(context.Context, pgtype.UUID, string, pgtype.UUID) error

func NewRoomArtifactTargets(issues *IssueService) *RoomArtifactTargets {
	return &RoomArtifactTargets{issues: issues}
}

func (targets *RoomArtifactTargets) SetWikiPageCreator(creator RoomWikiPageCreator) {
	if targets != nil {
		targets.wiki = creator
	}
}

func (targets *RoomArtifactTargets) SetWikiPagePublisher(publisher RoomWikiPagePublisher) {
	if targets != nil {
		targets.wikiPublish = publisher
	}
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
		if targets == nil || targets.wiki == nil {
			return pgtype.UUID{}, fmt.Errorf("wiki knowledge service is unavailable")
		}
		wikiPath := path.Join("rooms", util.UUIDToString(artifact.RoomID), util.UUIDToString(artifact.ID)+".md")
		page, err := targets.wiki(ctx, queries, RoomWikiPageCreateInput{
			WorkspaceID: artifact.WorkspaceID,
			Scope:       "workspace",
			Path:        wikiPath,
			Title:       artifact.Title,
			Content:     artifact.Body,
			ActorType:   "member",
			ActorID:     artifact.CreatedByUserID,
			SourceKind:  "room_promotion",
			SourceRefID: artifact.ID,
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
	if targets == nil || !artifact.TargetID.Valid {
		return
	}
	switch artifact.Kind {
	case "issue":
		if targets.issues == nil {
			return
		}
		issue, err := targets.issues.Queries.GetIssue(ctx, artifact.TargetID)
		if err != nil {
			return
		}
		actorID := util.UUIDToString(artifact.CreatedByUserID)
		targets.issues.publishIssueCreated(issue, nil, nil, "member", actorID, IssueCreateOpts{Platform: "room"})
		targets.issues.captureCreatedAnalytics(issue, "member", actorID, IssueCreateOpts{Platform: "room"})
	case "wiki":
		if targets.wikiPublish != nil {
			_ = targets.wikiPublish(ctx, artifact.TargetID, "member", artifact.CreatedByUserID)
		}
	}
}
