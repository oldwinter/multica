package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const twinMaxContentBytes = 2 * 1024 * 1024

var (
	ErrTwinInvalidInput    = errors.New("invalid twin builder input")
	ErrTwinCitationMissing = errors.New("twin assertion citation is missing")
	ErrTwinContentTooLarge = errors.New("twin content exceeds size limit")
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

type TwinBuilderInput struct {
	SourceWikiRevisionID string
	SourceDigest         string
	Content              LMWikiContent
	Citations            []LMWikiCitation
	PriorAssertions      []TwinAssertion
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

type TwinAssertion struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	SourceSummary string   `json:"source_summary"`
	SourceStatus  string   `json:"source_status"`
	CitationKeys  []string `json:"citation_keys"`
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
	Unchanged []string `json:"unchanged"`
}

type twinAssertionSource struct {
	CitationKey string
	Text        string
	Summary     string
	Status      string
}

func BuildTwinProposal(input TwinBuilderInput) (TwinProposalBuild, error) {
	if err := validateTwinInput(input); err != nil {
		return TwinProposalBuild{}, err
	}
	citations, err := twinCitationKeys(input.Citations)
	if err != nil {
		return TwinProposalBuild{}, err
	}
	assertions := make([]TwinAssertion, 0, len(input.Content.Issues)+len(input.Content.Projects)+len(input.Content.ProjectResources)+len(input.Content.AutopilotRuns))
	topics := make([]TwinTopic, 0, len(input.Content.Issues))
	for _, item := range input.Content.Issues {
		assertion, err := newTwinAssertion(citations, item, twinAssertionSource{CitationKey: item.CitationKey, Text: fmt.Sprintf("Issue %d: %s", item.Number, item.Title), Summary: item.Description, Status: item.Status})
		if err != nil {
			return TwinProposalBuild{}, err
		}
		assertions = append(assertions, assertion)
		if !isTerminalTwinIssue(item.Status) {
			topics = append(topics, TwinTopic{ID: assertion.ID, IssueID: item.ID, IssueNumber: item.Number, Title: item.Title, Status: item.Status, CitationKeys: assertion.CitationKeys})
		}
	}
	for _, item := range input.Content.Projects {
		assertion, err := newTwinAssertion(citations, item, twinAssertionSource{CitationKey: item.CitationKey, Text: "Project: " + item.Title, Summary: item.Description, Status: item.Status})
		if err != nil {
			return TwinProposalBuild{}, err
		}
		assertions = append(assertions, assertion)
	}
	for _, item := range input.Content.ProjectResources {
		assertion, err := newTwinAssertion(citations, item, twinAssertionSource{CitationKey: item.CitationKey, Text: "Repository: " + item.Ref.Host + "/" + item.Ref.RepositoryPath, Summary: item.Label})
		if err != nil {
			return TwinProposalBuild{}, err
		}
		assertions = append(assertions, assertion)
	}
	for _, item := range input.Content.AutopilotRuns {
		assertion, err := newTwinAssertion(citations, item, twinAssertionSource{CitationKey: item.CitationKey, Text: "Autopilot " + item.AutopilotTitle + " completed", Summary: item.AutopilotTitle, Status: "completed"})
		if err != nil {
			return TwinProposalBuild{}, err
		}
		assertions = append(assertions, assertion)
	}
	sortTwinAssertions(assertions)
	sort.Slice(topics, func(i, j int) bool { return topics[i].ID < topics[j].ID })
	content := TwinProposalContent{SchemaVersion: 1, SourceWikiRevisionID: input.SourceWikiRevisionID, SourceDigest: input.SourceDigest, Name: "Workspace Twin", Assertions: assertions, Topics: topics, Diff: diffTwinAssertions(assertions, input.PriorAssertions)}
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
	if input.SourceWikiRevisionID == "" {
		return &TwinInputError{Field: "source wiki revision id"}
	}
	if input.SourceDigest == "" {
		return &TwinInputError{Field: "source digest"}
	}
	if input.Content.SchemaVersion != 1 {
		return &TwinInputError{Field: "wiki content schema version"}
	}
	return nil
}

func twinCitationKeys(citations []LMWikiCitation) (map[string]struct{}, error) {
	keys := make(map[string]struct{}, len(citations))
	for _, citation := range citations {
		if citation.CitationKey == "" {
			return nil, &TwinInputError{Field: "citation key"}
		}
		if _, exists := keys[citation.CitationKey]; exists {
			return nil, &TwinInputError{Field: "duplicate citation key"}
		}
		keys[citation.CitationKey] = struct{}{}
	}
	return keys, nil
}

func newTwinAssertion[T comparable](citations map[string]struct{}, item T, source twinAssertionSource) (TwinAssertion, error) {
	if _, exists := citations[source.CitationKey]; !exists {
		return TwinAssertion{}, &TwinCitationError{CitationKey: source.CitationKey}
	}
	canonical, err := json.Marshal(item)
	if err != nil {
		return TwinAssertion{}, fmt.Errorf("marshal twin assertion item: %w", err)
	}
	return TwinAssertion{ID: digestTwin([]byte(source.CitationKey + string(canonical))), Text: source.Text, SourceSummary: source.Summary, SourceStatus: source.Status, CitationKeys: []string{source.CitationKey}}, nil
}

func sortTwinAssertions(assertions []TwinAssertion) {
	sort.Slice(assertions, func(i, j int) bool { return assertions[i].ID < assertions[j].ID })
}

func diffTwinAssertions(assertions, prior []TwinAssertion) TwinDiff {
	currentIDs := make(map[string]struct{}, len(assertions))
	priorIDs := make(map[string]struct{}, len(prior))
	for _, assertion := range assertions {
		currentIDs[assertion.ID] = struct{}{}
	}
	for _, assertion := range prior {
		priorIDs[assertion.ID] = struct{}{}
	}
	diff := TwinDiff{Added: make([]string, 0), Removed: make([]string, 0), Unchanged: make([]string, 0)}
	for id := range currentIDs {
		if _, exists := priorIDs[id]; exists {
			diff.Unchanged = append(diff.Unchanged, id)
		} else {
			diff.Added = append(diff.Added, id)
		}
	}
	for id := range priorIDs {
		if _, exists := currentIDs[id]; !exists {
			diff.Removed = append(diff.Removed, id)
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Unchanged)
	return diff
}

func isTerminalTwinIssue(status string) bool {
	return status == "done" || status == "cancelled"
}

func digestTwin(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
