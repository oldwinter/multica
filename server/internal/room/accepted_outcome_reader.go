package room

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	MaxAcceptedOutcomeRefs     = 100
	acceptedOutcomeSourceState = "accepted_current"
)

var (
	ErrAcceptedOutcomeLimit          = errors.New("room accepted outcome reference limit is invalid")
	ErrInvalidAcceptedOutcomeRef     = errors.New("room accepted outcome reference is invalid")
	ErrAcceptedOutcomeNotCurrent     = fmt.Errorf("room accepted outcome is not current: %w", ErrInvalidInput)
	ErrAcceptedOutcomeDigestDrift    = fmt.Errorf("room accepted outcome digest drift: %w", ErrInvalidSynthesis)
	ErrAcceptedOutcomeSourceMismatch = fmt.Errorf("room accepted outcome source mismatch: %w", ErrPromotionSourceMismatch)
)

type AcceptedOutcomeSignals interface {
	ListAcceptedOutcomeRefs(context.Context, pgtype.UUID, int) ([]AcceptedOutcomeRef, error)
	LoadAcceptedOutcomeEvidence(context.Context, pgtype.UUID, AcceptedOutcomeRef) (AcceptedOutcomeEvidence, error)
}

var _ AcceptedOutcomeSignals = (*Service)(nil)

// AcceptedOutcomeRef is the bounded, content-free identity returned during
// signal discovery. Recommendation content is available only through the
// workspace-scoped LoadAcceptedOutcomeEvidence call.
type AcceptedOutcomeRef struct {
	WorkspaceID        pgtype.UUID
	RoomID             pgtype.UUID
	MemoryRevisionID   pgtype.UUID
	CycleID            pgtype.UUID
	RecommendationKey  string
	RecommendationKind string
	SourceState        string
	Digest             string
	ObservedAt         time.Time
}

type AcceptedOutcomeEvidence struct {
	Ref              AcceptedOutcomeRef
	ReviewedByUserID pgtype.UUID
	CycleCompletedAt time.Time
	Title            string
	Body             string
	Rationale        string
	CitationEntryIDs []pgtype.UUID
	Confidence       float64
}

type acceptedCurrentOutcome struct {
	room      db.Room
	revision  db.RoomMemoryRevision
	cycle     db.RoomCycle
	synthesis Synthesis
}

func (s *Service) ListAcceptedOutcomeRefs(ctx context.Context, workspaceID pgtype.UUID, limit int) ([]AcceptedOutcomeRef, error) {
	if !workspaceID.Valid {
		return nil, ErrInvalidInput
	}
	if limit <= 0 || limit > MaxAcceptedOutcomeRefs {
		return nil, ErrAcceptedOutcomeLimit
	}
	signals, err := s.ListValueSignals(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	refs := make([]AcceptedOutcomeRef, 0, min(limit, len(signals)))
	for _, signal := range signals {
		if !signal.RoomID.Valid || !signal.LastAcceptedRevisionID.Valid {
			continue
		}
		roomRow, loadErr := s.queries.GetRoom(ctx, db.GetRoomParams{ID: signal.RoomID, WorkspaceID: workspaceID})
		if errors.Is(loadErr, pgx.ErrNoRows) {
			continue
		}
		if loadErr != nil {
			return nil, fmt.Errorf("load Room accepted outcome source: %w", loadErr)
		}
		outcome, loadErr := loadAcceptedCurrentOutcome(ctx, s.queries, roomRow, workspaceID, signal.RoomID, signal.LastAcceptedRevisionID)
		if errors.Is(loadErr, ErrAcceptedOutcomeNotCurrent) {
			// The accepted pointer can advance between the bounded signal list and
			// this source-owned reload. A later list observes the new revision.
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		for _, recommendation := range outcome.synthesis.Recommendations {
			ref, refErr := newAcceptedOutcomeRef(outcome, recommendation)
			if refErr != nil {
				return nil, refErr
			}
			refs = append(refs, ref)
			if len(refs) == limit {
				return refs, nil
			}
		}
	}
	return refs, nil
}

func (s *Service) LoadAcceptedOutcomeEvidence(ctx context.Context, workspaceID pgtype.UUID, ref AcceptedOutcomeRef) (AcceptedOutcomeEvidence, error) {
	if !workspaceID.Valid || !validAcceptedOutcomeRef(ref) {
		return AcceptedOutcomeEvidence{}, ErrInvalidAcceptedOutcomeRef
	}
	if workspaceID != ref.WorkspaceID {
		return AcceptedOutcomeEvidence{}, ErrAcceptedOutcomeSourceMismatch
	}
	roomRow, err := s.queries.GetRoom(ctx, db.GetRoomParams{ID: ref.RoomID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptedOutcomeEvidence{}, ErrAcceptedOutcomeNotCurrent
	}
	if err != nil {
		return AcceptedOutcomeEvidence{}, fmt.Errorf("load Room accepted outcome source: %w", err)
	}
	outcome, err := loadAcceptedCurrentOutcome(ctx, s.queries, roomRow, workspaceID, ref.RoomID, ref.MemoryRevisionID)
	if err != nil {
		return AcceptedOutcomeEvidence{}, err
	}
	recommendation, ok := FindRecommendation(outcome.synthesis, ref.RecommendationKey)
	if !ok || recommendation.Kind != ref.RecommendationKind {
		return AcceptedOutcomeEvidence{}, ErrAcceptedOutcomeSourceMismatch
	}
	currentRef, err := newAcceptedOutcomeRef(outcome, recommendation)
	if err != nil {
		return AcceptedOutcomeEvidence{}, err
	}
	if !sameAcceptedOutcomeRef(currentRef, ref) {
		return AcceptedOutcomeEvidence{}, ErrAcceptedOutcomeSourceMismatch
	}
	citations, err := acceptedOutcomeCitations(recommendation.CitationEntryIDs)
	if err != nil {
		return AcceptedOutcomeEvidence{}, err
	}
	return AcceptedOutcomeEvidence{
		Ref: currentRef, ReviewedByUserID: outcome.revision.ReviewedByUserID,
		CycleCompletedAt: outcome.cycle.CompletedAt.Time.UTC(), Title: recommendation.Title,
		Body: recommendation.Body, Rationale: recommendation.Rationale,
		CitationEntryIDs: citations, Confidence: recommendation.Confidence,
	}, nil
}

func loadAcceptedCurrentOutcome(
	ctx context.Context,
	queries *db.Queries,
	roomRow db.Room,
	workspaceID, roomID, revisionID pgtype.UUID,
) (acceptedCurrentOutcome, error) {
	if queries == nil || !workspaceID.Valid || !roomID.Valid || !revisionID.Valid ||
		roomRow.WorkspaceID != workspaceID || roomRow.ID != roomID {
		return acceptedCurrentOutcome{}, ErrAcceptedOutcomeNotCurrent
	}
	revision, err := queries.GetRoomMemoryRevision(ctx, db.GetRoomMemoryRevisionParams{
		ID: revisionID, WorkspaceID: workspaceID, RoomID: roomID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return acceptedCurrentOutcome{}, ErrAcceptedOutcomeNotCurrent
	}
	if err != nil {
		return acceptedCurrentOutcome{}, fmt.Errorf("load Room accepted outcome revision: %w", err)
	}
	if revision.WorkspaceID != workspaceID || revision.RoomID != roomID || revision.ID != revisionID ||
		revision.ReviewStatus != "accepted" || !revision.ReviewedByUserID.Valid || !revision.ReviewedAt.Valid ||
		!roomRow.AcceptedMemoryRevisionID.Valid || roomRow.AcceptedMemoryRevisionID != revision.ID {
		return acceptedCurrentOutcome{}, ErrAcceptedOutcomeNotCurrent
	}
	cycle, err := queries.GetRoomCycle(ctx, db.GetRoomCycleParams{
		ID: revision.CycleID, WorkspaceID: workspaceID, RoomID: roomID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return acceptedCurrentOutcome{}, ErrAcceptedOutcomeNotCurrent
	}
	if err != nil {
		return acceptedCurrentOutcome{}, fmt.Errorf("load Room accepted outcome cycle: %w", err)
	}
	if cycle.WorkspaceID != workspaceID || cycle.RoomID != roomID || cycle.ID != revision.CycleID ||
		cycle.Status != "completed" || cycle.Phase != "completed" || !cycle.CompletedAt.Valid ||
		!cycle.MemoryRevisionID.Valid || cycle.MemoryRevisionID != revision.ID {
		return acceptedCurrentOutcome{}, ErrAcceptedOutcomeNotCurrent
	}
	if revision.SchemaVersion != RoomSynthesisSchemaVersion {
		return acceptedCurrentOutcome{}, ErrAcceptedOutcomeDigestDrift
	}
	stored, err := decodeSynthesis(revision.Synthesis)
	if err != nil {
		return acceptedCurrentOutcome{}, err
	}
	synthesis, _, digest, err := validateRoomSynthesis(ctx, queries, workspaceID, roomID, revision.Synthesis)
	if err != nil {
		return acceptedCurrentOutcome{}, err
	}
	if !reflect.DeepEqual(stored, synthesis) || revision.Digest != digest || !validSHA256Digest(revision.Digest) {
		return acceptedCurrentOutcome{}, ErrAcceptedOutcomeDigestDrift
	}
	return acceptedCurrentOutcome{room: roomRow, revision: revision, cycle: cycle, synthesis: synthesis}, nil
}

func newAcceptedOutcomeRef(outcome acceptedCurrentOutcome, recommendation ArtifactRecommendation) (AcceptedOutcomeRef, error) {
	digest, err := acceptedOutcomeDigest(outcome, recommendation)
	if err != nil {
		return AcceptedOutcomeRef{}, err
	}
	ref := AcceptedOutcomeRef{
		WorkspaceID: outcome.room.WorkspaceID, RoomID: outcome.room.ID,
		MemoryRevisionID: outcome.revision.ID, CycleID: outcome.cycle.ID,
		RecommendationKey: recommendation.Key, RecommendationKind: recommendation.Kind,
		SourceState: acceptedOutcomeSourceState, Digest: digest,
		ObservedAt: outcome.revision.ReviewedAt.Time.UTC(),
	}
	if !validAcceptedOutcomeRef(ref) {
		return AcceptedOutcomeRef{}, ErrInvalidAcceptedOutcomeRef
	}
	return ref, nil
}

func acceptedOutcomeDigest(outcome acceptedCurrentOutcome, recommendation ArtifactRecommendation) (string, error) {
	payload := struct {
		Version            int      `json:"version"`
		WorkspaceID        string   `json:"workspace_id"`
		RoomID             string   `json:"room_id"`
		MemoryRevisionID   string   `json:"memory_revision_id"`
		CycleID            string   `json:"cycle_id"`
		RevisionDigest     string   `json:"revision_digest"`
		ReviewedByUserID   string   `json:"reviewed_by_user_id"`
		ReviewedAt         string   `json:"reviewed_at"`
		CycleCompletedAt   string   `json:"cycle_completed_at"`
		RecommendationKey  string   `json:"recommendation_key"`
		RecommendationKind string   `json:"recommendation_kind"`
		Title              string   `json:"title"`
		Body               string   `json:"body"`
		Rationale          string   `json:"rationale"`
		CitationEntryIDs   []string `json:"citation_entry_ids"`
		Confidence         float64  `json:"confidence"`
	}{
		Version: 1, WorkspaceID: util.UUIDToString(outcome.room.WorkspaceID),
		RoomID: util.UUIDToString(outcome.room.ID), MemoryRevisionID: util.UUIDToString(outcome.revision.ID),
		CycleID: util.UUIDToString(outcome.cycle.ID), RevisionDigest: outcome.revision.Digest,
		ReviewedByUserID:  util.UUIDToString(outcome.revision.ReviewedByUserID),
		ReviewedAt:        outcome.revision.ReviewedAt.Time.UTC().Format(time.RFC3339Nano),
		CycleCompletedAt:  outcome.cycle.CompletedAt.Time.UTC().Format(time.RFC3339Nano),
		RecommendationKey: recommendation.Key, RecommendationKind: recommendation.Kind,
		Title: recommendation.Title, Body: recommendation.Body, Rationale: recommendation.Rationale,
		CitationEntryIDs: append([]string(nil), recommendation.CitationEntryIDs...), Confidence: recommendation.Confidence,
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode Room accepted outcome digest: %w", err)
	}
	sum := sha256.Sum256(append([]byte("room-accepted-outcome-v1\x00"), canonical...))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func acceptedOutcomeCitations(values []string) ([]pgtype.UUID, error) {
	citations := make([]pgtype.UUID, len(values))
	seen := make(map[pgtype.UUID]struct{}, len(values))
	for index, value := range values {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return nil, ErrAcceptedOutcomeDigestDrift
		}
		citation := pgtype.UUID{Bytes: parsed, Valid: true}
		if _, duplicate := seen[citation]; duplicate {
			return nil, ErrAcceptedOutcomeDigestDrift
		}
		seen[citation] = struct{}{}
		citations[index] = citation
	}
	return citations, nil
}

func validAcceptedOutcomeRef(ref AcceptedOutcomeRef) bool {
	return ref.WorkspaceID.Valid && ref.RoomID.Valid && ref.MemoryRevisionID.Valid && ref.CycleID.Valid &&
		validSHA256Digest(ref.RecommendationKey) && ref.RecommendationKind != "" &&
		ref.SourceState == acceptedOutcomeSourceState && validSHA256Digest(ref.Digest) && !ref.ObservedAt.IsZero()
}

func validSHA256Digest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || value[:len("sha256:")] != "sha256:" {
		return false
	}
	raw := value[len("sha256:"):]
	if raw != strings.ToLower(raw) {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func sameAcceptedOutcomeRef(left, right AcceptedOutcomeRef) bool {
	return left.WorkspaceID == right.WorkspaceID && left.RoomID == right.RoomID &&
		left.MemoryRevisionID == right.MemoryRevisionID && left.CycleID == right.CycleID &&
		left.RecommendationKey == right.RecommendationKey && left.RecommendationKind == right.RecommendationKind &&
		left.SourceState == right.SourceState && left.Digest == right.Digest && left.ObservedAt.Equal(right.ObservedAt)
}
