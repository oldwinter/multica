package skillevolution

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestSkillEvolutionMetricsAreContentFreeAndCoverLifecycleOutcomes(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordEligibleSignals(3)
	metrics.RecordProposalGenerated()
	metrics.RecordValidationFailure()
	metrics.RecordProposalAccepted()
	metrics.RecordPublication(false)
	metrics.RecordPublication(true)
	metrics.RecordFeedbackCoverage(false)
	metrics.RecordFeedbackCoverage(true)
	metrics.RecordManifestCoverage(false)
	metrics.RecordManifestCoverage(true)
	metrics.RecordRevision()
	metrics.RecordCost(42)
	metrics.RecordLatency(1500 * time.Millisecond)

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(metrics.Collectors()...)
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(families) != len(metrics.Collectors()) {
		t.Fatalf("metric families = %d, want %d", len(families), len(metrics.Collectors()))
	}
	for _, family := range families {
		searchable := family.GetName() + " " + family.GetHelp()
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() != "le" {
					t.Fatalf("metric %s has custom label %q", family.GetName(), label.GetName())
				}
				searchable += " " + label.GetName() + " " + label.GetValue()
			}
		}
		for _, forbidden := range []string{"workspace_id", "skill_id", "source_id", "path", "prompt", "output", "feedback_note"} {
			if strings.Contains(searchable, forbidden) {
				t.Errorf("metric %s contains forbidden content key %q", family.GetName(), forbidden)
			}
		}
	}

	wantCounters := map[string]float64{
		"multica_skill_evolution_proposals_accepted_total":       1,
		"multica_skill_evolution_publications_total":             1,
		"multica_skill_evolution_rollbacks_total":                1,
		"multica_skill_evolution_feedback_eligible_runs_total":   2,
		"multica_skill_evolution_feedback_covered_runs_total":    1,
		"multica_skill_evolution_manifest_eligible_runs_total":   2,
		"multica_skill_evolution_manifest_attributed_runs_total": 1,
		"multica_skill_evolution_revisions_total":                1,
		"multica_skill_evolution_cost_usd_ticks_total":           42,
	}
	for _, family := range families {
		want, ok := wantCounters[family.GetName()]
		if !ok {
			continue
		}
		if len(family.Metric) != 1 || family.Metric[0].GetCounter().GetValue() != want {
			t.Errorf("metric %s = %#v, want counter %v", family.GetName(), family.Metric, want)
		}
		delete(wantCounters, family.GetName())
	}
	if len(wantCounters) != 0 {
		t.Fatalf("missing counters: %#v", wantCounters)
	}
}

func TestSkillEvolutionMetricsIgnoreInvalidNumericSamplesAndNilReceiver(t *testing.T) {
	var nilMetrics *Metrics
	nilMetrics.RecordEligibleSignals(1)
	nilMetrics.RecordProposalGenerated()
	nilMetrics.RecordValidationFailure()
	nilMetrics.RecordProposalAccepted()
	nilMetrics.RecordPublication(false)
	nilMetrics.RecordFeedbackCoverage(true)
	nilMetrics.RecordManifestCoverage(true)
	nilMetrics.RecordRevision()
	nilMetrics.RecordCost(1)
	nilMetrics.RecordLatency(time.Second)

	metrics := NewMetrics()
	metrics.RecordEligibleSignals(-1)
	metrics.RecordCost(-1)
	metrics.RecordLatency(-time.Second)
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(metrics.Collectors()...)
	if _, err := registry.Gather(); err != nil {
		t.Fatalf("gather metrics after invalid samples: %v", err)
	}
}
