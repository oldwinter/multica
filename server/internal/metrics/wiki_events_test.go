package metrics_test

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/metrics"
	dto "github.com/prometheus/client_model/go"
)

func TestWikiEventsUseBoundedMetricLabels(t *testing.T) {
	t.Parallel()

	m := metrics.NewBusinessMetrics()
	metrics.RecordEvent(nil, m, analytics.WikiSearch("workspace-secret-value", 1))
	metrics.RecordEvent(nil, m, analytics.WikiProposalReview("future-decision-secret", false))
	metrics.RecordEvent(nil, m, analytics.LMWikiReview("accepted"))
	families := metrics.GatherForTest(t, m)

	assertMetricLabels(t, families, "multica_wiki_search_total", map[string]string{
		"scope": "unknown", "result": "hit",
	})
	assertMetricLabels(t, families, "multica_wiki_proposal_review_total", map[string]string{
		"decision": "unknown", "edited": "false",
	})
	assertMetricLabels(t, families, "multica_lm_wiki_review_total", map[string]string{
		"decision": "accepted",
	})
}

func assertMetricLabels(t *testing.T, families map[string]*dto.MetricFamily, name string, want map[string]string) {
	t.Helper()
	family, ok := families[name]
	if !ok || len(family.Metric) != 1 {
		t.Fatalf("metric family %s = %#v", name, family)
	}
	got := make(map[string]string)
	for _, label := range family.Metric[0].Label {
		got[label.GetName()] = label.GetValue()
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s label %s = %q, want %q", name, key, got[key], value)
		}
	}
}
