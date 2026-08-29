package skillevolution

import (
	"context"
	"errors"
	"time"

	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

const (
	MaxImprovementRationaleBytes = 2 * 1024
	MaxImproverTimeout           = 5 * time.Minute
)

var (
	ErrImproverUnavailable = errors.New("skill evolution improver is unavailable")
	ErrImproverLimit       = errors.New("skill evolution improver exceeded a configured limit")
	ErrImproverOutput      = errors.New("invalid skill evolution improver output")
)

// ImprovementRequest is the complete authorized context an Improvement Room
// or model adapter may receive. It contains no task transcript and no source
// payload outside the bounded evidence envelopes.
type ImprovementRequest struct {
	Base             skillbundle.Skill
	Evidence         []ResolvedEvidence
	PolicyVersion    string
	MaxCostUSDTicks  int64
	MaxChangedFiles  int
	MaxPrimaryGrowth int
}

// ImprovementCandidate is structured so provenance and review rationale do
// not need to be inferred from free-form model output.
type ImprovementCandidate struct {
	Bundle            skillbundle.Skill
	ObservedPattern   string
	ExpectedBenefit   string
	RegressionRisk    string
	EvidenceDigests   []Digest
	AuthorizedChanges []ChangeAuthorization
	CostUSDTicks      int64
}

// ChangeAuthorization binds one exact, deterministic bundle change to the
// evidence that authorizes it. Value is Skill content, never source evidence.
type ChangeAuthorization struct {
	Path            string   `json:"path"`
	Operation       string   `json:"operation"`
	Value           string   `json:"value"`
	EvidenceDigests []Digest `json:"evidence_digests"`
}

// Improver is the lifecycle-facing capability. Production should pass a
// ProductionImprover; deterministic fixtures may implement it directly.
type Improver interface {
	Improve(context.Context, ImprovementRequest) (ImprovementCandidate, error)
}

// ImprovementEngine is the production port. A Room-backed implementation can
// invoke existing Room orchestration behind this boundary; lifecycle code does
// not create or coordinate Rooms itself.
type ImprovementEngine interface {
	Improve(context.Context, ImprovementRequest) (ImprovementCandidate, error)
}

type ProductionImprover struct {
	engine  ImprovementEngine
	timeout time.Duration
}

func NewProductionImprover(engine ImprovementEngine, timeout time.Duration) *ProductionImprover {
	return &ProductionImprover{engine: engine, timeout: timeout}
}

func (i *ProductionImprover) Improve(ctx context.Context, request ImprovementRequest) (ImprovementCandidate, error) {
	if i == nil || i.engine == nil || i.timeout <= 0 || i.timeout > MaxImproverTimeout ||
		request.MaxCostUSDTicks < 0 || request.MaxChangedFiles <= 0 || request.MaxPrimaryGrowth < 0 ||
		!boundedToken(request.PolicyVersion, 80) || len(request.Evidence) == 0 ||
		len(request.Evidence) > MaxEvidenceRefs {
		return ImprovementCandidate{}, ErrImproverUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()
	type response struct {
		candidate ImprovementCandidate
		err       error
	}
	responses := make(chan response, 1)
	go func() {
		candidate, err := i.engine.Improve(callCtx, cloneImprovementRequest(request))
		responses <- response{candidate: candidate, err: err}
	}()
	var candidate ImprovementCandidate
	select {
	case <-callCtx.Done():
		return ImprovementCandidate{}, callCtx.Err()
	case result := <-responses:
		if result.err != nil {
			if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
				return ImprovementCandidate{}, context.DeadlineExceeded
			}
			return ImprovementCandidate{}, result.err
		}
		candidate = result.candidate
	}
	if candidate.CostUSDTicks < 0 || candidate.CostUSDTicks > request.MaxCostUSDTicks {
		return ImprovementCandidate{}, ErrImproverLimit
	}
	if !validRationale(candidate.ObservedPattern) || !validRationale(candidate.ExpectedBenefit) ||
		!validRationale(candidate.RegressionRisk) || len(candidate.EvidenceDigests) == 0 ||
		len(candidate.EvidenceDigests) > len(request.Evidence) || len(candidate.AuthorizedChanges) == 0 ||
		len(candidate.AuthorizedChanges) > MaxCandidateChangeAuthorizations {
		return ImprovementCandidate{}, ErrImproverOutput
	}
	return cloneImprovementCandidate(candidate), nil
}

// DeterministicImprover is intentionally small and is suitable for lifecycle
// tests and deterministic development fixtures.
type DeterministicImprover struct {
	Candidate ImprovementCandidate
	Err       error
	Calls     int
}

func (i *DeterministicImprover) Improve(_ context.Context, _ ImprovementRequest) (ImprovementCandidate, error) {
	if i == nil {
		return ImprovementCandidate{}, ErrImproverUnavailable
	}
	i.Calls++
	return cloneImprovementCandidate(i.Candidate), i.Err
}

func cloneImprovementRequest(request ImprovementRequest) ImprovementRequest {
	cloned := request
	cloned.Base = cloneSkillBundle(request.Base)
	cloned.Evidence = make([]ResolvedEvidence, len(request.Evidence))
	for index, evidence := range request.Evidence {
		cloned.Evidence[index] = ResolvedEvidence{Ref: evidence.Ref, Payload: append([]byte(nil), evidence.Payload...)}
	}
	return cloned
}

func cloneImprovementCandidate(candidate ImprovementCandidate) ImprovementCandidate {
	cloned := candidate
	cloned.Bundle = cloneSkillBundle(candidate.Bundle)
	cloned.EvidenceDigests = append([]Digest(nil), candidate.EvidenceDigests...)
	cloned.AuthorizedChanges = cloneChangeAuthorizations(candidate.AuthorizedChanges)
	return cloned
}

func cloneChangeAuthorizations(values []ChangeAuthorization) []ChangeAuthorization {
	cloned := make([]ChangeAuthorization, len(values))
	for index, value := range values {
		cloned[index] = value
		cloned[index].EvidenceDigests = append([]Digest(nil), value.EvidenceDigests...)
	}
	return cloned
}

func cloneSkillBundle(bundle skillbundle.Skill) skillbundle.Skill {
	cloned := bundle
	cloned.Files = append([]skillbundle.File(nil), bundle.Files...)
	return cloned
}

func validRationale(value string) bool {
	return value != "" && len(value) <= MaxImprovementRationaleBytes
}
