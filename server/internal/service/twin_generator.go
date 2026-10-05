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
	"unicode/utf8"
)

const twinMaxModelResponseBytes = 512 * 1024

type TwinProposalGenerationInput struct {
	BuilderInput   TwinBuilderInput
	EgressEligible bool
}

type TwinProposalGenerator interface {
	Generate(context.Context, TwinProposalGenerationInput) (TwinProposalCandidate, error)
}

func GenerateTwinProposal(ctx context.Context, generator TwinProposalGenerator, input TwinProposalGenerationInput) (TwinProposalBuild, error) {
	if generator == nil {
		return TwinProposalBuild{}, &TwinInputError{Field: "proposal generator"}
	}
	if err := validateTwinInput(input.BuilderInput); err != nil {
		return TwinProposalBuild{}, err
	}
	candidate, err := generator.Generate(ctx, input)
	if err != nil {
		return TwinProposalBuild{}, err
	}
	return ValidateTwinProposal(input.BuilderInput, candidate)
}

type TwinJSONModel interface {
	GenerateJSON(context.Context, TwinModelRequest) ([]byte, error)
}

type TwinModelRequest struct {
	Instruction           string              `json:"instruction"`
	Evidence              json.RawMessage     `json:"evidence"`
	CitationKeys          []string            `json:"citation_keys"`
	PriorAssertions       []TwinAssertion     `json:"prior_assertions"`
	AllowedAssertionTypes []TwinAssertionType `json:"allowed_assertion_types"`
	MaxAssertions         int                 `json:"max_assertions"`
	MaxTextCodePoints     int                 `json:"max_text_code_points"`
}

type ModelTwinProposalGenerator struct {
	model       TwinJSONModel
	generatorID string
}

func NewModelTwinProposalGenerator(model TwinJSONModel, generatorID string) (*ModelTwinProposalGenerator, error) {
	if model == nil {
		return nil, &TwinInputError{Field: "json model"}
	}
	canonicalID := canonicalGeneratorID(generatorID)
	if canonicalID == "" {
		return nil, &TwinInputError{Field: "generator id"}
	}
	return &ModelTwinProposalGenerator{model: model, generatorID: canonicalID}, nil
}

func (g *ModelTwinProposalGenerator) Generate(ctx context.Context, input TwinProposalGenerationInput) (TwinProposalCandidate, error) {
	if !input.EgressEligible {
		return TwinProposalCandidate{}, ErrTwinGenerationDenied
	}
	if err := validateTwinModelEgressPolicy(input.BuilderInput.CanonicalEvidence); err != nil {
		return TwinProposalCandidate{}, err
	}
	request, err := twinModelRequest(input.BuilderInput)
	if err != nil {
		return TwinProposalCandidate{}, err
	}
	raw, err := g.model.GenerateJSON(ctx, request)
	if err != nil {
		return TwinProposalCandidate{}, fmt.Errorf("generate twin proposal JSON: %w", err)
	}
	if len(raw) > twinMaxModelResponseBytes {
		return TwinProposalCandidate{}, &TwinSizeError{Limit: twinMaxModelResponseBytes, Actual: len(raw)}
	}
	var response struct {
		Assertions []TwinAssertion `json:"assertions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return TwinProposalCandidate{}, fmt.Errorf("%w: decode JSON: %v", ErrTwinGeneratorOutput, err)
	}
	if err := ensureTwinJSONEOF(decoder); err != nil {
		return TwinProposalCandidate{}, err
	}
	if response.Assertions == nil {
		return TwinProposalCandidate{}, fmt.Errorf("%w: assertions must be a JSON array", ErrTwinGeneratorOutput)
	}
	for index := range response.Assertions {
		response.Assertions[index].Provenance = TwinAssertionProvenance{Kind: TwinProvenanceModel, Generator: g.generatorID}
	}
	return TwinProposalCandidate{Assertions: response.Assertions}, nil
}

type twinModelEgressPolicy struct {
	RemoteGenerationEnabled bool   `json:"remote_generation_enabled"`
	PolicyVersion           int64  `json:"policy_version"`
	PolicyDigest            string `json:"policy_digest"`
}

// validateTwinModelEgressPolicy is the last local gate before accepted Wiki
// evidence can reach a remote model. Wiki's provider verifies this immutable
// block against the accepted revision's persisted source-policy columns; Twin
// independently requires the frozen block so a stale caller or old schema can
// never turn an explicit Build request into implicit egress authorization.
func validateTwinModelEgressPolicy(canonical json.RawMessage) error {
	_, err := parseTwinModelEgressPolicy(canonical)
	return err
}

func parseTwinModelEgressPolicy(canonical json.RawMessage) (twinModelEgressPolicy, error) {
	var envelope struct {
		SchemaVersion int             `json:"schema_version"`
		EgressPolicy  json.RawMessage `json:"egress_policy"`
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	if err := decoder.Decode(&envelope); err != nil || envelope.SchemaVersion != 2 || len(envelope.EgressPolicy) == 0 {
		return twinModelEgressPolicy{}, ErrTwinGenerationDenied
	}
	if err := ensureTwinJSONEOF(decoder); err != nil {
		return twinModelEgressPolicy{}, ErrTwinGenerationDenied
	}
	var policy twinModelEgressPolicy
	policyDecoder := json.NewDecoder(bytes.NewReader(envelope.EgressPolicy))
	policyDecoder.DisallowUnknownFields()
	if err := policyDecoder.Decode(&policy); err != nil {
		return twinModelEgressPolicy{}, ErrTwinGenerationDenied
	}
	if err := ensureTwinJSONEOF(policyDecoder); err != nil {
		return twinModelEgressPolicy{}, ErrTwinGenerationDenied
	}
	if !policy.RemoteGenerationEnabled || policy.PolicyVersion <= 0 || !validTwinExecutionDigest(policy.PolicyDigest) {
		return twinModelEgressPolicy{}, ErrTwinGenerationDenied
	}
	return policy, nil
}

func twinModelRequest(input TwinBuilderInput) (TwinModelRequest, error) {
	citations, err := twinCitationKeys(input.Citations)
	if err != nil {
		return TwinModelRequest{}, err
	}
	citationKeys := make([]string, 0, len(citations))
	for key := range citations {
		citationKeys = append(citationKeys, key)
	}
	sort.Strings(citationKeys)
	prior := append([]TwinAssertion(nil), input.PriorAssertions...)
	sortTwinAssertions(prior)
	_, evidence, err := canonicalTwinEvidence(input)
	if err != nil {
		return TwinModelRequest{}, err
	}
	return TwinModelRequest{
		Instruction:           "Derive concise evidence-backed working assertions. Return JSON only. Each applicability value must be an object with optional project_id, issue_id, or keywords; never invent task_id, workspace_id, agent_id, issue_id, or project_id. Every assertion must cite supplied evidence.",
		Evidence:              append(json.RawMessage(nil), evidence...),
		CitationKeys:          citationKeys,
		PriorAssertions:       prior,
		AllowedAssertionTypes: []TwinAssertionType{TwinAssertionPreference, TwinAssertionConstraint, TwinAssertionProcedure, TwinAssertionQualityBar},
		MaxAssertions:         twinMaxAssertions,
		MaxTextCodePoints:     twinMaxAssertionTextRunes,
	}, nil
}

func ensureTwinJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("%w: trailing JSON: %v", ErrTwinGeneratorOutput, err)
	}
	return fmt.Errorf("%w: trailing JSON value", ErrTwinGeneratorOutput)
}

func canonicalGeneratorID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	if value == "" || utf8.RuneCountInString(value) > twinMaxProvenanceGeneratorRunes || !twinStableIDPattern.MatchString(value) {
		return ""
	}
	return value
}

type DeterministicTwinProposalGenerator struct {
	Candidate TwinProposalCandidate
	Err       error
}

func (g DeterministicTwinProposalGenerator) Generate(context.Context, TwinProposalGenerationInput) (TwinProposalCandidate, error) {
	if g.Err != nil {
		return TwinProposalCandidate{}, g.Err
	}
	assertions := append([]TwinAssertion(nil), g.Candidate.Assertions...)
	for index := range assertions {
		assertions[index].EvidenceCitations = append([]string(nil), assertions[index].EvidenceCitations...)
	}
	return TwinProposalCandidate{Assertions: assertions}, nil
}

type InventoryTwinProposalGenerator struct{}

func (InventoryTwinProposalGenerator) Generate(_ context.Context, input TwinProposalGenerationInput) (TwinProposalCandidate, error) {
	content := input.BuilderInput.Content
	assertions := make([]TwinAssertion, 0, len(content.Issues)+len(content.Projects)+len(content.ProjectResources)+len(content.AutopilotRuns))
	for _, item := range content.Issues {
		assertion, err := newInventoryTwinAssertion(item, item.CitationKey, fmt.Sprintf("Issue %d: %s", item.Number, item.Title), TwinAssertionApplicability{IssueID: item.ID})
		if err != nil {
			return TwinProposalCandidate{}, err
		}
		assertions = append(assertions, assertion)
	}
	for _, item := range content.Projects {
		assertion, err := newInventoryTwinAssertion(item, item.CitationKey, "Project: "+item.Title, TwinAssertionApplicability{ProjectID: item.ID})
		if err != nil {
			return TwinProposalCandidate{}, err
		}
		assertions = append(assertions, assertion)
	}
	for _, item := range content.ProjectResources {
		assertion, err := newInventoryTwinAssertion(item, item.CitationKey, "Repository: "+item.Ref.Host+"/"+item.Ref.RepositoryPath, TwinAssertionApplicability{ProjectID: item.ProjectID})
		if err != nil {
			return TwinProposalCandidate{}, err
		}
		assertions = append(assertions, assertion)
	}
	for _, item := range content.AutopilotRuns {
		applicability := TwinAssertionApplicability{Keywords: []string{"autopilot"}}
		if item.IssueID != "" {
			applicability = TwinAssertionApplicability{IssueID: item.IssueID}
		}
		assertion, err := newInventoryTwinAssertion(item, item.CitationKey, "Autopilot "+item.AutopilotTitle+" completed", applicability)
		if err != nil {
			return TwinProposalCandidate{}, err
		}
		assertions = append(assertions, assertion)
	}
	return TwinProposalCandidate{Assertions: assertions}, nil
}

func newInventoryTwinAssertion[T comparable](item T, citationKey, text string, applicability TwinAssertionApplicability) (TwinAssertion, error) {
	canonical, err := json.Marshal(item)
	if err != nil {
		return TwinAssertion{}, fmt.Errorf("marshal twin inventory item: %w", err)
	}
	return TwinAssertion{
		ID:                digestTwin([]byte(citationKey + string(canonical))),
		Type:              TwinAssertionProcedure,
		Text:              text,
		Applicability:     applicability,
		EvidenceCitations: []string{citationKey},
		Confidence:        1,
		Provenance:        TwinAssertionProvenance{Kind: TwinProvenanceDeterministicInventory, Generator: "inventory-v1"},
	}, nil
}
