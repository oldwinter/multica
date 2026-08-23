package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	TwinBriefingCompilerVersion = "twin-briefing/v1"
	TwinBriefingMaxBytes        = 8 * 1024
	TwinBriefingMaxTokens       = 2 * 1024
)

var (
	ErrTwinUsePolicyInvalid     = errors.New("invalid twin use policy")
	ErrTwinBriefingInvalidInput = errors.New("invalid twin briefing input")
)

type TwinUsePolicyState string

const (
	TwinUseOff     TwinUsePolicyState = "off"
	TwinUsePreview TwinUsePolicyState = "preview"
	TwinUseEnabled TwinUsePolicyState = "enabled"
)

type TwinUsePolicyScope string

const (
	TwinUseScopeWorkspace TwinUsePolicyScope = "workspace"
	TwinUseScopeAgent     TwinUsePolicyScope = "agent"
	TwinUseScopeProject   TwinUsePolicyScope = "project"
	TwinUseScopeIssue     TwinUsePolicyScope = "issue"
	TwinUseScopeOneOff    TwinUsePolicyScope = "one_off"
)

type TwinUsePolicyContext struct {
	WorkspaceID string
	AgentID     string
	ProjectID   string
	IssueID     string
	RunID       string
}

type TwinUsePolicyBinding struct {
	ID      string
	Scope   TwinUsePolicyScope
	ScopeID string
	State   TwinUsePolicyState
}

type TwinPolicyDecisionReason string

const (
	TwinPolicyExplicitBinding   TwinPolicyDecisionReason = "explicit_binding"
	TwinPolicyNoExplicitBinding TwinPolicyDecisionReason = "no_explicit_binding"
)

type TwinPolicyBindingExclusionCode string

const (
	TwinPolicyBindingNotApplicable TwinPolicyBindingExclusionCode = "not_applicable"
	TwinPolicyBindingShadowed      TwinPolicyBindingExclusionCode = "shadowed_by_more_specific_binding"
)

type TwinPolicyBindingExclusion struct {
	BindingID string
	Scope     TwinUsePolicyScope
	Code      TwinPolicyBindingExclusionCode
}

type TwinEffectiveUsePolicy struct {
	State      TwinUsePolicyState
	Scope      TwinUsePolicyScope
	ScopeID    string
	BindingID  string
	Explicit   bool
	Reason     TwinPolicyDecisionReason
	Exclusions []TwinPolicyBindingExclusion
}

type TwinUsePolicyError struct {
	Field string
}

func (e *TwinUsePolicyError) Error() string {
	return "invalid twin use policy: " + e.Field
}

func (e *TwinUsePolicyError) Unwrap() error {
	return ErrTwinUsePolicyInvalid
}

// ResolveTwinUsePolicy resolves only explicit bindings and defaults to off.
// Its precedence is one-off, Issue, Project, Agent, then workspace.
func ResolveTwinUsePolicy(ctx TwinUsePolicyContext, bindings []TwinUsePolicyBinding) (TwinEffectiveUsePolicy, error) {
	if ctx.WorkspaceID == "" {
		return TwinEffectiveUsePolicy{}, &TwinUsePolicyError{Field: "workspace id"}
	}
	ordered := append([]TwinUsePolicyBinding(nil), bindings...)
	sort.Slice(ordered, func(i, j int) bool {
		left, right := twinPolicyScopeRank(ordered[i].Scope), twinPolicyScopeRank(ordered[j].Scope)
		if left != right {
			return left > right
		}
		if ordered[i].ScopeID != ordered[j].ScopeID {
			return ordered[i].ScopeID < ordered[j].ScopeID
		}
		return ordered[i].ID < ordered[j].ID
	})

	seen := make(map[string]struct{}, len(ordered))
	for _, binding := range ordered {
		if binding.ID == "" {
			return TwinEffectiveUsePolicy{}, &TwinUsePolicyError{Field: "binding id"}
		}
		if binding.ScopeID == "" {
			return TwinEffectiveUsePolicy{}, &TwinUsePolicyError{Field: "binding scope id"}
		}
		if twinPolicyScopeRank(binding.Scope) == 0 {
			return TwinEffectiveUsePolicy{}, &TwinUsePolicyError{Field: "binding scope"}
		}
		if !validTwinUseState(binding.State) {
			return TwinEffectiveUsePolicy{}, &TwinUsePolicyError{Field: "binding state"}
		}
		key := string(binding.Scope) + "\x00" + binding.ScopeID
		if _, exists := seen[key]; exists {
			return TwinEffectiveUsePolicy{}, &TwinUsePolicyError{Field: "duplicate binding for scope"}
		}
		seen[key] = struct{}{}
	}

	decision := TwinEffectiveUsePolicy{
		State:      TwinUseOff,
		Reason:     TwinPolicyNoExplicitBinding,
		Exclusions: make([]TwinPolicyBindingExclusion, 0, len(ordered)),
	}
	var selected *TwinUsePolicyBinding
	for i := range ordered {
		binding := ordered[i]
		if !twinPolicyBindingApplies(ctx, binding) {
			decision.Exclusions = append(decision.Exclusions, TwinPolicyBindingExclusion{
				BindingID: binding.ID,
				Scope:     binding.Scope,
				Code:      TwinPolicyBindingNotApplicable,
			})
			continue
		}
		if selected == nil {
			selected = &ordered[i]
			decision.State = binding.State
			decision.Scope = binding.Scope
			decision.ScopeID = binding.ScopeID
			decision.BindingID = binding.ID
			decision.Explicit = true
			decision.Reason = TwinPolicyExplicitBinding
			continue
		}
		decision.Exclusions = append(decision.Exclusions, TwinPolicyBindingExclusion{
			BindingID: binding.ID,
			Scope:     binding.Scope,
			Code:      TwinPolicyBindingShadowed,
		})
	}
	return decision, nil
}

func twinPolicyScopeRank(scope TwinUsePolicyScope) int {
	switch scope {
	case TwinUseScopeWorkspace:
		return 1
	case TwinUseScopeAgent:
		return 2
	case TwinUseScopeProject:
		return 3
	case TwinUseScopeIssue:
		return 4
	case TwinUseScopeOneOff:
		return 5
	default:
		return 0
	}
}

func twinPolicyBindingApplies(ctx TwinUsePolicyContext, binding TwinUsePolicyBinding) bool {
	switch binding.Scope {
	case TwinUseScopeWorkspace:
		return binding.ScopeID == ctx.WorkspaceID
	case TwinUseScopeAgent:
		return ctx.AgentID != "" && binding.ScopeID == ctx.AgentID
	case TwinUseScopeProject:
		return ctx.ProjectID != "" && binding.ScopeID == ctx.ProjectID
	case TwinUseScopeIssue:
		return ctx.IssueID != "" && binding.ScopeID == ctx.IssueID
	case TwinUseScopeOneOff:
		return ctx.RunID != "" && binding.ScopeID == ctx.RunID
	default:
		return false
	}
}

func validTwinUseState(state TwinUsePolicyState) bool {
	return state == TwinUseOff || state == TwinUsePreview || state == TwinUseEnabled
}

type TwinTaskEligibility struct {
	TaskID      string
	WorkspaceID string
	AgentID     string
	ProjectID   string
	IssueID     string
	RunID       string
	Request     string
	Tags        []string
	Eligible    bool
	Authorized  bool
	LocalOnly   bool
}

type TwinVersionLifecycle string

const (
	TwinVersionDraft    TwinVersionLifecycle = "draft"
	TwinVersionProposal TwinVersionLifecycle = "proposal"
	TwinVersionSigned   TwinVersionLifecycle = "signed"
)

type TwinAssertionLifecycle string

const (
	TwinAssertionDraft    TwinAssertionLifecycle = "draft"
	TwinAssertionProposal TwinAssertionLifecycle = "proposal"
	TwinAssertionSigned   TwinAssertionLifecycle = "signed"
)

type TwinAssertionApplicability struct {
	TaskID      string   `json:"task_id,omitempty"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
	AgentID     string   `json:"agent_id,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	IssueID     string   `json:"issue_id,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

type TwinBriefingAssertion struct {
	ID            string
	Lifecycle     TwinAssertionLifecycle
	Type          TwinAssertionType
	Text          string
	CitationIDs   []string
	Applicability TwinAssertionApplicability
}

// TwinSignedAssertionEnvelope intentionally carries assertions and citation
// identifiers only. Mutable proposals and raw evidence do not belong at this
// boundary.
type TwinSignedAssertionEnvelope struct {
	VersionID       string
	SignatureDigest string
	Lifecycle       TwinVersionLifecycle
	Stale           bool
	Authorized      bool
	LocalOnly       bool
	Assertions      []TwinBriefingAssertion
}

type TwinBriefingInput struct {
	Task    TwinTaskEligibility
	Version TwinSignedAssertionEnvelope
	Policy  TwinEffectiveUsePolicy
}

type TwinInstructionAuthority string

const (
	TwinAuthoritySystemSafety        TwinInstructionAuthority = "system_safety"
	TwinAuthorityWorkspacePermission TwinInstructionAuthority = "workspace_permissions"
	TwinAuthorityUserRequest         TwinInstructionAuthority = "current_user_request"
	TwinAuthoritySignedBriefing      TwinInstructionAuthority = "signed_twin_briefing"
)

var twinBriefingAuthorityOrder = []TwinInstructionAuthority{
	TwinAuthoritySystemSafety,
	TwinAuthorityWorkspacePermission,
	TwinAuthorityUserRequest,
	TwinAuthoritySignedBriefing,
}

type TwinBriefingExclusionCode string

const (
	TwinBriefingPolicyOff           TwinBriefingExclusionCode = "policy_off"
	TwinBriefingPreviewOnly         TwinBriefingExclusionCode = "preview_only"
	TwinBriefingTaskIneligible      TwinBriefingExclusionCode = "task_ineligible"
	TwinBriefingTaskUnauthorized    TwinBriefingExclusionCode = "task_unauthorized"
	TwinBriefingTaskLocalOnly       TwinBriefingExclusionCode = "task_local_only"
	TwinBriefingUnsignedVersion     TwinBriefingExclusionCode = "unsigned_version"
	TwinBriefingMutableProposal     TwinBriefingExclusionCode = "mutable_proposal"
	TwinBriefingStaleVersion        TwinBriefingExclusionCode = "stale_version"
	TwinBriefingVersionUnauthorized TwinBriefingExclusionCode = "version_unauthorized"
	TwinBriefingVersionLocalOnly    TwinBriefingExclusionCode = "version_local_only"
	TwinBriefingUnsignedAssertion   TwinBriefingExclusionCode = "unsigned_assertion"
	TwinBriefingIrrelevant          TwinBriefingExclusionCode = "irrelevant"
	TwinBriefingOverBudget          TwinBriefingExclusionCode = "over_budget"
	TwinBriefingNoRelevantAssertion TwinBriefingExclusionCode = "no_relevant_assertion"
)

type TwinBriefingExclusion struct {
	AssertionID string
	Code        TwinBriefingExclusionCode
}

type TwinCompiledBriefing struct {
	Briefing             string
	VersionID            string
	Digest               string
	SelectedAssertionIDs []string
	CitationIDs          []string
	PolicyDecision       TwinEffectiveUsePolicy
	CompilerVersion      string
	Exclusions           []TwinBriefingExclusion
	AuthorityOrder       []TwinInstructionAuthority
	ByteCount            int
	TokenCount           int
	Inject               bool
	PreviewOnly          bool
}

type TwinBriefingInputError struct {
	Field string
}

func (e *TwinBriefingInputError) Error() string {
	return "invalid twin briefing input: " + e.Field
}

func (e *TwinBriefingInputError) Unwrap() error {
	return ErrTwinBriefingInvalidInput
}

type TwinBriefingCompiler interface {
	Compile(TwinBriefingInput) (TwinCompiledBriefing, error)
}

type deterministicTwinBriefingCompiler struct{}

func NewTwinBriefingCompiler() TwinBriefingCompiler {
	return deterministicTwinBriefingCompiler{}
}

func (deterministicTwinBriefingCompiler) Compile(input TwinBriefingInput) (TwinCompiledBriefing, error) {
	result := TwinCompiledBriefing{
		PolicyDecision:  copyTwinEffectivePolicy(input.Policy),
		CompilerVersion: TwinBriefingCompilerVersion,
		AuthorityOrder:  append([]TwinInstructionAuthority(nil), twinBriefingAuthorityOrder...),
		Exclusions:      make([]TwinBriefingExclusion, 0),
	}
	if input.Task.TaskID == "" {
		return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "task id"}
	}
	if input.Task.WorkspaceID == "" {
		return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "workspace id"}
	}
	if !validTwinUseState(input.Policy.State) {
		return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "policy state"}
	}
	if input.Policy.State != TwinUseOff {
		binding := TwinUsePolicyBinding{
			ID:      input.Policy.BindingID,
			Scope:   input.Policy.Scope,
			ScopeID: input.Policy.ScopeID,
			State:   input.Policy.State,
		}
		policyContext := TwinUsePolicyContext{
			WorkspaceID: input.Task.WorkspaceID,
			AgentID:     input.Task.AgentID,
			ProjectID:   input.Task.ProjectID,
			IssueID:     input.Task.IssueID,
			RunID:       input.Task.RunID,
		}
		if !input.Policy.Explicit || binding.ID == "" || twinPolicyScopeRank(binding.Scope) == 0 || !twinPolicyBindingApplies(policyContext, binding) {
			return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "effective policy binding"}
		}
	}
	if !input.Task.Authorized {
		return excludeTwinBriefing(result, TwinBriefingTaskUnauthorized), nil
	}
	if input.Task.LocalOnly {
		return excludeTwinBriefing(result, TwinBriefingTaskLocalOnly), nil
	}
	if !input.Task.Eligible {
		return excludeTwinBriefing(result, TwinBriefingTaskIneligible), nil
	}
	if input.Policy.State == TwinUseOff {
		return excludeTwinBriefing(result, TwinBriefingPolicyOff), nil
	}
	if input.Version.Lifecycle == TwinVersionProposal {
		return excludeTwinBriefing(result, TwinBriefingMutableProposal), nil
	}
	if input.Version.Lifecycle != TwinVersionSigned {
		return excludeTwinBriefing(result, TwinBriefingUnsignedVersion), nil
	}
	if input.Version.VersionID == "" {
		return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "version id"}
	}
	if input.Version.SignatureDigest == "" {
		return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "signature digest"}
	}
	result.VersionID = input.Version.VersionID
	if input.Version.Stale {
		return excludeTwinBriefing(result, TwinBriefingStaleVersion), nil
	}
	if !input.Version.Authorized {
		return excludeTwinBriefing(result, TwinBriefingVersionUnauthorized), nil
	}
	if input.Version.LocalOnly {
		return excludeTwinBriefing(result, TwinBriefingVersionLocalOnly), nil
	}

	assertions := append([]TwinBriefingAssertion(nil), input.Version.Assertions...)
	sort.Slice(assertions, func(i, j int) bool { return assertions[i].ID < assertions[j].ID })
	seenIDs := make(map[string]struct{}, len(assertions))
	candidates := make([]twinBriefingCandidate, 0, len(assertions))
	for _, assertion := range assertions {
		if assertion.ID == "" {
			return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "assertion id"}
		}
		if _, exists := seenIDs[assertion.ID]; exists {
			return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "duplicate assertion id"}
		}
		seenIDs[assertion.ID] = struct{}{}
		if assertion.Lifecycle == TwinAssertionProposal {
			result.Exclusions = append(result.Exclusions, TwinBriefingExclusion{AssertionID: assertion.ID, Code: TwinBriefingMutableProposal})
			continue
		}
		if assertion.Lifecycle != TwinAssertionSigned {
			result.Exclusions = append(result.Exclusions, TwinBriefingExclusion{AssertionID: assertion.ID, Code: TwinBriefingUnsignedAssertion})
			continue
		}
		if !validTwinAssertionType(assertion.Type) {
			return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "assertion type"}
		}
		assertion.Text = strings.Join(strings.Fields(assertion.Text), " ")
		if assertion.Text == "" {
			return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "assertion text"}
		}
		assertion.CitationIDs = sortedUniqueStrings(assertion.CitationIDs)
		if len(assertion.CitationIDs) == 0 || assertion.CitationIDs[0] == "" {
			return TwinCompiledBriefing{}, &TwinBriefingInputError{Field: "assertion citations"}
		}
		score, relevant := twinAssertionRelevance(input.Task, assertion.Applicability)
		if !relevant {
			result.Exclusions = append(result.Exclusions, TwinBriefingExclusion{AssertionID: assertion.ID, Code: TwinBriefingIrrelevant})
			continue
		}
		candidates = append(candidates, twinBriefingCandidate{assertion: assertion, relevance: score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].relevance != candidates[j].relevance {
			return candidates[i].relevance > candidates[j].relevance
		}
		return candidates[i].assertion.ID < candidates[j].assertion.ID
	})

	var briefing strings.Builder
	fmt.Fprintf(&briefing, "Twin briefing (lower-priority working context)\n")
	fmt.Fprintf(&briefing, "system safety, workspace permissions, and the current user request take precedence.\n")
	fmt.Fprintf(&briefing, "Signed Twin version: %s\n", input.Version.VersionID)
	selectedCitations := make(map[string]struct{})
	for _, candidate := range candidates {
		line := formatTwinBriefingAssertion(candidate.assertion)
		prospective := briefing.String() + line
		if len([]byte(prospective)) > TwinBriefingMaxBytes || estimateTwinBriefingTokens(prospective) > TwinBriefingMaxTokens {
			result.Exclusions = append(result.Exclusions, TwinBriefingExclusion{AssertionID: candidate.assertion.ID, Code: TwinBriefingOverBudget})
			continue
		}
		briefing.WriteString(line)
		result.SelectedAssertionIDs = append(result.SelectedAssertionIDs, candidate.assertion.ID)
		for _, citationID := range candidate.assertion.CitationIDs {
			selectedCitations[citationID] = struct{}{}
		}
	}
	result.Briefing = briefing.String()
	result.ByteCount = len([]byte(result.Briefing))
	result.TokenCount = estimateTwinBriefingTokens(result.Briefing)
	result.Digest = digestTwin([]byte(result.Briefing))
	result.CitationIDs = make([]string, 0, len(selectedCitations))
	for citationID := range selectedCitations {
		result.CitationIDs = append(result.CitationIDs, citationID)
	}
	sort.Strings(result.CitationIDs)
	result.Inject = input.Policy.State == TwinUseEnabled && len(result.SelectedAssertionIDs) > 0
	result.PreviewOnly = input.Policy.State == TwinUsePreview
	if len(result.SelectedAssertionIDs) == 0 {
		result.Exclusions = append(result.Exclusions, TwinBriefingExclusion{Code: TwinBriefingNoRelevantAssertion})
	}
	if result.PreviewOnly {
		result.Exclusions = append(result.Exclusions, TwinBriefingExclusion{Code: TwinBriefingPreviewOnly})
	}
	sortTwinBriefingExclusions(result.Exclusions)
	return result, nil
}

type twinBriefingCandidate struct {
	assertion TwinBriefingAssertion
	relevance int
}

func validTwinAssertionType(assertionType TwinAssertionType) bool {
	return assertionType == TwinAssertionPreference || assertionType == TwinAssertionConstraint || assertionType == TwinAssertionProcedure || assertionType == TwinAssertionQualityBar
}

func twinAssertionRelevance(task TwinTaskEligibility, applicability TwinAssertionApplicability) (int, bool) {
	score := 0
	for _, match := range []struct {
		want   string
		got    string
		weight int
	}{
		{want: applicability.TaskID, got: task.TaskID, weight: 64},
		{want: applicability.IssueID, got: task.IssueID, weight: 32},
		{want: applicability.ProjectID, got: task.ProjectID, weight: 16},
		{want: applicability.AgentID, got: task.AgentID, weight: 8},
		{want: applicability.WorkspaceID, got: task.WorkspaceID, weight: 4},
	} {
		if match.want == "" {
			continue
		}
		if match.want != match.got {
			return 0, false
		}
		score += match.weight
	}
	if len(applicability.Keywords) == 0 {
		return score, true
	}
	corpus := strings.ToLower(strings.Join(append([]string{task.Request}, task.Tags...), " "))
	for _, keyword := range applicability.Keywords {
		normalized := strings.ToLower(strings.Join(strings.Fields(keyword), " "))
		if normalized != "" && strings.Contains(corpus, normalized) {
			return score + 1, true
		}
	}
	return 0, false
}

func formatTwinBriefingAssertion(assertion TwinBriefingAssertion) string {
	return fmt.Sprintf("- [%s] %s (assertion: %s; citations: %s)\n", assertion.Type, assertion.Text, assertion.ID, strings.Join(assertion.CitationIDs, ", "))
}

// estimateTwinBriefingTokens deliberately uses one Unicode code point per
// token. It is conservative for typical prose and deterministic across hosts.
func estimateTwinBriefingTokens(value string) int {
	return utf8.RuneCountInString(value)
}

func sortedUniqueStrings(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		unique[value] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func excludeTwinBriefing(result TwinCompiledBriefing, code TwinBriefingExclusionCode) TwinCompiledBriefing {
	result.Exclusions = append(result.Exclusions, TwinBriefingExclusion{Code: code})
	return result
}

func sortTwinBriefingExclusions(exclusions []TwinBriefingExclusion) {
	sort.Slice(exclusions, func(i, j int) bool {
		if exclusions[i].AssertionID != exclusions[j].AssertionID {
			return exclusions[i].AssertionID < exclusions[j].AssertionID
		}
		return exclusions[i].Code < exclusions[j].Code
	})
}

func copyTwinEffectivePolicy(policy TwinEffectiveUsePolicy) TwinEffectiveUsePolicy {
	policy.Exclusions = append([]TwinPolicyBindingExclusion(nil), policy.Exclusions...)
	return policy
}
