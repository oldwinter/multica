package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRoomReviewableTargetConstructorsExposeCreatorSeam(t *testing.T) {
	if NewRoomWikiProposalTarget() == nil || NewRoomTwinProposalTarget() == nil {
		t.Fatal("Room reviewable target constructor returned nil")
	}
}

func TestRoomWikiProposalTargetFailsClosedBeforeDatabaseAccess(t *testing.T) {
	artifact := roomReviewableArtifact(roomdomain.RecommendationTargetKnowledge)
	artifact.Body = `{"schema_version":1,"page_id":"not-a-uuid"}`
	_, err := NewRoomWikiProposalTarget()(context.Background(), nil, &db.Queries{}, artifact)
	if !errors.Is(err, roomdomain.ErrRecommendationTargetRefused) {
		t.Fatalf("Wiki target error = %v", err)
	}

	artifact.Body = `{"schema_version":1,"page_id":"550e8400-e29b-41d4-a716-446655440000","base_revision_id":"550e8400-e29b-41d4-a716-446655440001","base_revision_number":1,"base_content_digest":"sha256:` + strings.Repeat("a", 64) + `","path":"knowledge.md","title":"Knowledge","content":"Body","unknown":true}`
	var envelope roomWikiProposalEnvelope
	if err := decodeRoomTargetEnvelope(artifact.Body, &envelope); err == nil {
		t.Fatal("Wiki target envelope accepted an unknown field")
	}
	_, err = NewRoomWikiProposalTarget()(context.Background(), nil, &db.Queries{}, artifact)
	if !errors.Is(err, roomdomain.ErrRecommendationTargetRefused) {
		t.Fatalf("Wiki unknown-field error = %v", err)
	}
}

func TestRoomTwinProposalTargetRequiresExactRoomAndAttributionEnvelope(t *testing.T) {
	artifact := roomReviewableArtifact(roomdomain.RecommendationTargetPreference)
	artifact.Body = `{"schema_version":1,"task_id":"550e8400-e29b-41d4-a716-446655440000","attribution_id":"550e8400-e29b-41d4-a716-446655440001","twin_version_id":"550e8400-e29b-41d4-a716-446655440002","briefing_digest":"sha256:` + strings.Repeat("a", 64) + `","assertion_ids":["pref:one"]}`
	_, err := NewRoomTwinProposalTarget()(context.Background(), nil, &db.Queries{}, artifact)
	if !errors.Is(err, roomdomain.ErrRecommendationTargetRefused) {
		t.Fatalf("Twin target without transaction error = %v", err)
	}

	artifact.MemoryRevisionID = pgtype.UUID{}
	_, err = NewRoomTwinProposalTarget()(context.Background(), nil, &db.Queries{}, artifact)
	if !errors.Is(err, roomdomain.ErrRecommendationTargetRefused) {
		t.Fatalf("Twin target without accepted Room provenance error = %v", err)
	}
}

func TestSameRoomTwinAssertionsIsClosedByTargetAndExactIdentity(t *testing.T) {
	preference := []TwinExecutionAssertion{{ID: "pref:b", Type: TwinAssertionPreference}, {ID: "pref:a", Type: TwinAssertionPreference}}
	if !sameRoomTwinAssertions(roomdomain.RecommendationTargetPreference, preference, []string{"pref:a", "pref:b"}) {
		t.Fatal("exact preference attribution was rejected")
	}
	if sameRoomTwinAssertions(roomdomain.RecommendationTargetConstraint, preference, []string{"pref:a", "pref:b"}) {
		t.Fatal("preference attribution crossed into constraint target")
	}
	if sameRoomTwinAssertions(roomdomain.RecommendationTargetPreference, preference, []string{"pref:a", "pref:a"}) {
		t.Fatal("duplicate expected attribution identity was accepted")
	}
	if sameRoomTwinAssertions(roomdomain.RecommendationTargetPreference, preference, []string{"pref:a", "pref:c"}) {
		t.Fatal("mismatched attribution identity was accepted")
	}
}

func TestRoomWikiProposalReplayChecksWholeIntent(t *testing.T) {
	workspaceID, pageID, artifactID := roomArtifactTestUUID(), roomArtifactTestUUID(), roomArtifactTestUUID()
	evidence, _ := json.Marshal([]string{"room:" + artifactID.String()})
	intent := db.CreateRoomWikiPageEditProposalParams{
		WorkspaceID: workspaceID, SourceRefID: artifactID, IdempotencyKey: "room:wiki:v1",
		BaseRevisionNumber: 2, ProposedPath: "knowledge.md", ProposedTitle: "Title",
		ProposedContent: "Content", Rationale: "Reason", EvidenceRefs: evidence, PageID: pageID,
	}
	proposal := db.WikiPageEditProposal{
		WorkspaceID: workspaceID, PageID: pageID, IdempotencyKey: intent.IdempotencyKey,
		BaseRevisionNumber: intent.BaseRevisionNumber, ProposedPath: intent.ProposedPath,
		ProposedTitle: intent.ProposedTitle, ProposedContent: intent.ProposedContent,
		Rationale: intent.Rationale, EvidenceRefs: evidence,
	}
	if !sameRoomWikiProposal(proposal, intent) {
		t.Fatal("exact Wiki proposal replay was rejected")
	}
	proposal.ProposedContent = "changed"
	if sameRoomWikiProposal(proposal, intent) {
		t.Fatal("conflicting Wiki proposal replay was accepted")
	}
}

func TestRoomWikiProposalTargetCreatesPendingProposalWithoutMutatingPage(t *testing.T) {
	pool := roomReviewableTargetTestPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	queries := db.New(pool).WithTx(tx)

	workspaceID, pageID, revisionID := roomArtifactTestUUID(), roomArtifactTestUUID(), roomArtifactTestUUID()
	baseDigest := "sha256:" + strings.Repeat("a", 64)
	if _, err := tx.Exec(ctx, `
INSERT INTO wiki_page (id, workspace_id, scope, path, title, content, current_revision_number, current_revision_id, content_digest)
VALUES ($1, $2, 'workspace', 'knowledge.md', 'Current', 'current body', 3, $3, $4)`,
		pageID, workspaceID, revisionID, baseDigest); err != nil {
		t.Fatal(err)
	}
	artifact := roomReviewableArtifact(roomdomain.RecommendationTargetKnowledge)
	artifact.WorkspaceID = workspaceID
	artifact.Body = `{"schema_version":1,"page_id":"` + pageID.String() + `","base_revision_id":"` + revisionID.String() +
		`","base_revision_number":3,"base_content_digest":"` + baseDigest + `","path":"knowledge.md","title":"Proposed","content":"proposed body"}`

	creator := NewRoomWikiProposalTarget()
	proposalID, err := creator(ctx, tx, queries, artifact)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := creator(ctx, tx, queries, artifact)
	if err != nil || replayed != proposalID {
		t.Fatalf("replay = (%v, %v), want %v", replayed, err, proposalID)
	}
	proposal, err := queries.GetRoomWikiPageEditProposalByIdempotencyKey(ctx, db.GetRoomWikiPageEditProposalByIdempotencyKeyParams{
		WorkspaceID: workspaceID, SourceRefID: artifact.ID, IdempotencyKey: artifact.IdempotencyKey,
	})
	if err != nil || proposal.ID != proposalID || proposal.Status != "pending" || proposal.SourceKind != "room" ||
		proposal.SourceRefID != artifact.ID || proposal.AgentID.Valid {
		t.Fatalf("proposal = (%+v, %v)", proposal, err)
	}
	page, err := queries.GetWikiPageInWorkspace(ctx, db.GetWikiPageInWorkspaceParams{ID: pageID, WorkspaceID: workspaceID})
	if err != nil || page.Content != "current body" || page.CurrentRevisionNumber != 3 || page.CurrentRevisionID != revisionID {
		t.Fatalf("page mutated = (%+v, %v)", page, err)
	}

	artifact.Body = strings.Replace(artifact.Body, "proposed body", "conflicting body", 1)
	if _, err := creator(ctx, tx, queries, artifact); !errors.Is(err, roomdomain.ErrRecommendationTargetRefused) {
		t.Fatalf("conflicting replay error = %v", err)
	}
}

func roomReviewableTargetTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("connect to Postgres: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Skipf("Postgres unavailable: %v", err)
	}
	schema := "room_reviewable_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+identifier+" CASCADE")
		admin.Close()
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `
CREATE TABLE wiki_page (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id UUID, scope TEXT NOT NULL,
    project_id UUID, owner_user_id UUID, path TEXT NOT NULL, title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '', created_by UUID, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), current_revision_number BIGINT NOT NULL DEFAULT 1,
    current_revision_id UUID NOT NULL DEFAULT gen_random_uuid(), content_digest TEXT NOT NULL,
    last_source_kind TEXT NOT NULL DEFAULT 'human', last_actor_type TEXT NOT NULL DEFAULT 'member', last_actor_id UUID
);
CREATE TABLE wiki_page_edit_proposal (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id UUID NOT NULL, page_id UUID NOT NULL,
    base_revision_number BIGINT NOT NULL, proposed_path TEXT NOT NULL, proposed_title TEXT NOT NULL,
    proposed_content TEXT NOT NULL, content_digest TEXT NOT NULL, rationale TEXT NOT NULL,
    evidence_refs JSONB NOT NULL DEFAULT '[]', agent_id UUID, idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', reviewed_by_id UUID, review_reason TEXT,
    reviewed_at TIMESTAMPTZ, accepted_revision_id UUID, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source_kind TEXT NOT NULL DEFAULT 'agent', source_ref_id UUID
);
CREATE UNIQUE INDEX wiki_page_edit_proposal_room_idempotency_uidx
    ON wiki_page_edit_proposal (workspace_id, source_ref_id, idempotency_key) WHERE source_kind = 'room';`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func roomReviewableArtifact(target roomdomain.RecommendationTarget) db.RoomArtifact {
	return db.RoomArtifact{
		ID: roomArtifactTestUUID(), WorkspaceID: roomArtifactTestUUID(), RoomID: roomArtifactTestUUID(),
		Kind: string(target), IdempotencyKey: "room:target:v1", SourceDigest: "sha256:" + strings.Repeat("b", 64),
		CreatedByUserID: roomArtifactTestUUID(), MemoryRevisionID: roomArtifactTestUUID(),
		RecommendationKey: pgtype.Text{String: "recommendation:1", Valid: true},
		Rationale:         pgtype.Text{String: "Reviewed recommendation", Valid: true},
	}
}
