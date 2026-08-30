package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewRegistryRegistersExtraCollectors(t *testing.T) {
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "multica_test_leaf_events_total",
		Help: "Test-only leaf events.",
	})
	counter.Add(3)

	registry := NewRegistry(RegistryOptions{ExtraCollectors: []prometheus.Collector{nil, counter}})
	families, err := registry.Gatherer.Gather()
	if err != nil {
		t.Fatalf("gather registry: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "multica_test_leaf_events_total" {
			continue
		}
		if len(family.Metric) != 1 || family.Metric[0].GetCounter().GetValue() != 3 {
			t.Fatalf("extra collector metric = %#v, want one counter with value 3", family.Metric)
		}
		return
	}
	t.Fatal("extra collector was not registered")
}
