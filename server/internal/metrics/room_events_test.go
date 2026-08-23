package metrics

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRoomOutcomeMetricsUseBoundedLabels(t *testing.T) {
	m := NewBusinessMetrics()
	m.IncForEvent(analytics.Event{
		Name: analytics.EventRoomCycleFailed,
		Properties: map[string]any{
			"source": "private source", "reason": "private reason", "kind": "private kind",
		},
	})
	got := testutil.ToFloat64(m.events.roomOutcome.WithLabelValues("cycle_failed", "other", "none", "none"))
	if got != 1 {
		t.Fatalf("bounded Room outcome metric = %v, want 1", got)
	}
}
