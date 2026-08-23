package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	twinExecutionRequestMaxCodepoints = 16 * 1024
	twinExecutionTagMaxCount          = 64
	twinExecutionTagMaxCodepoints     = 200
	twinDepositionEditMaxBytes        = 512 * 1024
)

var (
	ErrTwinExecutionDisabled           = errors.New("twin execution is disabled")
	ErrTwinExecutionUnsupportedVersion = errors.New("twin execution requires a signed schema-v2 version")
	ErrTwinExecutionTaskNotCompleted   = errors.New("twin deposition requires a completed task")
	ErrTwinExecutionAttributionMissing = errors.New("twin task attribution is missing")
	ErrTwinDepositionUnavailable       = errors.New("twin deposition proposal generation is unavailable")
	ErrTwinDepositionEvidenceStale     = errors.New("twin deposition execution evidence is stale")
)

// TwinExecutionService is the control-plane boundary for Twin use. It owns
// policy resolution, signed-version decoding, briefing compilation, and
// privacy-safe execution evidence. HTTP and runtime adapters share this API.
type TwinExecutionService struct {
	Queries           *db.Queries
	Store             *TwinExecutionStore
	Compiler          TwinBriefingCompiler
	FeatureEnabled    bool
	DepositionCreator TwinDepositionProposalCreator
}

func NewTwinExecutionService(queries *db.Queries, featureEnabled bool) *TwinExecutionService {
	return &TwinExecutionService{
		Queries:        queries,
		Store:          NewTwinExecutionStore(queries),
		Compiler:       NewTwinBriefingCompiler(),
		FeatureEnabled: featureEnabled,
	}
}

type TwinExecutionKillSwitch struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
}

func (s *TwinExecutionService) KillSwitch() TwinExecutionKillSwitch {
	if s.FeatureEnabled {
		return TwinExecutionKillSwitch{Enabled: true}
	}
	return TwinExecutionKillSwitch{Enabled: false, Reason: "disabled_by_operator"}
}

func (s *TwinExecutionService) ListBindings(ctx context.Context, workspaceID pgtype.UUID) ([]db.TwinBinding, error) {
	return s.Store.ListBindings(ctx, workspaceID)
}

func (s *TwinExecutionService) UpsertBinding(ctx context.Context, input TwinBindingInput) (db.TwinBinding, error) {
	if !s.FeatureEnabled && input.State != string(TwinUseOff) {
		return db.TwinBinding{}, ErrTwinExecutionDisabled
	}
	return s.Store.UpsertBinding(ctx, input)
}

func (s *TwinExecutionService) DeleteBinding(ctx context.Context, workspaceID, bindingID pgtype.UUID) error {
	return s.Store.DeleteBinding(ctx, workspaceID, bindingID)
}

// TwinExecutionOneOffPolicy is an immutable task snapshot supplied by the
// queue/runtime boundary. It is deliberately not persisted as a long-lived
// binding. The task snapshot owner validates and stores it before this seam.
type TwinExecutionOneOffPolicy struct {
	ID            string
	RunID         string
	State         TwinUsePolicyState
	TwinVersionID pgtype.UUID
}

type TwinExecutionPolicyInput struct {
	WorkspaceID string
	AgentID     string
	ProjectID   string
	IssueID     string
	RunID       string
	OneOff      *TwinExecutionOneOffPolicy
}

type TwinExecutionPolicyResolution struct {
	Decision      TwinEffectiveUsePolicy
	TwinVersionID pgtype.UUID
}

// ValidateTaskUseSnapshot canonicalizes a task-level override and verifies
// that preview/enabled pins an immutable signed schema-v2 version in the same
// workspace. Callers persist the returned value on the task row; they must not
// resolve it again from mutable bindings at claim time.
func (s *TwinExecutionService) ValidateTaskUseSnapshot(ctx context.Context, workspaceID pgtype.UUID, input TwinOneOffUsePolicyOverride) (TwinOneOffUsePolicyOverride, error) {
	if !s.FeatureEnabled {
		return TwinOneOffUsePolicyOverride{}, ErrTwinExecutionDisabled
	}
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return TwinOneOffUsePolicyOverride{}, err
	}
	if !validTwinUseState(input.State) {
		return TwinOneOffUsePolicyOverride{}, invalidTwinExecutionInput("one-off policy state")
	}
	version := strings.TrimSpace(input.VersionID)
	if input.State == TwinUseOff {
		if version != "" {
			return TwinOneOffUsePolicyOverride{}, invalidTwinExecutionInput("off one-off Twin version id")
		}
		return TwinOneOffUsePolicyOverride{State: TwinUseOff}, nil
	}
	versionID, err := util.ParseUUID(version)
	if err != nil {
		return TwinOneOffUsePolicyOverride{}, invalidTwinExecutionInput("one-off Twin version id")
	}
	if _, _, err := s.loadSignedVersion(ctx, workspaceID, versionID); err != nil {
		return TwinOneOffUsePolicyOverride{}, err
	}
	return TwinOneOffUsePolicyOverride{State: input.State, VersionID: versionID.String()}, nil
}

// PrepareOneOffPreview resolves the exact immutable policy the preview
// compiler should use. A missing version for preview/enabled means "the current
// signed version now"; the returned policy pins that ID so subsequent UI state
// changes and the queued task snapshot can keep showing and using the same
// version instead of re-resolving a mutable workspace binding.
func (s *TwinExecutionService) PrepareOneOffPreview(ctx context.Context, workspaceID pgtype.UUID, runID string, state TwinUsePolicyState, versionID string) (*TwinExecutionOneOffPolicy, error) {
	if !s.FeatureEnabled {
		return nil, ErrTwinExecutionDisabled
	}
	runID = strings.TrimSpace(runID)
	if _, err := util.ParseUUID(runID); err != nil {
		return nil, invalidTwinExecutionInput("one-off preview run id")
	}
	if !validTwinUseState(state) {
		return nil, invalidTwinExecutionInput("one-off preview state")
	}
	if state != TwinUseOff && strings.TrimSpace(versionID) == "" {
		current, err := s.Queries.GetCurrentTwinVersion(ctx, workspaceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTwinExecutionNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("load current Twin version for one-off preview: %w", err)
		}
		versionID = current.ID.String()
	}
	canonical, err := s.ValidateTaskUseSnapshot(ctx, workspaceID, TwinOneOffUsePolicyOverride{State: state, VersionID: versionID})
	if err != nil {
		return nil, err
	}
	policy := &TwinExecutionOneOffPolicy{ID: "one-off-preview:" + runID, RunID: runID, State: canonical.State}
	if canonical.VersionID != "" {
		policy.TwinVersionID, err = util.ParseUUID(canonical.VersionID)
		if err != nil {
			return nil, invalidTwinExecutionInput("one-off preview Twin version id")
		}
	}
	return policy, nil
}

func (s *TwinExecutionService) ResolvePolicy(ctx context.Context, workspaceID pgtype.UUID, input TwinExecutionPolicyInput) (TwinExecutionPolicyResolution, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return TwinExecutionPolicyResolution{}, err
	}
	if input.WorkspaceID != workspaceID.String() {
		return TwinExecutionPolicyResolution{}, invalidTwinExecutionInput("policy workspace id")
	}
	rows, err := s.Store.ListBindings(ctx, workspaceID)
	if err != nil {
		return TwinExecutionPolicyResolution{}, err
	}
	bindings := make([]TwinUsePolicyBinding, 0, len(rows)+1)
	versionByBinding := make(map[string]pgtype.UUID, len(rows)+1)
	for _, row := range rows {
		id := row.ID.String()
		bindings = append(bindings, TwinUsePolicyBinding{
			ID: id, Scope: TwinUsePolicyScope(row.ScopeType), ScopeID: row.ScopeID.String(), State: TwinUsePolicyState(row.State),
		})
		versionByBinding[id] = row.TwinVersionID
	}
	if input.OneOff != nil {
		if input.OneOff.ID == "" || input.OneOff.RunID == "" || input.OneOff.RunID != input.RunID {
			return TwinExecutionPolicyResolution{}, invalidTwinExecutionInput("one-off policy run id")
		}
		if !validTwinUseState(input.OneOff.State) {
			return TwinExecutionPolicyResolution{}, invalidTwinExecutionInput("one-off policy state")
		}
		if input.OneOff.State != TwinUseOff {
			if err := requireTwinExecutionUUID("one-off Twin version id", input.OneOff.TwinVersionID); err != nil {
				return TwinExecutionPolicyResolution{}, err
			}
		}
		bindings = append(bindings, TwinUsePolicyBinding{
			ID: input.OneOff.ID, Scope: TwinUseScopeOneOff, ScopeID: input.OneOff.RunID, State: input.OneOff.State,
		})
		versionByBinding[input.OneOff.ID] = input.OneOff.TwinVersionID
	}
	decision, err := ResolveTwinUsePolicy(TwinUsePolicyContext{
		WorkspaceID: input.WorkspaceID,
		AgentID:     input.AgentID,
		ProjectID:   input.ProjectID,
		IssueID:     input.IssueID,
		RunID:       input.RunID,
	}, bindings)
	if err != nil {
		return TwinExecutionPolicyResolution{}, err
	}
	return TwinExecutionPolicyResolution{Decision: decision, TwinVersionID: versionByBinding[decision.BindingID]}, nil
}

type TwinExecutionBriefingInput struct {
	Task   TwinTaskEligibility
	OneOff *TwinExecutionOneOffPolicy
}

func (s *TwinExecutionService) CompileBriefing(ctx context.Context, workspaceID pgtype.UUID, input TwinExecutionBriefingInput) (TwinCompiledBriefing, error) {
	preview, err := s.CompileBriefingPreview(ctx, workspaceID, input)
	return preview.Briefing, err
}

type TwinExecutionVersionReference struct {
	ID            string `json:"id"`
	VersionNumber int64  `json:"version_number"`
	ContentDigest string `json:"content_digest"`
}

type TwinExecutionBriefingPreview struct {
	Briefing TwinCompiledBriefing
	Version  *TwinExecutionVersionReference
}

func (s *TwinExecutionService) CompileBriefingPreview(ctx context.Context, workspaceID pgtype.UUID, input TwinExecutionBriefingInput) (TwinExecutionBriefingPreview, error) {
	if !s.FeatureEnabled {
		return TwinExecutionBriefingPreview{}, ErrTwinExecutionDisabled
	}
	task, err := s.validateTaskEligibility(ctx, workspaceID, input.Task)
	if err != nil {
		return TwinExecutionBriefingPreview{}, err
	}
	policy, err := s.ResolvePolicy(ctx, workspaceID, TwinExecutionPolicyInput{
		WorkspaceID: task.WorkspaceID,
		AgentID:     task.AgentID,
		ProjectID:   task.ProjectID,
		IssueID:     task.IssueID,
		RunID:       task.RunID,
		OneOff:      input.OneOff,
	})
	if err != nil {
		return TwinExecutionBriefingPreview{}, err
	}
	envelope := TwinSignedAssertionEnvelope{}
	var reference *TwinExecutionVersionReference
	if policy.Decision.State != TwinUseOff {
		if err := requireTwinExecutionUUID("effective Twin version id", policy.TwinVersionID); err != nil {
			return TwinExecutionBriefingPreview{}, err
		}
		var version db.TwinVersion
		envelope, version, err = s.loadSignedVersion(ctx, workspaceID, policy.TwinVersionID)
		if err != nil {
			return TwinExecutionBriefingPreview{}, err
		}
		reference = &TwinExecutionVersionReference{ID: version.ID.String(), VersionNumber: version.VersionNumber, ContentDigest: version.ContentDigest}
	}
	compiled, err := s.Compiler.Compile(TwinBriefingInput{Task: task, Version: envelope, Policy: policy.Decision})
	if err != nil {
		return TwinExecutionBriefingPreview{}, err
	}
	return TwinExecutionBriefingPreview{Briefing: compiled, Version: reference}, nil
}

func (s *TwinExecutionService) validateTaskEligibility(ctx context.Context, workspaceID pgtype.UUID, task TwinTaskEligibility) (TwinTaskEligibility, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return TwinTaskEligibility{}, err
	}
	if task.WorkspaceID != workspaceID.String() {
		return TwinTaskEligibility{}, invalidTwinExecutionInput("task workspace id")
	}
	if task.TaskID == "" || !validTwinExecutionText(task.TaskID, 200) {
		return TwinTaskEligibility{}, invalidTwinExecutionInput("task id")
	}
	if !utf8.ValidString(task.Request) || strings.IndexByte(task.Request, 0) >= 0 || utf8.RuneCountInString(task.Request) > twinExecutionRequestMaxCodepoints {
		return TwinTaskEligibility{}, invalidTwinExecutionInput("task request")
	}
	if len(task.Tags) > twinExecutionTagMaxCount {
		return TwinTaskEligibility{}, invalidTwinExecutionInput("task tags")
	}
	for _, tag := range task.Tags {
		if !utf8.ValidString(tag) || strings.IndexByte(tag, 0) >= 0 || utf8.RuneCountInString(tag) > twinExecutionTagMaxCodepoints {
			return TwinTaskEligibility{}, invalidTwinExecutionInput("task tag")
		}
	}
	agentID, err := util.ParseUUID(task.AgentID)
	if err != nil {
		return TwinTaskEligibility{}, invalidTwinExecutionInput("task agent id")
	}
	if _, err := s.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: workspaceID}); errors.Is(err, pgx.ErrNoRows) {
		return TwinTaskEligibility{}, ErrTwinExecutionNotFound
	} else if err != nil {
		return TwinTaskEligibility{}, fmt.Errorf("load Twin task agent: %w", err)
	}
	var projectID pgtype.UUID
	if task.ProjectID != "" {
		projectID, err = util.ParseUUID(task.ProjectID)
		if err != nil {
			return TwinTaskEligibility{}, invalidTwinExecutionInput("task project id")
		}
		if _, err := s.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: projectID, WorkspaceID: workspaceID}); errors.Is(err, pgx.ErrNoRows) {
			return TwinTaskEligibility{}, ErrTwinExecutionNotFound
		} else if err != nil {
			return TwinTaskEligibility{}, fmt.Errorf("load Twin task project: %w", err)
		}
	}
	if task.IssueID != "" {
		issueID, err := util.ParseUUID(task.IssueID)
		if err != nil {
			return TwinTaskEligibility{}, invalidTwinExecutionInput("task issue id")
		}
		issue, err := s.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: workspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return TwinTaskEligibility{}, ErrTwinExecutionNotFound
		}
		if err != nil {
			return TwinTaskEligibility{}, fmt.Errorf("load Twin task issue: %w", err)
		}
		if issue.ProjectID.Valid {
			if projectID.Valid && projectID != issue.ProjectID {
				return TwinTaskEligibility{}, invalidTwinExecutionInput("task issue project")
			}
			task.ProjectID = issue.ProjectID.String()
		} else if projectID.Valid {
			return TwinTaskEligibility{}, invalidTwinExecutionInput("task issue project")
		}
	}
	if task.RunID != "" {
		if _, err := util.ParseUUID(task.RunID); err != nil {
			return TwinTaskEligibility{}, invalidTwinExecutionInput("task run id")
		}
	}
	task.Authorized = true
	return task, nil
}

func (s *TwinExecutionService) loadSignedVersion(ctx context.Context, workspaceID, versionID pgtype.UUID) (TwinSignedAssertionEnvelope, db.TwinVersion, error) {
	version, err := s.Queries.GetTwinVersion(ctx, db.GetTwinVersionParams{WorkspaceID: workspaceID, ID: versionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinSignedAssertionEnvelope{}, db.TwinVersion{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return TwinSignedAssertionEnvelope{}, db.TwinVersion{}, fmt.Errorf("load signed Twin version: %w", err)
	}
	if version.SchemaVersion != 2 {
		return TwinSignedAssertionEnvelope{}, db.TwinVersion{}, ErrTwinExecutionUnsupportedVersion
	}
	// content is JSONB, so PostgreSQL may change whitespace and object-key order
	// on the round trip. Re-marshal the complete frozen schema before checking
	// the digest that was signed at proposal creation.
	var content TwinProposalContent
	if err := json.Unmarshal(version.Content, &content); err != nil || content.SchemaVersion != 2 || content.Assertions == nil {
		return TwinSignedAssertionEnvelope{}, db.TwinVersion{}, ErrTwinExecutionUnsupportedVersion
	}
	canonical, err := json.Marshal(content)
	if err != nil || digestTwin(canonical) != version.ContentDigest {
		return TwinSignedAssertionEnvelope{}, db.TwinVersion{}, ErrTwinExecutionUnsupportedVersion
	}
	assertions := make([]TwinBriefingAssertion, len(content.Assertions))
	for index, assertion := range content.Assertions {
		assertions[index] = TwinBriefingAssertion{
			ID: assertion.ID, Lifecycle: TwinAssertionSigned, Type: assertion.Type,
			Text: assertion.Text, CitationIDs: append([]string(nil), assertion.EvidenceCitations...),
			Applicability: TwinAssertionApplicability{
				TaskID: assertion.Applicability.TaskID, WorkspaceID: assertion.Applicability.WorkspaceID,
				AgentID: assertion.Applicability.AgentID, ProjectID: assertion.Applicability.ProjectID,
				IssueID: assertion.Applicability.IssueID, Keywords: append([]string(nil), assertion.Applicability.Keywords...),
			},
		}
	}
	return TwinSignedAssertionEnvelope{
		VersionID: version.ID.String(), SignatureDigest: version.ContentDigest,
		Lifecycle: TwinVersionSigned, Authorized: true, Assertions: assertions,
	}, version, nil
}

type TwinExecutionAssertion struct {
	ID            string                     `json:"id"`
	Type          TwinAssertionType          `json:"type"`
	Text          string                     `json:"text"`
	Applicability TwinAssertionApplicability `json:"applicability"`
	CitationKeys  []string                   `json:"citation_keys"`
}

type TwinExecutionAttribution struct {
	ID               string                   `json:"id"`
	TaskID           string                   `json:"task_id"`
	TwinVersionID    string                   `json:"twin_version_id"`
	VersionNumber    int64                    `json:"version_number"`
	VersionDigest    string                   `json:"version_digest"`
	Briefing         string                   `json:"briefing"`
	BriefingDigest   string                   `json:"briefing_digest"`
	ByteCount        int                      `json:"byte_count"`
	TokenCount       int                      `json:"token_count"`
	Assertions       []TwinExecutionAssertion `json:"assertions"`
	CitationKeys     []string                 `json:"citation_keys"`
	PolicyScopeType  string                   `json:"policy_scope_type"`
	PolicyScopeID    string                   `json:"policy_scope_id"`
	PolicyState      string                   `json:"policy_state"`
	CompilerVersion  string                   `json:"compiler_version"`
	CreatedAt        pgtype.Timestamptz       `json:"created_at"`
	SourceRevisionID pgtype.UUID              `json:"-"`
}

type TwinExecutionCitation struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	SourceType string `json:"source_type"`
	Locator    string `json:"locator"`
}

type TwinExecutionTaskContext struct {
	TaskID      string                    `json:"task_id"`
	Attribution *TwinExecutionAttribution `json:"attribution"`
	Feedback    *db.TwinRunFeedback       `json:"feedback"`
	Depositions []db.TwinDeposition       `json:"depositions"`
	Assertions  []TwinExecutionAssertion  `json:"assertions"`
	Citations   []TwinExecutionCitation   `json:"citations"`
}

func (s *TwinExecutionService) GetTaskContext(ctx context.Context, workspaceID, taskID pgtype.UUID) (TwinExecutionTaskContext, error) {
	task, err := s.Queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: taskID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinExecutionTaskContext{}, ErrTwinExecutionNotFound
	} else if err != nil {
		return TwinExecutionTaskContext{}, fmt.Errorf("load Twin task: %w", err)
	}
	attributions, err := s.Store.ListTaskAttributions(ctx, workspaceID, taskID)
	if err != nil {
		return TwinExecutionTaskContext{}, err
	}
	depositions, err := s.Store.ListDepositionsForTask(ctx, workspaceID, taskID)
	if err != nil {
		return TwinExecutionTaskContext{}, err
	}
	result := TwinExecutionTaskContext{TaskID: taskID.String(), Depositions: depositions, Assertions: []TwinExecutionAssertion{}, Citations: []TwinExecutionCitation{}}
	if result.Depositions == nil {
		result.Depositions = []db.TwinDeposition{}
	}
	feedback, err := s.Store.GetRunFeedback(ctx, workspaceID, taskID)
	if err == nil {
		result.Feedback = &feedback
	} else if !errors.Is(err, ErrTwinExecutionNotFound) {
		return TwinExecutionTaskContext{}, err
	}
	if len(attributions) == 0 {
		return result, nil
	}
	mapped, ok := s.mapTaskAttribution(ctx, workspaceID, task, attributions[0])
	if ok {
		result.Assertions = append([]TwinExecutionAssertion(nil), mapped.Assertions...)
		citationRows, err := s.Queries.ListLMWikiCitations(ctx, db.ListLMWikiCitationsParams{WorkspaceID: workspaceID, RevisionID: mapped.SourceRevisionID})
		if err != nil {
			return TwinExecutionTaskContext{}, fmt.Errorf("load Twin execution citations: %w", err)
		}
		selected := make(map[string]struct{}, len(mapped.CitationKeys))
		for _, key := range mapped.CitationKeys {
			selected[key] = struct{}{}
		}
		for _, citation := range citationRows {
			if _, exists := selected[citation.CitationKey]; !exists {
				continue
			}
			result.Citations = append(result.Citations, TwinExecutionCitation{
				Key: citation.CitationKey, Label: citation.Label, SourceType: citation.SourceType, Locator: citation.Locator,
			})
		}
		if len(result.Citations) != len(selected) {
			return TwinExecutionTaskContext{TaskID: taskID.String(), Feedback: result.Feedback, Depositions: result.Depositions, Assertions: []TwinExecutionAssertion{}, Citations: []TwinExecutionCitation{}}, nil
		}
		sort.Slice(result.Citations, func(i, j int) bool { return result.Citations[i].Key < result.Citations[j].Key })
		result.Attribution = &mapped
	}
	return result, nil
}

func (s *TwinExecutionService) mapTaskAttribution(ctx context.Context, workspaceID pgtype.UUID, task db.AgentTaskQueue, row db.TwinTaskAttribution) (TwinExecutionAttribution, bool) {
	if row.WorkspaceID != workspaceID || row.TaskID != task.ID ||
		row.AgentID != task.AgentID || row.RuntimeID != task.RuntimeID ||
		!row.TaskDispatchedAt.Valid || !task.DispatchedAt.Valid || !row.TaskDispatchedAt.Time.Equal(task.DispatchedAt.Time) ||
		!row.PolicyScopeID.Valid ||
		!isOneOf(row.PolicyScopeType, "workspace", "agent", "project", "issue", "one_off") ||
		row.BriefingDigest != TwinBriefingDigest(row.Briefing) ||
		!validTwinExecutionText(row.CompilerVersion, twinCompilerVersionMaxBytes) ||
		row.PolicyState != string(TwinUseEnabled) {
		return TwinExecutionAttribution{}, false
	}
	switch row.PolicyScopeType {
	case "workspace":
		if row.PolicyScopeID != workspaceID {
			return TwinExecutionAttribution{}, false
		}
	case "agent":
		if row.PolicyScopeID != task.AgentID {
			return TwinExecutionAttribution{}, false
		}
	case "issue":
		if !task.IssueID.Valid || row.PolicyScopeID != task.IssueID {
			return TwinExecutionAttribution{}, false
		}
	case "one_off":
		if row.PolicyScopeID != task.ID {
			return TwinExecutionAttribution{}, false
		}
	}
	assertionIDs, ok := decodeTwinExecutionStringList(row.AssertionIds, twinAssertionIDMaxCount, twinAssertionIDMaxBytes)
	if !ok {
		return TwinExecutionAttribution{}, false
	}
	citationKeys, ok := decodeTwinExecutionStringList(row.CitationKeys, twinCitationKeyMaxCount, twinCitationKeyMaxBytes)
	if !ok {
		return TwinExecutionAttribution{}, false
	}
	envelope, version, err := s.loadSignedVersion(ctx, workspaceID, row.TwinVersionID)
	if err != nil {
		return TwinExecutionAttribution{}, false
	}
	byID := make(map[string]TwinBriefingAssertion, len(envelope.Assertions))
	for _, assertion := range envelope.Assertions {
		byID[assertion.ID] = assertion
	}
	if len(assertionIDs) == 0 || len(citationKeys) == 0 {
		return TwinExecutionAttribution{}, false
	}
	assertions := make([]TwinExecutionAssertion, 0, len(assertionIDs))
	selectedCitations := make(map[string]struct{}, len(citationKeys))
	for _, id := range assertionIDs {
		assertion, exists := byID[id]
		if !exists {
			return TwinExecutionAttribution{}, false
		}
		assertions = append(assertions, TwinExecutionAssertion{
			ID: id, Type: assertion.Type, Text: assertion.Text,
			Applicability: assertion.Applicability,
			CitationKeys:  append([]string(nil), assertion.CitationIDs...),
		})
		for _, citationKey := range assertion.CitationIDs {
			selectedCitations[citationKey] = struct{}{}
		}
	}
	if len(selectedCitations) != len(citationKeys) {
		return TwinExecutionAttribution{}, false
	}
	for _, citationKey := range citationKeys {
		if _, exists := selectedCitations[citationKey]; !exists {
			return TwinExecutionAttribution{}, false
		}
	}
	return TwinExecutionAttribution{
		ID: row.ID.String(), TaskID: task.ID.String(), TwinVersionID: version.ID.String(),
		VersionNumber: version.VersionNumber, VersionDigest: version.ContentDigest,
		Briefing: row.Briefing, BriefingDigest: row.BriefingDigest,
		ByteCount: len([]byte(row.Briefing)), TokenCount: estimateTwinBriefingTokens(row.Briefing),
		Assertions: assertions, CitationKeys: citationKeys,
		PolicyScopeType: row.PolicyScopeType, PolicyScopeID: row.PolicyScopeID.String(),
		PolicyState: row.PolicyState, CompilerVersion: row.CompilerVersion, CreatedAt: row.CreatedAt,
		SourceRevisionID: version.SourceWikiRevisionID,
	}, true
}

func decodeTwinExecutionStringList(raw []byte, maxCount, maxItemBytes int) ([]string, bool) {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil || len(values) > maxCount {
		return nil, false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validTwinExecutionText(value, maxItemBytes) {
			return nil, false
		}
		if _, exists := seen[value]; exists {
			return nil, false
		}
		seen[value] = struct{}{}
	}
	return values, true
}

func (s *TwinExecutionService) UpsertFeedback(ctx context.Context, input TwinRunFeedbackInput) (db.TwinRunFeedback, error) {
	return s.Store.UpsertRunFeedback(ctx, input)
}

type TwinExecutionMetricsResponse struct {
	AttributedRuns  int64                          `json:"attributed_runs"`
	Feedback        TwinExecutionFeedbackMetrics   `json:"feedback"`
	Depositions     TwinExecutionDepositionMetrics `json:"depositions"`
	Bindings        TwinExecutionBindingMetrics    `json:"bindings"`
	HelpfulnessRate *float64                       `json:"helpfulness_rate"`
	KillSwitch      TwinExecutionKillSwitch        `json:"kill_switch"`
}

type TwinExecutionFeedbackMetrics struct {
	Total      int64 `json:"total"`
	Helped     int64 `json:"helped"`
	Irrelevant int64 `json:"irrelevant"`
	Mismatch   int64 `json:"mismatch"`
}

type TwinExecutionDepositionMetrics struct {
	Total    int64 `json:"total"`
	Pending  int64 `json:"pending"`
	Accepted int64 `json:"accepted"`
	Rejected int64 `json:"rejected"`
}

type TwinExecutionBindingMetrics struct {
	Off     int64 `json:"off"`
	Preview int64 `json:"preview"`
	Enabled int64 `json:"enabled"`
}

func (s *TwinExecutionService) GetMetrics(ctx context.Context, workspaceID pgtype.UUID) (TwinExecutionMetricsResponse, error) {
	metrics, err := s.Store.GetMetrics(ctx, workspaceID)
	if err != nil {
		return TwinExecutionMetricsResponse{}, err
	}
	var rate *float64
	if metrics.FeedbackTotal > 0 {
		value := float64(metrics.FeedbackHelped) / float64(metrics.FeedbackTotal)
		rate = &value
	}
	return TwinExecutionMetricsResponse{
		AttributedRuns:  metrics.AttributedRuns,
		Feedback:        TwinExecutionFeedbackMetrics{Total: metrics.FeedbackTotal, Helped: metrics.FeedbackHelped, Irrelevant: metrics.FeedbackIrrelevant, Mismatch: metrics.FeedbackMismatch},
		Depositions:     TwinExecutionDepositionMetrics{Total: metrics.DepositionsTotal, Pending: metrics.DepositionsPending, Accepted: metrics.DepositionsAccepted, Rejected: metrics.DepositionsRejected},
		Bindings:        TwinExecutionBindingMetrics{Off: metrics.BindingsOff, Preview: metrics.BindingsPreview, Enabled: metrics.BindingsEnabled},
		HelpfulnessRate: rate, KillSwitch: s.KillSwitch(),
	}, nil
}

type TwinDepositionRequest struct {
	RequestedByID      pgtype.UUID
	ReplacesProposalID pgtype.UUID
	EditedAssertions   json.RawMessage
}

type TwinDepositionProposalInput struct {
	WorkspaceID        pgtype.UUID
	TaskID             pgtype.UUID
	AgentID            pgtype.UUID
	AttributionID      pgtype.UUID
	BaseTwinVersion    db.TwinVersion
	BriefingDigest     string
	AssertionIDs       []string
	CitationKeys       []string
	PolicyScopeType    string
	PolicyScopeID      pgtype.UUID
	FeedbackRating     string
	EvidenceDigest     string
	RequestedByID      pgtype.UUID
	ReplacesProposalID pgtype.UUID
	EditedAssertions   json.RawMessage
	EditedDigest       string
}

// TwinDepositionProposalCreator owns validated append-only proposal creation.
// Production implementations may use a model, but receive only sanitized
// metadata: raw task results, errors, paths, prompts, and credentials are not
// part of this interface.
type TwinDepositionProposalCreator interface {
	CreateDepositionProposal(context.Context, TwinDepositionProposalInput) (TwinDepositionResult, error)
}

type TwinDepositionResult struct {
	Created    bool
	Proposal   db.TwinProposal
	Deposition db.TwinDeposition
}

func (s *TwinExecutionService) CreateDeposition(ctx context.Context, workspaceID, taskID pgtype.UUID, request TwinDepositionRequest) (TwinDepositionResult, error) {
	if !s.FeatureEnabled {
		return TwinDepositionResult{}, ErrTwinExecutionDisabled
	}
	if err := requireTwinExecutionUUID("requested by id", request.RequestedByID); err != nil {
		return TwinDepositionResult{}, err
	}
	if len(request.EditedAssertions) > twinDepositionEditMaxBytes {
		return TwinDepositionResult{}, invalidTwinExecutionInput("edited assertions")
	}
	editedAssertionsDigest := ""
	if len(request.EditedAssertions) > 0 {
		canonical, _, err := canonicalTwinDepositionEdit(request.EditedAssertions)
		if err != nil {
			return TwinDepositionResult{}, err
		}
		request.EditedAssertions = canonical
		editedAssertionsDigest = digestTwin(canonical)
	}
	if len(request.EditedAssertions) > 0 && !request.ReplacesProposalID.Valid {
		return TwinDepositionResult{}, invalidTwinExecutionInput("edited assertions replacement")
	}
	task, err := s.Queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: taskID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinDepositionResult{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return TwinDepositionResult{}, fmt.Errorf("load deposition task: %w", err)
	}
	if task.Status != "completed" || !task.CompletedAt.Valid {
		return TwinDepositionResult{}, ErrTwinExecutionTaskNotCompleted
	}
	attributions, err := s.Store.ListTaskAttributions(ctx, workspaceID, taskID)
	if err != nil {
		return TwinDepositionResult{}, err
	}
	if len(attributions) == 0 {
		return TwinDepositionResult{}, ErrTwinExecutionAttributionMissing
	}
	depositions, err := s.Store.ListDepositionsForTask(ctx, workspaceID, taskID)
	if err != nil {
		return TwinDepositionResult{}, err
	}
	if !request.ReplacesProposalID.Valid && len(request.EditedAssertions) == 0 && len(depositions) > 0 {
		proposal, err := s.Queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: depositions[0].ProposalID})
		if err != nil {
			return TwinDepositionResult{}, fmt.Errorf("load repeated deposition proposal: %w", err)
		}
		return TwinDepositionResult{Proposal: proposal, Deposition: depositions[0]}, nil
	}
	if request.ReplacesProposalID.Valid {
		found := false
		for _, deposition := range depositions {
			if deposition.ProposalID == request.ReplacesProposalID {
				found = true
				break
			}
		}
		if !found {
			return TwinDepositionResult{}, ErrTwinExecutionConflict
		}
	}
	attribution := attributions[0]
	mappedAttribution, ok := s.mapTaskAttribution(ctx, workspaceID, task, attribution)
	if !ok {
		return TwinDepositionResult{}, ErrTwinExecutionConflict
	}
	assertionIDs := make([]string, len(mappedAttribution.Assertions))
	for index, assertion := range mappedAttribution.Assertions {
		assertionIDs[index] = assertion.ID
	}
	citationKeys := append([]string(nil), mappedAttribution.CitationKeys...)
	_, baseVersion, err := s.loadSignedVersion(ctx, workspaceID, attribution.TwinVersionID)
	if err != nil {
		return TwinDepositionResult{}, err
	}
	feedbackRating := ""
	if feedback, err := s.Store.GetRunFeedback(ctx, workspaceID, taskID); err == nil {
		feedbackRating = feedback.Rating
	} else if !errors.Is(err, ErrTwinExecutionNotFound) {
		return TwinDepositionResult{}, err
	}
	evidenceDigest, err := twinDepositionEvidenceDigest(twinDepositionEvidence{
		TaskID: taskID.String(), AttributionID: attribution.ID.String(), TwinVersionID: baseVersion.ID.String(),
		BriefingDigest: attribution.BriefingDigest, AssertionIDs: assertionIDs, CitationKeys: citationKeys,
		PolicyScopeType: attribution.PolicyScopeType, PolicyScopeID: attribution.PolicyScopeID.String(),
		FeedbackRating: feedbackRating, EditedAssertionsDigest: editedAssertionsDigest,
		CompletedAt: task.CompletedAt.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	})
	if err != nil {
		return TwinDepositionResult{}, err
	}
	if request.ReplacesProposalID.Valid {
		replacedState := ""
		for _, deposition := range depositions {
			if deposition.ReplacesProposalID == request.ReplacesProposalID && deposition.EvidenceDigest == evidenceDigest {
				proposal, err := s.Queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: deposition.ProposalID})
				if err != nil {
					return TwinDepositionResult{}, fmt.Errorf("load repeated replacement proposal: %w", err)
				}
				return TwinDepositionResult{Proposal: proposal, Deposition: deposition}, nil
			}
			if deposition.ProposalID == request.ReplacesProposalID {
				replacedState = deposition.State
			}
		}
		if replacedState != "pending" {
			return TwinDepositionResult{}, ErrTwinExecutionConflict
		}
	}
	if s.DepositionCreator == nil {
		return TwinDepositionResult{}, ErrTwinDepositionUnavailable
	}
	result, err := s.DepositionCreator.CreateDepositionProposal(ctx, TwinDepositionProposalInput{
		WorkspaceID: workspaceID, TaskID: taskID, AgentID: task.AgentID,
		AttributionID: attribution.ID, BaseTwinVersion: baseVersion,
		BriefingDigest: attribution.BriefingDigest, AssertionIDs: assertionIDs,
		CitationKeys: citationKeys, PolicyScopeType: attribution.PolicyScopeType,
		PolicyScopeID: attribution.PolicyScopeID, FeedbackRating: feedbackRating,
		EvidenceDigest: evidenceDigest, RequestedByID: request.RequestedByID,
		ReplacesProposalID: request.ReplacesProposalID,
		EditedAssertions:   append(json.RawMessage(nil), request.EditedAssertions...),
		EditedDigest:       editedAssertionsDigest,
	})
	if err != nil {
		return TwinDepositionResult{}, err
	}
	if result.Proposal.WorkspaceID != workspaceID || result.Proposal.Kind != "deposition" || result.Proposal.SchemaVersion != 2 || result.Proposal.BaseTwinVersionID != baseVersion.ID ||
		result.Deposition.WorkspaceID != workspaceID || result.Deposition.TaskID != taskID || result.Deposition.ProposalID != result.Proposal.ID {
		return TwinDepositionResult{}, ErrTwinExecutionConflict
	}
	return result, nil
}

type twinDepositionEvidence struct {
	SchemaVersion          int      `json:"schema_version"`
	TaskID                 string   `json:"task_id"`
	AttributionID          string   `json:"attribution_id"`
	TwinVersionID          string   `json:"twin_version_id"`
	BriefingDigest         string   `json:"briefing_digest"`
	AssertionIDs           []string `json:"assertion_ids"`
	CitationKeys           []string `json:"citation_keys"`
	PolicyScopeType        string   `json:"policy_scope_type"`
	PolicyScopeID          string   `json:"policy_scope_id"`
	FeedbackRating         string   `json:"feedback_rating,omitempty"`
	EditedAssertionsDigest string   `json:"edited_assertions_digest,omitempty"`
	CompletedAt            string   `json:"completed_at"`
}

func twinDepositionEvidenceDigest(evidence twinDepositionEvidence) (string, error) {
	evidence.SchemaVersion = 1
	evidence.AssertionIDs = append([]string(nil), evidence.AssertionIDs...)
	evidence.CitationKeys = append([]string(nil), evidence.CitationKeys...)
	sort.Strings(evidence.AssertionIDs)
	sort.Strings(evidence.CitationKeys)
	canonical, err := json.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("marshal Twin deposition evidence: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
