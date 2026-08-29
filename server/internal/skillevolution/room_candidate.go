package skillevolution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

const improvementRoomTemplateID = "improvement"

var (
	ErrRoomCandidateInvalid     = errors.New("invalid Improvement Room Skill candidate")
	ErrRoomCandidateNotReady    = errors.New("no accepted Improvement Room Skill candidate is ready")
	ErrRoomCandidateSourceDrift = errors.New("accepted Improvement Room Skill candidate changed")
)

type roomMetadataReader interface {
	GetRoom(context.Context, db.GetRoomParams) (db.Room, error)
	ListRoomArtifacts(context.Context, db.ListRoomArtifactsParams) ([]db.RoomArtifact, error)
	GetRoomRecommendationReview(context.Context, db.GetRoomRecommendationReviewParams) (db.RoomRecommendationReview, error)
}

// RoomCandidateSource is the sole parser for the accepted Improvement Room v1
// envelope. It also supplies the production improver/replay ports: both consume
// only already-authorized ResolvedEvidence and never read task transcripts.
type RoomCandidateSource struct {
	outcomes room.AcceptedOutcomeSignals
	rooms    roomMetadataReader
	signals  *signalSet
}

func NewRoomCandidateSource(outcomes room.AcceptedOutcomeSignals, rooms roomMetadataReader, sources ...SignalSource) (*RoomCandidateSource, error) {
	set, err := newSignalSet(sources)
	if err != nil {
		return nil, err
	}
	if outcomes == nil || rooms == nil || len(sources) == 0 {
		return nil, ErrRoomCandidateInvalid
	}
	return &RoomCandidateSource{outcomes: outcomes, rooms: rooms, signals: set}, nil
}

func NewRoomCandidateEngine(outcomes room.AcceptedOutcomeSignals, queries *db.Queries, sources ...SignalSource) (*RoomCandidateSource, error) {
	return NewRoomCandidateSource(outcomes, queries, sources...)
}

type roomCandidateEnvelope struct {
	SchemaVersion   int                 `json:"schema_version"`
	BaseSkillID     string              `json:"base_skill_id"`
	BaseHash        string              `json:"base_hash"`
	Bundle          roomCandidateBundle `json:"bundle"`
	ObservedPattern string              `json:"observed_pattern"`
	ExpectedBenefit string              `json:"expected_benefit"`
	RegressionRisk  string              `json:"regression_risk"`
	EvidenceDigests []string            `json:"evidence_digests"`
}

type roomCandidateBundle struct {
	ID          string              `json:"id"`
	Source      string              `json:"source"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Content     string              `json:"content"`
	Files       []roomCandidateFile `json:"files"`
}

type roomCandidateFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (source *RoomCandidateSource) LoadAcceptedImprovement(ctx context.Context, request RoomRecommendationRequest) (AcceptedImprovementRecommendation, error) {
	if source == nil || source.outcomes == nil || source.rooms == nil || source.signals == nil ||
		!validUUID(request.WorkspaceID) || !validUUID(request.SkillID) || !boundedToken(request.RecommendationID, 160) {
		return AcceptedImprovementRecommendation{}, ErrRoomCandidateInvalid
	}
	ref, evidence, err := source.loadRecommendation(ctx, request.WorkspaceID, request.RecommendationID)
	if err != nil {
		return AcceptedImprovementRecommendation{}, err
	}
	artifact, review, err := source.approvedArtifact(ctx, ref)
	if err != nil {
		return AcceptedImprovementRecommendation{}, err
	}
	accepted, err := source.acceptedFromEvidence(ctx, request, ref, evidence, review.ReviewedByUserID)
	if err != nil {
		return AcceptedImprovementRecommendation{}, err
	}
	_, artifactCandidate, artifactBaseHash, err := decodeRoomCandidate(artifact.Body)
	if err != nil || artifactBaseHash != accepted.ExpectedBaseHash || !reflect.DeepEqual(artifactCandidate, accepted.Candidate) {
		return AcceptedImprovementRecommendation{}, ErrRoomCandidateSourceDrift
	}
	authorization, err := newChangeAuthorizationArtifact(
		uuidText(artifact.ID), artifactBaseHash, artifactCandidate.Bundle, artifactCandidate.EvidenceDigests,
	)
	if err != nil {
		return AcceptedImprovementRecommendation{}, ErrRoomCandidateInvalid
	}
	accepted.AcceptedByID = review.ReviewedByUserID
	accepted.Authorization = authorization
	return accepted, nil
}

// loadAcceptedImprovement is limited to Room's transaction-time target
// creation, before the approval row is inserted later in the same transaction.
func (source *RoomCandidateSource) loadAcceptedImprovement(ctx context.Context, request RoomRecommendationRequest, approvingActorID pgtype.UUID) (AcceptedImprovementRecommendation, room.AcceptedOutcomeRef, error) {
	if source == nil || source.outcomes == nil || source.rooms == nil || source.signals == nil ||
		!validUUID(request.WorkspaceID) || !validUUID(request.SkillID) || !validUUID(approvingActorID) ||
		!boundedToken(request.RecommendationID, 160) {
		return AcceptedImprovementRecommendation{}, room.AcceptedOutcomeRef{}, ErrRoomCandidateInvalid
	}
	ref, evidence, err := source.loadRecommendation(ctx, request.WorkspaceID, request.RecommendationID)
	if err != nil {
		return AcceptedImprovementRecommendation{}, room.AcceptedOutcomeRef{}, err
	}
	accepted, err := source.acceptedFromEvidence(ctx, request, ref, evidence, approvingActorID)
	return accepted, ref, err
}

func (source *RoomCandidateSource) acceptedFromEvidence(
	ctx context.Context,
	request RoomRecommendationRequest,
	ref room.AcceptedOutcomeRef,
	evidence room.AcceptedOutcomeEvidence,
	approvingActorID pgtype.UUID,
) (AcceptedImprovementRecommendation, error) {
	envelope, candidate, expectedHash, err := decodeRoomCandidate(evidence.Body)
	if err != nil || envelope.BaseSkillID != uuidText(request.SkillID) {
		return AcceptedImprovementRecommendation{}, ErrRoomCandidateInvalid
	}
	synthesisEvidence, err := source.resolveEvidenceDigests(ctx, request.WorkspaceID, request.SkillID, approvingActorID, candidate.EvidenceDigests)
	if err != nil {
		return AcceptedImprovementRecommendation{}, err
	}
	replayEvidence, err := source.resolveHeldOutEvidence(ctx, request.WorkspaceID, request.SkillID, approvingActorID, synthesisEvidence)
	if err != nil {
		return AcceptedImprovementRecommendation{}, err
	}
	return AcceptedImprovementRecommendation{
		WorkspaceID: request.WorkspaceID, SkillID: request.SkillID,
		RecommendationID: improvementRecommendationID(ref), ExpectedBaseHash: expectedHash,
		AcceptedByID: approvingActorID, Candidate: candidate,
		SynthesisEvidence: synthesisEvidence, ReplayEvidence: replayEvidence,
	}, nil
}

// FindAcceptedImprovement selects the newest still-current accepted
// executable_procedure recommendation for a Skill. Invalid or unrelated
// outcomes cannot block a later valid recommendation.
func (source *RoomCandidateSource) FindAcceptedImprovement(ctx context.Context, workspaceID, skillID pgtype.UUID) (RoomRecommendationRequest, error) {
	if source == nil || !validUUID(workspaceID) || !validUUID(skillID) {
		return RoomRecommendationRequest{}, ErrRoomCandidateInvalid
	}
	refs, err := source.outcomes.ListAcceptedOutcomeRefs(ctx, workspaceID, room.MaxAcceptedOutcomeRefs)
	if err != nil {
		return RoomRecommendationRequest{}, err
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].ObservedAt.After(refs[j].ObservedAt) })
	for _, ref := range refs {
		if ref.RecommendationKind != string(room.RecommendationTargetExecutableProcedure) {
			continue
		}
		request := RoomRecommendationRequest{WorkspaceID: workspaceID, SkillID: skillID, RecommendationID: improvementRecommendationID(ref)}
		accepted, loadErr := source.LoadAcceptedImprovement(ctx, request)
		if loadErr == nil && accepted.SkillID == skillID {
			return request, nil
		}
		if loadErr != nil && !errors.Is(loadErr, ErrRoomCandidateInvalid) && !errors.Is(loadErr, ErrRoomCandidateSourceDrift) &&
			!errors.Is(loadErr, ErrSignalSourceDrift) && !errors.Is(loadErr, ErrRoomCandidateNotReady) {
			return RoomRecommendationRequest{}, loadErr
		}
	}
	return RoomRecommendationRequest{}, ErrRoomCandidateNotReady
}

func (source *RoomCandidateSource) Improve(_ context.Context, request ImprovementRequest) (ImprovementCandidate, error) {
	base, err := skillbundle.BuildValidatedManifest(request.Base)
	if err != nil {
		return ImprovementCandidate{}, ErrImproverOutput
	}
	for _, evidence := range request.Evidence {
		if evidence.Ref.Kind != EvidenceKindRoomOutcome {
			continue
		}
		var payload roomOutcomePayload
		if decodeStrictJSON(evidence.Payload, &payload) != nil || payload.RecommendationKind != string(room.RecommendationTargetExecutableProcedure) {
			continue
		}
		envelope, candidate, expectedHash, err := decodeRoomCandidate(payload.Body)
		if err != nil || envelope.BaseSkillID != request.Base.ID || expectedHash != Digest(base.Hash) ||
			!candidateClaimsResolvedEvidence(candidate.EvidenceDigests, request.Evidence) {
			continue
		}
		return candidate, nil
	}
	return ImprovementCandidate{}, ErrRoomCandidateNotReady
}

func (source *RoomCandidateSource) Replay(_ context.Context, request ReplayRequest) (ReplayResult, error) {
	base, baseErr := skillbundle.BuildValidatedManifest(request.Base)
	candidate, candidateErr := ValidateCandidateBundle(request.Candidate)
	if baseErr != nil || candidateErr != nil || request.Base.ID != request.Candidate.ID || base.Hash == candidate.Hash || len(request.Evidence) == 0 {
		return ReplayResult{Result: EvaluationResultFailed, ReasonCode: "invalid_candidate"}, nil
	}
	samples := min(request.Limits.MaxSamples, len(request.Evidence))
	for _, evidence := range request.Evidence[:samples] {
		if evidence.Ref.Validate() != nil || evidence.Ref.Eligibility != EvidenceEligibilityEligible ||
			len(evidence.Payload) > MaxResolvedEvidenceBytes || !replayEvidenceContractValid(evidence) {
			return ReplayResult{Result: EvaluationResultFailed, SampleCount: samples, FailureCount: 1, ReasonCode: "evidence_revalidation_failed"}, nil
		}
	}
	// The server has no source-owned runner capable of executing the same held-
	// out task with both bundles. Structural evidence validation is not a
	// behavioral comparison, so production records an explicit unsupported
	// outcome and blocks review instead of claiming a pass.
	return ReplayResult{Result: EvaluationResultUnknown, SampleCount: samples, ReasonCode: "behavioral_runner_unavailable"}, nil
}

func replayEvidenceContractValid(evidence ResolvedEvidence) bool {
	switch evidence.Ref.Kind {
	case EvidenceKindTaskReview:
		var payload struct {
			Outcome    service.TaskRunReviewOutcome `json:"outcome"`
			Correction string                       `json:"correction,omitempty"`
			Reason     string                       `json:"reason"`
		}
		return decodeStrictJSON(evidence.Payload, &payload) == nil && payload.Outcome.Valid() &&
			string(payload.Outcome) == evidence.Ref.SourceState && strings.TrimSpace(payload.Reason) != "" &&
			(payload.Outcome != service.TaskRunReviewOutcomeNeedsCorrection || strings.TrimSpace(payload.Correction) != "")
	case EvidenceKindManualRerun:
		var payload struct {
			Classification    string `json:"classification"`
			RequestedByUserID string `json:"requested_by_user_id"`
			TaskStatus        string `json:"task_status"`
			SourceTaskStatus  string `json:"source_task_status"`
		}
		if decodeStrictJSON(evidence.Payload, &payload) != nil {
			return false
		}
		_, actorErr := parseUUIDText(payload.RequestedByUserID)
		return actorErr == nil && payload.Classification == service.ManualRerunClassificationCorrectionRequested &&
			payload.Classification == evidence.Ref.SourceState && replayTerminalTaskState(payload.TaskStatus) &&
			replayTerminalTaskState(payload.SourceTaskStatus)
	case EvidenceKindWikiProposal:
		var payload struct {
			Path         string   `json:"path"`
			Title        string   `json:"title"`
			Content      string   `json:"content"`
			Rationale    string   `json:"rationale"`
			ReviewReason string   `json:"review_reason,omitempty"`
			Citations    []string `json:"citations,omitempty"`
		}
		return decodeStrictJSON(evidence.Payload, &payload) == nil &&
			(evidence.Ref.SourceState == "accepted" || evidence.Ref.SourceState == "rejected") &&
			strings.TrimSpace(payload.Path) != "" && strings.TrimSpace(payload.Title) != "" && strings.TrimSpace(payload.Rationale) != ""
	case EvidenceKindRoomOutcome:
		var payload roomOutcomePayload
		if decodeStrictJSON(evidence.Payload, &payload) != nil {
			return false
		}
		target := room.RecommendationTarget(payload.RecommendationKind)
		return evidence.Ref.SourceState == "accepted_current" && target.Valid() &&
			strings.TrimSpace(payload.RecommendationKey) != "" && strings.TrimSpace(payload.Title) != "" &&
			strings.TrimSpace(payload.Body) != "" && strings.TrimSpace(payload.Rationale) != "" &&
			payload.Confidence >= 0 && payload.Confidence <= 1
	case EvidenceKindTwinRunFeedback:
		var payload struct {
			Rating string  `json:"rating"`
			Note   *string `json:"note,omitempty"`
		}
		return decodeStrictJSON(evidence.Payload, &payload) == nil && payload.Rating == evidence.Ref.SourceState &&
			(payload.Rating == "helped" || payload.Rating == "irrelevant" || payload.Rating == "mismatch")
	case EvidenceKindTwinDeposition:
		var payload struct {
			ProposalContent json.RawMessage `json:"proposal_content"`
		}
		return decodeStrictJSON(evidence.Payload, &payload) == nil && evidence.Ref.SourceState == "accepted" &&
			len(payload.ProposalContent) > 0 && json.Valid(payload.ProposalContent) && string(payload.ProposalContent) != "null"
	default:
		return false
	}
}

func replayTerminalTaskState(value string) bool {
	return value == "completed" || value == "failed" || value == "cancelled"
}

func (source *RoomCandidateSource) loadRecommendation(ctx context.Context, workspaceID pgtype.UUID, recommendationID string) (room.AcceptedOutcomeRef, room.AcceptedOutcomeEvidence, error) {
	refs, err := source.outcomes.ListAcceptedOutcomeRefs(ctx, workspaceID, room.MaxAcceptedOutcomeRefs)
	if err != nil {
		return room.AcceptedOutcomeRef{}, room.AcceptedOutcomeEvidence{}, err
	}
	var matched room.AcceptedOutcomeRef
	for _, ref := range refs {
		if improvementRecommendationID(ref) == recommendationID {
			if matched.RoomID.Valid {
				return room.AcceptedOutcomeRef{}, room.AcceptedOutcomeEvidence{}, ErrRoomCandidateInvalid
			}
			matched = ref
		}
	}
	if !matched.RoomID.Valid || matched.RecommendationKind != string(room.RecommendationTargetExecutableProcedure) ||
		matched.SourceState != "accepted_current" {
		return room.AcceptedOutcomeRef{}, room.AcceptedOutcomeEvidence{}, ErrRoomCandidateNotReady
	}
	roomRow, err := source.rooms.GetRoom(ctx, db.GetRoomParams{ID: matched.RoomID, WorkspaceID: workspaceID})
	if err != nil || roomRow.WorkspaceID != workspaceID || roomRow.ID != matched.RoomID ||
		!roomRow.TemplateID.Valid || roomRow.TemplateID.String != improvementRoomTemplateID {
		return room.AcceptedOutcomeRef{}, room.AcceptedOutcomeEvidence{}, ErrRoomCandidateInvalid
	}
	evidence, err := source.outcomes.LoadAcceptedOutcomeEvidence(ctx, workspaceID, matched)
	if err != nil {
		return room.AcceptedOutcomeRef{}, room.AcceptedOutcomeEvidence{}, err
	}
	if !evidence.ReviewedByUserID.Valid || !sameAcceptedOutcomeRef(evidence.Ref, matched) {
		return room.AcceptedOutcomeRef{}, room.AcceptedOutcomeEvidence{}, ErrRoomCandidateSourceDrift
	}
	return matched, evidence, nil
}

func sameAcceptedOutcomeRef(left, right room.AcceptedOutcomeRef) bool {
	return left.WorkspaceID == right.WorkspaceID && left.RoomID == right.RoomID &&
		left.MemoryRevisionID == right.MemoryRevisionID && left.CycleID == right.CycleID &&
		left.RecommendationKey == right.RecommendationKey && left.RecommendationKind == right.RecommendationKind &&
		left.SourceState == right.SourceState && left.Digest == right.Digest && left.ObservedAt.Equal(right.ObservedAt)
}

func (source *RoomCandidateSource) approvedArtifact(ctx context.Context, ref room.AcceptedOutcomeRef) (db.RoomArtifact, db.RoomRecommendationReview, error) {
	if source == nil || source.rooms == nil || !ref.WorkspaceID.Valid || !ref.RoomID.Valid || !ref.MemoryRevisionID.Valid {
		return db.RoomArtifact{}, db.RoomRecommendationReview{}, ErrRoomCandidateInvalid
	}
	artifacts, err := source.rooms.ListRoomArtifacts(ctx, db.ListRoomArtifactsParams{WorkspaceID: ref.WorkspaceID, RoomID: ref.RoomID})
	if err != nil {
		return db.RoomArtifact{}, db.RoomRecommendationReview{}, err
	}
	var matched db.RoomArtifact
	for _, artifact := range artifacts {
		if artifact.WorkspaceID != ref.WorkspaceID || artifact.RoomID != ref.RoomID ||
			artifact.MemoryRevisionID != ref.MemoryRevisionID || artifact.Kind != string(room.RecommendationTargetExecutableProcedure) ||
			!artifact.RecommendationKey.Valid || artifact.RecommendationKey.String != ref.RecommendationKey || !artifact.TargetID.Valid {
			continue
		}
		if matched.ID.Valid {
			return db.RoomArtifact{}, db.RoomRecommendationReview{}, ErrRoomCandidateInvalid
		}
		matched = artifact
	}
	if !matched.ID.Valid {
		return db.RoomArtifact{}, db.RoomRecommendationReview{}, ErrRoomCandidateNotReady
	}
	review, err := source.rooms.GetRoomRecommendationReview(ctx, db.GetRoomRecommendationReviewParams{
		WorkspaceID: ref.WorkspaceID, RoomID: ref.RoomID, MemoryRevisionID: ref.MemoryRevisionID,
		RecommendationKey: ref.RecommendationKey,
	})
	if err != nil {
		return db.RoomArtifact{}, db.RoomRecommendationReview{}, err
	}
	if review.WorkspaceID != ref.WorkspaceID || review.RoomID != ref.RoomID || review.MemoryRevisionID != ref.MemoryRevisionID ||
		review.RecommendationKey != ref.RecommendationKey || review.Status != "approved" || !review.ArtifactID.Valid ||
		review.ArtifactID != matched.ID || !review.ReviewedByUserID.Valid || !review.ReviewedAt.Valid {
		return db.RoomArtifact{}, db.RoomRecommendationReview{}, ErrRoomCandidateNotReady
	}
	return matched, review, nil
}

func (source *RoomCandidateSource) FindApprovedArtifact(ctx context.Context, workspaceID, skillID pgtype.UUID) (db.RoomArtifact, error) {
	request, err := source.FindAcceptedImprovement(ctx, workspaceID, skillID)
	if err != nil {
		return db.RoomArtifact{}, err
	}
	refs, err := source.outcomes.ListAcceptedOutcomeRefs(ctx, workspaceID, room.MaxAcceptedOutcomeRefs)
	if err != nil {
		return db.RoomArtifact{}, err
	}
	for _, ref := range refs {
		if improvementRecommendationID(ref) != request.RecommendationID {
			continue
		}
		artifact, _, err := source.approvedArtifact(ctx, ref)
		return artifact, err
	}
	return db.RoomArtifact{}, ErrRoomCandidateNotReady
}

func (source *RoomCandidateSource) resolveEvidenceDigests(
	ctx context.Context,
	workspaceID, skillID, actorID pgtype.UUID,
	digests []Digest,
) ([]ResolvedEvidence, error) {
	if source.signals == nil || len(digests) == 0 || len(digests) > MaxEvidenceRefs {
		return nil, ErrRoomCandidateInvalid
	}
	wanted := make(map[Digest]struct{}, len(digests))
	for _, digest := range digests {
		if !digest.Valid() {
			return nil, ErrRoomCandidateInvalid
		}
		if _, duplicate := wanted[digest]; duplicate {
			return nil, ErrRoomCandidateInvalid
		}
		wanted[digest] = struct{}{}
	}
	query := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, ActorID: actorID, Limit: MaxEvidenceRefs}
	type located struct {
		source SignalSource
		ref    EvidenceRef
	}
	found := make(map[Digest]located, len(digests))
	for _, adapter := range source.signals.ordered {
		refs, err := adapter.List(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			if _, needed := wanted[ref.Digest]; !needed || ref.Eligibility != EvidenceEligibilityEligible ||
				(ref.TargetSkillID != "" && ref.TargetSkillID != uuidText(skillID)) {
				continue
			}
			if _, ambiguous := found[ref.Digest]; ambiguous {
				return nil, ErrRoomCandidateInvalid
			}
			found[ref.Digest] = located{source: adapter, ref: ref}
		}
	}
	resolved := make([]ResolvedEvidence, 0, len(digests))
	totalBytes := 0
	for _, digest := range digests {
		match, ok := found[digest]
		if !ok {
			return nil, ErrRoomCandidateNotReady
		}
		evidence, err := match.source.Load(ctx, query, match.ref)
		if err != nil {
			return nil, err
		}
		totalBytes += len(evidence.Payload)
		if evidence.Ref.Digest != digest || totalBytes > MaxResolvedEvidenceBytes {
			return nil, ErrRoomCandidateSourceDrift
		}
		resolved = append(resolved, evidence)
	}
	return resolved, nil
}

func (source *RoomCandidateSource) resolveHeldOutEvidence(
	ctx context.Context,
	workspaceID, skillID, actorID pgtype.UUID,
	synthesis []ResolvedEvidence,
) ([]ResolvedEvidence, error) {
	query := SignalQuery{WorkspaceID: workspaceID, SkillID: skillID, ActorID: actorID, Limit: MaxEvidenceRefs}
	refs, err := source.signals.discover(ctx, query)
	if err != nil {
		return nil, err
	}
	heldOut, err := selectHeldOutEvidenceRefs(refs, synthesis, MaxEvidenceRefs)
	if err != nil {
		return nil, err
	}
	return source.signals.resolve(ctx, query, heldOut)
}

func selectHeldOutEvidenceRefs(refs []EvidenceRef, synthesis []ResolvedEvidence, limit int) ([]EvidenceRef, error) {
	if limit < len(synthesis) || limit > MaxEvidenceRefs {
		return nil, ErrRoomCandidateInvalid
	}
	excludedLineages := make(map[string]struct{}, len(synthesis))
	excludedDigests := make(map[Digest]struct{}, len(synthesis))
	for _, evidence := range synthesis {
		lineage, ok := evidenceCaseLineageKey(evidence.Ref)
		if !ok {
			return nil, ErrRoomCandidateInvalid
		}
		excludedLineages[lineage] = struct{}{}
		excludedDigests[evidence.Ref.Digest] = struct{}{}
	}
	selectedLineages := make(map[string]struct{}, len(refs))
	heldOut := make([]EvidenceRef, 0, min(len(refs), limit-len(synthesis)))
	for _, ref := range refs {
		if len(heldOut)+len(synthesis) >= limit {
			break
		}
		// The accepted Room outcome contains the synthesized candidate itself;
		// it can never be an independent behavioral replay case.
		if ref.Kind == EvidenceKindRoomOutcome {
			continue
		}
		lineage, ok := evidenceCaseLineageKey(ref)
		if !ok {
			continue
		}
		if _, used := excludedLineages[lineage]; used {
			continue
		}
		if _, duplicate := selectedLineages[lineage]; duplicate {
			continue
		}
		if _, used := excludedDigests[ref.Digest]; used {
			continue
		}
		selectedLineages[lineage] = struct{}{}
		heldOut = append(heldOut, ref)
	}
	return heldOut, nil
}

func decodeRoomCandidate(raw string) (roomCandidateEnvelope, ImprovementCandidate, Digest, error) {
	var envelope roomCandidateEnvelope
	if len(raw) == 0 || len(raw) > 2*skillbundle.MaxBundleBytes+MaxImprovementRationaleBytes*3 ||
		!utf8.ValidString(raw) || strings.IndexByte(raw, 0) >= 0 || decodeStrictJSON([]byte(raw), &envelope) != nil ||
		envelope.SchemaVersion != 1 || envelope.BaseSkillID == "" || envelope.Bundle.ID != envelope.BaseSkillID ||
		envelope.Bundle.Source != skillbundle.SourceWorkspace || !validRationale(envelope.ObservedPattern) ||
		!validRationale(envelope.ExpectedBenefit) || !validRationale(envelope.RegressionRisk) ||
		len(envelope.EvidenceDigests) == 0 || len(envelope.EvidenceDigests) > MaxEvidenceRefs {
		return roomCandidateEnvelope{}, ImprovementCandidate{}, "", ErrRoomCandidateInvalid
	}
	baseID, err := uuid.Parse(envelope.BaseSkillID)
	if err != nil || baseID == uuid.Nil {
		return roomCandidateEnvelope{}, ImprovementCandidate{}, "", ErrRoomCandidateInvalid
	}
	baseHash, err := ParseDigest(envelope.BaseHash)
	if err != nil {
		return roomCandidateEnvelope{}, ImprovementCandidate{}, "", ErrRoomCandidateInvalid
	}
	bundle := skillbundle.Skill{ID: envelope.Bundle.ID, Source: envelope.Bundle.Source, Name: envelope.Bundle.Name,
		Description: envelope.Bundle.Description, Content: envelope.Bundle.Content, Files: make([]skillbundle.File, len(envelope.Bundle.Files))}
	for index, file := range envelope.Bundle.Files {
		bundle.Files[index] = skillbundle.File{Path: file.Path, Content: file.Content}
	}
	if _, err := ValidateCandidateBundle(bundle); err != nil {
		return roomCandidateEnvelope{}, ImprovementCandidate{}, "", ErrRoomCandidateInvalid
	}
	digests := make([]Digest, len(envelope.EvidenceDigests))
	seen := make(map[Digest]struct{}, len(digests))
	for index, value := range envelope.EvidenceDigests {
		digest, err := ParseDigest(value)
		if err != nil {
			return roomCandidateEnvelope{}, ImprovementCandidate{}, "", ErrRoomCandidateInvalid
		}
		if _, duplicate := seen[digest]; duplicate {
			return roomCandidateEnvelope{}, ImprovementCandidate{}, "", ErrRoomCandidateInvalid
		}
		seen[digest] = struct{}{}
		digests[index] = digest
	}
	return envelope, ImprovementCandidate{Bundle: bundle, ObservedPattern: envelope.ObservedPattern,
		ExpectedBenefit: envelope.ExpectedBenefit, RegressionRisk: envelope.RegressionRisk,
		EvidenceDigests: digests}, baseHash, nil
}

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrRoomCandidateInvalid
	}
	return nil
}

func improvementRecommendationID(ref room.AcceptedOutcomeRef) string {
	if !ref.RoomID.Valid || !ref.MemoryRevisionID.Valid || ref.RecommendationKey == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", uuidText(ref.RoomID), uuidText(ref.MemoryRevisionID), ref.RecommendationKey)
}

// RoomProposalRequester is the production proposal entry point for both HTTP
// and scheduled work. It only processes a current human-accepted Improvement
// Room recommendation; it never falls back to Lifecycle.Generate.
type RoomProposalRequester struct {
	source *RoomCandidateSource
	target roomProposalProcessor
	queuer ImprovementRoomQueuer
}

type roomProposalProcessor interface {
	ProcessRoomArtifactTarget(context.Context, db.RoomArtifact) (Generation, error)
}

type ImprovementRoomQueueResult struct {
	RoomID          pgtype.UUID
	EligibleSignals int
}

type ImprovementRoomQueuer interface {
	EnsureImprovementRoom(context.Context, pgtype.UUID, pgtype.UUID, pgtype.UUID, string) (ImprovementRoomQueueResult, error)
}

type ProposalRequestResult struct {
	State           string
	RoomID          pgtype.UUID
	Generation      *Generation
	EligibleSignals int
}

func NewRoomProposalRequester(source *RoomCandidateSource, target roomProposalProcessor, queuers ...ImprovementRoomQueuer) *RoomProposalRequester {
	requester := &RoomProposalRequester{source: source, target: target}
	if len(queuers) > 0 {
		requester.queuer = queuers[0]
	}
	return requester
}

func (requester *RoomProposalRequester) RequestProposal(
	ctx context.Context,
	workspaceID, skillID, actorID pgtype.UUID,
	idempotencyKey string,
) (ProposalRequestResult, error) {
	if requester == nil || requester.source == nil || requester.target == nil || !validUUID(workspaceID) ||
		!validUUID(skillID) || !validOptionalUUID(actorID) || !boundedToken(idempotencyKey, MaxIdempotencyKeyBytes) {
		return ProposalRequestResult{}, ErrRoomCandidateInvalid
	}
	artifact, err := requester.source.FindApprovedArtifact(ctx, workspaceID, skillID)
	if err == nil {
		// Recovery retains the promotion's original idempotency key and queued
		// proposal identity. A caller-supplied key must never create a sibling.
		generation, processErr := requester.target.ProcessRoomArtifactTarget(ctx, artifact)
		if processErr != nil {
			return ProposalRequestResult{}, processErr
		}
		return ProposalRequestResult{
			State: "proposal_" + generation.Proposal.State, RoomID: artifact.RoomID,
			Generation: &generation, EligibleSignals: roomCandidateEvidenceCount(artifact.Body),
		}, nil
	}
	if !errors.Is(err, ErrRoomCandidateNotReady) {
		return ProposalRequestResult{}, err
	}
	if requester.queuer == nil {
		return ProposalRequestResult{}, err
	}
	queued, err := requester.queuer.EnsureImprovementRoom(ctx, workspaceID, skillID, actorID, idempotencyKey)
	if err != nil {
		return ProposalRequestResult{}, err
	}
	if !validUUID(queued.RoomID) || queued.EligibleSignals < 0 || queued.EligibleSignals > MaxEvidenceRefs {
		return ProposalRequestResult{}, ErrRoomCandidateInvalid
	}
	return ProposalRequestResult{State: "improvement_room_queued", RoomID: queued.RoomID, EligibleSignals: queued.EligibleSignals}, nil
}

func roomCandidateEvidenceCount(raw string) int {
	_, candidate, _, err := decodeRoomCandidate(raw)
	if err != nil {
		return 0
	}
	return len(candidate.EvidenceDigests)
}
