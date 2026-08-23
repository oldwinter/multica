package room

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"
)

const (
	RoomSynthesisSchemaVersion = 1
	maxSynthesisBytes          = 262144
	maxSynthesisSummaryRunes   = 20000
	maxSynthesisItemRunes      = 10000
	maxSynthesisItems          = 100
	maxRecommendationBodyRunes = 200000
)

var ErrInvalidSynthesis = errors.New("invalid room synthesis")

type SynthesisItem struct {
	Text             string   `json:"text"`
	CitationEntryIDs []string `json:"citation_entry_ids"`
	Confidence       float64  `json:"confidence"`
}

type ArtifactRecommendation struct {
	Key              string   `json:"key"`
	Kind             string   `json:"kind"`
	Title            string   `json:"title"`
	Body             string   `json:"body"`
	Rationale        string   `json:"rationale"`
	CitationEntryIDs []string `json:"citation_entry_ids"`
	Confidence       float64  `json:"confidence"`
}

type Synthesis struct {
	SchemaVersion   int                      `json:"schema_version"`
	Summary         string                   `json:"summary"`
	Facts           []SynthesisItem          `json:"facts"`
	Decisions       []SynthesisItem          `json:"decisions"`
	OpenQuestions   []SynthesisItem          `json:"open_questions"`
	Disagreements   []SynthesisItem          `json:"disagreements"`
	ActionItems     []SynthesisItem          `json:"action_items"`
	Recommendations []ArtifactRecommendation `json:"recommendations"`
	Confidence      float64                  `json:"confidence"`
}

func ValidateSynthesis(raw []byte, allowedEntryIDs map[string]struct{}) (Synthesis, []byte, string, error) {
	synthesis, err := decodeSynthesis(raw)
	if err != nil {
		return Synthesis{}, nil, "", err
	}
	return validateDecodedSynthesis(synthesis, allowedEntryIDs)
}

func synthesisCitationIDs(raw []byte) ([]string, error) {
	synthesis, err := decodeSynthesis(raw)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for _, group := range [][]SynthesisItem{
		synthesis.Facts, synthesis.Decisions, synthesis.OpenQuestions,
		synthesis.Disagreements, synthesis.ActionItems,
	} {
		for _, item := range group {
			ids = append(ids, item.CitationEntryIDs...)
		}
	}
	for _, recommendation := range synthesis.Recommendations {
		ids = append(ids, recommendation.CitationEntryIDs...)
	}
	return ids, nil
}

func decodeSynthesis(raw []byte) (Synthesis, error) {
	if len(raw) == 0 || len(raw) > maxSynthesisBytes {
		return Synthesis{}, fmt.Errorf("synthesis size: %w", ErrInvalidSynthesis)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var synthesis Synthesis
	if err := decoder.Decode(&synthesis); err != nil {
		return Synthesis{}, fmt.Errorf("decode synthesis: %w: %v", ErrInvalidSynthesis, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Synthesis{}, fmt.Errorf("trailing synthesis data: %w", ErrInvalidSynthesis)
	}
	return synthesis, nil
}

func validateDecodedSynthesis(synthesis Synthesis, allowedEntryIDs map[string]struct{}) (Synthesis, []byte, string, error) {
	if synthesis.SchemaVersion != RoomSynthesisSchemaVersion {
		return Synthesis{}, nil, "", fmt.Errorf("synthesis schema version %d: %w", synthesis.SchemaVersion, ErrInvalidSynthesis)
	}
	synthesis.Summary = strings.TrimSpace(synthesis.Summary)
	if synthesis.Summary == "" || utf8.RuneCountInString(synthesis.Summary) > maxSynthesisSummaryRunes {
		return Synthesis{}, nil, "", fmt.Errorf("synthesis summary: %w", ErrInvalidSynthesis)
	}
	if !validConfidence(synthesis.Confidence) {
		return Synthesis{}, nil, "", fmt.Errorf("synthesis confidence: %w", ErrInvalidSynthesis)
	}
	itemGroups := [][]SynthesisItem{
		synthesis.Facts, synthesis.Decisions, synthesis.OpenQuestions,
		synthesis.Disagreements, synthesis.ActionItems,
	}
	itemCount := len(synthesis.Recommendations)
	for _, group := range itemGroups {
		itemCount += len(group)
		for index := range group {
			item := &group[index]
			item.Text = strings.TrimSpace(item.Text)
			normalizeCitations(item.CitationEntryIDs)
			if err := validateSynthesisItem(*item, allowedEntryIDs); err != nil {
				return Synthesis{}, nil, "", err
			}
		}
	}
	if itemCount == 0 || itemCount > maxSynthesisItems {
		return Synthesis{}, nil, "", fmt.Errorf("synthesis item count: %w", ErrInvalidSynthesis)
	}
	for index := range synthesis.Recommendations {
		recommendation := &synthesis.Recommendations[index]
		recommendation.Key = ""
		recommendation.Kind = strings.TrimSpace(recommendation.Kind)
		recommendation.Title = strings.TrimSpace(recommendation.Title)
		recommendation.Body = strings.TrimSpace(recommendation.Body)
		recommendation.Rationale = strings.TrimSpace(recommendation.Rationale)
		normalizeCitations(recommendation.CitationEntryIDs)
		if recommendation.Kind != "issue" && recommendation.Kind != "wiki" && recommendation.Kind != "decision" {
			return Synthesis{}, nil, "", fmt.Errorf("recommendation kind: %w", ErrInvalidSynthesis)
		}
		if recommendation.Title == "" || utf8.RuneCountInString(recommendation.Title) > 300 ||
			recommendation.Body == "" || utf8.RuneCountInString(recommendation.Body) > maxRecommendationBodyRunes ||
			utf8.RuneCountInString(recommendation.Rationale) > 20000 || !validConfidence(recommendation.Confidence) {
			return Synthesis{}, nil, "", fmt.Errorf("recommendation content: %w", ErrInvalidSynthesis)
		}
		if err := validateCitations(recommendation.CitationEntryIDs, allowedEntryIDs); err != nil {
			return Synthesis{}, nil, "", err
		}
	}
	withoutKeys, err := json.Marshal(synthesis)
	if err != nil {
		return Synthesis{}, nil, "", fmt.Errorf("encode synthesis: %w", err)
	}
	base := sha256.Sum256(withoutKeys)
	for index := range synthesis.Recommendations {
		recommendation := &synthesis.Recommendations[index]
		canonical, marshalErr := json.Marshal(recommendation)
		if marshalErr != nil {
			return Synthesis{}, nil, "", fmt.Errorf("encode recommendation: %w", marshalErr)
		}
		keyInput := append([]byte(hex.EncodeToString(base[:])+fmt.Sprintf(":%d:", index)), canonical...)
		key := sha256.Sum256(keyInput)
		recommendation.Key = "sha256:" + hex.EncodeToString(key[:])
	}
	canonical, err := json.Marshal(synthesis)
	if err != nil {
		return Synthesis{}, nil, "", fmt.Errorf("encode canonical synthesis: %w", err)
	}
	digestBytes := sha256.Sum256(canonical)
	return synthesis, canonical, "sha256:" + hex.EncodeToString(digestBytes[:]), nil
}

func validateSynthesisItem(item SynthesisItem, allowedEntryIDs map[string]struct{}) error {
	if strings.TrimSpace(item.Text) == "" || utf8.RuneCountInString(strings.TrimSpace(item.Text)) > maxSynthesisItemRunes || !validConfidence(item.Confidence) {
		return fmt.Errorf("synthesis item: %w", ErrInvalidSynthesis)
	}
	return validateCitations(item.CitationEntryIDs, allowedEntryIDs)
}

func validateCitations(citations []string, allowedEntryIDs map[string]struct{}) error {
	if len(citations) == 0 || len(citations) > 100 {
		return fmt.Errorf("synthesis citations: %w", ErrInvalidSynthesis)
	}
	seen := make(map[string]struct{}, len(citations))
	for _, citation := range citations {
		citation = strings.TrimSpace(citation)
		if _, ok := allowedEntryIDs[citation]; !ok {
			return fmt.Errorf("synthesis citation %q: %w", citation, ErrInvalidSynthesis)
		}
		if _, duplicate := seen[citation]; duplicate {
			return fmt.Errorf("duplicate synthesis citation %q: %w", citation, ErrInvalidSynthesis)
		}
		seen[citation] = struct{}{}
	}
	return nil
}

func normalizeCitations(citations []string) {
	for index := range citations {
		citations[index] = strings.TrimSpace(citations[index])
	}
}

func validConfidence(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func FindRecommendation(synthesis Synthesis, key string) (ArtifactRecommendation, bool) {
	for _, recommendation := range synthesis.Recommendations {
		if recommendation.Key == key {
			return recommendation, true
		}
	}
	return ArtifactRecommendation{}, false
}
