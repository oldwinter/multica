package skillevolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

const (
	DeterministicValidatorName    = "candidate-policy"
	DeterministicValidatorVersion = "v1"
	MaxReplayTimeout              = 5 * time.Minute
)

var (
	ErrEvaluationUnavailable = errors.New("skill evolution evaluation is unavailable")
	ErrEvaluationLimit       = errors.New("skill evolution evaluation exceeded a configured limit")
)

type CandidatePolicy struct {
	MaxChangedFiles         int
	MaxUnrelatedChangeUnits int
	MaxPrimaryGrowth        int
	MaxCostUSDTicks         int64
}

func DefaultCandidatePolicy() CandidatePolicy {
	return CandidatePolicy{MaxChangedFiles: 8, MaxUnrelatedChangeUnits: 3, MaxPrimaryGrowth: 16 * 1024, MaxCostUSDTicks: 1_000_000_000}
}

type ValidationOutcome struct {
	Result             EvaluationResult `json:"result"`
	RuleCodes          []string         `json:"rule_codes"`
	ChangedFiles       int              `json:"changed_files"`
	AddedFiles         int              `json:"added_files"`
	DeletedFiles       int              `json:"deleted_files"`
	PrimaryGrowthBytes int              `json:"primary_growth_bytes"`
	Digest             Digest           `json:"-"`
}

var (
	secretAssignmentPattern = regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|client[_-]?secret|password)\s*[:=]\s*["']?[A-Za-z0-9_./+=-]{12,}`)
	localPathPattern        = regexp.MustCompile(`(?i)(file://|/home/[^\s]+|/Users/[^\s]+|[A-Z]:\\Users\\[^\s]+)`)
	capabilityPattern       = regexp.MustCompile(`(?i)(bypass (platform|workspace) policy|ignore (platform|workspace) policy|grant (a )?(tool|credential|permission|workspace access)|read (all )?credentials)`)
)

// ValidateCandidatePolicy runs after T1's canonical bundle/frontmatter gate
// and before behavioral replay. Its result contains rule codes and counts only.
func ValidateCandidatePolicy(base skillbundle.Skill, candidate ImprovementCandidate, evidence []ResolvedEvidence, policy CandidatePolicy) ValidationOutcome {
	codes := make([]string, 0, 8)
	manifest, err := ValidateCandidateBundle(candidate.Bundle)
	if err != nil {
		codes = append(codes, "canonical_bundle_invalid")
	}
	baseManifest, baseErr := skillbundle.BuildValidatedManifest(base)
	if baseErr != nil {
		codes = append(codes, "base_bundle_invalid")
	}
	if candidate.Bundle.ID != base.ID || candidate.Bundle.Source != base.Source {
		codes = append(codes, "identity_changed")
	}
	if err == nil && baseErr == nil && manifest.Hash == baseManifest.Hash {
		codes = append(codes, "candidate_noop")
	}
	if policy.MaxChangedFiles <= 0 || policy.MaxUnrelatedChangeUnits <= 0 || policy.MaxPrimaryGrowth < 0 || policy.MaxCostUSDTicks < 0 ||
		policy.MaxCostUSDTicks > 1_000_000_000 {
		codes = append(codes, "policy_invalid")
	}
	if candidate.CostUSDTicks < 0 || candidate.CostUSDTicks > policy.MaxCostUSDTicks {
		codes = append(codes, "cost_limit_exceeded")
	}
	if !validRationale(candidate.ObservedPattern) || !validRationale(candidate.ExpectedBenefit) || !validRationale(candidate.RegressionRisk) {
		codes = append(codes, "rationale_invalid")
	}

	changed, added, deleted := bundleFileChanges(base.Files, candidate.Bundle.Files)
	primaryGrowth := len(candidate.Bundle.Content) - len(base.Content)
	if primaryGrowth < 0 {
		primaryGrowth = 0
	}
	if changed > policy.MaxChangedFiles {
		codes = append(codes, "too_many_changed_files")
	}
	changeUnits := changed
	if candidate.Bundle.Content != base.Content {
		changeUnits++
	}
	if changeUnits > policy.MaxUnrelatedChangeUnits {
		codes = append(codes, "unrelated_change_budget_exceeded")
	}
	if primaryGrowth > policy.MaxPrimaryGrowth {
		codes = append(codes, "primary_growth_exceeded")
	}
	if destructivePrimaryRewrite(base.Content, candidate.Bundle.Content) {
		codes = append(codes, "primary_rewrite_excessive")
	}
	duplicate, conflict := instructionConflicts(candidate.Bundle)
	if duplicate {
		codes = append(codes, "duplicate_instruction")
	}
	if conflict {
		codes = append(codes, "conflicting_instruction")
	}
	if !candidateClaimsResolvedEvidence(candidate.EvidenceDigests, evidence) {
		codes = append(codes, "provenance_invalid")
	}
	for _, value := range candidateText(candidate.Bundle) {
		switch {
		case strings.Contains(value, "-----BEGIN PRIVATE KEY-----") || secretAssignmentPattern.MatchString(value):
			codes = append(codes, "secret_like_content")
		case localPathPattern.MatchString(value):
			codes = append(codes, "local_filesystem_path")
		case capabilityPattern.MatchString(value) || containsAuthorityManipulation(value):
			codes = append(codes, "authority_expansion")
		}
	}

	codes = uniqueSortedStrings(codes)
	result := EvaluationResultPassed
	if len(codes) > 0 {
		result = EvaluationResultFailed
	}
	outcome := ValidationOutcome{
		Result: result, RuleCodes: codes, ChangedFiles: changed, AddedFiles: added,
		DeletedFiles: deleted, PrimaryGrowthBytes: primaryGrowth,
	}
	outcome.Digest = digestSafeValue("candidate-validation-v1", outcome)
	return outcome
}

func destructivePrimaryRewrite(base, candidate string) bool {
	baseBody := skillPrimaryBody(base)
	if len(baseBody) < 256 || baseBody == candidate {
		return false
	}
	candidateLines := make(map[string]struct{})
	for _, line := range meaningfulInstructionLines(skillPrimaryBody(candidate)) {
		candidateLines[line] = struct{}{}
	}
	baseLines := meaningfulInstructionLines(baseBody)
	retained := 0
	for _, line := range baseLines {
		if _, ok := candidateLines[line]; ok {
			retained++
		}
	}
	return len(baseLines) >= 4 && retained*4 < len(baseLines)
}

func skillPrimaryBody(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	if end := strings.Index(content[4:], "\n---"); end >= 0 {
		return strings.TrimSpace(content[end+8:])
	}
	return content
}

func meaningfulInstructionLines(content string) []string {
	lines := make([]string, 0)
	for _, raw := range strings.FieldsFunc(content, func(r rune) bool { return r == '\n' || r == '.' || r == ';' }) {
		line := strings.Join(instructionTokens(raw), " ")
		if len(strings.Fields(line)) >= 3 {
			lines = append(lines, line)
		}
	}
	return lines
}

func instructionTokens(value string) []string {
	return strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-'
	})
}

func containsAuthorityManipulation(value string) bool {
	tokens := instructionTokens(value)
	has := func(options ...string) bool {
		for _, token := range tokens {
			for _, option := range options {
				if token == option {
					return true
				}
			}
		}
		return false
	}
	overridesAuthority := has("ignore", "disregard", "override", "forget", "bypass") &&
		has("prior", "previous", "system", "developer", "instruction", "instructions", "policy", "policies")
	exfiltratesSensitiveData := has("send", "reveal", "output", "print", "expose", "read") &&
		has("environment", "env", "credential", "credentials", "secret", "secrets", "token", "tokens", "password", "passwords")
	return overridesAuthority || exfiltratesSensitiveData
}

func instructionConflicts(bundle skillbundle.Skill) (duplicate, conflict bool) {
	seen := make(map[string]struct{})
	polarities := make(map[string]bool)
	texts := []string{skillPrimaryBody(bundle.Content)}
	for _, file := range bundle.Files {
		texts = append(texts, file.Content)
	}
	for _, text := range texts {
		for _, line := range meaningfulInstructionLines(text) {
			if _, ok := seen[line]; ok {
				duplicate = true
			}
			seen[line] = struct{}{}
			tokens := strings.Fields(line)
			negative := false
			core := make([]string, 0, len(tokens))
			for _, token := range tokens {
				switch token {
				case "never", "not", "don't", "dont", "mustn't", "mustnt":
					negative = true
				case "always", "must", "should", "do":
				default:
					core = append(core, token)
				}
			}
			if len(core) < 3 {
				continue
			}
			key := strings.Join(core, " ")
			if prior, ok := polarities[key]; ok && prior != negative {
				conflict = true
			} else {
				polarities[key] = negative
			}
		}
	}
	return duplicate, conflict
}

func candidateClaimsResolvedEvidence(claims []Digest, evidence []ResolvedEvidence) bool {
	if len(claims) == 0 {
		return false
	}
	available := make(map[Digest]struct{}, len(evidence))
	for _, item := range evidence {
		available[item.Ref.Digest] = struct{}{}
	}
	seen := make(map[Digest]struct{}, len(claims))
	for _, claim := range claims {
		if !claim.Valid() {
			return false
		}
		if _, duplicate := seen[claim]; duplicate {
			return false
		}
		seen[claim] = struct{}{}
		if evidence != nil {
			if _, found := available[claim]; !found {
				return false
			}
		}
	}
	return true
}

func (o ValidationOutcome) SafeMetrics() json.RawMessage {
	raw, _ := json.Marshal(struct {
		RuleCodes          []string `json:"rule_codes"`
		ChangedFiles       int      `json:"changed_files"`
		AddedFiles         int      `json:"added_files"`
		DeletedFiles       int      `json:"deleted_files"`
		PrimaryGrowthBytes int      `json:"primary_growth_bytes"`
	}{o.RuleCodes, o.ChangedFiles, o.AddedFiles, o.DeletedFiles, o.PrimaryGrowthBytes})
	return raw
}

type ReplayLimits struct {
	Timeout         time.Duration
	MaxSamples      int
	MaxCostUSDTicks int64
	PolicyVersion   string
}

type ReplayRequest struct {
	Base      skillbundle.Skill
	Candidate skillbundle.Skill
	Evidence  []ResolvedEvidence
	Limits    ReplayLimits
}

type ReplayResult struct {
	Result           EvaluationResult
	SampleCount      int
	FailureCount     int
	CostUSDTicks     int64
	Nondeterministic bool
	ReasonCode       string
}

type ReplayOutcome struct {
	ReplayResult
	Adapter        string
	AdapterVersion string
	Duration       time.Duration
	Digest         Digest
}

func (o ReplayOutcome) SafeMetrics() json.RawMessage {
	raw, _ := json.Marshal(struct {
		Samples          int    `json:"samples"`
		Failures         int    `json:"failures"`
		Nondeterministic bool   `json:"nondeterministic"`
		ReasonCode       string `json:"reason_code,omitempty"`
	}{o.SampleCount, o.FailureCount, o.Nondeterministic, o.ReasonCode})
	return raw
}

type ReplayEngine interface {
	Replay(context.Context, ReplayRequest) (ReplayResult, error)
}

type BehavioralEvaluator interface {
	Evaluate(context.Context, ReplayRequest) (ReplayOutcome, error)
}

type ProductionReplayEvaluator struct {
	engine  ReplayEngine
	adapter string
	version string
}

func NewProductionReplayEvaluator(engine ReplayEngine, adapter, version string) *ProductionReplayEvaluator {
	return &ProductionReplayEvaluator{engine: engine, adapter: adapter, version: version}
}

func (e *ProductionReplayEvaluator) Evaluate(ctx context.Context, request ReplayRequest) (ReplayOutcome, error) {
	if e == nil || e.engine == nil || !boundedToken(e.adapter, 80) || !boundedToken(e.version, 80) ||
		!validReplayLimits(request.Limits) || len(request.Evidence) == 0 || len(request.Evidence) > MaxEvidenceRefs {
		return ReplayOutcome{}, ErrEvaluationUnavailable
	}
	started := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, request.Limits.Timeout)
	defer cancel()
	type response struct {
		result ReplayResult
		err    error
	}
	responses := make(chan response, 1)
	go func() {
		result, err := e.engine.Replay(callCtx, cloneReplayRequest(request))
		responses <- response{result: result, err: err}
	}()
	var result ReplayResult
	var err error
	select {
	case <-callCtx.Done():
		err = callCtx.Err()
	case response := <-responses:
		result, err = response.result, response.err
	}
	duration := time.Since(started)
	if err != nil {
		reason := "adapter_error"
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			reason = "timeout"
		}
		return finalizeReplayOutcome(ReplayOutcome{
			ReplayResult: ReplayResult{Result: EvaluationResultUnknown, ReasonCode: reason},
			Adapter:      e.adapter, AdapterVersion: e.version, Duration: duration,
		}), nil
	}
	outcome := ReplayOutcome{ReplayResult: result, Adapter: e.adapter, AdapterVersion: e.version, Duration: duration}
	if !result.Result.Valid() || result.SampleCount < 0 || result.FailureCount < 0 || result.FailureCount > result.SampleCount ||
		result.CostUSDTicks < 0 {
		return ReplayOutcome{}, ErrEvaluationUnavailable
	}
	switch {
	case result.SampleCount > request.Limits.MaxSamples || result.CostUSDTicks > request.Limits.MaxCostUSDTicks:
		outcome.Result = EvaluationResultFailed
		outcome.ReasonCode = "configured_limit_exceeded"
	case result.Nondeterministic:
		outcome.Result = EvaluationResultInconclusive
		outcome.ReasonCode = "nondeterministic"
	case result.Result == EvaluationResultPassed && result.SampleCount == 0:
		outcome.Result = EvaluationResultInconclusive
		outcome.ReasonCode = "no_samples"
	case result.Result == EvaluationResultPassed && result.SampleCount < request.Limits.MaxSamples:
		outcome.Result = EvaluationResultInconclusive
		outcome.ReasonCode = "low_sample"
	case result.Result == EvaluationResultPassed && result.FailureCount > 0:
		outcome.Result = EvaluationResultFailed
		outcome.ReasonCode = "replay_failure"
	}
	return finalizeReplayOutcome(outcome), nil
}

type DeterministicReplayEvaluator struct {
	Outcome ReplayOutcome
	Err     error
	Calls   int
}

func (e *DeterministicReplayEvaluator) Evaluate(_ context.Context, request ReplayRequest) (ReplayOutcome, error) {
	if e == nil {
		return ReplayOutcome{}, ErrEvaluationUnavailable
	}
	e.Calls++
	outcome := e.Outcome
	if outcome.Adapter == "" {
		outcome.Adapter = "deterministic-replay"
	}
	if outcome.AdapterVersion == "" {
		outcome.AdapterVersion = "v1"
	}
	if outcome.Duration == 0 {
		outcome.Duration = time.Millisecond
	}
	return finalizeReplayOutcome(outcome), e.Err
}

func validReplayLimits(limits ReplayLimits) bool {
	return limits.Timeout > 0 && limits.Timeout <= MaxReplayTimeout && limits.MaxSamples > 0 &&
		limits.MaxSamples <= 32 && limits.MaxCostUSDTicks >= 0 && limits.MaxCostUSDTicks <= 1_000_000_000 &&
		boundedToken(limits.PolicyVersion, 80)
}

func finalizeReplayOutcome(outcome ReplayOutcome) ReplayOutcome {
	outcome.Digest = digestSafeValue("behavioral-replay-v1", struct {
		Result           EvaluationResult `json:"result"`
		SampleCount      int              `json:"sample_count"`
		FailureCount     int              `json:"failure_count"`
		CostUSDTicks     int64            `json:"cost_usd_ticks"`
		Nondeterministic bool             `json:"nondeterministic"`
		ReasonCode       string           `json:"reason_code"`
		Adapter          string           `json:"adapter"`
		AdapterVersion   string           `json:"adapter_version"`
	}{outcome.Result, outcome.SampleCount, outcome.FailureCount, outcome.CostUSDTicks,
		outcome.Nondeterministic, outcome.ReasonCode, outcome.Adapter, outcome.AdapterVersion})
	return outcome
}

func cloneReplayRequest(request ReplayRequest) ReplayRequest {
	cloned := request
	cloned.Base = cloneSkillBundle(request.Base)
	cloned.Candidate = cloneSkillBundle(request.Candidate)
	cloned.Evidence = make([]ResolvedEvidence, len(request.Evidence))
	for index, evidence := range request.Evidence {
		cloned.Evidence[index] = ResolvedEvidence{Ref: evidence.Ref, Payload: append([]byte(nil), evidence.Payload...)}
	}
	return cloned
}

func bundleFileChanges(base, candidate []skillbundle.File) (changed, added, deleted int) {
	baseFiles := make(map[string]string, len(base))
	for _, file := range base {
		baseFiles[file.Path] = file.Content
	}
	for _, file := range candidate {
		content, found := baseFiles[file.Path]
		if !found {
			added++
			changed++
		} else if content != file.Content {
			changed++
		}
		delete(baseFiles, file.Path)
	}
	deleted = len(baseFiles)
	changed += deleted
	return changed, added, deleted
}

func candidateText(bundle skillbundle.Skill) []string {
	values := []string{bundle.Name, bundle.Description, bundle.Content}
	for _, file := range bundle.Files {
		values = append(values, file.Content)
	}
	return values
}

func uniqueSortedStrings(values []string) []string {
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func digestSafeValue(namespace string, value any) Digest {
	raw, _ := json.Marshal(value)
	h := sha256.New()
	writeDigestValue(h, namespace)
	_, _ = h.Write(raw)
	return Digest("sha256:" + hex.EncodeToString(h.Sum(nil)))
}

func evaluationCost(value int64) pgtype.Int8 {
	if value < 0 || value > 1_000_000_000 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

func enforceReplayLimits(outcome ReplayOutcome, limits ReplayLimits) ReplayOutcome {
	if !outcome.Result.Valid() || outcome.SampleCount < 0 || outcome.FailureCount < 0 ||
		outcome.FailureCount > outcome.SampleCount || outcome.CostUSDTicks < 0 {
		outcome.Result = EvaluationResultUnknown
		outcome.ReasonCode = "adapter_output_invalid"
	} else {
		switch {
		case outcome.SampleCount > limits.MaxSamples || outcome.CostUSDTicks > limits.MaxCostUSDTicks:
			outcome.Result = EvaluationResultFailed
			outcome.ReasonCode = "configured_limit_exceeded"
		case outcome.Nondeterministic:
			outcome.Result = EvaluationResultInconclusive
			outcome.ReasonCode = "nondeterministic"
		case outcome.Result == EvaluationResultPassed && outcome.SampleCount == 0:
			outcome.Result = EvaluationResultInconclusive
			outcome.ReasonCode = "no_samples"
		case outcome.Result == EvaluationResultPassed && outcome.SampleCount < limits.MaxSamples:
			outcome.Result = EvaluationResultInconclusive
			outcome.ReasonCode = "low_sample"
		case outcome.Result == EvaluationResultPassed && outcome.FailureCount > 0:
			outcome.Result = EvaluationResultFailed
			outcome.ReasonCode = "replay_failure"
		}
	}
	return finalizeReplayOutcome(outcome)
}
