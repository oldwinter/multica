package room

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAcceptedOutcomeReaderListsBoundedContentFreeRefsAndLoadsAuthorizedEvidence(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	replacePendingOutcomeWithTwoRecommendations(t, fixture)
	accepted, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "accepted-outcome-reader:accept",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.service.ListAcceptedOutcomeRefs(ctx, fixture.workspaceID, 0); !errors.Is(err, ErrAcceptedOutcomeLimit) {
		t.Fatalf("zero limit error = %v, want accepted outcome limit", err)
	}
	if _, err := fixture.service.ListAcceptedOutcomeRefs(ctx, fixture.workspaceID, MaxAcceptedOutcomeRefs+1); !errors.Is(err, ErrAcceptedOutcomeLimit) {
		t.Fatalf("oversize limit error = %v, want accepted outcome limit", err)
	}
	refs, err := fixture.service.ListAcceptedOutcomeRefs(ctx, fixture.workspaceID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("bounded refs = %+v, want one", refs)
	}
	allRefs, err := fixture.service.ListAcceptedOutcomeRefs(ctx, fixture.workspaceID, MaxAcceptedOutcomeRefs)
	if err != nil {
		t.Fatal(err)
	}
	if len(allRefs) != 2 {
		t.Fatalf("all refs = %+v, want two", allRefs)
	}
	ref := refs[0]
	if ref.WorkspaceID != fixture.workspaceID || ref.RoomID != fixture.detail.Room.ID ||
		ref.MemoryRevisionID != accepted.MemoryRevision.ID || ref.CycleID != fixture.detail.Cycles[0].ID ||
		ref.SourceState != acceptedOutcomeSourceState || !validSHA256Digest(ref.RecommendationKey) ||
		!validSHA256Digest(ref.Digest) || ref.ObservedAt.IsZero() {
		t.Fatalf("accepted outcome ref = %+v", ref)
	}
	for _, forbidden := range []string{"Title", "Body", "Rationale", "Citation", "Summary", "Synthesis"} {
		if _, ok := reflect.TypeOf(ref).FieldByName(forbidden); ok {
			t.Fatalf("AcceptedOutcomeRef must be content-free; found field %q", forbidden)
		}
	}

	evidence, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, ref)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Ref != ref || evidence.Title == "" || evidence.Body == "" ||
		len(evidence.CitationEntryIDs) != len(fixture.participantIDs) || evidence.ReviewedByUserID != fixture.userID ||
		evidence.CycleCompletedAt.IsZero() {
		t.Fatalf("accepted outcome evidence = %+v", evidence)
	}

	otherWorkspace := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, otherWorkspace, ref); !errors.Is(err, ErrAcceptedOutcomeSourceMismatch) {
		t.Fatalf("cross-workspace load error = %v, want source mismatch", err)
	}
	tampered := ref
	tampered.Digest = "sha256:" + strings.Repeat("0", 64)
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, tampered); !errors.Is(err, ErrAcceptedOutcomeSourceMismatch) {
		t.Fatalf("digest mismatch error = %v, want source mismatch", err)
	}
	tampered = ref
	tampered.RecommendationKind = "wiki"
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, tampered); !errors.Is(err, ErrAcceptedOutcomeSourceMismatch) {
		t.Fatalf("kind mismatch error = %v, want source mismatch", err)
	}
	tampered = ref
	tampered.ObservedAt = tampered.ObservedAt.Add(time.Second)
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, tampered); !errors.Is(err, ErrAcceptedOutcomeSourceMismatch) {
		t.Fatalf("observation mismatch error = %v, want source mismatch", err)
	}
}

func TestAcceptedOutcomeReaderRejectsStaleLifecycleAndCanonicalSourceDrift(t *testing.T) {
	fixture := setupPendingOutcome(t)
	ctx := context.Background()
	accepted, err := fixture.service.Review(ctx, ReviewInput{
		WorkspaceID: fixture.workspaceID, RoomID: fixture.detail.Room.ID, CycleID: fixture.detail.Cycles[0].ID,
		ActorUserID: fixture.userID, Action: "accept", ExpectedMemoryVersion: fixture.detail.Room.MemoryVersion,
		IdempotencyKey: "accepted-outcome-reader:drift:accept",
	})
	if err != nil {
		t.Fatal(err)
	}
	refs, err := fixture.service.ListAcceptedOutcomeRefs(ctx, fixture.workspaceID, MaxAcceptedOutcomeRefs)
	if err != nil || len(refs) != 1 {
		t.Fatalf("accepted refs = %+v, %v", refs, err)
	}
	ref := refs[0]
	revision := accepted.MemoryRevision
	cycleID := fixture.detail.Cycles[0].ID

	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET accepted_memory_revision_id = NULL WHERE id = $1`, fixture.detail.Room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, ref); !errors.Is(err, ErrAcceptedOutcomeNotCurrent) {
		t.Fatalf("stale current pointer error = %v, want not current", err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room SET accepted_memory_revision_id = $2 WHERE id = $1`, fixture.detail.Room.ID, revision.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.pool.Exec(ctx, `UPDATE room_cycle SET status = 'running', phase = 'awaiting_review' WHERE id = $1`, cycleID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, ref); !errors.Is(err, ErrAcceptedOutcomeNotCurrent) {
		t.Fatalf("non-completed cycle error = %v, want not current", err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room_cycle SET status = 'completed', phase = 'completed' WHERE id = $1`, cycleID); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.pool.Exec(ctx, `UPDATE room_memory_revision SET digest = $2 WHERE id = $1`, revision.ID, "sha256:"+strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, ref); !errors.Is(err, ErrAcceptedOutcomeDigestDrift) {
		t.Fatalf("revision digest drift error = %v, want digest drift", err)
	}
	if _, err := fixture.pool.Exec(ctx, `UPDATE room_memory_revision SET digest = $2 WHERE id = $1`, revision.ID, revision.Digest); err != nil {
		t.Fatal(err)
	}

	var synthesis Synthesis
	if err := json.Unmarshal(revision.Synthesis, &synthesis); err != nil {
		t.Fatal(err)
	}
	canonicalDrift := synthesis
	canonicalDrift.Recommendations = append([]ArtifactRecommendation(nil), synthesis.Recommendations...)
	canonicalDrift.Recommendations[0].Title = " " + canonicalDrift.Recommendations[0].Title
	updateAcceptedOutcomeSynthesis(t, fixture, revision.ID, canonicalDrift)
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, ref); !errors.Is(err, ErrAcceptedOutcomeDigestDrift) {
		t.Fatalf("non-canonical synthesis error = %v, want digest drift", err)
	}

	keyDrift := synthesis
	keyDrift.Recommendations = append([]ArtifactRecommendation(nil), synthesis.Recommendations...)
	keyDrift.Recommendations[0].Key = "sha256:" + strings.Repeat("0", 64)
	updateAcceptedOutcomeSynthesis(t, fixture, revision.ID, keyDrift)
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, ref); !errors.Is(err, ErrAcceptedOutcomeDigestDrift) {
		t.Fatalf("recommendation key drift error = %v, want digest drift", err)
	}

	citationDrift := synthesis
	citationDrift.Recommendations = append([]ArtifactRecommendation(nil), synthesis.Recommendations...)
	citationDrift.Recommendations[0].CitationEntryIDs = []string{uuid.NewString()}
	updateAcceptedOutcomeSynthesis(t, fixture, revision.ID, citationDrift)
	if _, err := fixture.service.LoadAcceptedOutcomeEvidence(ctx, fixture.workspaceID, ref); !errors.Is(err, ErrInvalidSynthesis) {
		t.Fatalf("citation ownership drift error = %v, want invalid synthesis", err)
	}
}

func replacePendingOutcomeWithTwoRecommendations(t *testing.T, fixture pendingOutcomeFixture) {
	t.Helper()
	var synthesis Synthesis
	if err := json.Unmarshal(fixture.detail.MemoryRevisions[0].Synthesis, &synthesis); err != nil {
		t.Fatal(err)
	}
	second := synthesis.Recommendations[0]
	second.Key = ""
	second.Title += " follow-up"
	second.Body += " Follow up after the initial decision."
	synthesis.Recommendations = append(synthesis.Recommendations, second)
	raw, err := json.Marshal(synthesis)
	if err != nil {
		t.Fatal(err)
	}
	allowed := make(map[string]struct{}, len(fixture.participantIDs))
	for _, id := range fixture.participantIDs {
		allowed[id] = struct{}{}
	}
	_, canonical, digest, err := ValidateSynthesis(raw, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `
		UPDATE room_memory_revision SET synthesis = $2::jsonb, digest = $3 WHERE id = $1
	`, fixture.detail.MemoryRevisions[0].ID, canonical, digest); err != nil {
		t.Fatal(err)
	}
}

func updateAcceptedOutcomeSynthesis(t *testing.T, fixture pendingOutcomeFixture, revisionID pgtype.UUID, synthesis Synthesis) {
	t.Helper()
	raw, err := json.Marshal(synthesis)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(context.Background(), `UPDATE room_memory_revision SET synthesis = $2::jsonb WHERE id = $1`, revisionID, raw); err != nil {
		t.Fatal(err)
	}
}
