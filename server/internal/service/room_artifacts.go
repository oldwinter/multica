package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/issueposition"
	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type RoomArtifactTargets struct {
	router *roomdomain.ArtifactTargetRouter
}

type RoomArtifactTargetCreator func(context.Context, pgx.Tx, *db.Queries, db.RoomArtifact) (pgtype.UUID, error)
type RoomArtifactTargetPublisher func(context.Context, db.RoomArtifact)

type roomArtifactTargetContributor struct {
	create  RoomArtifactTargetCreator
	publish RoomArtifactTargetPublisher
}

func (target roomArtifactTargetContributor) CreateRoomArtifactTarget(ctx context.Context, tx pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	return target.create(ctx, tx, queries, artifact)
}

func (target roomArtifactTargetContributor) RoomArtifactTargetCreated(ctx context.Context, artifact db.RoomArtifact) {
	if target.publish != nil {
		target.publish(ctx, artifact)
	}
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
	targets := &RoomArtifactTargets{router: roomdomain.NewArtifactTargetRouter()}
	if issues != nil {
		contributor := roomArtifactTargetContributor{
			create: func(ctx context.Context, tx pgx.Tx, _ *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
				issue, err := issues.createRoomIssueTarget(ctx, tx, artifact)
				if err != nil {
					return pgtype.UUID{}, err
				}
				return issue.ID, nil
			},
			publish: func(ctx context.Context, artifact db.RoomArtifact) {
				if !artifact.TargetID.Valid {
					return
				}
				issue, err := issues.Queries.GetIssue(ctx, artifact.TargetID)
				if err != nil {
					return
				}
				actorID := util.UUIDToString(artifact.CreatedByUserID)
				issues.publishIssueCreated(issue, nil, nil, "member", actorID, IssueCreateOpts{Platform: "room"})
				issues.captureCreatedAnalytics(issue, "member", actorID, IssueCreateOpts{Platform: "room"})
			},
		}
		_ = targets.router.Register(roomdomain.RecommendationTargetImplementationDefect, contributor)
	}
	return targets
}

// SetWikiPageCreator is kept as a no-op compatibility hook while composition
// moves to RegisterProposalTarget. A Room recommendation must never create a
// Wiki page directly.
func (targets *RoomArtifactTargets) SetWikiPageCreator(creator RoomWikiPageCreator) {
	_ = creator
}

// SetWikiPagePublisher is intentionally inert; proposal publication belongs to
// Wiki's human review lifecycle.
func (targets *RoomArtifactTargets) SetWikiPagePublisher(publisher RoomWikiPagePublisher) {
	_ = publisher
}

func (targets *RoomArtifactTargets) RegisterProposalTarget(target roomdomain.RecommendationTarget, create RoomArtifactTargetCreator, publish RoomArtifactTargetPublisher) error {
	if targets == nil || targets.router == nil || create == nil {
		return roomdomain.ErrInvalidTargetRegistration
	}
	return targets.router.Register(target, roomArtifactTargetContributor{create: create, publish: publish})
}

func (targets *RoomArtifactTargets) CreateRoomArtifactTarget(ctx context.Context, tx pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	if targets == nil || targets.router == nil {
		return pgtype.UUID{}, fmt.Errorf("Room artifact target router is unavailable")
	}
	return targets.router.CreateRoomArtifactTarget(ctx, tx, queries, artifact)
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
	if targets == nil || targets.router == nil || !artifact.TargetID.Valid {
		return
	}
	targets.router.RoomArtifactTargetCreated(ctx, artifact)
}
