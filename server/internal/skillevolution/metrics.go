package skillevolution

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics contains only unlabeled, content-free measurements. Rates and
// coverage are derived from counter pairs in PromQL, avoiding tenant or Skill
// identifiers and any temptation to attach source content as labels.
type Metrics struct {
	EligibleSignals        prometheus.Histogram
	ProposalsGenerated     prometheus.Counter
	ValidationFailures     prometheus.Counter
	ProposalsAccepted      prometheus.Counter
	Publications           prometheus.Counter
	Rollbacks              prometheus.Counter
	FeedbackEligibleRuns   prometheus.Counter
	FeedbackCoveredRuns    prometheus.Counter
	ManifestEligibleRuns   prometheus.Counter
	ManifestAttributedRuns prometheus.Counter
	Revisions              prometheus.Counter
	CostUSDTicks           prometheus.Counter
	LatencySeconds         prometheus.Histogram
}

func NewMetrics() *Metrics {
	counter := func(name, help string) prometheus.Counter {
		return prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "multica", Subsystem: "skill_evolution", Name: name, Help: help,
		})
	}
	return &Metrics{
		EligibleSignals: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "multica", Subsystem: "skill_evolution", Name: "eligible_signals",
			Help:    "Eligible content-free evidence references observed per loop run.",
			Buckets: []float64{0, 1, 2, 4, 8, 16, 32, 64},
		}),
		ProposalsGenerated:     counter("proposals_generated_total", "Reviewable proposal candidates generated."),
		ValidationFailures:     counter("validation_failures_total", "Proposal candidates rejected by deterministic or replay evaluation."),
		ProposalsAccepted:      counter("proposals_accepted_total", "Proposal candidates accepted by an explicit human review."),
		Publications:           counter("publications_total", "Successful Skill bundle publications."),
		Rollbacks:              counter("rollbacks_total", "Successful append-only rollback publications."),
		FeedbackEligibleRuns:   counter("feedback_eligible_runs_total", "Exactly attributed terminal runs eligible for feedback coverage."),
		FeedbackCoveredRuns:    counter("feedback_covered_runs_total", "Exactly attributed terminal runs with explicit feedback."),
		ManifestEligibleRuns:   counter("manifest_eligible_runs_total", "Terminal runs eligible for exact-manifest attribution."),
		ManifestAttributedRuns: counter("manifest_attributed_runs_total", "Terminal runs with a verified exact execution manifest."),
		Revisions:              counter("revisions_total", "Immutable Skill evolution revisions recorded."),
		CostUSDTicks:           counter("cost_usd_ticks_total", "Bounded model and replay cost in fixed-point USD ticks."),
		LatencySeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "multica", Subsystem: "skill_evolution", Name: "proposal_latency_seconds",
			Help:    "Elapsed proposal generation and evaluation time.",
			Buckets: prometheus.ExponentialBuckets(0.25, 2, 12),
		}),
	}
}

func (m *Metrics) Collectors() []prometheus.Collector {
	if m == nil {
		return nil
	}
	return []prometheus.Collector{
		m.EligibleSignals,
		m.ProposalsGenerated,
		m.ValidationFailures,
		m.ProposalsAccepted,
		m.Publications,
		m.Rollbacks,
		m.FeedbackEligibleRuns,
		m.FeedbackCoveredRuns,
		m.ManifestEligibleRuns,
		m.ManifestAttributedRuns,
		m.Revisions,
		m.CostUSDTicks,
		m.LatencySeconds,
	}
}

func (m *Metrics) RecordEligibleSignals(count int) {
	if m != nil && count >= 0 {
		m.EligibleSignals.Observe(float64(count))
	}
}

func (m *Metrics) RecordProposalGenerated() {
	if m != nil {
		m.ProposalsGenerated.Inc()
	}
}

func (m *Metrics) RecordValidationFailure() {
	if m != nil {
		m.ValidationFailures.Inc()
	}
}

func (m *Metrics) RecordProposalAccepted() {
	if m != nil {
		m.ProposalsAccepted.Inc()
	}
}

func (m *Metrics) RecordPublication(rollback bool) {
	if m == nil {
		return
	}
	if rollback {
		m.Rollbacks.Inc()
		return
	}
	m.Publications.Inc()
}

func (m *Metrics) RecordFeedbackCoverage(covered bool) {
	if m == nil {
		return
	}
	m.FeedbackEligibleRuns.Inc()
	if covered {
		m.FeedbackCoveredRuns.Inc()
	}
}

func (m *Metrics) RecordManifestCoverage(attributed bool) {
	if m == nil {
		return
	}
	m.ManifestEligibleRuns.Inc()
	if attributed {
		m.ManifestAttributedRuns.Inc()
	}
}

func (m *Metrics) RecordRevision() {
	if m != nil {
		m.Revisions.Inc()
	}
}

func (m *Metrics) RecordCost(usdTicks int64) {
	if m != nil && usdTicks > 0 {
		m.CostUSDTicks.Add(float64(usdTicks))
	}
}

func (m *Metrics) RecordLatency(duration time.Duration) {
	if m != nil && duration >= 0 {
		m.LatencySeconds.Observe(duration.Seconds())
	}
}
