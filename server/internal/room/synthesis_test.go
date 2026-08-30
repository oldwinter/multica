package room

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func validSynthesisJSON(citation string) []byte {
	payload := Synthesis{
		SchemaVersion: RoomSynthesisSchemaVersion,
		Summary:       "The review found one actionable risk.",
		Facts: []SynthesisItem{{
			Text: "Retries can duplicate the write.", CitationEntryIDs: []string{citation}, Confidence: 0.9,
		}},
		Decisions:     []SynthesisItem{},
		OpenQuestions: []SynthesisItem{},
		Disagreements: []SynthesisItem{},
		ActionItems:   []SynthesisItem{},
		Recommendations: []ArtifactRecommendation{{
			Kind: string(RecommendationTargetImplementationDefect), Title: "Make the write idempotent", Body: "Add a durable request key.",
			Rationale:        "The participant supplied a concrete failure mode.",
			CitationEntryIDs: []string{citation}, Confidence: 0.8,
		}},
		Confidence: 0.85,
	}
	encoded, _ := json.Marshal(payload)
	return encoded
}

func TestValidateSynthesisAcceptsClosedRecommendationTaxonomy(t *testing.T) {
	const entryID = "5b592564-7835-4fde-83b1-4a3c5166db45"
	allowed := map[string]struct{}{entryID: {}}
	targets := []RecommendationTarget{
		RecommendationTargetKnowledge,
		RecommendationTargetPreference,
		RecommendationTargetConstraint,
		RecommendationTargetExecutableProcedure,
		RecommendationTargetImplementationDefect,
		RecommendationTargetDecision,
		RecommendationTargetUnsupported,
	}

	for _, target := range targets {
		t.Run(string(target), func(t *testing.T) {
			var synthesis Synthesis
			if err := json.Unmarshal(validSynthesisJSON(entryID), &synthesis); err != nil {
				t.Fatal(err)
			}
			synthesis.Recommendations[0].Kind = string(target)
			payload, err := json.Marshal(synthesis)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, _, err := ValidateSynthesis(payload, allowed); err != nil {
				t.Fatalf("target %q rejected: %v", target, err)
			}
		})
	}
}

func TestValidateSynthesisRejectsUnknownRecommendationTarget(t *testing.T) {
	const entryID = "5b592564-7835-4fde-83b1-4a3c5166db45"
	var synthesis Synthesis
	if err := json.Unmarshal(validSynthesisJSON(entryID), &synthesis); err != nil {
		t.Fatal(err)
	}
	synthesis.Recommendations[0].Kind = "future_target"
	payload, err := json.Marshal(synthesis)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = ValidateSynthesis(payload, map[string]struct{}{entryID: {}})
	if !errors.Is(err, ErrInvalidSynthesis) {
		t.Fatalf("error = %v, want ErrInvalidSynthesis", err)
	}
}

func TestValidateSynthesisCanonicalizesDigestAndRecommendationKey(t *testing.T) {
	const entryID = "5b592564-7835-4fde-83b1-4a3c5166db45"
	allowed := map[string]struct{}{entryID: {}}

	first, canonical, digest, err := ValidateSynthesis(validSynthesisJSON(entryID), allowed)
	if err != nil {
		t.Fatal(err)
	}
	second, secondCanonical, secondDigest, err := ValidateSynthesis(validSynthesisJSON(entryID), allowed)
	if err != nil {
		t.Fatal(err)
	}
	if digest != secondDigest || !bytes.Equal(canonical, secondCanonical) {
		t.Fatalf("canonical synthesis is not deterministic: %s / %s", digest, secondDigest)
	}
	if !strings.HasPrefix(digest, "sha256:") || len(first.Recommendations) != 1 ||
		!strings.HasPrefix(first.Recommendations[0].Key, "sha256:") || first.Recommendations[0].Key != second.Recommendations[0].Key {
		t.Fatalf("missing stable digest/key: digest=%q recommendations=%+v", digest, first.Recommendations)
	}
}

func TestValidateSynthesisRejectsInvalidContracts(t *testing.T) {
	const entryID = "5b592564-7835-4fde-83b1-4a3c5166db45"
	allowed := map[string]struct{}{entryID: {}}
	cases := map[string][]byte{
		"cross room citation": validSynthesisJSON("b5ac2ffd-8405-42aa-a0b6-9b1d68a821ab"),
		"oversize":            []byte(`{"schema_version":1,"summary":"` + strings.Repeat("x", maxSynthesisBytes) + `"}`),
		"unknown field":       []byte(`{"schema_version":1,"summary":"ok","facts":[],"decisions":[],"open_questions":[],"disagreements":[],"action_items":[],"recommendations":[],"confidence":1,"private_path":"/tmp/secret"}`),
	}
	wrongVersion := validSynthesisJSON(entryID)
	wrongVersion = bytes.Replace(wrongVersion, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1)
	cases["wrong version"] = wrongVersion
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := ValidateSynthesis(raw, allowed)
			if !errors.Is(err, ErrInvalidSynthesis) {
				t.Fatalf("error = %v, want ErrInvalidSynthesis", err)
			}
		})
	}
}

func TestValidateSynthesisRejectsDuplicateAndMissingCitations(t *testing.T) {
	const entryID = "5b592564-7835-4fde-83b1-4a3c5166db45"
	allowed := map[string]struct{}{entryID: {}}
	for name, citations := range map[string][]string{
		"missing":   {},
		"duplicate": {entryID, entryID},
	} {
		t.Run(name, func(t *testing.T) {
			payload := validSynthesisJSON(entryID)
			var synthesis Synthesis
			if err := json.Unmarshal(payload, &synthesis); err != nil {
				t.Fatal(err)
			}
			synthesis.Facts[0].CitationEntryIDs = citations
			payload, _ = json.Marshal(synthesis)
			_, _, _, err := ValidateSynthesis(payload, allowed)
			if !errors.Is(err, ErrInvalidSynthesis) {
				t.Fatalf("error = %v, want ErrInvalidSynthesis", err)
			}
		})
	}
}
