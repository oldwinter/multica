package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const MaxTwinSignalRefs = 100

var (
	ErrTwinSignalInvalidInput = errors.New("invalid Twin signal input")
	ErrTwinSignalUnauthorized = errors.New("Twin signal read is not authorized")
	ErrTwinSignalNotFound     = errors.New("Twin signal not found")
	ErrTwinSignalIneligible   = errors.New("Twin signal is not eligible")
	ErrTwinSignalStale        = errors.New("Twin signal changed after it was listed")
)

// TwinSignalAuthorizer owns the caller-specific privacy decision. It must
// enforce workspace membership and any private-Agent/task visibility policy.
// The adapter calls it before loading feedback notes or Twin assertion content.
type TwinSignalAuthorizer interface {
	AuthorizeTwinSignal(ctx context.Context, workspaceID, taskID pgtype.UUID) error
}

type TwinSignalAuthorizerFunc func(context.Context, pgtype.UUID, pgtype.UUID) error

func (f TwinSignalAuthorizerFunc) AuthorizeTwinSignal(ctx context.Context, workspaceID, taskID pgtype.UUID) error {
	if f == nil {
		return ErrTwinSignalUnauthorized
	}
	return f(ctx, workspaceID, taskID)
}

// TwinFeedbackSignalRef is a content-free identity and integrity envelope.
// In particular, it cannot carry the mutable feedback note.
type TwinFeedbackSignalRef struct {
	WorkspaceID   pgtype.UUID
	FeedbackID    pgtype.UUID
	TaskID        pgtype.UUID
	AttributionID pgtype.UUID
	TwinVersionID pgtype.UUID
	State         string
	Digest        string
	ObservedAt    time.Time
}

// TwinFeedbackSignalEvidence is returned only after authorization and digest
// revalidation. It never includes task prompts, task output, Twin assertions,
// citations, or Wiki content.
type TwinFeedbackSignalEvidence struct {
	Ref         TwinFeedbackSignalRef
	Rating      string
	Note        *string
	CompletedAt time.Time
}

// TwinAcceptedDepositionSignalRef binds an accepted deposition to the exact
// proposal and Twin version without exposing either assertion payload.
type TwinAcceptedDepositionSignalRef struct {
	WorkspaceID       pgtype.UUID
	DepositionID      pgtype.UUID
	TaskID            pgtype.UUID
	AttributionID     pgtype.UUID
	ProposalID        pgtype.UUID
	AcceptedVersionID pgtype.UUID
	SourceVersionID   pgtype.UUID
	State             string
	Digest            string
	ObservedAt        time.Time
}

// TwinAcceptedDepositionSignalEvidence is the authorized contentful view.
// ProposalContent is bounded by Twin's existing proposal validation; the
// source Wiki revision body is deliberately not represented.
type TwinAcceptedDepositionSignalEvidence struct {
	Ref                      TwinAcceptedDepositionSignalRef
	DepositionEvidenceDigest string
	ProposalContentDigest    string
	VersionContentDigest     string
	ProposalContent          json.RawMessage
	CompletedAt              time.Time
}

type twinSignalQueries interface {
	GetAgentTaskInWorkspace(context.Context, db.GetAgentTaskInWorkspaceParams) (db.AgentTaskQueue, error)
	ListTwinTaskAttributions(context.Context, db.ListTwinTaskAttributionsParams) ([]db.TwinTaskAttribution, error)
	GetTwinRunFeedback(context.Context, db.GetTwinRunFeedbackParams) (db.TwinRunFeedback, error)
	ListTwinDepositionsForTask(context.Context, db.ListTwinDepositionsForTaskParams) ([]db.TwinDeposition, error)
	GetTwinDeposition(context.Context, db.GetTwinDepositionParams) (db.TwinDeposition, error)
	GetTwinProposal(context.Context, db.GetTwinProposalParams) (db.TwinProposal, error)
	GetTwinProposalReview(context.Context, db.GetTwinProposalReviewParams) (db.TwinProposalReview, error)
	GetTwinVersionByProposal(context.Context, db.GetTwinVersionByProposalParams) (db.TwinVersion, error)
	GetTwinVersion(context.Context, db.GetTwinVersionParams) (db.TwinVersion, error)
	GetCurrentTwinVersion(context.Context, pgtype.UUID) (db.TwinVersion, error)
	GetMemberByUserAndWorkspace(context.Context, db.GetMemberByUserAndWorkspaceParams) (db.Member, error)
}

type TwinSignalAdapter struct {
	queries    twinSignalQueries
	authorizer TwinSignalAuthorizer
}

func NewTwinSignalAdapter(queries *db.Queries, authorizer TwinSignalAuthorizer) *TwinSignalAdapter {
	return newTwinSignalAdapter(queries, authorizer)
}

func newTwinSignalAdapter(queries twinSignalQueries, authorizer TwinSignalAuthorizer) *TwinSignalAdapter {
	return &TwinSignalAdapter{queries: queries, authorizer: authorizer}
}

// ListFeedbackRefs lists the single mutable feedback record owned by a task.
// The task-scoped API is intentionally bounded and avoids a workspace-history
// scan or the broad Twin task-context projection.
func (a *TwinSignalAdapter) ListFeedbackRefs(ctx context.Context, workspaceID, taskID pgtype.UUID, limit int) ([]TwinFeedbackSignalRef, error) {
	if err := validateTwinSignalListInput(workspaceID, taskID, limit); err != nil {
		return nil, err
	}
	if limit == 0 {
		return []TwinFeedbackSignalRef{}, nil
	}
	if err := a.authorize(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	current, err := a.loadFeedback(ctx, workspaceID, taskID)
	if errors.Is(err, ErrTwinSignalNotFound) || errors.Is(err, ErrTwinSignalIneligible) {
		return []TwinFeedbackSignalRef{}, nil
	}
	if err != nil {
		return nil, err
	}
	return []TwinFeedbackSignalRef{current.ref}, nil
}

func (a *TwinSignalAdapter) LoadFeedbackEvidence(ctx context.Context, workspaceID pgtype.UUID, ref TwinFeedbackSignalRef) (TwinFeedbackSignalEvidence, error) {
	if !validTwinFeedbackSignalRef(workspaceID, ref) {
		return TwinFeedbackSignalEvidence{}, ErrTwinSignalInvalidInput
	}
	if err := a.authorize(ctx, workspaceID, ref.TaskID); err != nil {
		return TwinFeedbackSignalEvidence{}, err
	}
	current, err := a.loadFeedback(ctx, workspaceID, ref.TaskID)
	if err != nil {
		return TwinFeedbackSignalEvidence{}, err
	}
	if !sameTwinFeedbackSignalRef(current.ref, ref) {
		return TwinFeedbackSignalEvidence{}, ErrTwinSignalStale
	}
	return TwinFeedbackSignalEvidence{
		Ref: ref, Rating: current.feedback.Rating, Note: twinSignalNote(current.feedback.Note),
		CompletedAt: current.task.CompletedAt.Time.UTC(),
	}, nil
}

// ListAcceptedDepositionRefs returns only current accepted deposition heads.
// It reads one task's existing deposition chain and caps the public result.
func (a *TwinSignalAdapter) ListAcceptedDepositionRefs(ctx context.Context, workspaceID, taskID pgtype.UUID, limit int) ([]TwinAcceptedDepositionSignalRef, error) {
	if err := validateTwinSignalListInput(workspaceID, taskID, limit); err != nil {
		return nil, err
	}
	if limit == 0 {
		return []TwinAcceptedDepositionSignalRef{}, nil
	}
	if err := a.authorize(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	depositions, err := a.queries.ListTwinDepositionsForTask(ctx, db.ListTwinDepositionsForTaskParams{WorkspaceID: workspaceID, TaskID: taskID})
	if err != nil {
		return nil, fmt.Errorf("list Twin signal depositions: %w", err)
	}
	refs := make([]TwinAcceptedDepositionSignalRef, 0, min(limit, len(depositions)))
	for _, deposition := range depositions {
		if deposition.State != "accepted" {
			continue
		}
		current, loadErr := a.loadAcceptedDeposition(ctx, workspaceID, taskID, deposition.ID)
		if errors.Is(loadErr, ErrTwinSignalNotFound) || errors.Is(loadErr, ErrTwinSignalIneligible) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		refs = append(refs, current.ref)
		if len(refs) == limit {
			break
		}
	}
	return refs, nil
}

func (a *TwinSignalAdapter) LoadAcceptedDepositionEvidence(ctx context.Context, workspaceID pgtype.UUID, ref TwinAcceptedDepositionSignalRef) (TwinAcceptedDepositionSignalEvidence, error) {
	if !validTwinAcceptedDepositionSignalRef(workspaceID, ref) {
		return TwinAcceptedDepositionSignalEvidence{}, ErrTwinSignalInvalidInput
	}
	if err := a.authorize(ctx, workspaceID, ref.TaskID); err != nil {
		return TwinAcceptedDepositionSignalEvidence{}, err
	}
	current, err := a.loadAcceptedDeposition(ctx, workspaceID, ref.TaskID, ref.DepositionID)
	if err != nil {
		return TwinAcceptedDepositionSignalEvidence{}, err
	}
	if !sameTwinAcceptedDepositionSignalRef(current.ref, ref) {
		return TwinAcceptedDepositionSignalEvidence{}, ErrTwinSignalStale
	}
	return TwinAcceptedDepositionSignalEvidence{
		Ref: ref, DepositionEvidenceDigest: current.deposition.EvidenceDigest,
		ProposalContentDigest: current.proposal.ContentDigest, VersionContentDigest: current.version.ContentDigest,
		ProposalContent: append(json.RawMessage(nil), current.proposal.Content...),
		CompletedAt:     current.task.CompletedAt.Time.UTC(),
	}, nil
}

type loadedTwinFeedbackSignal struct {
	ref         TwinFeedbackSignalRef
	task        db.AgentTaskQueue
	attribution db.TwinTaskAttribution
	feedback    db.TwinRunFeedback
}

func (a *TwinSignalAdapter) loadFeedback(ctx context.Context, workspaceID, taskID pgtype.UUID) (loadedTwinFeedbackSignal, error) {
	task, attribution, err := a.loadExactCompletedAttribution(ctx, workspaceID, taskID)
	if err != nil {
		return loadedTwinFeedbackSignal{}, err
	}
	feedback, err := a.queries.GetTwinRunFeedback(ctx, db.GetTwinRunFeedbackParams{WorkspaceID: workspaceID, TaskID: taskID})
	if errors.Is(err, pgx.ErrNoRows) {
		return loadedTwinFeedbackSignal{}, ErrTwinSignalNotFound
	}
	if err != nil {
		return loadedTwinFeedbackSignal{}, fmt.Errorf("load Twin feedback signal: %w", err)
	}
	if feedback.WorkspaceID != workspaceID || feedback.TaskID != taskID || !feedback.ID.Valid || !feedback.UpdatedAt.Valid ||
		!isOneOf(feedback.Rating, "helped", "irrelevant", "mismatch") || !validTwinSignalNote(feedback.Note) {
		return loadedTwinFeedbackSignal{}, ErrTwinSignalIneligible
	}
	digest, err := twinFeedbackSignalDigest(feedback, attribution, task)
	if err != nil {
		return loadedTwinFeedbackSignal{}, err
	}
	return loadedTwinFeedbackSignal{
		ref: TwinFeedbackSignalRef{
			WorkspaceID: workspaceID, FeedbackID: feedback.ID, TaskID: taskID,
			AttributionID: attribution.ID, TwinVersionID: attribution.TwinVersionID,
			State: feedback.Rating, Digest: digest, ObservedAt: feedback.UpdatedAt.Time.UTC(),
		},
		task: task, attribution: attribution, feedback: feedback,
	}, nil
}

type loadedTwinDepositionSignal struct {
	ref         TwinAcceptedDepositionSignalRef
	task        db.AgentTaskQueue
	attribution db.TwinTaskAttribution
	deposition  db.TwinDeposition
	proposal    db.TwinProposal
	version     db.TwinVersion
}

func (a *TwinSignalAdapter) loadAcceptedDeposition(ctx context.Context, workspaceID, expectedTaskID, depositionID pgtype.UUID) (loadedTwinDepositionSignal, error) {
	deposition, err := a.queries.GetTwinDeposition(ctx, db.GetTwinDepositionParams{WorkspaceID: workspaceID, ID: depositionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return loadedTwinDepositionSignal{}, ErrTwinSignalNotFound
	}
	if err != nil {
		return loadedTwinDepositionSignal{}, fmt.Errorf("load Twin deposition signal: %w", err)
	}
	if deposition.WorkspaceID != workspaceID || deposition.TaskID != expectedTaskID || deposition.State != "accepted" || !deposition.TaskID.Valid ||
		!deposition.ProposalID.Valid || !deposition.BaseTwinVersionID.Valid || !deposition.UpdatedAt.Valid ||
		!validTwinExecutionDigest(deposition.EvidenceDigest) {
		return loadedTwinDepositionSignal{}, ErrTwinSignalIneligible
	}
	task, attribution, err := a.loadExactCompletedAttribution(ctx, workspaceID, deposition.TaskID)
	if err != nil {
		return loadedTwinDepositionSignal{}, err
	}
	if attribution.TwinVersionID != deposition.BaseTwinVersionID {
		return loadedTwinDepositionSignal{}, ErrTwinSignalIneligible
	}
	depositions, err := a.queries.ListTwinDepositionsForTask(ctx, db.ListTwinDepositionsForTaskParams{WorkspaceID: workspaceID, TaskID: deposition.TaskID})
	if err != nil {
		return loadedTwinDepositionSignal{}, fmt.Errorf("load Twin deposition replacement chain: %w", err)
	}
	for _, candidate := range depositions {
		if candidate.WorkspaceID != workspaceID || candidate.TaskID != deposition.TaskID {
			return loadedTwinDepositionSignal{}, ErrTwinSignalIneligible
		}
		if candidate.ReplacesProposalID == deposition.ProposalID {
			return loadedTwinDepositionSignal{}, ErrTwinSignalIneligible
		}
	}
	proposal, err := a.queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: deposition.ProposalID})
	if err != nil {
		return loadedTwinDepositionSignal{}, twinSignalLoadError("proposal", err)
	}
	review, err := a.queries.GetTwinProposalReview(ctx, db.GetTwinProposalReviewParams{WorkspaceID: workspaceID, ProposalID: proposal.ID})
	if err != nil {
		return loadedTwinDepositionSignal{}, twinSignalLoadError("review", err)
	}
	version, err := a.queries.GetTwinVersionByProposal(ctx, db.GetTwinVersionByProposalParams{WorkspaceID: workspaceID, ProposalID: proposal.ID})
	if err != nil {
		return loadedTwinDepositionSignal{}, twinSignalLoadError("accepted version", err)
	}
	base, err := a.queries.GetTwinVersion(ctx, db.GetTwinVersionParams{WorkspaceID: workspaceID, ID: deposition.BaseTwinVersionID})
	if err != nil {
		return loadedTwinDepositionSignal{}, twinSignalLoadError("source version", err)
	}
	current, err := a.queries.GetCurrentTwinVersion(ctx, workspaceID)
	if err != nil {
		return loadedTwinDepositionSignal{}, twinSignalLoadError("current accepted version", err)
	}
	member, err := a.queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: review.ReviewerID, WorkspaceID: workspaceID})
	if err != nil {
		return loadedTwinDepositionSignal{}, twinSignalLoadError("accepting reviewer", err)
	}
	proposalDigest, proposalCanonical := canonicalTwinSignalContentDigest(proposal.Content)
	versionDigest, versionCanonical := canonicalTwinSignalContentDigest(version.Content)
	baseDigest, baseCanonical := canonicalTwinSignalContentDigest(base.Content)
	if proposal.WorkspaceID != workspaceID || proposal.Kind != "deposition" || proposal.ID != deposition.ProposalID ||
		proposal.SchemaVersion != 2 || proposal.BaseTwinVersionID != base.ID || proposal.SourceWikiRevisionID != base.SourceWikiRevisionID ||
		len(proposal.Content) == 0 || len(proposal.Content) > twinMaxContentBytes ||
		review.WorkspaceID != workspaceID || review.ProposalID != proposal.ID || review.Decision != "accepted" ||
		!review.ID.Valid || !review.ReviewerID.Valid ||
		!isOneOf(member.Role, "owner", "admin") || member.UserID != review.ReviewerID || member.WorkspaceID != workspaceID ||
		version.WorkspaceID != workspaceID || version.SchemaVersion != 2 || version.ProposalID != proposal.ID || version.PriorVersionID != base.ID ||
		version.SourceWikiRevisionID != proposal.SourceWikiRevisionID || version.SignedOffByID != review.ReviewerID || current.ID != version.ID ||
		!version.ID.Valid || !version.SignedOffAt.Valid ||
		!validTwinExecutionDigest(proposal.ContentDigest) || !validTwinExecutionDigest(version.ContentDigest) ||
		!proposalCanonical || !versionCanonical || proposal.ContentDigest != proposalDigest || version.ContentDigest != versionDigest ||
		proposal.ContentDigest != version.ContentDigest || !bytes.Equal(proposal.Content, version.Content) ||
		base.WorkspaceID != workspaceID || base.SchemaVersion != 2 || !base.ID.Valid || !validTwinExecutionDigest(base.ContentDigest) ||
		!baseCanonical || base.ContentDigest != baseDigest {
		return loadedTwinDepositionSignal{}, ErrTwinSignalIneligible
	}
	if err := validateCurrentTwinDepositionEvidence(deposition, task, attribution, a.queries, ctx); err != nil {
		return loadedTwinDepositionSignal{}, err
	}
	digest, err := twinAcceptedDepositionSignalDigest(deposition, attribution, proposal, version)
	if err != nil {
		return loadedTwinDepositionSignal{}, err
	}
	return loadedTwinDepositionSignal{
		ref: TwinAcceptedDepositionSignalRef{
			WorkspaceID: workspaceID, DepositionID: deposition.ID, TaskID: deposition.TaskID,
			AttributionID: attribution.ID, ProposalID: proposal.ID, AcceptedVersionID: version.ID,
			SourceVersionID: base.ID, State: "accepted", Digest: digest, ObservedAt: version.SignedOffAt.Time.UTC(),
		},
		task: task, attribution: attribution, deposition: deposition, proposal: proposal, version: version,
	}, nil
}

func (a *TwinSignalAdapter) loadExactCompletedAttribution(ctx context.Context, workspaceID, taskID pgtype.UUID) (db.AgentTaskQueue, db.TwinTaskAttribution, error) {
	task, err := a.queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{WorkspaceID: workspaceID, ID: taskID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, ErrTwinSignalNotFound
	}
	if err != nil {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, fmt.Errorf("load Twin signal task: %w", err)
	}
	if task.ID != taskID || task.Status != "completed" || !task.CompletedAt.Valid || !task.DispatchedAt.Valid ||
		!task.AgentID.Valid || !task.RuntimeID.Valid {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, ErrTwinSignalIneligible
	}
	attributions, err := a.queries.ListTwinTaskAttributions(ctx, db.ListTwinTaskAttributionsParams{WorkspaceID: workspaceID, TaskID: taskID})
	if err != nil {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, fmt.Errorf("load Twin signal attribution: %w", err)
	}
	if len(attributions) != 1 {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, ErrTwinSignalIneligible
	}
	attribution := attributions[0]
	if attribution.WorkspaceID != workspaceID || attribution.TaskID != task.ID || attribution.AgentID != task.AgentID ||
		attribution.RuntimeID != task.RuntimeID || attribution.TaskDispatchedAt != task.DispatchedAt ||
		!attribution.ID.Valid || !attribution.TwinVersionID.Valid || attribution.PolicyState != string(TwinUseEnabled) ||
		!validTwinExecutionDigest(attribution.BriefingDigest) || attribution.BriefingDigest != TwinBriefingDigest(attribution.Briefing) {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, ErrTwinSignalIneligible
	}
	if _, ok := decodeTwinExecutionStringList(attribution.AssertionIds, twinAssertionIDMaxCount, twinAssertionIDMaxBytes); !ok {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, ErrTwinSignalIneligible
	}
	if _, ok := decodeTwinExecutionStringList(attribution.CitationKeys, twinCitationKeyMaxCount, twinCitationKeyMaxBytes); !ok {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, ErrTwinSignalIneligible
	}
	if task.TwinVersionID.Valid && task.TwinVersionID != attribution.TwinVersionID {
		return db.AgentTaskQueue{}, db.TwinTaskAttribution{}, ErrTwinSignalIneligible
	}
	return task, attribution, nil
}

func (a *TwinSignalAdapter) authorize(ctx context.Context, workspaceID, taskID pgtype.UUID) error {
	if a == nil || a.queries == nil || a.authorizer == nil {
		return ErrTwinSignalUnauthorized
	}
	if err := a.authorizer.AuthorizeTwinSignal(ctx, workspaceID, taskID); err != nil {
		return fmt.Errorf("authorize Twin signal: %w", err)
	}
	return nil
}

func validateCurrentTwinDepositionEvidence(deposition db.TwinDeposition, task db.AgentTaskQueue, attribution db.TwinTaskAttribution, queries twinSignalQueries, ctx context.Context) error {
	assertionIDs, ok := decodeTwinExecutionStringList(attribution.AssertionIds, twinAssertionIDMaxCount, twinAssertionIDMaxBytes)
	if !ok {
		return ErrTwinSignalIneligible
	}
	citationKeys, ok := decodeTwinExecutionStringList(attribution.CitationKeys, twinCitationKeyMaxCount, twinCitationKeyMaxBytes)
	if !ok {
		return ErrTwinSignalIneligible
	}
	feedbackRating := ""
	feedback, err := queries.GetTwinRunFeedback(ctx, db.GetTwinRunFeedbackParams{WorkspaceID: deposition.WorkspaceID, TaskID: deposition.TaskID})
	if err == nil {
		feedbackRating = feedback.Rating
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load Twin deposition signal feedback: %w", err)
	}
	digest, err := twinDepositionEvidenceDigest(twinDepositionEvidence{
		TaskID: deposition.TaskID.String(), AttributionID: attribution.ID.String(), TwinVersionID: attribution.TwinVersionID.String(),
		BriefingDigest: attribution.BriefingDigest, AssertionIDs: assertionIDs, CitationKeys: citationKeys,
		PolicyScopeType: attribution.PolicyScopeType, PolicyScopeID: attribution.PolicyScopeID.String(),
		FeedbackRating: feedbackRating, EditedAssertionsDigest: deposition.EditedAssertionsDigest,
		CompletedAt: task.CompletedAt.Time.UTC().Format(time.RFC3339Nano),
	})
	if err != nil || digest != deposition.EvidenceDigest {
		return ErrTwinSignalIneligible
	}
	return nil
}

func twinFeedbackSignalDigest(feedback db.TwinRunFeedback, attribution db.TwinTaskAttribution, task db.AgentTaskQueue) (string, error) {
	return twinSignalDigest(struct {
		SchemaVersion int    `json:"schema_version"`
		WorkspaceID   string `json:"workspace_id"`
		FeedbackID    string `json:"feedback_id"`
		TaskID        string `json:"task_id"`
		AttributionID string `json:"attribution_id"`
		TwinVersionID string `json:"twin_version_id"`
		CompletedAt   string `json:"completed_at"`
		Rating        string `json:"rating"`
		NotePresent   bool   `json:"note_present"`
		Note          string `json:"note"`
		UpdatedAt     string `json:"updated_at"`
	}{
		SchemaVersion: 1, WorkspaceID: feedback.WorkspaceID.String(), FeedbackID: feedback.ID.String(),
		TaskID: feedback.TaskID.String(), AttributionID: attribution.ID.String(), TwinVersionID: attribution.TwinVersionID.String(),
		CompletedAt: task.CompletedAt.Time.UTC().Format(time.RFC3339Nano), Rating: feedback.Rating,
		NotePresent: feedback.Note.Valid, Note: feedback.Note.String, UpdatedAt: feedback.UpdatedAt.Time.UTC().Format(time.RFC3339Nano),
	})
}

func twinAcceptedDepositionSignalDigest(deposition db.TwinDeposition, attribution db.TwinTaskAttribution, proposal db.TwinProposal, version db.TwinVersion) (string, error) {
	return twinSignalDigest(struct {
		SchemaVersion            int    `json:"schema_version"`
		DepositionID             string `json:"deposition_id"`
		TaskID                   string `json:"task_id"`
		AttributionID            string `json:"attribution_id"`
		SourceVersionID          string `json:"source_version_id"`
		DepositionEvidenceDigest string `json:"deposition_evidence_digest"`
		ProposalID               string `json:"proposal_id"`
		ProposalContentDigest    string `json:"proposal_content_digest"`
		AcceptedVersionID        string `json:"accepted_version_id"`
		AcceptedVersionDigest    string `json:"accepted_version_digest"`
		SignedOffAt              string `json:"signed_off_at"`
	}{
		SchemaVersion: 1, DepositionID: deposition.ID.String(), TaskID: deposition.TaskID.String(),
		AttributionID: attribution.ID.String(), SourceVersionID: deposition.BaseTwinVersionID.String(),
		DepositionEvidenceDigest: deposition.EvidenceDigest, ProposalID: proposal.ID.String(),
		ProposalContentDigest: proposal.ContentDigest, AcceptedVersionID: version.ID.String(),
		AcceptedVersionDigest: version.ContentDigest, SignedOffAt: version.SignedOffAt.Time.UTC().Format(time.RFC3339Nano),
	})
}

func twinSignalDigest(value any) (string, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal Twin signal digest: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func canonicalTwinSignalContentDigest(raw []byte) (string, bool) {
	if len(raw) == 0 || len(raw) > twinMaxContentBytes {
		return "", false
	}
	var content TwinProposalContent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&content); err != nil || content.SchemaVersion != 2 {
		return "", false
	}
	if err := ensureTwinJSONEOF(decoder); err != nil {
		return "", false
	}
	canonical, err := json.Marshal(content)
	if err != nil || len(canonical) > twinMaxContentBytes {
		return "", false
	}
	return digestTwin(canonical), true
}

func validateTwinSignalListInput(workspaceID, taskID pgtype.UUID, limit int) error {
	if !validTwinSignalUUID(workspaceID) || !validTwinSignalUUID(taskID) || limit < 0 || limit > MaxTwinSignalRefs {
		return ErrTwinSignalInvalidInput
	}
	return nil
}

func validTwinFeedbackSignalRef(workspaceID pgtype.UUID, ref TwinFeedbackSignalRef) bool {
	return validTwinSignalUUID(workspaceID) && ref.WorkspaceID == workspaceID && validTwinSignalUUID(ref.FeedbackID) && validTwinSignalUUID(ref.TaskID) &&
		validTwinSignalUUID(ref.AttributionID) && validTwinSignalUUID(ref.TwinVersionID) && isOneOf(ref.State, "helped", "irrelevant", "mismatch") &&
		validTwinExecutionDigest(ref.Digest) && !ref.ObservedAt.IsZero()
}

func validTwinAcceptedDepositionSignalRef(workspaceID pgtype.UUID, ref TwinAcceptedDepositionSignalRef) bool {
	return validTwinSignalUUID(workspaceID) && ref.WorkspaceID == workspaceID && validTwinSignalUUID(ref.DepositionID) && validTwinSignalUUID(ref.TaskID) &&
		validTwinSignalUUID(ref.AttributionID) && validTwinSignalUUID(ref.ProposalID) && validTwinSignalUUID(ref.AcceptedVersionID) && validTwinSignalUUID(ref.SourceVersionID) &&
		ref.State == "accepted" && validTwinExecutionDigest(ref.Digest) && !ref.ObservedAt.IsZero()
}

func sameTwinFeedbackSignalRef(left, right TwinFeedbackSignalRef) bool {
	return left.WorkspaceID == right.WorkspaceID && left.FeedbackID == right.FeedbackID && left.TaskID == right.TaskID &&
		left.AttributionID == right.AttributionID && left.TwinVersionID == right.TwinVersionID && left.State == right.State &&
		left.Digest == right.Digest && left.ObservedAt.Equal(right.ObservedAt)
}

func sameTwinAcceptedDepositionSignalRef(left, right TwinAcceptedDepositionSignalRef) bool {
	return left.WorkspaceID == right.WorkspaceID && left.DepositionID == right.DepositionID && left.TaskID == right.TaskID &&
		left.AttributionID == right.AttributionID && left.ProposalID == right.ProposalID &&
		left.AcceptedVersionID == right.AcceptedVersionID && left.SourceVersionID == right.SourceVersionID &&
		left.State == right.State && left.Digest == right.Digest && left.ObservedAt.Equal(right.ObservedAt)
}

func validTwinSignalNote(note pgtype.Text) bool {
	return !note.Valid || utf8.ValidString(note.String) && strings.IndexByte(note.String, 0) < 0 && utf8.RuneCountInString(note.String) <= twinFeedbackNoteMaxCodepoints
}

func validTwinSignalUUID(value pgtype.UUID) bool {
	return value.Valid && value.Bytes != ([16]byte{})
}

func twinSignalNote(note pgtype.Text) *string {
	if !note.Valid {
		return nil
	}
	value := note.String
	return &value
}

func twinSignalLoadError(source string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTwinSignalIneligible
	}
	return fmt.Errorf("load Twin signal %s: %w", source, err)
}
