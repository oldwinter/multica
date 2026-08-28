package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRoomKnowledgePromotionUsesRegisteredProposalContributor(t *testing.T) {
	artifact := db.RoomArtifact{
		ID:              roomArtifactTestUUID(),
		WorkspaceID:     roomArtifactTestUUID(),
		RoomID:          roomArtifactTestUUID(),
		Kind:            string(roomdomain.RecommendationTargetKnowledge),
		Title:           "Reviewed outcome",
		Body:            "Human-edited outcome body.",
		CreatedByUserID: roomArtifactTestUUID(),
	}
	wantProposalID := roomArtifactTestUUID()
	queries := &db.Queries{}
	var captured db.RoomArtifact
	var published db.RoomArtifact
	directPageMutationCalled := false
	targets := NewRoomArtifactTargets(nil)
	targets.SetWikiPageCreator(func(context.Context, *db.Queries, RoomWikiPageCreateInput) (db.WikiPage, error) {
		directPageMutationCalled = true
		return db.WikiPage{}, nil
	})
	if err := targets.RegisterProposalTarget(
		roomdomain.RecommendationTargetKnowledge,
		func(_ context.Context, _ pgx.Tx, gotQueries *db.Queries, got db.RoomArtifact) (pgtype.UUID, error) {
			if gotQueries != queries {
				t.Fatal("proposal contributor did not receive transaction-scoped queries")
			}
			captured = got
			return wantProposalID, nil
		},
		func(_ context.Context, got db.RoomArtifact) { published = got },
	); err != nil {
		t.Fatal(err)
	}

	proposalID, err := targets.CreateRoomArtifactTarget(context.Background(), nil, queries, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if proposalID != wantProposalID || captured.ID != artifact.ID {
		t.Fatalf("proposal route = id %v, artifact %+v", proposalID, captured)
	}
	if directPageMutationCalled || published.ID.Valid {
		t.Fatal("knowledge route mutated or published before commit")
	}
	artifact.TargetID = proposalID
	targets.RoomArtifactTargetCreated(context.Background(), artifact)
	if published.TargetID != proposalID {
		t.Fatalf("published artifact = %+v", published)
	}
}

func TestLegacyWikiPageMutationHooksRemainFailClosed(t *testing.T) {
	artifact := db.RoomArtifact{
		ID: roomArtifactTestUUID(), WorkspaceID: roomArtifactTestUUID(), RoomID: roomArtifactTestUUID(),
		Kind: "wiki", Title: "Outcome", Body: "Body", CreatedByUserID: roomArtifactTestUUID(),
	}
	called := false
	targets := NewRoomArtifactTargets(nil)
	targets.SetWikiPageCreator(func(context.Context, *db.Queries, RoomWikiPageCreateInput) (db.WikiPage, error) {
		called = true
		return db.WikiPage{}, nil
	})
	_, err := targets.CreateRoomArtifactTarget(context.Background(), nil, &db.Queries{}, artifact)
	if !errors.Is(err, roomdomain.ErrRecommendationTargetRefused) {
		t.Fatalf("error = %v, want reviewable refusal", err)
	}
	if called {
		t.Fatal("legacy Wiki creator mutated a page")
	}
}

func TestRoomProposalTargetRegistrationFollowsClosedMatrix(t *testing.T) {
	targets := NewRoomArtifactTargets(nil)
	creator := func(context.Context, pgx.Tx, *db.Queries, db.RoomArtifact) (pgtype.UUID, error) {
		return roomArtifactTestUUID(), nil
	}
	for _, target := range []roomdomain.RecommendationTarget{
		roomdomain.RecommendationTargetKnowledge,
		roomdomain.RecommendationTargetPreference,
		roomdomain.RecommendationTargetConstraint,
		roomdomain.RecommendationTargetExecutableProcedure,
	} {
		if err := targets.RegisterProposalTarget(target, creator, nil); err != nil {
			t.Fatalf("register %q: %v", target, err)
		}
	}
	for _, target := range []roomdomain.RecommendationTarget{
		roomdomain.RecommendationTargetDecision,
		roomdomain.RecommendationTargetUnsupported,
		"future_target",
	} {
		if err := targets.RegisterProposalTarget(target, creator, nil); !errors.Is(err, roomdomain.ErrInvalidTargetRegistration) {
			t.Fatalf("register %q error = %v", target, err)
		}
	}
}

func roomArtifactTestUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}
