package room

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type recordingTargetContributor struct {
	targetID       pgtype.UUID
	createdQueries *db.Queries
	createdKind    string
	published      []db.RoomArtifact
}

func (contributor *recordingTargetContributor) CreateRoomArtifactTarget(_ context.Context, _ pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	contributor.createdQueries = queries
	contributor.createdKind = artifact.Kind
	return contributor.targetID, nil
}

func (contributor *recordingTargetContributor) RoomArtifactTargetCreated(_ context.Context, artifact db.RoomArtifact) {
	contributor.published = append(contributor.published, artifact)
}

func TestArtifactTargetRouterClosedMatrix(t *testing.T) {
	queries := &db.Queries{}
	artifactID := targetRouterTestUUID()
	targets := []RecommendationTarget{
		RecommendationTargetKnowledge,
		RecommendationTargetPreference,
		RecommendationTargetConstraint,
		RecommendationTargetExecutableProcedure,
		RecommendationTargetImplementationDefect,
	}

	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			router := NewArtifactTargetRouter()
			contributor := &recordingTargetContributor{targetID: targetRouterTestUUID()}
			if err := router.Register(target, contributor); err != nil {
				t.Fatal(err)
			}
			artifact := db.RoomArtifact{ID: artifactID, Kind: string(target)}
			got, err := router.CreateRoomArtifactTarget(context.Background(), nil, queries, artifact)
			if err != nil {
				t.Fatal(err)
			}
			if got != contributor.targetID || contributor.createdQueries != queries || contributor.createdKind != string(target) {
				t.Fatalf("create route = id %v, queries %p, kind %q", got, contributor.createdQueries, contributor.createdKind)
			}
			if len(contributor.published) != 0 {
				t.Fatal("target published during transaction-time creation")
			}
			artifact.TargetID = got
			router.RoomArtifactTargetCreated(context.Background(), artifact)
			if len(contributor.published) != 1 || contributor.published[0].TargetID != got {
				t.Fatalf("post-commit publication = %+v", contributor.published)
			}
		})
	}

	t.Run("decision", func(t *testing.T) {
		router := NewArtifactTargetRouter()
		artifact := db.RoomArtifact{ID: artifactID, Kind: string(RecommendationTargetDecision)}
		got, err := router.CreateRoomArtifactTarget(context.Background(), nil, queries, artifact)
		if err != nil {
			t.Fatal(err)
		}
		if got != artifactID {
			t.Fatalf("decision target = %v, want immutable artifact %v", got, artifactID)
		}
	})

	for _, kind := range []string{string(RecommendationTargetUnsupported), "future_target"} {
		t.Run(kind, func(t *testing.T) {
			_, err := NewArtifactTargetRouter().CreateRoomArtifactTarget(context.Background(), nil, queries, db.RoomArtifact{ID: artifactID, Kind: kind})
			if !errors.Is(err, ErrRecommendationTargetRefused) || !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want reviewable refusal", err)
			}
		})
	}
}

func TestArtifactTargetRouterRegistrationFailsClosed(t *testing.T) {
	router := NewArtifactTargetRouter()
	contributor := &recordingTargetContributor{targetID: targetRouterTestUUID()}
	if err := router.Register(RecommendationTargetKnowledge, contributor); err != nil {
		t.Fatal(err)
	}
	if err := router.Register(RecommendationTargetKnowledge, contributor); !errors.Is(err, ErrDuplicateTargetRegistration) {
		t.Fatalf("duplicate error = %v", err)
	}
	for _, target := range []RecommendationTarget{RecommendationTargetDecision, RecommendationTargetUnsupported, "future_target"} {
		if err := router.Register(target, contributor); !errors.Is(err, ErrInvalidTargetRegistration) {
			t.Fatalf("registration %q error = %v", target, err)
		}
	}
	_, err := (*ArtifactTargetRouter)(nil).CreateRoomArtifactTarget(context.Background(), nil, &db.Queries{}, db.RoomArtifact{Kind: string(RecommendationTargetKnowledge)})
	if !errors.Is(err, ErrRecommendationTargetRefused) {
		t.Fatalf("nil router error = %v", err)
	}
}

func TestArtifactTargetRouterSupportsLegacyAliases(t *testing.T) {
	for legacy, target := range map[string]RecommendationTarget{
		"issue": RecommendationTargetImplementationDefect,
		"wiki":  RecommendationTargetKnowledge,
	} {
		t.Run(legacy, func(t *testing.T) {
			router := NewArtifactTargetRouter()
			contributor := &recordingTargetContributor{targetID: targetRouterTestUUID()}
			if err := router.Register(target, contributor); err != nil {
				t.Fatal(err)
			}
			got, err := router.CreateRoomArtifactTarget(context.Background(), nil, &db.Queries{}, db.RoomArtifact{Kind: legacy})
			if err != nil || got != contributor.targetID {
				t.Fatalf("legacy %q route = %v, %v", legacy, got, err)
			}
		})
	}
}

func targetRouterTestUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}
