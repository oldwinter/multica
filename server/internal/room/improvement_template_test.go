package room

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestImprovementRoomTemplateIsWhitelisted(t *testing.T) {
	if !validRoomTemplate("improvement") {
		t.Fatal("improvement Room template is not whitelisted")
	}
	if validRoomTemplate("future-template") {
		t.Fatal("unknown Room template was accepted")
	}
}

func TestRoomSynthesisPromptUsesClosedRecommendationTaxonomy(t *testing.T) {
	prompt := synthesisInstructions(db.Room{})
	for _, target := range []RecommendationTarget{
		RecommendationTargetKnowledge,
		RecommendationTargetPreference,
		RecommendationTargetConstraint,
		RecommendationTargetExecutableProcedure,
		RecommendationTargetImplementationDefect,
		RecommendationTargetDecision,
		RecommendationTargetUnsupported,
	} {
		if !strings.Contains(prompt, string(target)) {
			t.Errorf("synthesis prompt omits recommendation target %q", target)
		}
	}
}

func TestImprovementSynthesisPromptCarriesParseableVersionedSkillCandidate(t *testing.T) {
	type file struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	type bundle struct {
		ID          string `json:"id"`
		Source      string `json:"source"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Files       []file `json:"files"`
	}
	type body struct {
		SchemaVersion   int      `json:"schema_version"`
		BaseSkillID     string   `json:"base_skill_id"`
		BaseHash        string   `json:"base_hash"`
		Bundle          bundle   `json:"bundle"`
		ObservedPattern string   `json:"observed_pattern"`
		ExpectedBenefit string   `json:"expected_benefit"`
		RegressionRisk  string   `json:"regression_risk"`
		EvidenceDigests []string `json:"evidence_digests"`
	}

	decoder := json.NewDecoder(strings.NewReader(skillImprovementRecommendationBodyExample))
	decoder.DisallowUnknownFields()
	var candidate body
	if err := decoder.Decode(&candidate); err != nil {
		t.Fatalf("decode executable_procedure body fixture: %v", err)
	}
	digest := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	if candidate.SchemaVersion != 1 || candidate.BaseSkillID == "" || !digest.MatchString(candidate.BaseHash) ||
		candidate.Bundle.ID != candidate.BaseSkillID || candidate.Bundle.Source != "workspace" ||
		candidate.Bundle.Name == "" || candidate.Bundle.Content == "" || len(candidate.Bundle.Files) == 0 ||
		candidate.ObservedPattern == "" || candidate.ExpectedBenefit == "" || candidate.RegressionRisk == "" ||
		len(candidate.EvidenceDigests) == 0 || !digest.MatchString(candidate.EvidenceDigests[0]) {
		t.Fatalf("incomplete executable_procedure body fixture: %+v", candidate)
	}

	prompt := synthesisInstructions(db.Room{TemplateID: pgtype.Text{String: "improvement", Valid: true}})
	for _, required := range []string{
		"exactly these fields and no others",
		"base_skill_id",
		"base_hash",
		"evidence_digests",
		"human promotes this recommendation",
		"use kind unsupported",
		skillImprovementRecommendationBodyExample,
	} {
		if !strings.Contains(prompt, required) {
			t.Errorf("improvement synthesis prompt omits %q", required)
		}
	}
	if strings.Contains(synthesisInstructions(db.Room{}), skillImprovementRecommendationBodyExample) {
		t.Fatal("non-improvement Room received the Skill candidate body contract")
	}
}
