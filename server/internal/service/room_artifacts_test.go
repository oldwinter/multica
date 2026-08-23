package service

import (
	"context"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRoomWikiPromotionUsesInjectedKnowledgeCreatorWithProvenance(t *testing.T) {
	artifact := db.RoomArtifact{
		ID:              roomArtifactTestUUID(),
		WorkspaceID:     roomArtifactTestUUID(),
		RoomID:          roomArtifactTestUUID(),
		Kind:            "wiki",
		Title:           "Reviewed outcome",
		Body:            "Human-edited outcome body.",
		CreatedByUserID: roomArtifactTestUUID(),
	}
	wantPageID := roomArtifactTestUUID()
	queries := &db.Queries{}
	var captured RoomWikiPageCreateInput
	var publishedPageID pgtype.UUID
	var publishedActorType string
	var publishedActorID pgtype.UUID
	targets := NewRoomArtifactTargets(nil)
	targets.SetWikiPageCreator(func(_ context.Context, gotQueries *db.Queries, input RoomWikiPageCreateInput) (db.WikiPage, error) {
		if gotQueries != queries {
			t.Fatal("Wiki creator did not receive the transaction-scoped queries")
		}
		captured = input
		return db.WikiPage{ID: wantPageID}, nil
	})
	targets.SetWikiPagePublisher(func(_ context.Context, pageID pgtype.UUID, actorType string, actorID pgtype.UUID) error {
		publishedPageID = pageID
		publishedActorType = actorType
		publishedActorID = actorID
		return nil
	})

	pageID, err := targets.CreateRoomArtifactTarget(context.Background(), nil, queries, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if pageID != wantPageID {
		t.Fatalf("page ID = %v, want %v", pageID, wantPageID)
	}
	if captured.WorkspaceID != artifact.WorkspaceID || captured.ProjectID.Valid || captured.OwnerUserID.Valid ||
		captured.Scope != "workspace" || captured.Path != path.Join("rooms", util.UUIDToString(artifact.RoomID), util.UUIDToString(artifact.ID)+".md") ||
		captured.Title != artifact.Title || captured.Content != artifact.Body || captured.ActorType != "member" ||
		captured.ActorID != artifact.CreatedByUserID || captured.SourceKind != "room_promotion" || captured.SourceRefID != artifact.ID {
		t.Fatalf("Wiki promotion input = %+v", captured)
	}
	artifact.TargetID = wantPageID
	targets.RoomArtifactTargetCreated(context.Background(), artifact)
	if publishedPageID != wantPageID || publishedActorType != "member" || publishedActorID != artifact.CreatedByUserID {
		t.Fatalf("Wiki promotion publish = page %v, actor %q/%v", publishedPageID, publishedActorType, publishedActorID)
	}
}

func TestRoomWikiPromotionFailsClosedWithoutKnowledgeCreator(t *testing.T) {
	artifact := db.RoomArtifact{
		ID: roomArtifactTestUUID(), WorkspaceID: roomArtifactTestUUID(), RoomID: roomArtifactTestUUID(),
		Kind: "wiki", Title: "Outcome", Body: "Body", CreatedByUserID: roomArtifactTestUUID(),
	}
	_, err := NewRoomArtifactTargets(nil).CreateRoomArtifactTarget(context.Background(), nil, &db.Queries{}, artifact)
	if err == nil || err.Error() != "wiki knowledge service is unavailable" {
		t.Fatalf("missing Wiki creator error = %v", err)
	}
}

func roomArtifactTestUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}
