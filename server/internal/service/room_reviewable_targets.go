package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	roomdomain "github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const roomReviewableTargetSchemaVersion = 1

type roomWikiProposalEnvelope struct {
	SchemaVersion      int    `json:"schema_version"`
	PageID             string `json:"page_id"`
	BaseRevisionID     string `json:"base_revision_id"`
	BaseRevisionNumber int64  `json:"base_revision_number"`
	BaseContentDigest  string `json:"base_content_digest"`
	Path               string `json:"path"`
	Title              string `json:"title"`
	Content            string `json:"content"`
}

type roomTwinProposalEnvelope struct {
	SchemaVersion  int      `json:"schema_version"`
	TaskID         string   `json:"task_id"`
	AttributionID  string   `json:"attribution_id"`
	TwinVersionID  string   `json:"twin_version_id"`
	BriefingDigest string   `json:"briefing_digest"`
	AssertionIDs   []string `json:"assertion_ids"`
}

// NewRoomWikiProposalTarget creates a pending Wiki edit proposal inside the
// Room promotion transaction. It never creates or mutates a Wiki page.
func NewRoomWikiProposalTarget() RoomArtifactTargetCreator {
	return createRoomWikiProposalTarget
}

// NewRoomTwinProposalTarget creates a pending Twin deposition inside the Room
// promotion transaction. Sign-off remains exclusively in Twin's human review.
func NewRoomTwinProposalTarget() RoomArtifactTargetCreator {
	return createRoomTwinProposalTarget
}

func createRoomWikiProposalTarget(ctx context.Context, tx pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	if tx == nil || queries == nil || !validRoomReviewableArtifact(artifact, roomdomain.RecommendationTargetKnowledge) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "invalid_room_provenance")
	}
	var envelope roomWikiProposalEnvelope
	if err := decodeRoomTargetEnvelope(artifact.Body, &envelope); err != nil ||
		envelope.SchemaVersion != roomReviewableTargetSchemaVersion || envelope.BaseRevisionNumber <= 0 ||
		!validWikiKnowledgePath(envelope.Path) || !validRoomTargetDigest(envelope.BaseContentDigest) ||
		len([]rune(envelope.Title)) > 500 || len(envelope.Content) > 2*1024*1024 {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "invalid_wiki_proposal")
	}
	pageID, pageErr := util.ParseUUID(envelope.PageID)
	baseRevisionID, revisionErr := util.ParseUUID(envelope.BaseRevisionID)
	if pageErr != nil || revisionErr != nil {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "invalid_wiki_provenance")
	}
	page, err := queries.GetWikiPageInWorkspace(ctx, db.GetWikiPageInWorkspaceParams{ID: pageID, WorkspaceID: artifact.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "wiki_page_unavailable")
	}
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("load Room Wiki proposal page: %w", err)
	}
	if (page.Scope != "workspace" && page.Scope != "project") || page.CurrentRevisionID != baseRevisionID ||
		page.CurrentRevisionNumber != envelope.BaseRevisionNumber || page.ContentDigest != envelope.BaseContentDigest {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "stale_wiki_provenance")
	}
	envelope.Path = normalizeWikiKnowledgePath(envelope.Path)
	evidence, _ := json.Marshal([]string{"room:" + util.UUIDToString(artifact.ID)})
	intent := db.CreateRoomWikiPageEditProposalParams{
		WorkspaceID: artifact.WorkspaceID, SourceRefID: artifact.ID,
		IdempotencyKey: artifact.IdempotencyKey, BaseRevisionNumber: envelope.BaseRevisionNumber,
		ProposedPath: envelope.Path, ProposedTitle: strings.TrimSpace(envelope.Title), ProposedContent: envelope.Content,
		Rationale: strings.TrimSpace(artifact.Rationale.String), EvidenceRefs: evidence, PageID: page.ID,
	}
	if existing, err := queries.GetRoomWikiPageEditProposalByIdempotencyKey(ctx, db.GetRoomWikiPageEditProposalByIdempotencyKeyParams{
		WorkspaceID: artifact.WorkspaceID, SourceRefID: artifact.ID, IdempotencyKey: artifact.IdempotencyKey,
	}); err == nil {
		if !sameRoomWikiProposal(existing, intent) {
			return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "wiki_proposal_idempotency_conflict")
		}
		return existing.ID, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, fmt.Errorf("load repeated Room Wiki proposal: %w", err)
	}

	created, err := queries.CreateRoomWikiPageEditProposal(ctx, intent)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "stale_wiki_provenance")
	}
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("create Room Wiki proposal: %w", err)
	}
	proposal := db.WikiPageEditProposal(created)
	if !sameRoomWikiProposal(proposal, intent) || proposal.SourceKind != "room" || proposal.SourceRefID != artifact.ID || proposal.Status != "pending" {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "wiki_proposal_idempotency_conflict")
	}
	return proposal.ID, nil
}

func createRoomTwinProposalTarget(ctx context.Context, tx pgx.Tx, queries *db.Queries, artifact db.RoomArtifact) (pgtype.UUID, error) {
	target := roomdomain.RecommendationTarget(artifact.Kind)
	if tx == nil || queries == nil || (target != roomdomain.RecommendationTargetPreference && target != roomdomain.RecommendationTargetConstraint) ||
		!validRoomReviewableArtifact(artifact, target) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "invalid_room_provenance")
	}
	var envelope roomTwinProposalEnvelope
	if err := decodeRoomTargetEnvelope(artifact.Body, &envelope); err != nil ||
		envelope.SchemaVersion != roomReviewableTargetSchemaVersion || !validTwinExecutionDigest(envelope.BriefingDigest) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "invalid_twin_provenance")
	}
	taskID, taskErr := util.ParseUUID(envelope.TaskID)
	attributionID, attributionErr := util.ParseUUID(envelope.AttributionID)
	versionID, versionErr := util.ParseUUID(envelope.TwinVersionID)
	if taskErr != nil || attributionErr != nil || versionErr != nil {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "invalid_twin_provenance")
	}
	task, err := queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: taskID, WorkspaceID: artifact.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (task.Status != "completed" || !task.CompletedAt.Valid) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "twin_task_unavailable")
	}
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("load Room Twin task: %w", err)
	}
	execution := NewTwinExecutionService(queries, true)
	attributions, err := execution.Store.ListTaskAttributions(ctx, artifact.WorkspaceID, taskID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("load Room Twin attribution: %w", err)
	}
	if len(attributions) == 0 || attributions[0].ID != attributionID || attributions[0].TwinVersionID != versionID ||
		attributions[0].BriefingDigest != envelope.BriefingDigest {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "twin_attribution_unavailable")
	}
	mapped, ok := execution.mapTaskAttribution(ctx, artifact.WorkspaceID, task, attributions[0])
	if !ok || !sameRoomTwinAssertions(target, mapped.Assertions, envelope.AssertionIDs) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "twin_attribution_mismatch")
	}
	if err := lockTwinWorkspace(ctx, queries, artifact.WorkspaceID); err != nil {
		return pgtype.UUID{}, err
	}
	execution.DepositionCreator = &roomTwinDepositionCreator{queries: queries}
	result, err := execution.CreateDeposition(ctx, artifact.WorkspaceID, taskID, TwinDepositionRequest{RequestedByID: artifact.CreatedByUserID})
	if errors.Is(err, ErrTwinExecutionNotFound) || errors.Is(err, ErrTwinExecutionAttributionMissing) ||
		errors.Is(err, ErrTwinExecutionConflict) || errors.Is(err, ErrTwinBaseStale) ||
		errors.Is(err, ErrTwinWikiStale) || errors.Is(err, ErrTwinDepositionEvidenceStale) {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "stale_twin_provenance")
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if !result.Proposal.ID.Valid || result.Proposal.Kind != "deposition" || result.Proposal.WorkspaceID != artifact.WorkspaceID ||
		result.Deposition.ProposalID != result.Proposal.ID || result.Deposition.State != "pending" {
		return pgtype.UUID{}, refuseRoomTarget(artifact.Kind, "invalid_twin_proposal")
	}
	return result.Proposal.ID, nil
}

// roomTwinDepositionCreator is the transaction-scoped form of TwinService's
// existing deposition persistence. The caller already owns the Room write
// transaction, so this adapter must neither begin nor commit a second one.
type roomTwinDepositionCreator struct {
	queries *db.Queries
}

func (creator *roomTwinDepositionCreator) CreateDepositionProposal(ctx context.Context, input TwinDepositionProposalInput) (TwinDepositionResult, error) {
	if creator == nil || creator.queries == nil {
		return TwinDepositionResult{}, ErrTwinDepositionUnavailable
	}
	if err := validateTwinDepositionProposalInput(input); err != nil {
		return TwinDepositionResult{}, err
	}
	if existing, ok, err := resolveTwinDepositionRequest(ctx, creator.queries, input); err != nil {
		return TwinDepositionResult{}, err
	} else if ok {
		return existing, nil
	}
	_, base, err := NewTwinExecutionService(creator.queries, true).loadSignedVersion(ctx, input.WorkspaceID, input.BaseTwinVersion.ID)
	if err != nil || base.ID != input.BaseTwinVersion.ID || base.ContentDigest != input.BaseTwinVersion.ContentDigest ||
		base.SourceWikiRevisionID != input.BaseTwinVersion.SourceWikiRevisionID {
		return TwinDepositionResult{}, ErrTwinBaseStale
	}
	current, err := creator.queries.GetCurrentTwinVersion(ctx, input.WorkspaceID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && current.ID != base.ID {
		return TwinDepositionResult{}, ErrTwinBaseStale
	}
	if err != nil {
		return TwinDepositionResult{}, fmt.Errorf("load current Twin deposition base: %w", err)
	}
	evidence, err := NewDBTwinEvidenceProvider(creator.queries).LoadAcceptedEvidence(ctx, input.WorkspaceID, base.SourceWikiRevisionID)
	if err != nil {
		return TwinDepositionResult{}, err
	}
	build, err := buildTwinDepositionProposal(evidence, base, input)
	if err != nil {
		return TwinDepositionResult{}, err
	}
	proposal, err := creator.queries.CreateTwinDepositionProposalV2(ctx, db.CreateTwinDepositionProposalV2Params{
		WorkspaceID: input.WorkspaceID, Content: build.CanonicalJSON, ContentDigest: build.ContentDigest,
		RequestedByID: input.RequestedByID, BaseTwinVersionID: base.ID, SourceWikiRevisionID: base.SourceWikiRevisionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinDepositionResult{}, ErrTwinBaseStale
	}
	if err != nil {
		return TwinDepositionResult{}, fmt.Errorf("create Room Twin deposition proposal: %w", err)
	}
	deposition, err := NewTwinExecutionStore(creator.queries).LinkDeposition(ctx, TwinDepositionInput{
		WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, BaseTwinVersionID: base.ID,
		ProposalID: proposal.ID, ReplacesProposalID: input.ReplacesProposalID,
		EvidenceDigest: input.EvidenceDigest, EditedAssertionsDigest: input.EditedDigest,
	})
	if err != nil {
		return TwinDepositionResult{}, err
	}
	if deposition.ProposalID != proposal.ID {
		existing, err := creator.queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: input.WorkspaceID, ID: deposition.ProposalID})
		if err != nil {
			return TwinDepositionResult{}, fmt.Errorf("load repeated Room Twin deposition: %w", err)
		}
		return TwinDepositionResult{Proposal: existing, Deposition: deposition}, nil
	}
	return TwinDepositionResult{Created: true, Proposal: proposal, Deposition: deposition}, nil
}

func validRoomReviewableArtifact(artifact db.RoomArtifact, target roomdomain.RecommendationTarget) bool {
	return artifact.ID.Valid && artifact.WorkspaceID.Valid && artifact.RoomID.Valid && artifact.CreatedByUserID.Valid &&
		artifact.MemoryRevisionID.Valid && artifact.RecommendationKey.Valid && strings.TrimSpace(artifact.RecommendationKey.String) != "" &&
		roomdomain.RecommendationTarget(artifact.Kind) == target && strings.TrimSpace(artifact.IdempotencyKey) != "" &&
		len(artifact.IdempotencyKey) <= 200 && artifact.Rationale.Valid &&
		strings.TrimSpace(artifact.Rationale.String) != "" && len([]rune(artifact.Rationale.String)) <= 8000 &&
		validRoomTargetDigest(artifact.SourceDigest)
}

func decodeRoomTargetEnvelope(raw string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Room target envelope contains trailing JSON")
	}
	return nil
}

func sameRoomWikiProposal(proposal db.WikiPageEditProposal, intent db.CreateRoomWikiPageEditProposalParams) bool {
	return proposal.WorkspaceID == intent.WorkspaceID && proposal.PageID == intent.PageID &&
		proposal.BaseRevisionNumber == intent.BaseRevisionNumber && proposal.ProposedPath == intent.ProposedPath &&
		proposal.ProposedTitle == intent.ProposedTitle && proposal.ProposedContent == intent.ProposedContent &&
		proposal.Rationale == intent.Rationale && bytes.Equal(proposal.EvidenceRefs, intent.EvidenceRefs) &&
		proposal.IdempotencyKey == intent.IdempotencyKey
}

func sameRoomTwinAssertions(target roomdomain.RecommendationTarget, assertions []TwinExecutionAssertion, expected []string) bool {
	if len(assertions) == 0 || len(assertions) != len(expected) {
		return false
	}
	wantType := TwinAssertionPreference
	if target == roomdomain.RecommendationTargetConstraint {
		wantType = TwinAssertionConstraint
	}
	actualIDs := make([]string, len(assertions))
	for index, assertion := range assertions {
		if assertion.Type != wantType {
			return false
		}
		actualIDs[index] = assertion.ID
	}
	expected = append([]string(nil), expected...)
	sort.Strings(actualIDs)
	sort.Strings(expected)
	for index := range actualIDs {
		if actualIDs[index] == "" || actualIDs[index] != expected[index] || index > 0 && expected[index] == expected[index-1] {
			return false
		}
	}
	return true
}

func validRoomTargetDigest(value string) bool {
	return validTwinExecutionDigest(value)
}

func refuseRoomTarget(target, reason string) error {
	return &roomdomain.RecommendationTargetRefusal{Target: target, Reason: reason}
}

var _ TwinDepositionProposalCreator = (*roomTwinDepositionCreator)(nil)
