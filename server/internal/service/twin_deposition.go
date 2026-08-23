package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CreateDepositionProposal converts privacy-safe execution evidence into a
// reviewable schema-v2 proposal. It never sends execution data to the model:
// only an explicit human edit or a deterministic confidence adjustment can
// change signed assertions, and acceptance remains a separate human action.
func (s *TwinService) CreateDepositionProposal(ctx context.Context, input TwinDepositionProposalInput) (TwinDepositionResult, error) {
	if err := validateTwinDepositionProposalInput(input); err != nil {
		return TwinDepositionResult{}, err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return TwinDepositionResult{}, fmt.Errorf("begin Twin deposition proposal: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := s.Queries.WithTx(tx)
	if err := lockTwinWorkspace(ctx, queries, input.WorkspaceID); err != nil {
		return TwinDepositionResult{}, err
	}
	if existing, ok, err := resolveTwinDepositionRequest(ctx, queries, input); err != nil {
		return TwinDepositionResult{}, err
	} else if ok {
		return existing, nil
	}
	_, base, err := NewTwinExecutionService(queries, true).loadSignedVersion(ctx, input.WorkspaceID, input.BaseTwinVersion.ID)
	if errors.Is(err, ErrTwinExecutionNotFound) {
		return TwinDepositionResult{}, ErrTwinBaseStale
	}
	if err != nil {
		return TwinDepositionResult{}, fmt.Errorf("verify Twin deposition base: %w", err)
	}
	current, err := queries.GetCurrentTwinVersion(ctx, input.WorkspaceID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.ID != base.ID) {
		return TwinDepositionResult{}, ErrTwinBaseStale
	}
	if err != nil {
		return TwinDepositionResult{}, fmt.Errorf("load current Twin deposition base: %w", err)
	}
	if base.SchemaVersion != 2 || base.ContentDigest != input.BaseTwinVersion.ContentDigest || base.SourceWikiRevisionID != input.BaseTwinVersion.SourceWikiRevisionID {
		return TwinDepositionResult{}, ErrTwinBaseStale
	}

	provider := s.EvidenceProvider
	if provider == nil {
		return TwinDepositionResult{}, ErrTwinDepositionUnavailable
	}
	if transactional, ok := provider.(transactionalTwinEvidenceProvider); ok {
		provider = transactional.withQueries(queries)
	}
	evidence, err := provider.LoadAcceptedEvidence(ctx, input.WorkspaceID, base.SourceWikiRevisionID)
	if err != nil {
		return TwinDepositionResult{}, err
	}
	build, err := buildTwinDepositionProposal(evidence, base, input)
	if err != nil {
		return TwinDepositionResult{}, err
	}
	proposal, err := queries.CreateTwinDepositionProposalV2(ctx, db.CreateTwinDepositionProposalV2Params{
		WorkspaceID: input.WorkspaceID, BaseTwinVersionID: base.ID,
		SourceWikiRevisionID: base.SourceWikiRevisionID, Content: build.CanonicalJSON,
		ContentDigest: build.ContentDigest, RequestedByID: input.RequestedByID,
	})
	if err != nil {
		return TwinDepositionResult{}, fmt.Errorf("create Twin deposition proposal: %w", err)
	}
	deposition, err := NewTwinExecutionStore(queries).LinkDeposition(ctx, TwinDepositionInput{
		WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, BaseTwinVersionID: base.ID,
		ProposalID: proposal.ID, ReplacesProposalID: input.ReplacesProposalID,
		EvidenceDigest: input.EvidenceDigest, EditedAssertionsDigest: input.EditedDigest,
	})
	if err != nil {
		return TwinDepositionResult{}, err
	}
	if deposition.ProposalID != proposal.ID {
		existing, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: input.WorkspaceID, ID: deposition.ProposalID})
		if err != nil {
			return TwinDepositionResult{}, fmt.Errorf("load concurrent Twin deposition proposal: %w", err)
		}
		return TwinDepositionResult{Proposal: existing, Deposition: deposition}, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return TwinDepositionResult{}, fmt.Errorf("commit Twin deposition proposal: %w", err)
	}
	return TwinDepositionResult{Created: true, Proposal: proposal, Deposition: deposition}, nil
}

func resolveTwinDepositionRequest(ctx context.Context, queries *db.Queries, input TwinDepositionProposalInput) (TwinDepositionResult, bool, error) {
	depositions, err := queries.ListTwinDepositionsForTask(ctx, db.ListTwinDepositionsForTaskParams{WorkspaceID: input.WorkspaceID, TaskID: input.TaskID})
	if err != nil {
		return TwinDepositionResult{}, false, fmt.Errorf("list Twin deposition request chain: %w", err)
	}
	for _, deposition := range depositions {
		if deposition.BaseTwinVersionID == input.BaseTwinVersion.ID && deposition.ReplacesProposalID == input.ReplacesProposalID &&
			deposition.EvidenceDigest == input.EvidenceDigest && deposition.EditedAssertionsDigest == input.EditedDigest {
			proposal, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: input.WorkspaceID, ID: deposition.ProposalID})
			if err != nil {
				return TwinDepositionResult{}, false, fmt.Errorf("load repeated Twin deposition proposal: %w", err)
			}
			return TwinDepositionResult{Proposal: proposal, Deposition: deposition}, true, nil
		}
	}
	if !input.ReplacesProposalID.Valid && len(input.EditedAssertions) == 0 {
		for _, deposition := range depositions {
			if deposition.BaseTwinVersionID != input.BaseTwinVersion.ID || deposition.ReplacesProposalID.Valid {
				continue
			}
			proposal, err := queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: input.WorkspaceID, ID: deposition.ProposalID})
			if err != nil {
				return TwinDepositionResult{}, false, fmt.Errorf("load initial Twin deposition proposal: %w", err)
			}
			return TwinDepositionResult{Proposal: proposal, Deposition: deposition}, true, nil
		}
		return TwinDepositionResult{}, false, nil
	}
	if !input.ReplacesProposalID.Valid {
		return TwinDepositionResult{}, false, invalidTwinExecutionInput("edited assertions replacement")
	}
	targetPending := false
	for _, deposition := range depositions {
		if deposition.ProposalID == input.ReplacesProposalID {
			targetPending = deposition.State == "pending"
		}
		if deposition.ReplacesProposalID == input.ReplacesProposalID {
			return TwinDepositionResult{}, false, ErrTwinExecutionConflict
		}
	}
	if !targetPending {
		return TwinDepositionResult{}, false, ErrTwinExecutionConflict
	}
	return TwinDepositionResult{}, false, nil
}

func validateTwinDepositionReview(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, proposal db.TwinProposal, decision string) error {
	if proposal.Kind != "deposition" {
		return nil
	}
	deposition, err := queries.GetTwinDepositionByProposal(ctx, db.GetTwinDepositionByProposalParams{WorkspaceID: workspaceID, ProposalID: proposal.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTwinNotFound
	}
	if err != nil {
		return fmt.Errorf("load Twin deposition review chain: %w", err)
	}
	depositions, err := queries.ListTwinDepositionsForTask(ctx, db.ListTwinDepositionsForTaskParams{WorkspaceID: workspaceID, TaskID: deposition.TaskID})
	if err != nil {
		return fmt.Errorf("list Twin deposition review chain: %w", err)
	}
	for _, candidate := range depositions {
		if candidate.ReplacesProposalID == proposal.ID {
			return ErrTwinExecutionConflict
		}
	}
	if decision != "accepted" {
		return nil
	}
	task, err := queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{WorkspaceID: workspaceID, ID: deposition.TaskID})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (task.Status != "completed" || !task.CompletedAt.Valid) {
		return ErrTwinDepositionEvidenceStale
	}
	if err != nil {
		return fmt.Errorf("load Twin deposition review task: %w", err)
	}
	attributions, err := queries.ListTwinTaskAttributions(ctx, db.ListTwinTaskAttributionsParams{WorkspaceID: workspaceID, TaskID: deposition.TaskID})
	if err != nil {
		return fmt.Errorf("load Twin deposition review attribution: %w", err)
	}
	if len(attributions) == 0 || attributions[0].TwinVersionID != deposition.BaseTwinVersionID {
		return ErrTwinDepositionEvidenceStale
	}
	attribution := attributions[0]
	assertionIDs, ok := decodeTwinExecutionStringList(attribution.AssertionIds, twinAssertionIDMaxCount, twinAssertionIDMaxBytes)
	if !ok {
		return ErrTwinDepositionEvidenceStale
	}
	citationKeys, ok := decodeTwinExecutionStringList(attribution.CitationKeys, twinCitationKeyMaxCount, twinCitationKeyMaxBytes)
	if !ok {
		return ErrTwinDepositionEvidenceStale
	}
	feedbackRating := ""
	feedback, err := queries.GetTwinRunFeedback(ctx, db.GetTwinRunFeedbackParams{WorkspaceID: workspaceID, TaskID: deposition.TaskID})
	if err == nil {
		feedbackRating = feedback.Rating
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load Twin deposition review feedback: %w", err)
	}
	evidenceDigest, err := twinDepositionEvidenceDigest(twinDepositionEvidence{
		TaskID: deposition.TaskID.String(), AttributionID: attribution.ID.String(), TwinVersionID: attribution.TwinVersionID.String(),
		BriefingDigest: attribution.BriefingDigest, AssertionIDs: assertionIDs, CitationKeys: citationKeys,
		PolicyScopeType: attribution.PolicyScopeType, PolicyScopeID: attribution.PolicyScopeID.String(),
		FeedbackRating: feedbackRating, EditedAssertionsDigest: deposition.EditedAssertionsDigest,
		CompletedAt: task.CompletedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	})
	if err != nil || evidenceDigest != deposition.EvidenceDigest {
		return ErrTwinDepositionEvidenceStale
	}
	return nil
}

func validateTwinDepositionProposalInput(input TwinDepositionProposalInput) error {
	for field, value := range map[string]pgtype.UUID{
		"workspace id": input.WorkspaceID, "task id": input.TaskID,
		"agent id": input.AgentID, "attribution id": input.AttributionID,
		"base Twin version id": input.BaseTwinVersion.ID,
		"policy scope id":      input.PolicyScopeID, "requested by id": input.RequestedByID,
	} {
		if err := requireTwinExecutionUUID(field, value); err != nil {
			return err
		}
	}
	if input.BaseTwinVersion.WorkspaceID != input.WorkspaceID || !validTwinExecutionDigest(input.BaseTwinVersion.ContentDigest) || !validTwinExecutionDigest(input.BriefingDigest) || !validTwinExecutionDigest(input.EvidenceDigest) {
		return invalidTwinExecutionInput("deposition signed evidence")
	}
	if !isOneOf(input.PolicyScopeType, "workspace", "agent", "project", "issue", "one_off") || !isOneOf(input.FeedbackRating, "", "helped", "irrelevant", "mismatch") {
		return invalidTwinExecutionInput("deposition execution context")
	}
	if len(input.EditedAssertions) > twinDepositionEditMaxBytes {
		return invalidTwinExecutionInput("edited assertions")
	}
	if input.EditedDigest != "" && !validTwinExecutionDigest(input.EditedDigest) {
		return invalidTwinExecutionInput("edited assertions digest")
	}
	if (len(input.EditedAssertions) > 0) != (input.EditedDigest != "") || len(input.EditedAssertions) > 0 && !input.ReplacesProposalID.Valid {
		return invalidTwinExecutionInput("edited assertions replacement")
	}
	if input.ReplacesProposalID.Valid && input.ReplacesProposalID.Bytes == ([16]byte{}) {
		return invalidTwinExecutionInput("replaced proposal id")
	}
	return nil
}

func buildTwinDepositionProposal(evidence TwinAcceptedEvidence, base db.TwinVersion, input TwinDepositionProposalInput) (TwinProposalBuild, error) {
	if evidence.RevisionID != base.SourceWikiRevisionID.String() {
		return TwinProposalBuild{}, ErrTwinWikiStale
	}
	evidenceSchemaVersion, _, err := canonicalTwinEvidence(TwinBuilderInput{CanonicalEvidence: evidence.CanonicalContent, Citations: evidence.Citations})
	if err != nil {
		return TwinProposalBuild{}, err
	}
	compatibility := LMWikiContent{SchemaVersion: evidenceSchemaVersion}
	if err := json.Unmarshal(evidence.CanonicalContent, &compatibility); err != nil {
		return TwinProposalBuild{}, fmt.Errorf("decode deposition evidence compatibility view: %w", err)
	}
	prior, err := twinAssertions(base.Content)
	if err != nil {
		return TwinProposalBuild{}, err
	}
	candidate, err := twinDepositionAssertions(prior, input)
	if err != nil {
		return TwinProposalBuild{}, err
	}
	return ValidateTwinProposal(TwinBuilderInput{
		SourceWikiRevisionID: evidence.RevisionID, SourceDigest: evidence.SourceDigest,
		CanonicalEvidence: append(json.RawMessage(nil), evidence.CanonicalContent...), EvidenceSchemaVersion: evidenceSchemaVersion,
		Content: compatibility, Citations: append([]LMWikiCitation(nil), evidence.Citations...), PriorAssertions: prior,
	}, TwinProposalCandidate{Assertions: candidate})
}

func twinDepositionAssertions(prior []TwinAssertion, input TwinDepositionProposalInput) ([]TwinAssertion, error) {
	if len(input.EditedAssertions) > 0 {
		_, edited, err := canonicalTwinDepositionEdit(input.EditedAssertions)
		if err != nil {
			return nil, err
		}
		return edited, nil
	}
	if len(input.AssertionIDs) == 0 {
		return nil, invalidTwinExecutionInput("deposition assertion ids")
	}
	selected := make(map[string]struct{}, len(input.AssertionIDs))
	for _, id := range input.AssertionIDs {
		if _, duplicate := selected[id]; duplicate || !validTwinExecutionText(id, twinMaxAssertionIDRunes) {
			return nil, invalidTwinExecutionInput("deposition assertion ids")
		}
		selected[id] = struct{}{}
	}
	delta := 0.01
	switch input.FeedbackRating {
	case "helped":
		delta = 0.05
	case "irrelevant":
		delta = -0.10
	case "mismatch":
		delta = -0.20
	}
	assertions := make([]TwinAssertion, len(prior))
	for index, assertion := range prior {
		assertions[index] = cloneTwinAssertion(assertion)
		if _, ok := selected[assertion.ID]; !ok {
			continue
		}
		delete(selected, assertion.ID)
		assertions[index].Confidence = math.Round(math.Max(0.05, math.Min(1, assertion.Confidence+delta))*100) / 100
		assertions[index].Provenance = TwinAssertionProvenance{Kind: TwinProvenanceDeposition, Generator: "execution-feedback-v1"}
	}
	if len(selected) != 0 {
		return nil, invalidTwinExecutionInput("unknown deposition assertion id")
	}
	return assertions, nil
}

func canonicalTwinDepositionEdit(raw json.RawMessage) (json.RawMessage, []TwinAssertion, error) {
	var edited []TwinAssertion
	if err := json.Unmarshal(raw, &edited); err != nil || edited == nil {
		return nil, nil, invalidTwinExecutionInput("edited assertions")
	}
	for index := range edited {
		edited[index].Provenance = TwinAssertionProvenance{Kind: TwinProvenanceDeposition, Generator: "human-edit-v1"}
		// Execution-owned scopes are derived from persisted run evidence, never
		// from manager-authored JSON in the review surface.
		edited[index].Applicability.TaskID = ""
		edited[index].Applicability.WorkspaceID = ""
		edited[index].Applicability.AgentID = ""
	}
	canonical, err := json.Marshal(edited)
	if err != nil {
		return nil, nil, invalidTwinExecutionInput("edited assertions")
	}
	return canonical, edited, nil
}

func cloneTwinAssertion(assertion TwinAssertion) TwinAssertion {
	assertion.Applicability.Keywords = append([]string(nil), assertion.Applicability.Keywords...)
	assertion.EvidenceCitations = append([]string(nil), assertion.EvidenceCitations...)
	return assertion
}
