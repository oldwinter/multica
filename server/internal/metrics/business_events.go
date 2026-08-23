package metrics

import (
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/prometheus/client_golang/prometheus"
)

// PR3: funnel / commercial / community counters paired with PostHog events.
//
// Every PostHog Capture(...) call site goes through metrics.RecordEvent(...)
// (see event_recorder.go) so the two sides cannot drift. Lint test in
// business_pairing_test.go enforces that.

// runtimeReadyBuckets covers cold-start runtime readiness from <1s to ~5min.
// Most provider boots land in 5–60s; the long tail catches stuck pulls.
var runtimeReadyBuckets = []float64{1, 2.5, 5, 10, 30, 60, 120, 300, 600}

var twinOperationLatencyBuckets = []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}
var twinBriefingByteBuckets = []float64{256, 512, 1024, 2048, 4096, 8192}
var twinBriefingTokenBuckets = []float64{64, 128, 256, 512, 1024, 2048}

// cloudRuntimeRequestBuckets covers outbound Fleet/Gateway calls from sub-100ms
// (status pings) to ~30s (provision). Aligns with cloudruntime.defaultTimeout.
var cloudRuntimeRequestBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30}

// prMergeSecondsBuckets covers PR-open → PR-merged latency from minutes to weeks.
var prMergeSecondsBuckets = []float64{
	300, 900, 1800,
	3600, 2 * 3600, 6 * 3600, 12 * 3600,
	24 * 3600, 2 * 24 * 3600, 7 * 24 * 3600, 30 * 24 * 3600,
}

// businessEventMetrics holds the PR3 collectors. Kept in a separate struct
// so business.go (PR2 task lifecycle / LLM) stays focused; both are exposed
// through the same BusinessMetrics receiver and the same Collectors() slice.
type businessEventMetrics struct {
	signup                          *prometheus.CounterVec
	workspaceCreated                *prometheus.CounterVec
	teamInviteSent                  *prometheus.CounterVec
	teamInviteAccepted              *prometheus.CounterVec
	onboardingStarted               *prometheus.CounterVec
	onboardingQuestionnaireSubmit   *prometheus.CounterVec
	onboardingSourceSubmit          *prometheus.CounterVec
	onboardingCompleted             *prometheus.CounterVec
	cloudWaitlistJoined             *prometheus.CounterVec
	issueCreated                    *prometheus.CounterVec
	chatMessageSent                 *prometheus.CounterVec
	agentCreated                    *prometheus.CounterVec
	squadCreated                    *prometheus.CounterVec
	autopilotCreated                *prometheus.CounterVec
	issueExecuted                   *prometheus.CounterVec
	runtimeRegistered               *prometheus.CounterVec
	runtimeReady                    *prometheus.CounterVec
	runtimeReadySeconds             *prometheus.HistogramVec
	runtimeFailed                   *prometheus.CounterVec
	runtimeOffline                  *prometheus.CounterVec
	daemonWSMessageReceived         *prometheus.CounterVec
	autopilotRunStarted             *prometheus.CounterVec
	autopilotRunTerminal            *prometheus.CounterVec
	autopilotRunSkipped             *prometheus.CounterVec
	webhookDelivery                 *prometheus.CounterVec
	webhookRateLimited              *prometheus.CounterVec
	emailRateLimited                *prometheus.CounterVec
	githubEventReceived             *prometheus.CounterVec
	githubPRReview                  *prometheus.CounterVec
	githubPRMergeSeconds            prometheus.Histogram
	cloudRuntimeRequest             *prometheus.CounterVec
	cloudRuntimeRequestDurationSecs *prometheus.HistogramVec
	feedbackSubmitted               *prometheus.CounterVec
	contactSalesSubmitted           *prometheus.CounterVec
	chatOutputLocalPath             *prometheus.CounterVec
	wikiSearch                      *prometheus.CounterVec
	wikiProposalReview              *prometheus.CounterVec
	lmWikiReview                    *prometheus.CounterVec
	twinProposalGeneration          *prometheus.CounterVec
	twinProposalGenerationSeconds   *prometheus.HistogramVec
	twinSignOff                     *prometheus.CounterVec
	twinBriefingCompilation         *prometheus.CounterVec
	twinBriefingCompilationSeconds  *prometheus.HistogramVec
	twinBriefingUse                 *prometheus.CounterVec
	twinBriefingBytes               *prometheus.HistogramVec
	twinBriefingTokens              *prometheus.HistogramVec
	twinRunFeedback                 *prometheus.CounterVec
	twinTaskRevision                *prometheus.CounterVec
	twinDepositionReview            *prometheus.CounterVec
	roomOutcome                     *prometheus.CounterVec
}

func newBusinessEventMetrics() *businessEventMetrics {
	return &businessEventMetrics{
		signup: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_signup_total",
			Help: "Total user signups (account creations).",
		}, metricLabels("multica_signup_total")),
		workspaceCreated: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_workspace_created_total",
			Help: "Total workspaces created.",
		}, metricLabels("multica_workspace_created_total")),
		teamInviteSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_team_invite_sent_total",
			Help: "Total workspace invitations sent.",
		}, metricLabels("multica_team_invite_sent_total")),
		teamInviteAccepted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_team_invite_accepted_total",
			Help: "Total workspace invitations accepted.",
		}, metricLabels("multica_team_invite_accepted_total")),
		onboardingStarted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_onboarding_started_total",
			Help: "Total onboarding flows started.",
		}, metricLabels("multica_onboarding_started_total")),
		onboardingQuestionnaireSubmit: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_onboarding_questionnaire_submitted_total",
			Help: "Total onboarding questionnaires submitted.",
		}, metricLabels("multica_onboarding_questionnaire_submitted_total")),
		onboardingSourceSubmit: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_onboarding_source_submitted_total",
			Help: "Total acquisition-source answers or declines recorded (workspace backfill prompt).",
		}, metricLabels("multica_onboarding_source_submitted_total")),
		onboardingCompleted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_onboarding_completed_total",
			Help: "Total onboarding flows completed.",
		}, metricLabels("multica_onboarding_completed_total")),
		cloudWaitlistJoined: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_cloud_waitlist_joined_total",
			Help: "Total users that joined the cloud waitlist.",
		}, metricLabels("multica_cloud_waitlist_joined_total")),
		issueCreated: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_issue_created_total",
			Help: "Total issues created (any source).",
		}, metricLabels("multica_issue_created_total")),
		chatMessageSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_chat_message_sent_total",
			Help: "Total user chat messages sent (excludes agent replies).",
		}, metricLabels("multica_chat_message_sent_total")),
		agentCreated: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_agent_created_total",
			Help: "Total agents created.",
		}, metricLabels("multica_agent_created_total")),
		squadCreated: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_squad_created_total",
			Help: "Total squads created.",
		}, metricLabels("multica_squad_created_total")),
		autopilotCreated: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_autopilot_created_total",
			Help: "Total autopilots created.",
		}, metricLabels("multica_autopilot_created_total")),
		issueExecuted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_issue_executed_total",
			Help: "First task completion per issue (per-issue exactly-once activation keystone).",
		}, metricLabels("multica_issue_executed_total")),
		runtimeRegistered: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_runtime_registered_total",
			Help: "Total first-time runtime registrations.",
		}, metricLabels("multica_runtime_registered_total")),
		runtimeReady: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_runtime_ready_total",
			Help: "Total runtimes that reached ready state.",
		}, metricLabels("multica_runtime_ready_total")),
		runtimeReadySeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "multica_runtime_ready_seconds",
			Help:    "Time from runtime registration to ready (seconds).",
			Buckets: runtimeReadyBuckets,
		}, metricLabels("multica_runtime_ready_seconds")),
		runtimeFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_runtime_failed_total",
			Help: "Total runtime failures by canonical reason.",
		}, metricLabels("multica_runtime_failed_total")),
		runtimeOffline: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_runtime_offline_total",
			Help: "Total runtime offline transitions.",
		}, metricLabels("multica_runtime_offline_total")),
		daemonWSMessageReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_daemon_ws_message_received_total",
			Help: "Total daemon WebSocket inbound messages by handler kind.",
		}, metricLabels("multica_daemon_ws_message_received_total")),
		autopilotRunStarted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_autopilot_run_started_total",
			Help: "Total autopilot runs started.",
		}, metricLabels("multica_autopilot_run_started_total")),
		autopilotRunTerminal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_autopilot_run_terminal_total",
			Help: "Total autopilot runs that reached a terminal status.",
		}, metricLabels("multica_autopilot_run_terminal_total")),
		autopilotRunSkipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_autopilot_run_skipped_total",
			Help: "Total autopilot runs that admission-skipped (concurrency / cooldown / other).",
		}, metricLabels("multica_autopilot_run_skipped_total")),
		webhookDelivery: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_webhook_delivery_total",
			Help: "Total inbound webhook deliveries by provider and outcome.",
		}, metricLabels("multica_webhook_delivery_total")),
		webhookRateLimited: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_webhook_rate_limited_total",
			Help: "Total webhook admissions or worker dispatches delayed by a bounded safety gate.",
		}, metricLabels("multica_webhook_rate_limited_total")),
		emailRateLimited: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_email_rate_limited_total",
			Help: "Total email-producing actions rejected by a bounded safety gate.",
		}, metricLabels("multica_email_rate_limited_total")),
		githubEventReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_github_event_received_total",
			Help: "Total GitHub webhook events received by event kind and action.",
		}, metricLabels("multica_github_event_received_total")),
		githubPRReview: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_github_pr_review_total",
			Help: "Total GitHub pull request reviews observed by result.",
		}, metricLabels("multica_github_pr_review_total")),
		githubPRMergeSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "multica_github_pr_merge_seconds",
			Help:    "Time from PR opened to merged (seconds).",
			Buckets: prMergeSecondsBuckets,
		}),
		cloudRuntimeRequest: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_cloudruntime_request_total",
			Help: "Total outbound cloud runtime requests by op and status bucket.",
		}, metricLabels("multica_cloudruntime_request_total")),
		cloudRuntimeRequestDurationSecs: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "multica_cloudruntime_request_duration_seconds",
			Help:    "Outbound cloud runtime request duration (seconds).",
			Buckets: cloudRuntimeRequestBuckets,
		}, metricLabels("multica_cloudruntime_request_duration_seconds")),
		feedbackSubmitted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_feedback_submitted_total",
			Help: "Total in-app feedback submissions.",
		}, metricLabels("multica_feedback_submitted_total")),
		contactSalesSubmitted: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_contact_sales_submitted_total",
			Help: "Total contact-sales inquiries submitted.",
		}, metricLabels("multica_contact_sales_submitted_total")),
		chatOutputLocalPath: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_chat_output_local_path_total",
			Help: "Total agent chat replies that referenced a runtime-local path, by evidence kind. Observation only — the reply is still delivered.",
		}, metricLabels("multica_chat_output_local_path_total")),
		wikiSearch: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_wiki_search_total",
			Help: "Total Wiki searches by bounded scope and hit outcome.",
		}, metricLabels("multica_wiki_search_total")),
		wikiProposalReview: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_wiki_proposal_review_total",
			Help: "Total human Wiki proposal reviews by decision and whether acceptance was edited.",
		}, metricLabels("multica_wiki_proposal_review_total")),
		lmWikiReview: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_lm_wiki_review_total",
			Help: "Total immutable LM Wiki revision reviews by decision.",
		}, metricLabels("multica_lm_wiki_review_total")),
		twinProposalGeneration: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_twin_proposal_generation_total",
			Help: "Total Twin proposal generation outcomes by bounded kind and state.",
		}, metricLabels("multica_twin_proposal_generation_total")),
		twinProposalGenerationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "multica_twin_proposal_generation_seconds",
			Help:    "Twin proposal generation latency in seconds.",
			Buckets: twinOperationLatencyBuckets,
		}, metricLabels("multica_twin_proposal_generation_seconds")),
		twinSignOff: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_twin_sign_off_total",
			Help: "Total Twin sign-off decisions by bounded proposal kind.",
		}, metricLabels("multica_twin_sign_off_total")),
		twinBriefingCompilation: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_twin_briefing_compilation_total",
			Help: "Total Twin briefing compilation outcomes by bounded policy scope and exclusion code.",
		}, metricLabels("multica_twin_briefing_compilation_total")),
		twinBriefingCompilationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "multica_twin_briefing_compilation_seconds",
			Help:    "Twin briefing compilation latency in seconds.",
			Buckets: twinOperationLatencyBuckets,
		}, metricLabels("multica_twin_briefing_compilation_seconds")),
		twinBriefingUse: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_twin_briefing_use_total",
			Help: "Total Twin briefing preview, injection, and skip outcomes.",
		}, metricLabels("multica_twin_briefing_use_total")),
		twinBriefingBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "multica_twin_briefing_bytes",
			Help:    "Bounded Twin briefing size used for preview or runtime injection.",
			Buckets: twinBriefingByteBuckets,
		}, metricLabels("multica_twin_briefing_bytes")),
		twinBriefingTokens: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "multica_twin_briefing_tokens",
			Help:    "Bounded Twin briefing token overhead used for preview or runtime injection.",
			Buckets: twinBriefingTokenBuckets,
		}, metricLabels("multica_twin_briefing_tokens")),
		twinRunFeedback: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_twin_run_feedback_total",
			Help: "Total Twin run feedback ratings.",
		}, metricLabels("multica_twin_run_feedback_total")),
		twinTaskRevision: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_twin_task_revision_total",
			Help: "Total accepted Twin-assisted tasks and requested revisions by bounded revision kind.",
		}, metricLabels("multica_twin_task_revision_total")),
		twinDepositionReview: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_twin_deposition_review_total",
			Help: "Total human Twin deposition review decisions.",
		}, metricLabels("multica_twin_deposition_review_total")),
		roomOutcome: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "multica_room_outcome_total",
			Help: "Total Room outcome lifecycle events by bounded event, source, reason, and target kind.",
		}, metricLabels("multica_room_outcome_total")),
	}
}

func (e *businessEventMetrics) collectors() []prometheus.Collector {
	if e == nil {
		return nil
	}
	return []prometheus.Collector{
		e.signup,
		e.workspaceCreated,
		e.teamInviteSent,
		e.teamInviteAccepted,
		e.onboardingStarted,
		e.onboardingQuestionnaireSubmit,
		e.onboardingSourceSubmit,
		e.onboardingCompleted,
		e.cloudWaitlistJoined,
		e.issueCreated,
		e.chatMessageSent,
		e.agentCreated,
		e.squadCreated,
		e.autopilotCreated,
		e.issueExecuted,
		e.runtimeRegistered,
		e.runtimeReady,
		e.runtimeReadySeconds,
		e.runtimeFailed,
		e.runtimeOffline,
		e.daemonWSMessageReceived,
		e.autopilotRunStarted,
		e.autopilotRunTerminal,
		e.autopilotRunSkipped,
		e.webhookDelivery,
		e.webhookRateLimited,
		e.emailRateLimited,
		e.githubEventReceived,
		e.githubPRReview,
		e.githubPRMergeSeconds,
		e.cloudRuntimeRequest,
		e.cloudRuntimeRequestDurationSecs,
		e.feedbackSubmitted,
		e.contactSalesSubmitted,
		e.chatOutputLocalPath,
		e.wikiSearch,
		e.wikiProposalReview,
		e.lmWikiReview,
		e.twinProposalGeneration,
		e.twinProposalGenerationSeconds,
		e.twinSignOff,
		e.twinBriefingCompilation,
		e.twinBriefingCompilationSeconds,
		e.twinBriefingUse,
		e.twinBriefingBytes,
		e.twinBriefingTokens,
		e.twinRunFeedback,
		e.twinTaskRevision,
		e.twinDepositionReview,
		e.roomOutcome,
	}
}

// RecordEvent increments the matching Prometheus counter and, for any event
// that still ships to PostHog, enqueues the PostHog event too — so the two
// cannot drift. Pass `client = nil` (no PostHog) or `m = nil` (no metrics)
// safely; both sides are best-effort and never block the request path.
//
// As of MUL-4127 every server-side event is flagged by analytics.IsMetricsOnly
// (all product events plus the runtime_* / autopilot_run_* lifecycle), so the
// client.Capture below is skipped for all of them — server analytics is served
// from the DB and Grafana, not PostHog. The Capture path is retained only so a
// future non-metrics-only event name would still ship.
//
// This is the canonical way to record any funnel / community / commercial event
// from server code. Direct analytics.Client.Capture(...) with an event
// constructed from analytics.* is rejected by the lint test in
// business_pairing_test.go.
func RecordEvent(client analytics.Client, m *BusinessMetrics, ev analytics.Event) {
	if client != nil && !analytics.IsMetricsOnly(ev.Name) {
		client.Capture(ev)
	}
	if m != nil {
		m.IncForEvent(ev)
	}
}

// IncForEvent dispatches an analytics.Event to the matching Prometheus counter.
// Unknown event names are silently ignored — the lint test in
// business_pairing_test.go is responsible for catching missing dispatch entries.
func (m *BusinessMetrics) IncForEvent(ev analytics.Event) {
	if m == nil || m.events == nil {
		return
	}
	switch ev.Name {
	case analytics.EventSignup:
		m.events.signup.WithLabelValues(NormalizeSignupSource(stringProp(ev.Properties, "signup_source"))).Inc()
	case analytics.EventWorkspaceCreated:
		m.events.workspaceCreated.WithLabelValues(NormalizeTaskSource(stringProp(ev.Properties, "source"))).Inc()
	case analytics.EventTeamInviteSent:
		m.events.teamInviteSent.WithLabelValues().Inc()
	case analytics.EventTeamInviteAccepted:
		m.events.teamInviteAccepted.WithLabelValues().Inc()
	case analytics.EventOnboardingStarted:
		m.events.onboardingStarted.WithLabelValues(NormalizePlatform(stringProp(ev.Properties, "platform"))).Inc()
	case analytics.EventOnboardingQuestionnaireSubmit:
		m.events.onboardingQuestionnaireSubmit.WithLabelValues().Inc()
	case analytics.EventOnboardingSourceSubmit:
		m.events.onboardingSourceSubmit.WithLabelValues().Inc()
	case analytics.EventOnboardingCompleted:
		m.events.onboardingCompleted.WithLabelValues(NormalizeOnboardingPath(stringProp(ev.Properties, "completion_path"))).Inc()
	case analytics.EventCloudWaitlistJoined:
		m.events.cloudWaitlistJoined.WithLabelValues().Inc()
	case analytics.EventIssueCreated:
		m.events.issueCreated.WithLabelValues(
			NormalizeTaskSource(stringProp(ev.Properties, "source")),
			NormalizePlatform(stringProp(ev.Properties, "platform")),
		).Inc()
	case analytics.EventChatMessageSent:
		m.events.chatMessageSent.WithLabelValues(NormalizePlatform(stringProp(ev.Properties, "platform"))).Inc()
	case analytics.EventAgentCreated:
		m.events.agentCreated.WithLabelValues(
			NormalizeRuntimeMode(stringProp(ev.Properties, "runtime_mode")),
			NormalizeTaskSource(stringProp(ev.Properties, "source")),
		).Inc()
	case analytics.EventSquadCreated:
		m.events.squadCreated.WithLabelValues().Inc()
	case analytics.EventAutopilotCreated:
		m.events.autopilotCreated.WithLabelValues(NormalizeAutopilotCadence(stringProp(ev.Properties, "cadence"))).Inc()
	case analytics.EventIssueExecuted:
		m.events.issueExecuted.WithLabelValues(NormalizeTaskSource(stringProp(ev.Properties, "source"))).Inc()
	case analytics.EventRuntimeRegistered:
		m.events.runtimeRegistered.WithLabelValues(
			NormalizeRuntimeMode(stringProp(ev.Properties, "runtime_mode")),
			NormalizeRuntimeProvider(stringProp(ev.Properties, "provider")),
		).Inc()
	case analytics.EventRuntimeReady:
		runtimeMode := NormalizeRuntimeMode(stringProp(ev.Properties, "runtime_mode"))
		provider := NormalizeRuntimeProvider(stringProp(ev.Properties, "provider"))
		m.events.runtimeReady.WithLabelValues(runtimeMode, provider).Inc()
		if d := int64Prop(ev.Properties, "ready_duration_ms"); d > 0 {
			m.events.runtimeReadySeconds.WithLabelValues(runtimeMode, provider).Observe(float64(d) / 1000.0)
		}
	case analytics.EventRuntimeFailed:
		m.events.runtimeFailed.WithLabelValues(
			NormalizeRuntimeMode(stringProp(ev.Properties, "runtime_mode")),
			NormalizeRuntimeProvider(stringProp(ev.Properties, "provider")),
			NormalizeFailureReason(stringProp(ev.Properties, "failure_reason")),
			boolLabel(boolProp(ev.Properties, "recoverable")),
		).Inc()
	case analytics.EventRuntimeOffline:
		m.events.runtimeOffline.WithLabelValues(
			NormalizeRuntimeMode(stringProp(ev.Properties, "runtime_mode")),
			NormalizeRuntimeProvider(stringProp(ev.Properties, "provider")),
		).Inc()
	case analytics.EventAutopilotRunStarted:
		m.events.autopilotRunStarted.WithLabelValues(
			NormalizeAutopilotCadence(stringProp(ev.Properties, "cadence")),
			NormalizeAutopilotTrigger(stringProp(ev.Properties, "trigger_kind")),
		).Inc()
	case analytics.EventAutopilotRunCompleted:
		m.events.autopilotRunTerminal.WithLabelValues(
			NormalizeAutopilotCadence(stringProp(ev.Properties, "cadence")),
			NormalizeAutopilotTrigger(stringProp(ev.Properties, "trigger_kind")),
			"completed",
		).Inc()
	case analytics.EventAutopilotRunFailed:
		m.events.autopilotRunTerminal.WithLabelValues(
			NormalizeAutopilotCadence(stringProp(ev.Properties, "cadence")),
			NormalizeAutopilotTrigger(stringProp(ev.Properties, "trigger_kind")),
			"failed",
		).Inc()
	case analytics.EventFeedbackSubmitted:
		m.events.feedbackSubmitted.WithLabelValues(
			NormalizeFeedbackKind(stringProp(ev.Properties, "kind")),
			NormalizePlatform(stringProp(ev.Properties, "platform")),
		).Inc()
	case analytics.EventContactSalesSubmitted:
		m.events.contactSalesSubmitted.WithLabelValues(NormalizeContactSalesSource(stringProp(ev.Properties, "form_source"))).Inc()
	case analytics.EventWikiSearch:
		m.events.wikiSearch.WithLabelValues(
			normalizeWikiScope(stringProp(ev.Properties, "scope")),
			normalizeWikiSearchResult(stringProp(ev.Properties, "result")),
		).Inc()
	case analytics.EventWikiProposalReview:
		m.events.wikiProposalReview.WithLabelValues(
			normalizeWikiReviewDecision(stringProp(ev.Properties, "decision")),
			boolLabel(boolProp(ev.Properties, "edited")),
		).Inc()
	case analytics.EventLMWikiReview:
		m.events.lmWikiReview.WithLabelValues(
			normalizeWikiReviewDecision(stringProp(ev.Properties, "decision")),
		).Inc()
	case analytics.EventTwinProposalGeneration:
		kind := NormalizeTwinProposalKind(stringProp(ev.Properties, "kind"))
		state := NormalizeTwinState(stringProp(ev.Properties, "state"))
		m.events.twinProposalGeneration.WithLabelValues(kind, state).Inc()
		if latencyMS := int64Prop(ev.Properties, "latency_ms"); latencyMS > 0 {
			m.events.twinProposalGenerationSeconds.WithLabelValues(kind, state).Observe(float64(latencyMS) / 1000)
		}
	case analytics.EventTwinSignOff:
		m.events.twinSignOff.WithLabelValues(
			NormalizeTwinProposalKind(stringProp(ev.Properties, "kind")),
			NormalizeTwinDecision(stringProp(ev.Properties, "decision")),
		).Inc()
	case analytics.EventTwinBriefingCompilation:
		state := NormalizeTwinState(stringProp(ev.Properties, "state"))
		scope := NormalizeTwinPolicyScope(stringProp(ev.Properties, "scope"))
		m.events.twinBriefingCompilation.WithLabelValues(
			state,
			scope,
			NormalizeTwinExclusionCode(stringProp(ev.Properties, "exclusion_code")),
		).Inc()
		if latencyMS := int64Prop(ev.Properties, "latency_ms"); latencyMS > 0 {
			m.events.twinBriefingCompilationSeconds.WithLabelValues(state, scope).Observe(float64(latencyMS) / 1000)
		}
	case analytics.EventTwinBriefingUse:
		state := NormalizeTwinState(stringProp(ev.Properties, "state"))
		scope := NormalizeTwinPolicyScope(stringProp(ev.Properties, "scope"))
		m.events.twinBriefingUse.WithLabelValues(
			state,
			scope,
			NormalizeTwinExclusionCode(stringProp(ev.Properties, "exclusion_code")),
		).Inc()
		if byteCount := int64Prop(ev.Properties, "byte_count"); byteCount > 0 {
			m.events.twinBriefingBytes.WithLabelValues(state, scope).Observe(float64(byteCount))
		}
		if tokenCount := int64Prop(ev.Properties, "token_count"); tokenCount > 0 {
			m.events.twinBriefingTokens.WithLabelValues(state, scope).Observe(float64(tokenCount))
		}
	case analytics.EventTwinRunFeedback:
		m.events.twinRunFeedback.WithLabelValues(NormalizeTwinFeedbackRating(stringProp(ev.Properties, "rating"))).Inc()
	case analytics.EventTwinTaskRevision:
		m.events.twinTaskRevision.WithLabelValues(
			NormalizeTwinDecision(stringProp(ev.Properties, "decision")),
			NormalizeTwinRevisionKind(stringProp(ev.Properties, "kind")),
		).Inc()
	case analytics.EventTwinDepositionReview:
		m.events.twinDepositionReview.WithLabelValues(NormalizeTwinDecision(stringProp(ev.Properties, "decision"))).Inc()
	case analytics.EventRoomCreated,
		analytics.EventRoomFirstCycleCompleted,
		analytics.EventRoomSynthesisAccepted,
		analytics.EventRoomSynthesisRejected,
		analytics.EventRoomSynthesisRetried,
		analytics.EventRoomArtifactPromoted,
		analytics.EventRoomBudgetRefused,
		analytics.EventRoomCycleFailed:
		m.events.roomOutcome.WithLabelValues(
			NormalizeRoomEvent(ev.Name),
			NormalizeRoomSource(stringProp(ev.Properties, "source")),
			NormalizeRoomReason(stringProp(ev.Properties, "reason")),
			NormalizeRoomKind(stringProp(ev.Properties, "kind")),
		).Inc()
	default:
		// agent_task_* lifecycle telemetry is recorded straight to Prometheus
		// via the typed BusinessMetrics.RecordTask* methods (they take
		// queue/run/total seconds that an analytics.Event does not carry), so
		// there is no analytics.Event to dispatch here. Anything else reaching
		// this default is a missing case and the lint test will fail CI.
	}
}

// ---- non-PostHog Record* helpers (typed; no analytics.Event source) -------

// RecordAutopilotRunSkipped counts an autopilot admission-skip with reason.
func (m *BusinessMetrics) RecordAutopilotRunSkipped(cadence, reason string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.autopilotRunSkipped.WithLabelValues(
		NormalizeAutopilotCadence(cadence),
		NormalizeAutopilotSkipReason(reason),
	).Inc()
}

// RecordWebhookDelivery counts an inbound webhook outcome.
func (m *BusinessMetrics) RecordWebhookDelivery(provider, status string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.webhookDelivery.WithLabelValues(
		NormalizeWebhookProvider(provider),
		NormalizeWebhookDeliveryStatus(status),
	).Inc()
}

func (m *BusinessMetrics) RecordWebhookRateLimited(gate string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.webhookRateLimited.WithLabelValues(NormalizeWebhookRateLimitGate(gate)).Inc()
}

func (m *BusinessMetrics) RecordEmailRateLimited(action, gate string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.emailRateLimited.WithLabelValues(
		NormalizeEmailRateLimitAction(action),
		NormalizeEmailRateLimitGate(gate),
	).Inc()
}

// RecordGithubEventReceived counts a GitHub webhook event by event kind / action.
func (m *BusinessMetrics) RecordGithubEventReceived(eventKind, action string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.githubEventReceived.WithLabelValues(
		NormalizeGithubEventKind(eventKind),
		NormalizeGithubAction(action),
	).Inc()
}

// RecordGithubPRReview counts a PR review observation by result.
func (m *BusinessMetrics) RecordGithubPRReview(result string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.githubPRReview.WithLabelValues(NormalizeGithubPRReviewResult(result)).Inc()
}

// ObserveGithubPRMergeSeconds records open→merge latency in seconds.
// Negative or zero values are ignored.
func (m *BusinessMetrics) ObserveGithubPRMergeSeconds(seconds float64) {
	if m == nil || m.events == nil || seconds <= 0 {
		return
	}
	m.events.githubPRMergeSeconds.Observe(seconds)
}

// RecordCloudRuntimeRequest counts an outbound Fleet/Gateway call by op +
// status bucket and observes its duration.
func (m *BusinessMetrics) RecordCloudRuntimeRequest(op, status string, durationSeconds float64) {
	if m == nil || m.events == nil {
		return
	}
	op = NormalizeCloudRuntimeOp(op)
	status = NormalizeCloudRuntimeStatus(status)
	m.events.cloudRuntimeRequest.WithLabelValues(op, status).Inc()
	if durationSeconds >= 0 {
		m.events.cloudRuntimeRequestDurationSecs.WithLabelValues(op).Observe(durationSeconds)
	}
}

// RecordChatOutputLocalPath counts a chat reply that referenced a runtime-local
// path, by evidence kind ("file_url" / "workdir_path").
//
// Observation only: the reply is delivered either way. The server cannot judge
// these paths the way the CLI lint can — it has no access to the daemon's
// filesystem to stat them — so this measures whether the MUL-4899 prompt
// contract is landing, and must never gate delivery on a lexical guess. The
// label is a closed enum precisely so no fragment of the path or reply body can
// reach Prometheus.
func (m *BusinessMetrics) RecordChatOutputLocalPath(kind string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.chatOutputLocalPath.WithLabelValues(NormalizeChatOutputLocalPathKind(kind)).Inc()
}

// RecordDaemonWSMessageReceived counts an inbound daemon WS message by handler kind.
func (m *BusinessMetrics) RecordDaemonWSMessageReceived(kind string) {
	if m == nil || m.events == nil {
		return
	}
	m.events.daemonWSMessageReceived.WithLabelValues(NormalizeDaemonWSKind(kind)).Inc()
}

// ---- property accessors ---------------------------------------------------

func stringProp(props map[string]any, key string) string {
	if props == nil {
		return ""
	}
	v, ok := props[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func int64Prop(props map[string]any, key string) int64 {
	if props == nil {
		return 0
	}
	v, ok := props[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func boolProp(props map[string]any, key string) bool {
	if props == nil {
		return false
	}
	v, ok := props[key]
	if !ok || v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
