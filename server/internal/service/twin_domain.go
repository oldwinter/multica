package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	twinProposalSchemaVersion        = 2
	twinMaxContentBytes              = 2 * 1024 * 1024
	twinMaxAssertions                = 128
	twinMaxAssertionIDRunes          = 128
	twinMaxAssertionTextRunes        = 1200
	twinMaxApplicabilityIDRunes      = 128
	twinMaxApplicabilityKeywords     = 16
	twinMaxApplicabilityKeywordRunes = 80
	twinMaxCitationsPerAssertion     = 16
	twinMaxCitationKeyRunes          = 256
	twinMaxProvenanceGeneratorRunes  = 80
)

var (
	ErrTwinInvalidInput      = errors.New("invalid twin builder input")
	ErrTwinCitationMissing   = errors.New("twin assertion citation is missing")
	ErrTwinContentTooLarge   = errors.New("twin content exceeds size limit")
	ErrTwinInvalidAssertion  = errors.New("invalid twin assertion")
	ErrTwinUnsafeAssertion   = errors.New("unsafe twin assertion")
	ErrTwinGenerationDenied  = errors.New("twin generation egress is not allowed")
	ErrTwinGeneratorOutput   = errors.New("invalid twin generator output")
	twinStableIDPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]*$`)
	twinIdentityClaimPattern = regexp.MustCompile(`(?i)\b(?:persona|personality|identity|impersonat(?:e|ion)|clone(?:d)?[[:space:]]+(?:the[[:space:]]+)?(?:user|owner|person))\b|\b(?:you|i|the[[:space:]]+twin|the[[:space:]]+user|the[[:space:]]+owner)[[:space:]]+(?:are|am|is)[[:space:]]+(?:a|an|the)\b`)
	twinSecretPattern        = regexp.MustCompile(`(?i)(?:-----BEGIN[[:space:]][A-Z ]*PRIVATE KEY-----|\b(?:sk|ghp|github_pat|xox[baprs]|mdt)_[a-z0-9_-]{8,}\b|\bAKIA[A-Z0-9]{12,}\b|\b(?:api[_ -]?key|password|secret|access[_ -]?token|bearer)[[:space:]]*[:=][[:space:]]*[^[:space:]]+)`)
	twinUnixPathPattern      = regexp.MustCompile(`(?i)(?:^|[[:space:]"'(])(?:~[/\\]|file://|/(?:home|users|root|private|etc|var|tmp)/|/(?:[a-z0-9._-]+/){2,}[a-z0-9._-]+)`)
	twinWindowsPathPattern   = regexp.MustCompile(`(?i)(?:^|[[:space:]"'(])(?:[a-z]:[/\\]|\\\\[a-z0-9._-]+[/\\])`)
	twinProfileLeakPattern   = regexp.MustCompile(`(?i)\b(?:daemon|cli|auth|local)[[:space:]_-]*profile(?:[[:space:]_-]*name)?\b|\bprofile[[:space:]]*[:=][[:space:]]*[^[:space:]]+`)
)

type TwinInputError struct{ Field string }

func (e *TwinInputError) Error() string { return "invalid twin builder input: " + e.Field }
func (e *TwinInputError) Unwrap() error { return ErrTwinInvalidInput }

type TwinCitationError struct{ CitationKey string }

func (e *TwinCitationError) Error() string {
	return "twin assertion citation is missing: " + e.CitationKey
}
func (e *TwinCitationError) Unwrap() error { return ErrTwinCitationMissing }

type TwinSizeError struct {
	Limit  int
	Actual int
}

func (e *TwinSizeError) Error() string {
	return fmt.Sprintf("twin content is %d bytes; limit is %d", e.Actual, e.Limit)
}
func (e *TwinSizeError) Unwrap() error { return ErrTwinContentTooLarge }

type TwinAssertionError struct {
	ID, Field, Reason string
	Cause             error
}

func (e *TwinAssertionError) Error() string {
	identifier := e.ID
	if identifier == "" {
		identifier = "<unknown>"
	}
	return fmt.Sprintf("twin assertion %q has invalid %s: %s", identifier, e.Field, e.Reason)
}
func (e *TwinAssertionError) Unwrap() error { return e.Cause }

type TwinBuilderInput struct {
	SourceWikiRevisionID  string
	SourceDigest          string
	CanonicalEvidence     json.RawMessage
	EvidenceSchemaVersion int
	// Content is the known-field compatibility view used for deterministic UI
	// projections. Production model generation receives CanonicalEvidence exactly.
	Content         LMWikiContent
	Citations       []LMWikiCitation
	PriorAssertions []TwinAssertion
}

type TwinProposalBuild struct {
	Content       TwinProposalContent
	CanonicalJSON []byte
	ContentDigest string
}

type TwinProposalContent struct {
	SchemaVersion        int             `json:"schema_version"`
	SourceWikiRevisionID string          `json:"source_wiki_revision_id"`
	SourceDigest         string          `json:"source_digest"`
	Name                 string          `json:"name"`
	Assertions           []TwinAssertion `json:"assertions"`
	Topics               []TwinTopic     `json:"topics"`
	Diff                 TwinDiff        `json:"diff"`
}

type TwinAssertionType string

const (
	TwinAssertionPreference TwinAssertionType = "preference"
	TwinAssertionConstraint TwinAssertionType = "constraint"
	TwinAssertionProcedure  TwinAssertionType = "procedure"
	TwinAssertionQualityBar TwinAssertionType = "quality_bar"
)

type TwinProvenanceKind string

const (
	TwinProvenanceModel                  TwinProvenanceKind = "model"
	TwinProvenanceDeterministicInventory TwinProvenanceKind = "deterministic_inventory"
	TwinProvenanceDeposition             TwinProvenanceKind = "deposition"
	TwinProvenanceHumanEdit              TwinProvenanceKind = "human_edit"
)

type TwinAssertionProvenance struct {
	Kind      TwinProvenanceKind `json:"kind"`
	Generator string             `json:"generator"`
}

type TwinAssertion struct {
	ID                string                     `json:"id"`
	Type              TwinAssertionType          `json:"type"`
	Text              string                     `json:"text"`
	Applicability     TwinAssertionApplicability `json:"applicability"`
	EvidenceCitations []string                   `json:"evidence_citations"`
	Confidence        float64                    `json:"confidence"`
	Provenance        TwinAssertionProvenance    `json:"provenance"`
}

type TwinProposalCandidate struct {
	Assertions []TwinAssertion
}

type TwinTopic struct {
	ID           string   `json:"id"`
	IssueID      string   `json:"issue_id"`
	IssueNumber  int32    `json:"issue_number"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	CitationKeys []string `json:"citation_keys"`
}

type TwinDiff struct {
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Changed   []string `json:"changed"`
	Unchanged []string `json:"unchanged"`
}

// BuildTwinProposal preserves the deterministic inventory projection for tests
// and compatibility. Production callers should use GenerateTwinProposal with a
// model-backed TwinProposalGenerator.
func BuildTwinProposal(input TwinBuilderInput) (TwinProposalBuild, error) {
	return GenerateTwinProposal(context.Background(), InventoryTwinProposalGenerator{}, TwinProposalGenerationInput{BuilderInput: input})
}

func ValidateTwinProposal(input TwinBuilderInput, candidate TwinProposalCandidate) (TwinProposalBuild, error) {
	if err := validateTwinInput(input); err != nil {
		return TwinProposalBuild{}, err
	}
	citations, err := twinCitationKeys(input.Citations)
	if err != nil {
		return TwinProposalBuild{}, err
	}
	if len(candidate.Assertions) > twinMaxAssertions {
		return TwinProposalBuild{}, &TwinAssertionError{Field: "assertions", Reason: fmt.Sprintf("count %d exceeds limit %d", len(candidate.Assertions), twinMaxAssertions), Cause: ErrTwinInvalidAssertion}
	}

	assertions := make([]TwinAssertion, 0, len(candidate.Assertions))
	seenIDs := make(map[string]struct{}, len(candidate.Assertions))
	for index, proposed := range candidate.Assertions {
		assertion, err := canonicalizeTwinAssertion(proposed, citations, input.Citations)
		if err != nil {
			return TwinProposalBuild{}, fmt.Errorf("validate generated assertion %d: %w", index, err)
		}
		if _, exists := seenIDs[assertion.ID]; exists {
			return TwinProposalBuild{}, &TwinAssertionError{ID: assertion.ID, Field: "id", Reason: "duplicate stable id", Cause: ErrTwinInvalidAssertion}
		}
		seenIDs[assertion.ID] = struct{}{}
		assertions = append(assertions, assertion)
	}
	sortTwinAssertions(assertions)
	content := TwinProposalContent{
		SchemaVersion:        twinProposalSchemaVersion,
		SourceWikiRevisionID: input.SourceWikiRevisionID,
		SourceDigest:         input.SourceDigest,
		Name:                 "Workspace Twin",
		Assertions:           assertions,
		Topics:               twinTopics(input.Content),
		Diff:                 diffTwinAssertions(assertions, input.PriorAssertions),
	}
	canonical, err := json.Marshal(content)
	if err != nil {
		return TwinProposalBuild{}, fmt.Errorf("marshal canonical twin proposal: %w", err)
	}
	if len(canonical) > twinMaxContentBytes {
		return TwinProposalBuild{}, &TwinSizeError{Limit: twinMaxContentBytes, Actual: len(canonical)}
	}
	return TwinProposalBuild{Content: content, CanonicalJSON: canonical, ContentDigest: digestTwin(canonical)}, nil
}

func validateTwinInput(input TwinBuilderInput) error {
	revisionID := strings.ToLower(strings.TrimSpace(input.SourceWikiRevisionID))
	if revisionID == "" || utf8.RuneCountInString(revisionID) > twinMaxCitationKeyRunes || !twinStableIDPattern.MatchString(revisionID) {
		return &TwinInputError{Field: "source wiki revision id"}
	}
	sourceDigest := strings.ToLower(strings.TrimSpace(input.SourceDigest))
	if sourceDigest == "" || utf8.RuneCountInString(sourceDigest) > twinMaxCitationKeyRunes || !twinStableIDPattern.MatchString(sourceDigest) {
		return &TwinInputError{Field: "source digest"}
	}
	evidenceSchemaVersion, canonicalEvidence, err := canonicalTwinEvidence(input)
	if err != nil {
		return err
	}
	if evidenceSchemaVersion != 1 && evidenceSchemaVersion != 2 {
		return &TwinInputError{Field: "wiki content schema version"}
	}
	if evidenceSchemaVersion == 1 {
		citations, err := twinCitationKeys(input.Citations)
		if err != nil {
			return err
		}
		if err := validateTwinEvidenceReferences(input.Content, citations); err != nil {
			return err
		}
	}
	if err := validateTwinCanonicalCitationKeys(canonicalEvidence, input.Citations); err != nil {
		return err
	}
	return nil
}

func canonicalTwinEvidence(input TwinBuilderInput) (int, json.RawMessage, error) {
	canonical := append(json.RawMessage(nil), input.CanonicalEvidence...)
	if len(canonical) == 0 {
		encoded, err := json.Marshal(input.Content)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal twin compatibility evidence: %w", err)
		}
		canonical = encoded
	}
	if len(canonical) > twinMaxContentBytes {
		return 0, nil, &TwinSizeError{Limit: twinMaxContentBytes, Actual: len(canonical)}
	}
	var envelope struct {
		SchemaVersion int `json:"schema_version"`
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	if err := decoder.Decode(&envelope); err != nil {
		return 0, nil, &TwinInputError{Field: "canonical evidence JSON object"}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return 0, nil, &TwinInputError{Field: "canonical evidence JSON object"}
	}
	schemaVersion := input.EvidenceSchemaVersion
	if schemaVersion == 0 {
		schemaVersion = envelope.SchemaVersion
	}
	if envelope.SchemaVersion != schemaVersion {
		return 0, nil, &TwinInputError{Field: "canonical evidence schema version"}
	}
	return schemaVersion, canonical, nil
}

func validateTwinCanonicalCitationKeys(canonical json.RawMessage, citations []LMWikiCitation) error {
	allowed, err := twinCitationKeys(citations)
	if err != nil {
		return err
	}
	var evidence any
	if err := json.Unmarshal(canonical, &evidence); err != nil {
		return &TwinInputError{Field: "canonical evidence JSON object"}
	}
	object, ok := evidence.(map[string]any)
	if !ok {
		return &TwinInputError{Field: "canonical evidence JSON object"}
	}
	var inspect func(any) error
	inspect = func(value any) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "citation_key" {
					citationKey, ok := child.(string)
					if !ok {
						return &TwinInputError{Field: "canonical evidence citation key"}
					}
					if _, exists := allowed[citationKey]; !exists {
						return &TwinCitationError{CitationKey: citationKey}
					}
				}
				if err := inspect(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := inspect(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return inspect(object)
}

func twinCitationKeys(citations []LMWikiCitation) (map[string]struct{}, error) {
	keys := make(map[string]struct{}, len(citations))
	for _, citation := range citations {
		key := strings.TrimSpace(citation.CitationKey)
		if key == "" {
			return nil, &TwinInputError{Field: "citation key"}
		}
		if utf8.RuneCountInString(key) > twinMaxCitationKeyRunes {
			return nil, &TwinInputError{Field: "citation key length"}
		}
		if key != strings.ToLower(key) || !twinStableIDPattern.MatchString(key) {
			return nil, &TwinInputError{Field: "citation key format"}
		}
		if _, exists := keys[key]; exists {
			return nil, &TwinInputError{Field: "duplicate citation key"}
		}
		keys[key] = struct{}{}
	}
	return keys, nil
}

func validateTwinEvidenceReferences(content LMWikiContent, citations map[string]struct{}) error {
	keys := make([]string, 0, len(content.Issues)+len(content.Projects)+len(content.ProjectResources)+len(content.AutopilotRuns))
	for _, item := range content.Issues {
		keys = append(keys, item.CitationKey)
	}
	for _, item := range content.Projects {
		keys = append(keys, item.CitationKey)
	}
	for _, item := range content.ProjectResources {
		keys = append(keys, item.CitationKey)
	}
	for _, item := range content.AutopilotRuns {
		keys = append(keys, item.CitationKey)
	}
	for _, key := range keys {
		if _, exists := citations[key]; !exists {
			return &TwinCitationError{CitationKey: key}
		}
	}
	return nil
}

func canonicalizeTwinAssertion(assertion TwinAssertion, citations map[string]struct{}, acceptedCitations []LMWikiCitation) (TwinAssertion, error) {
	assertion.ID = strings.ToLower(strings.TrimSpace(assertion.ID))
	if assertion.ID == "" || utf8.RuneCountInString(assertion.ID) > twinMaxAssertionIDRunes || !twinStableIDPattern.MatchString(assertion.ID) {
		return TwinAssertion{}, &TwinAssertionError{ID: assertion.ID, Field: "id", Reason: "must be a bounded lowercase stable identifier", Cause: ErrTwinInvalidAssertion}
	}
	assertion.Type = TwinAssertionType(strings.ToLower(strings.TrimSpace(string(assertion.Type))))
	if !allowedTwinAssertionType(assertion.Type) {
		return TwinAssertion{}, &TwinAssertionError{ID: assertion.ID, Field: "type", Reason: fmt.Sprintf("%q is not allowed", assertion.Type), Cause: ErrTwinInvalidAssertion}
	}

	var err error
	assertion.Text, err = canonicalTwinText(assertion.ID, "text", assertion.Text, twinMaxAssertionTextRunes)
	if err != nil {
		return TwinAssertion{}, err
	}
	if math.IsNaN(assertion.Confidence) || math.IsInf(assertion.Confidence, 0) || assertion.Confidence <= 0 || assertion.Confidence > 1 {
		return TwinAssertion{}, &TwinAssertionError{ID: assertion.ID, Field: "confidence", Reason: "must be greater than 0 and at most 1", Cause: ErrTwinInvalidAssertion}
	}
	assertion.Provenance.Kind = TwinProvenanceKind(strings.ToLower(strings.TrimSpace(string(assertion.Provenance.Kind))))
	if !allowedTwinProvenance(assertion.Provenance.Kind) {
		return TwinAssertion{}, &TwinAssertionError{ID: assertion.ID, Field: "provenance.kind", Reason: fmt.Sprintf("%q is not allowed", assertion.Provenance.Kind), Cause: ErrTwinInvalidAssertion}
	}
	assertion.Provenance.Generator = strings.ToLower(strings.TrimSpace(assertion.Provenance.Generator))
	if assertion.Provenance.Generator == "" || utf8.RuneCountInString(assertion.Provenance.Generator) > twinMaxProvenanceGeneratorRunes || !twinStableIDPattern.MatchString(assertion.Provenance.Generator) {
		return TwinAssertion{}, &TwinAssertionError{ID: assertion.ID, Field: "provenance.generator", Reason: "must be a bounded lowercase stable identifier", Cause: ErrTwinInvalidAssertion}
	}
	assertion.Applicability, err = canonicalizeTwinApplicability(assertion.ID, assertion.Provenance.Kind, assertion.Applicability, acceptedCitations)
	if err != nil {
		return TwinAssertion{}, err
	}
	if len(assertion.EvidenceCitations) == 0 || len(assertion.EvidenceCitations) > twinMaxCitationsPerAssertion {
		return TwinAssertion{}, &TwinAssertionError{ID: assertion.ID, Field: "evidence_citations", Reason: fmt.Sprintf("must contain between 1 and %d citations", twinMaxCitationsPerAssertion), Cause: ErrTwinInvalidAssertion}
	}
	canonicalCitations := make([]string, 0, len(assertion.EvidenceCitations))
	seenCitations := make(map[string]struct{}, len(assertion.EvidenceCitations))
	for _, rawKey := range assertion.EvidenceCitations {
		key := strings.TrimSpace(rawKey)
		if _, exists := citations[key]; !exists {
			return TwinAssertion{}, &TwinCitationError{CitationKey: key}
		}
		if _, exists := seenCitations[key]; exists {
			continue
		}
		seenCitations[key] = struct{}{}
		canonicalCitations = append(canonicalCitations, key)
	}
	sort.Strings(canonicalCitations)
	assertion.EvidenceCitations = canonicalCitations
	return assertion, nil
}

func canonicalizeTwinApplicability(id string, provenance TwinProvenanceKind, applicability TwinAssertionApplicability, citations []LMWikiCitation) (TwinAssertionApplicability, error) {
	if provenance != TwinProvenanceDeposition && (strings.TrimSpace(applicability.TaskID) != "" || strings.TrimSpace(applicability.WorkspaceID) != "" || strings.TrimSpace(applicability.AgentID) != "") {
		return TwinAssertionApplicability{}, &TwinAssertionError{ID: id, Field: "applicability", Reason: "task_id, workspace_id, and agent_id require trusted deposition context", Cause: ErrTwinInvalidAssertion}
	}
	for _, scopedID := range []struct {
		field string
		value *string
	}{
		{field: "task_id", value: &applicability.TaskID},
		{field: "workspace_id", value: &applicability.WorkspaceID},
		{field: "agent_id", value: &applicability.AgentID},
		{field: "project_id", value: &applicability.ProjectID},
		{field: "issue_id", value: &applicability.IssueID},
	} {
		canonical, err := canonicalTwinApplicabilityID(id, scopedID.field, *scopedID.value)
		if err != nil {
			return TwinAssertionApplicability{}, err
		}
		*scopedID.value = canonical
	}
	if applicability.IssueID != "" && !twinScopeHasAcceptedCitation(citations, "issue", applicability.IssueID) {
		return TwinAssertionApplicability{}, &TwinAssertionError{ID: id, Field: "applicability.issue_id", Reason: "does not match accepted issue evidence", Cause: ErrTwinCitationMissing}
	}
	if applicability.ProjectID != "" && !twinScopeHasAcceptedCitation(citations, "project", applicability.ProjectID) {
		return TwinAssertionApplicability{}, &TwinAssertionError{ID: id, Field: "applicability.project_id", Reason: "does not match accepted project evidence", Cause: ErrTwinCitationMissing}
	}
	if len(applicability.Keywords) > twinMaxApplicabilityKeywords {
		return TwinAssertionApplicability{}, &TwinAssertionError{ID: id, Field: "applicability.keywords", Reason: fmt.Sprintf("count exceeds %d", twinMaxApplicabilityKeywords), Cause: ErrTwinInvalidAssertion}
	}
	keywords := make([]string, 0, len(applicability.Keywords))
	seen := make(map[string]struct{}, len(applicability.Keywords))
	for _, keyword := range applicability.Keywords {
		canonical, err := canonicalTwinText(id, "applicability.keyword", strings.ToLower(keyword), twinMaxApplicabilityKeywordRunes)
		if err != nil {
			return TwinAssertionApplicability{}, err
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		keywords = append(keywords, canonical)
	}
	sort.Strings(keywords)
	applicability.Keywords = keywords
	return applicability, nil
}

func canonicalTwinApplicabilityID(id, field, value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > twinMaxApplicabilityIDRunes || !twinStableIDPattern.MatchString(value) {
		return "", &TwinAssertionError{ID: id, Field: "applicability." + field, Reason: "must be a bounded stable identifier", Cause: ErrTwinInvalidAssertion}
	}
	if reason := unsafeTwinTextReason(value); reason != "" {
		return "", &TwinAssertionError{ID: id, Field: "applicability." + field, Reason: reason, Cause: ErrTwinUnsafeAssertion}
	}
	return value, nil
}

func twinScopeHasAcceptedCitation(citations []LMWikiCitation, sourceType, sourceID string) bool {
	for _, citation := range citations {
		if strings.EqualFold(strings.TrimSpace(citation.SourceType), sourceType) && strings.EqualFold(strings.TrimSpace(citation.SourceID), sourceID) {
			return true
		}
	}
	return false
}

func canonicalTwinText(id, field, value string, limit int) (string, error) {
	for _, char := range value {
		if unicode.IsControl(char) && !unicode.IsSpace(char) {
			return "", &TwinAssertionError{ID: id, Field: field, Reason: "contains control characters", Cause: ErrTwinUnsafeAssertion}
		}
	}
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "", &TwinAssertionError{ID: id, Field: field, Reason: "is required", Cause: ErrTwinInvalidAssertion}
	}
	if utf8.RuneCountInString(value) > limit {
		return "", &TwinAssertionError{ID: id, Field: field, Reason: fmt.Sprintf("exceeds %d code points", limit), Cause: ErrTwinInvalidAssertion}
	}
	if reason := unsafeTwinTextReason(value); reason != "" {
		return "", &TwinAssertionError{ID: id, Field: field, Reason: reason, Cause: ErrTwinUnsafeAssertion}
	}
	return value, nil
}

func unsafeTwinTextReason(value string) string {
	if twinIdentityClaimPattern.MatchString(value) {
		return "contains a persona or identity claim"
	}
	if twinSecretPattern.MatchString(value) {
		return "contains secret-like material"
	}
	if twinUnixPathPattern.MatchString(value) || twinWindowsPathPattern.MatchString(value) {
		return "contains a raw filesystem path"
	}
	if twinProfileLeakPattern.MatchString(value) {
		return "contains a private profile reference"
	}
	return ""
}

func allowedTwinAssertionType(assertionType TwinAssertionType) bool {
	switch assertionType {
	case TwinAssertionPreference, TwinAssertionConstraint, TwinAssertionProcedure, TwinAssertionQualityBar:
		return true
	default:
		return false
	}
}

func allowedTwinProvenance(kind TwinProvenanceKind) bool {
	switch kind {
	case TwinProvenanceModel, TwinProvenanceDeterministicInventory, TwinProvenanceDeposition, TwinProvenanceHumanEdit:
		return true
	default:
		return false
	}
}

func twinTopics(content LMWikiContent) []TwinTopic {
	topics := make([]TwinTopic, 0, len(content.Issues))
	for _, item := range content.Issues {
		if isTerminalTwinIssue(item.Status) {
			continue
		}
		canonical, err := json.Marshal(item)
		if err != nil {
			continue
		}
		topics = append(topics, TwinTopic{
			ID:           digestTwin([]byte(item.CitationKey + string(canonical))),
			IssueID:      item.ID,
			IssueNumber:  item.Number,
			Title:        item.Title,
			Status:       item.Status,
			CitationKeys: []string{item.CitationKey},
		})
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].ID < topics[j].ID })
	return topics
}

func sortTwinAssertions(assertions []TwinAssertion) {
	sort.Slice(assertions, func(i, j int) bool { return assertions[i].ID < assertions[j].ID })
}

func diffTwinAssertions(assertions, prior []TwinAssertion) TwinDiff {
	currentByID := make(map[string]TwinAssertion, len(assertions))
	priorByID := make(map[string]TwinAssertion, len(prior))
	for _, assertion := range assertions {
		currentByID[assertion.ID] = assertion
	}
	for _, assertion := range prior {
		id := strings.ToLower(strings.TrimSpace(assertion.ID))
		if id != "" {
			assertion.ID = id
			priorByID[id] = assertion
		}
	}
	diff := TwinDiff{Added: []string{}, Removed: []string{}, Changed: []string{}, Unchanged: []string{}}
	for id, current := range currentByID {
		previous, exists := priorByID[id]
		if !exists {
			diff.Added = append(diff.Added, id)
			continue
		}
		if twinAssertionsEqual(current, previous) {
			diff.Unchanged = append(diff.Unchanged, id)
		} else {
			diff.Changed = append(diff.Changed, id)
		}
	}
	for id := range priorByID {
		if _, exists := currentByID[id]; !exists {
			diff.Removed = append(diff.Removed, id)
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Changed)
	sort.Strings(diff.Unchanged)
	return diff
}

func twinAssertionsEqual(left, right TwinAssertion) bool {
	left.ID, right.ID = "", ""
	left.EvidenceCitations = append([]string(nil), left.EvidenceCitations...)
	right.EvidenceCitations = append([]string(nil), right.EvidenceCitations...)
	sort.Strings(left.EvidenceCitations)
	sort.Strings(right.EvidenceCitations)
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func isTerminalTwinIssue(status string) bool {
	return status == "done" || status == "cancelled"
}

func digestTwin(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
