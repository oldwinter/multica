package analytics

import "strings"

// Event names. This file is the source-of-truth catalog; keep
// packages/core/analytics in sync with it.
const (
	EventSignup                        = "signup"
	EventWorkspaceCreated              = "workspace_created"
	EventRuntimeRegistered             = "runtime_registered"
	EventRuntimeReady                  = "runtime_ready"
	EventRuntimeFailed                 = "runtime_failed"
	EventRuntimeOffline                = "runtime_offline"
	EventIssueExecuted                 = "issue_executed"
	EventIssueCreated                  = "issue_created"
	EventChatMessageSent               = "chat_message_sent"
	EventAutopilotRunStarted           = "autopilot_run_started"
	EventAutopilotRunCompleted         = "autopilot_run_completed"
	EventAutopilotRunFailed            = "autopilot_run_failed"
	EventTeamInviteSent                = "team_invite_sent"
	EventTeamInviteAccepted            = "team_invite_accepted"
	EventOnboardingStarted             = "onboarding_started"
	EventOnboardingQuestionnaireSubmit = "onboarding_questionnaire_submitted"
	EventOnboardingSourceSubmit        = "onboarding_source_submitted"
	EventAgentCreated                  = "agent_created"
	EventOnboardingCompleted           = "onboarding_completed"
	EventCloudWaitlistJoined           = "cloud_waitlist_joined"
	EventFeedbackSubmitted             = "feedback_submitted"
	EventContactSalesSubmitted         = "contact_sales_submitted"
	EventSquadCreated                  = "squad_created"
	EventAutopilotCreated              = "autopilot_created"
	EventWikiSearch                    = "wiki_search"
	EventWikiProposalReview            = "wiki_proposal_review"
	EventLMWikiReview                  = "lm_wiki_review"
	EventTwinProposalGeneration        = "twin_proposal_generation"
	EventTwinSignOff                   = "twin_sign_off"
	EventTwinBriefingCompilation       = "twin_briefing_compilation"
	EventTwinBriefingUse               = "twin_briefing_use"
	EventTwinRunFeedback               = "twin_run_feedback"
	EventTwinTaskRevision              = "twin_task_revision"
	EventTwinDepositionReview          = "twin_deposition_review"
	EventRoomCreated                   = "room_created"
	EventRoomFirstCycleCompleted       = "room_first_cycle_completed"
	EventRoomSynthesisAccepted         = "room_synthesis_accepted"
	EventRoomSynthesisRejected         = "room_synthesis_rejected"
	EventRoomSynthesisRetried          = "room_synthesis_retried"
	EventRoomArtifactPromoted          = "room_artifact_promoted"
	EventRoomBudgetRefused             = "room_budget_refused"
	EventRoomCycleFailed               = "room_cycle_failed"
)

const EventSchemaVersion = 2

// metricsOnlyEvents lists every server-side event that is recorded to
// Prometheus (via metrics.IncForEvent, for Grafana) but deliberately NOT
// shipped to PostHog. metrics.RecordEvent consults this set and skips the
// PostHog Capture for these names while still incrementing the counter.
//
// As of MUL-4127, PostHog is no longer used for server-side product analytics:
// the acquisition / activation / expansion funnel is now read straight from the
// operational database and from these Grafana counters, so the redundant
// PostHog copy of every product event was retired. That makes ALL server-side
// events metrics-only — both the product-behaviour group and the original
// high-volume runtime/autopilot telemetry are Prometheus-only. PostHog now only
// receives frontend error/crash telemetry ($exception, client_crash,
// client_unresponsive); see packages/core/analytics.
//
// Note: agent_task_* lifecycle events are also Prometheus-only, but their
// Prometheus side is handled by typed BusinessMetrics.RecordTask* methods, so
// they never build an analytics.Event in the first place and don't need an
// entry here.
var metricsOnlyEvents = map[string]struct{}{
	// Product-behaviour events — DB + Grafana are the source of truth
	// (MUL-4127); the PostHog copy was redundant.
	EventSignup:                        {},
	EventWorkspaceCreated:              {},
	EventIssueCreated:                  {},
	EventIssueExecuted:                 {},
	EventChatMessageSent:               {},
	EventTeamInviteSent:                {},
	EventTeamInviteAccepted:            {},
	EventOnboardingStarted:             {},
	EventOnboardingQuestionnaireSubmit: {},
	EventOnboardingSourceSubmit:        {},
	EventAgentCreated:                  {},
	EventOnboardingCompleted:           {},
	EventCloudWaitlistJoined:           {},
	EventFeedbackSubmitted:             {},
	EventContactSalesSubmitted:         {},
	EventSquadCreated:                  {},
	EventAutopilotCreated:              {},
	EventWikiSearch:                    {},
	EventWikiProposalReview:            {},
	EventLMWikiReview:                  {},
	EventRoomCreated:                   {},
	EventRoomFirstCycleCompleted:       {},
	EventRoomSynthesisAccepted:         {},
	EventRoomSynthesisRejected:         {},
	EventRoomSynthesisRetried:          {},
	EventRoomArtifactPromoted:          {},
	EventRoomBudgetRefused:             {},
	EventRoomCycleFailed:               {},
	// High-volume runtime / autopilot execution-lifecycle telemetry — always
	// Prometheus-only (Grafana already carries the equivalent counters).
	EventRuntimeRegistered:       {},
	EventRuntimeReady:            {},
	EventRuntimeFailed:           {},
	EventRuntimeOffline:          {},
	EventAutopilotRunStarted:     {},
	EventAutopilotRunCompleted:   {},
	EventAutopilotRunFailed:      {},
	EventTwinProposalGeneration:  {},
	EventTwinSignOff:             {},
	EventTwinBriefingCompilation: {},
	EventTwinBriefingUse:         {},
	EventTwinRunFeedback:         {},
	EventTwinTaskRevision:        {},
	EventTwinDepositionReview:    {},
}

// IsMetricsOnly reports whether an event name is recorded to Prometheus but must
// not be sent to PostHog. As of MUL-4127 this is true for every server-side
// event. See metricsOnlyEvents.
func IsMetricsOnly(name string) bool {
	_, ok := metricsOnlyEvents[name]
	return ok
}

const (
	SourceOnboarding = "onboarding"
	SourceManual     = "manual"
	SourceChat       = "chat"
	SourceAutopilot  = "autopilot"
	SourceAPI        = "api"
)

// CoreProperties are the shared join and segmentation fields used by the
// canonical PostHog events. Empty values are omitted, except is_demo which is
// always stamped so dashboards can filter demo data without sparse-property
// edge cases.
type CoreProperties struct {
	UserID         string
	WorkspaceID    string
	AgentID        string
	TaskID         string
	IssueID        string
	ChatSessionID  string
	AutopilotRunID string
	Source         string
	RuntimeMode    string
	Provider       string
	IsDemo         bool
}

type TaskContext = CoreProperties

// TwinMetricContext is the complete identity surface accepted by Twin product
// metrics. In particular, Twin version, assertion, citation, and evidence IDs
// are deliberately absent: those belong in the auditable operational record,
// not analytics.
type TwinMetricContext struct {
	UserID      string
	WorkspaceID string
	TaskID      string
}

type TwinProposalKind string

const (
	TwinProposalKindInitial    TwinProposalKind = "initial"
	TwinProposalKindEvolution  TwinProposalKind = "evolution"
	TwinProposalKindDeposition TwinProposalKind = "deposition"
)

type TwinGenerationState string

const (
	TwinGenerationStateSucceeded TwinGenerationState = "succeeded"
	TwinGenerationStateFailed    TwinGenerationState = "failed"
	TwinGenerationStateBlocked   TwinGenerationState = "blocked"
)

type TwinPolicyScope string

const (
	TwinPolicyScopeOneOff    TwinPolicyScope = "one_off"
	TwinPolicyScopeIssue     TwinPolicyScope = "issue"
	TwinPolicyScopeProject   TwinPolicyScope = "project"
	TwinPolicyScopeAgent     TwinPolicyScope = "agent"
	TwinPolicyScopeWorkspace TwinPolicyScope = "workspace"
)

type TwinCompilationState string

const (
	TwinCompilationStateCompiled TwinCompilationState = "compiled"
	TwinCompilationStateExcluded TwinCompilationState = "excluded"
	TwinCompilationStateFailed   TwinCompilationState = "failed"
)

type TwinUseState string

const (
	TwinUseStatePreviewed TwinUseState = "previewed"
	TwinUseStateInjected  TwinUseState = "injected"
	TwinUseStateSkipped   TwinUseState = "skipped"
)

type TwinReviewDecision string

const (
	TwinReviewDecisionSigned   TwinReviewDecision = "signed"
	TwinReviewDecisionAccepted TwinReviewDecision = "accepted"
	TwinReviewDecisionRejected TwinReviewDecision = "rejected"
)

type TwinFeedbackRating string

const (
	TwinFeedbackRatingHelped     TwinFeedbackRating = "helped"
	TwinFeedbackRatingIrrelevant TwinFeedbackRating = "irrelevant"
	TwinFeedbackRatingMismatch   TwinFeedbackRating = "mismatch"
)

type TwinTaskRevisionDecision string

const (
	TwinTaskRevisionAccepted          TwinTaskRevisionDecision = "accepted"
	TwinTaskRevisionRequestedRevision TwinTaskRevisionDecision = "revision_requested"
)

type TwinTaskRevisionKind string

const (
	TwinTaskRevisionKindNone               TwinTaskRevisionKind = "none"
	TwinTaskRevisionKindInstructionRelated TwinTaskRevisionKind = "instruction_related"
	TwinTaskRevisionKindOther              TwinTaskRevisionKind = "other"
)

type TwinExclusionCode string

const (
	TwinExclusionNone               TwinExclusionCode = "none"
	TwinExclusionKillSwitch         TwinExclusionCode = "kill_switch"
	TwinExclusionPolicyOff          TwinExclusionCode = "policy_off"
	TwinExclusionNoSignedVersion    TwinExclusionCode = "no_signed_version"
	TwinExclusionIneligibleTask     TwinExclusionCode = "ineligible_task"
	TwinExclusionStaleVersion       TwinExclusionCode = "stale_version"
	TwinExclusionUnauthorized       TwinExclusionCode = "unauthorized"
	TwinExclusionLocalOnly          TwinExclusionCode = "local_only"
	TwinExclusionBudget             TwinExclusionCode = "budget"
	TwinExclusionUnsupportedRuntime TwinExclusionCode = "unsupported_runtime"
)

type TwinProposalGenerationMetric struct {
	Context                   TwinMetricContext
	Kind                      TwinProposalKind
	State                     TwinGenerationState
	AssertionCount            int
	CitationCount             int
	UnsupportedAssertionCount int
	LatencyMS                 int64
}

type TwinSignOffMetric struct {
	Context        TwinMetricContext
	Kind           TwinProposalKind
	Decision       TwinReviewDecision
	AssertionCount int
	CitationCount  int
}

type TwinBriefingCompilationMetric struct {
	Context        TwinMetricContext
	State          TwinCompilationState
	Scope          TwinPolicyScope
	ExclusionCode  TwinExclusionCode
	AssertionCount int
	CitationCount  int
	ExclusionCount int
	ByteCount      int
	TokenCount     int
	LatencyMS      int64
}

type TwinBriefingUseMetric struct {
	Context        TwinMetricContext
	State          TwinUseState
	Scope          TwinPolicyScope
	ExclusionCode  TwinExclusionCode
	AssertionCount int
	CitationCount  int
	ByteCount      int
	TokenCount     int
}

type TwinRunFeedbackMetric struct {
	Context TwinMetricContext
	Rating  TwinFeedbackRating
}

type TwinTaskRevisionMetric struct {
	Context       TwinMetricContext
	Decision      TwinTaskRevisionDecision
	Kind          TwinTaskRevisionKind
	RevisionCount int
}

type TwinDepositionReviewMetric struct {
	Context        TwinMetricContext
	Decision       TwinReviewDecision
	AssertionCount int
	CitationCount  int
}

// TwinProposalGeneration builds a metrics-only proposal-generation event. Its
// typed input intentionally cannot carry generated assertion text or evidence.
func TwinProposalGeneration(metric TwinProposalGenerationMetric) Event {
	return twinMetricEvent(EventTwinProposalGeneration, metric.Context, map[string]any{
		"kind":                        normalizedTwinDimension(metric.Kind, TwinProposalKindInitial, TwinProposalKindEvolution, TwinProposalKindDeposition),
		"state":                       normalizedTwinDimension(metric.State, TwinGenerationStateSucceeded, TwinGenerationStateFailed, TwinGenerationStateBlocked),
		"assertion_count":             nonNegative(metric.AssertionCount),
		"citation_count":              nonNegative(metric.CitationCount),
		"unsupported_assertion_count": nonNegative(metric.UnsupportedAssertionCount),
		"latency_ms":                  nonNegative64(metric.LatencyMS),
	})
}

func TwinSignOff(metric TwinSignOffMetric) Event {
	return twinMetricEvent(EventTwinSignOff, metric.Context, map[string]any{
		"kind":            normalizedTwinDimension(metric.Kind, TwinProposalKindInitial, TwinProposalKindEvolution, TwinProposalKindDeposition),
		"decision":        normalizedTwinDimension(metric.Decision, TwinReviewDecisionSigned, TwinReviewDecisionRejected),
		"assertion_count": nonNegative(metric.AssertionCount),
		"citation_count":  nonNegative(metric.CitationCount),
	})
}

func TwinBriefingCompilation(metric TwinBriefingCompilationMetric) Event {
	return twinMetricEvent(EventTwinBriefingCompilation, metric.Context, map[string]any{
		"state":           normalizedTwinDimension(metric.State, TwinCompilationStateCompiled, TwinCompilationStateExcluded, TwinCompilationStateFailed),
		"scope":           normalizedTwinDimension(metric.Scope, TwinPolicyScopeOneOff, TwinPolicyScopeIssue, TwinPolicyScopeProject, TwinPolicyScopeAgent, TwinPolicyScopeWorkspace),
		"exclusion_code":  normalizedTwinExclusion(metric.ExclusionCode),
		"assertion_count": nonNegative(metric.AssertionCount),
		"citation_count":  nonNegative(metric.CitationCount),
		"exclusion_count": nonNegative(metric.ExclusionCount),
		"byte_count":      nonNegative(metric.ByteCount),
		"token_count":     nonNegative(metric.TokenCount),
		"latency_ms":      nonNegative64(metric.LatencyMS),
	})
}

func TwinBriefingUse(metric TwinBriefingUseMetric) Event {
	return twinMetricEvent(EventTwinBriefingUse, metric.Context, map[string]any{
		"state":           normalizedTwinDimension(metric.State, TwinUseStatePreviewed, TwinUseStateInjected, TwinUseStateSkipped),
		"scope":           normalizedTwinDimension(metric.Scope, TwinPolicyScopeOneOff, TwinPolicyScopeIssue, TwinPolicyScopeProject, TwinPolicyScopeAgent, TwinPolicyScopeWorkspace),
		"exclusion_code":  normalizedTwinExclusion(metric.ExclusionCode),
		"assertion_count": nonNegative(metric.AssertionCount),
		"citation_count":  nonNegative(metric.CitationCount),
		"byte_count":      nonNegative(metric.ByteCount),
		"token_count":     nonNegative(metric.TokenCount),
	})
}

func TwinRunFeedback(metric TwinRunFeedbackMetric) Event {
	return twinMetricEvent(EventTwinRunFeedback, metric.Context, map[string]any{
		"rating": normalizedTwinDimension(metric.Rating, TwinFeedbackRatingHelped, TwinFeedbackRatingIrrelevant, TwinFeedbackRatingMismatch),
	})
}

func TwinTaskRevision(metric TwinTaskRevisionMetric) Event {
	return twinMetricEvent(EventTwinTaskRevision, metric.Context, map[string]any{
		"decision":       normalizedTwinDimension(metric.Decision, TwinTaskRevisionAccepted, TwinTaskRevisionRequestedRevision),
		"kind":           normalizedTwinDimension(metric.Kind, TwinTaskRevisionKindNone, TwinTaskRevisionKindInstructionRelated, TwinTaskRevisionKindOther),
		"revision_count": nonNegative(metric.RevisionCount),
	})
}

func TwinDepositionReview(metric TwinDepositionReviewMetric) Event {
	return twinMetricEvent(EventTwinDepositionReview, metric.Context, map[string]any{
		"decision":        normalizedTwinDimension(metric.Decision, TwinReviewDecisionAccepted, TwinReviewDecisionRejected),
		"assertion_count": nonNegative(metric.AssertionCount),
		"citation_count":  nonNegative(metric.CitationCount),
	})
}

func twinMetricEvent(name string, context TwinMetricContext, properties map[string]any) Event {
	if context.WorkspaceID != "" {
		properties["workspace_id"] = context.WorkspaceID
	}
	if context.UserID != "" {
		properties["user_id"] = context.UserID
	}
	if context.TaskID != "" {
		properties["task_id"] = context.TaskID
	}
	return Event{
		Name:        name,
		DistinctID:  context.UserID,
		WorkspaceID: context.WorkspaceID,
		Properties:  properties,
	}
}

func normalizedTwinDimension[T ~string](value T, allowed ...T) string {
	for _, candidate := range allowed {
		if value == candidate {
			return string(value)
		}
	}
	return "unknown"
}

func normalizedTwinExclusion(value TwinExclusionCode) string {
	return normalizedTwinDimension(value,
		TwinExclusionNone,
		TwinExclusionKillSwitch,
		TwinExclusionPolicyOff,
		TwinExclusionNoSignedVersion,
		TwinExclusionIneligibleTask,
		TwinExclusionStaleVersion,
		TwinExclusionUnauthorized,
		TwinExclusionLocalOnly,
		TwinExclusionBudget,
		TwinExclusionUnsupportedRuntime,
	)
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegative64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

// Onboarding completion paths. Keep in sync with packages/core/analytics.
const (
	OnboardingPathFull           = "full"            // reached first_issue end of flow
	OnboardingPathRuntimeSkipped = "runtime_skipped" // completed without connecting a runtime
	OnboardingPathCloudWaitlist  = "cloud_waitlist"  // completed via cloud waitlist soft exit
	OnboardingPathSkipExisting   = "skip_existing"   // "I've done this before" from welcome
	OnboardingPathInviteAccept   = "invite_accept"   // accepted at least one invitation from /invitations
	OnboardingPathUnknown        = "unknown"         // fallback when the server can't derive the path
)

// Platform is used as the "platform" event property so funnels can split by
// web / desktop / cli. Request-path events use PlatformServer as a fallback
// when the caller is a server-originating action (e.g. auto-created user);
// otherwise the frontend passes the real platform via a header / body field
// in later iterations.
const (
	PlatformServer  = "server"
	PlatformWeb     = "web"
	PlatformDesktop = "desktop"
	PlatformCLI     = "cli"
)

// Signup builds the signup event. signupSource is populated from the
// frontend's stored UTM/referrer cookie if present; leave empty otherwise.
func Signup(userID, email, signupSource string) Event {
	return Event{
		Name:       EventSignup,
		DistinctID: userID,
		Properties: map[string]any{
			"email_domain":  emailDomain(email),
			"signup_source": signupSource,
		},
		SetOnce: map[string]any{
			"email":         email,
			"signup_source": signupSource,
		},
	}
}

// WorkspaceCreated builds the workspace_created event. "Is this the user's
// first workspace?" is deliberately not stamped here — it's derived in
// PostHog by checking whether the user has a prior workspace_created event.
func WorkspaceCreated(userID, workspaceID string) Event {
	return Event{
		Name:        EventWorkspaceCreated,
		DistinctID:  userID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(nil, CoreProperties{
			UserID:      userID,
			WorkspaceID: workspaceID,
			Source:      SourceManual,
		}),
	}
}

// RuntimeRegistered fires on the first time a (workspace, daemon, provider)
// triple is upserted. The handler uses a `xmax = 0` flag returned from the
// upsert query to distinguish inserts from updates — heartbeats and repeat
// registrations never emit this event.
//
// ownerID may be empty when the daemon authenticates via a daemon token
// (no user context); downstream funnels that need per-user attribution
// fall back to `workspace_id` as the grouping key.
func RuntimeRegistered(ownerID, workspaceID, runtimeID, daemonID, provider, runtimeVersion, cliVersion string) Event {
	distinct := ownerID
	if distinct == "" {
		// A per-workspace synthetic id keeps PostHog from merging unrelated
		// daemon registrations across workspaces under a single "anonymous"
		// person. It's stable within a workspace so repeat heartbeats (which
		// don't emit anyway) would at least group correctly.
		distinct = "workspace:" + workspaceID
	}
	return Event{
		Name:        EventRuntimeRegistered,
		DistinctID:  distinct,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"runtime_id":      runtimeID,
			"daemon_id":       daemonID,
			"provider":        provider,
			"runtime_mode":    "local",
			"runtime_version": runtimeVersion,
			"cli_version":     cliVersion,
		}, CoreProperties{
			UserID:      ownerID,
			WorkspaceID: workspaceID,
			Source:      SourceManual,
			RuntimeMode: "local",
			Provider:    provider,
		}),
	}
}

func RuntimeReady(ownerID, workspaceID, runtimeID, daemonID, provider string, readyDurationMS int64) Event {
	distinct := ownerID
	if distinct == "" {
		distinct = "workspace:" + workspaceID
	}
	props := map[string]any{
		"runtime_id": runtimeID,
		"daemon_id":  daemonID,
	}
	if readyDurationMS > 0 {
		props["ready_duration_ms"] = readyDurationMS
	}
	return Event{
		Name:        EventRuntimeReady,
		DistinctID:  distinct,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(props, CoreProperties{
			UserID:      ownerID,
			WorkspaceID: workspaceID,
			Source:      SourceManual,
			RuntimeMode: "local",
			Provider:    provider,
		}),
	}
}

func RuntimeFailed(ownerID, workspaceID, daemonID, provider, failureReason, errorType string, recoverable bool) Event {
	distinct := ownerID
	if distinct == "" && workspaceID != "" {
		distinct = "workspace:" + workspaceID
	}
	return Event{
		Name:        EventRuntimeFailed,
		DistinctID:  distinct,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"daemon_id":      daemonID,
			"failure_reason": failureReason,
			"error_type":     errorType,
			"recoverable":    recoverable,
		}, CoreProperties{
			UserID:      ownerID,
			WorkspaceID: workspaceID,
			Source:      SourceManual,
			RuntimeMode: "local",
			Provider:    provider,
		}),
	}
}

func RuntimeOffline(ownerID, workspaceID, runtimeID, daemonID, provider string) Event {
	distinct := ownerID
	if distinct == "" {
		distinct = "workspace:" + workspaceID
	}
	return Event{
		Name:        EventRuntimeOffline,
		DistinctID:  distinct,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"runtime_id": runtimeID,
			"daemon_id":  daemonID,
		}, CoreProperties{
			UserID:      ownerID,
			WorkspaceID: workspaceID,
			Source:      SourceManual,
			RuntimeMode: "local",
			Provider:    provider,
		}),
	}
}

// IssueExecuted fires at most once per issue lifetime — on the first task
// completion that flips `issues.first_executed_at` from NULL via an atomic
// UPDATE. Retries, re-assignments, and comment-triggered follow-ups never
// re-emit, which is what keeps the ≥1/≥2/≥5/≥10 funnel buckets honest.
//
// Deliberately not stamped here: the workspace's Nth-issue ordinal.
// Computing it at emit time is not atomic (two concurrent first-completions
// both read count=1, both emit n=1), and PostHog derives the same number
// exactly at query time from the event stream.
func IssueExecuted(actorID, workspaceID, issueID, taskID, agentID, source, runtimeMode, provider string, taskDurationMS int64) Event {
	return Event{
		Name:        EventIssueExecuted,
		DistinctID:  actorID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"issue_id":         issueID,
			"task_id":          taskID,
			"agent_id":         agentID,
			"task_duration_ms": taskDurationMS,
			"duration_ms":      taskDurationMS,
		}, CoreProperties{
			UserID:      nonAgentUserID(actorID),
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			TaskID:      taskID,
			IssueID:     issueID,
			Source:      source,
			RuntimeMode: runtimeMode,
			Provider:    provider,
		}),
	}
}

func IssueCreated(actorID, workspaceID, issueID, agentID, taskID, autopilotRunID, source, platform string) Event {
	props := map[string]any{}
	if platform != "" {
		props["platform"] = platform
	}
	return Event{
		Name:        EventIssueCreated,
		DistinctID:  actorID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(props, CoreProperties{
			UserID:         nonAgentUserID(actorID),
			WorkspaceID:    workspaceID,
			AgentID:        agentID,
			TaskID:         taskID,
			IssueID:        issueID,
			AutopilotRunID: autopilotRunID,
			Source:         source,
		}),
	}
}

func ChatMessageSent(userID, workspaceID, chatSessionID, taskID, agentID, runtimeMode, provider, platform string) Event {
	props := map[string]any{}
	if platform != "" {
		props["platform"] = platform
	}
	return Event{
		Name:        EventChatMessageSent,
		DistinctID:  userID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(props, CoreProperties{
			UserID:        userID,
			WorkspaceID:   workspaceID,
			AgentID:       agentID,
			TaskID:        taskID,
			ChatSessionID: chatSessionID,
			Source:        SourceChat,
			RuntimeMode:   runtimeMode,
			Provider:      provider,
		}),
	}
}

// AutopilotAssignee describes the autopilot's configured target. agent_id is
// always the agent that will actually execute the work (the squad leader for
// squad autopilots) so funnels grouping by agent stay consistent. assignee_*
// fields record the original configuration so reports can tell a solo-agent
// autopilot apart from a squad one without joining back to the autopilot row.
type AutopilotAssignee struct {
	AgentID      string // executing agent — leader for squad autopilots
	AssigneeType string // "agent" or "squad"
	SquadID      string // empty when AssigneeType != "squad"
}

func AutopilotRunStarted(actorID, workspaceID, autopilotID, runID, cadence string, assignee AutopilotAssignee, triggerSource string) Event {
	return autopilotRunEvent(EventAutopilotRunStarted, actorID, workspaceID, autopilotID, runID, cadence, assignee, triggerSource, nil)
}

func AutopilotRunCompleted(actorID, workspaceID, autopilotID, runID, cadence string, assignee AutopilotAssignee, triggerSource string, durationMS int64) Event {
	return autopilotRunEvent(EventAutopilotRunCompleted, actorID, workspaceID, autopilotID, runID, cadence, assignee, triggerSource, map[string]any{
		"duration_ms": durationMS,
	})
}

func AutopilotRunFailed(actorID, workspaceID, autopilotID, runID, cadence string, assignee AutopilotAssignee, triggerSource, failureReason, errorType string, willRetry bool, durationMS int64) Event {
	return autopilotRunEvent(EventAutopilotRunFailed, actorID, workspaceID, autopilotID, runID, cadence, assignee, triggerSource, map[string]any{
		"duration_ms":    durationMS,
		"failure_reason": failureReason,
		"error_type":     errorType,
		"will_retry":     willRetry,
	})
}

// TeamInviteSent fires when a workspace admin creates an invitation.
// inviteMethod is "email" for now; future non-email invite flows can pass
// their own value to keep this stable.
func TeamInviteSent(inviterID, workspaceID, invitedEmail, inviteMethod string) Event {
	return Event{
		Name:        EventTeamInviteSent,
		DistinctID:  inviterID,
		WorkspaceID: workspaceID,
		Properties: map[string]any{
			"invited_email_domain": emailDomain(invitedEmail),
			"invite_method":        inviteMethod,
		},
	}
}

// TeamInviteAccepted fires when the invitee accepts and joins the workspace.
// daysSinceInvite lets us segment fast-acceptance (warm) from long-tail
// acceptance (someone dug through old email).
func TeamInviteAccepted(inviteeID, workspaceID string, daysSinceInvite int64) Event {
	return Event{
		Name:        EventTeamInviteAccepted,
		DistinctID:  inviteeID,
		WorkspaceID: workspaceID,
		Properties: map[string]any{
			"days_since_invite": daysSinceInvite,
		},
	}
}

// OnboardingQuestionnaireSubmitted fires the first time a user's
// `user.onboarding_questionnaire` transitions from "at least one slot
// unresolved" to "every slot has either an answer or a skip marker".
// The handler drives this transition — we emit from PatchOnboarding so
// the single emission site stays honest even if the frontend retries.
//
// `useCase` is multi-select (users can pick several); `source` is
// single-select (primary acquisition channel) but kept as a slice
// for back-compat with v2 multi-select rows — single-element in
// current data. `role` stays single-select. Empty slice = no answer
// (skip is captured separately via the *Skipped booleans).
//
// The three answers are also mirrored into person properties via $set
// so cohorting by source / role / use_case works across every event
// on the same user without re-joining back to the DB. PostHog accepts
// array property values; breakdowns on a multi-value property treat
// each element as a separate group.
//
// `*Skipped` booleans capture per-question skip intent. `*HasOther`
// are presence booleans for the free-text "other" override; the
// free-text content is kept in the DB for product research but not
// broadcast via analytics (PII risk + low cardinality ask).
// OnboardingStarted fires from the server side the first time a user's
// onboarding state transitions from untouched (no questionnaire payload
// recorded) to any non-empty patch. Frontends emit their own
// onboarding_started on first page open; the server emission is what
// lights up the Prometheus counter so Grafana can be cross-checked
// against the PostHog funnel without depending on the SDK roundtrip.
//
// platform is the X-Client-Platform header value at the time of the
// first onboarding interaction, fed into the
// `multica_onboarding_started_total{platform=...}` label via the fixed
// allow-list in metrics.NormalizePlatform.
func OnboardingStarted(userID, platform string) Event {
	props := map[string]any{}
	if platform != "" {
		props["platform"] = platform
	}
	return Event{
		Name:       EventOnboardingStarted,
		DistinctID: userID,
		Properties: withCoreProperties(props, CoreProperties{
			UserID: userID,
			Source: SourceOnboarding,
		}),
	}
}

func OnboardingQuestionnaireSubmitted(userID string, source []string, role string, useCase []string, sourceSkipped, roleSkipped, useCaseSkipped, sourceHasOther, roleHasOther, useCaseHasOther bool) Event {
	// Normalize nil slices to [] so PostHog property values are stable
	// (avoids null vs [] mixing in property type inference).
	if source == nil {
		source = []string{}
	}
	if useCase == nil {
		useCase = []string{}
	}
	return Event{
		Name:       EventOnboardingQuestionnaireSubmit,
		DistinctID: userID,
		Properties: withCoreProperties(map[string]any{
			"source":             source,
			"role":               role,
			"use_case":           useCase,
			"source_skipped":     sourceSkipped,
			"role_skipped":       roleSkipped,
			"use_case_skipped":   useCaseSkipped,
			"source_has_other":   sourceHasOther,
			"role_has_other":     roleHasOther,
			"use_case_has_other": useCaseHasOther,
		}, CoreProperties{
			UserID: userID,
			Source: SourceOnboarding,
		}),
		Set: map[string]any{
			"source":   source,
			"role":     role,
			"use_case": useCase,
		},
	}
}

// OnboardingSourceSubmitted fires when the user's acquisition source
// transitions from unresolved to resolved — answered or explicitly
// declined. The source question is no longer part of the onboarding
// flow (MUL-5159): it is asked by the workspace backfill prompt after
// agents have completed work for the user, so this lands well after
// `onboarding_questionnaire_submitted` (which now covers role +
// use_case only). A dedicated event gives the backfill prompt its own
// Grafana counter (answer/decline rate) without stalling the
// questionnaire funnel step. Metrics-only like every server event
// (MUL-4127); the per-user source value reaches analytics through the
// client-side person-property mirror in saveQuestionnaire.
//
// `source` stays a slice for the same v2 back-compat reason as the
// questionnaire event; the client commits a one-element array. $set
// is only attached when the user actually answered — moot while the
// event stays metrics-only, but kept accurate should it ever ship.
func OnboardingSourceSubmitted(userID string, source []string, skipped, hasOther bool) Event {
	if source == nil {
		source = []string{}
	}
	// Property key is acquisition_source, not source — core properties
	// stamp the event-source dimension into props["source"]
	// (withCoreProperties), and the acquisition answer must not fight
	// it for the slot. The $set person property below keeps the plain
	// "source" name for cohort continuity with the client-side mirror.
	ev := Event{
		Name:       EventOnboardingSourceSubmit,
		DistinctID: userID,
		Properties: withCoreProperties(map[string]any{
			"acquisition_source": source,
			"source_skipped":     skipped,
			"source_has_other":   hasOther,
		}, CoreProperties{
			UserID: userID,
			Source: SourceOnboarding,
		}),
	}
	if len(source) > 0 {
		ev.Set = map[string]any{"source": source}
	}
	return ev
}

// AgentCreated fires whenever a new agent is added to a workspace — not
// just inside onboarding. `isFirstAgentInWorkspace` lets the funnel
// isolate the Step 4 signal from later agent additions.
//
// template is the creation-source attribution supplied by the caller (for
// example, "agent_builder"); empty identifies a manually authored agent.
func AgentCreated(actorID, workspaceID, agentID, provider, runtimeMode, template string, isFirstAgentInWorkspace bool) Event {
	return Event{
		Name:        EventAgentCreated,
		DistinctID:  actorID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"agent_id":                    agentID,
			"provider":                    provider,
			"runtime_mode":                runtimeMode,
			"template":                    template,
			"is_first_agent_in_workspace": isFirstAgentInWorkspace,
		}, CoreProperties{
			UserID:      actorID,
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Source:      SourceManual,
			RuntimeMode: runtimeMode,
			Provider:    provider,
		}),
	}
}

// OnboardingCompleted fires from CompleteOnboarding. `completionPath`
// is derived server-side from the state the user arrived in (see the
// OnboardingPath* constants above). `joinedCloudWaitlist` is true when
// the user submitted the waitlist form at any point during the flow —
// it's orthogonal to `completion_path`; a user may submit the form and
// still pick CLI, so we keep both signals.
//
// onboardedAt is an RFC3339 timestamp set $set_once on the person so
// "onboarded before date X" cohorts are queryable directly from
// person_properties without re-emitting per-event.
func OnboardingCompleted(userID, workspaceID, completionPath, onboardedAt string, joinedCloudWaitlist bool) Event {
	return Event{
		Name:        EventOnboardingCompleted,
		DistinctID:  userID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"completion_path":       completionPath,
			"joined_cloud_waitlist": joinedCloudWaitlist,
		}, CoreProperties{
			UserID:      userID,
			WorkspaceID: workspaceID,
			Source:      SourceOnboarding,
		}),
		SetOnce: map[string]any{
			"onboarded_at": onboardedAt,
		},
	}
}

// CloudWaitlistJoined fires when a user submits the Step 3 cloud
// waitlist form. `hasReason` is a presence bool — the free-text reason
// stays in the DB for product research.
func CloudWaitlistJoined(userID string, hasReason bool) Event {
	return Event{
		Name:       EventCloudWaitlistJoined,
		DistinctID: userID,
		Properties: withCoreProperties(map[string]any{
			"has_reason": hasReason,
		}, CoreProperties{
			UserID: userID,
			Source: SourceOnboarding,
		}),
	}
}

// FeedbackSubmitted fires after a feedback row is successfully inserted.
// The raw message is stored in the DB and never broadcast — we only emit a
// coarse length bucket, an image-presence flag, the kind picker selection,
// and the client platform / version so support can segment without leaking
// content.
func FeedbackSubmitted(userID, workspaceID, kind string, messageLen int, hasImages bool, platform, appVersion string) Event {
	props := map[string]any{
		"message_length_bucket": feedbackLengthBucket(messageLen),
		"has_images":            hasImages,
	}
	if kind != "" {
		props["kind"] = kind
	}
	if platform != "" {
		props["platform"] = platform
	}
	if appVersion != "" {
		props["app_version"] = appVersion
	}
	return Event{
		Name:        EventFeedbackSubmitted,
		DistinctID:  userID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(props, CoreProperties{
			UserID:      userID,
			WorkspaceID: workspaceID,
			Source:      "ops_feedback",
		}),
	}
}

// ContactSalesSubmitted fires after a contact-sales inquiry is recorded.
// The form is public and unauthenticated, so DistinctID is empty (PostHog
// will treat it as an anonymous event). We carry the coarse company size,
// country, intended use case, and the form-location bucket (page /
// onboarding / agents_page) so sales / marketing can split inbound volume
// without having to query the operational DB.
//
// formSource is the page-context bucket; the CoreProperties Source stays
// "marketing_contact_sales" so PostHog dashboards keep the funnel join
// against other marketing events. The Prometheus side reads form_source
// directly via the metrics.NormalizeContactSalesSource allow-list.
func ContactSalesSubmitted(inquiryID, companySize, countryRegion, useCase, formSource string, hasGoals bool) Event {
	props := map[string]any{
		"inquiry_id":     inquiryID,
		"company_size":   companySize,
		"country_region": countryRegion,
		"use_case":       useCase,
		"has_goals":      hasGoals,
	}
	if formSource != "" {
		props["form_source"] = formSource
	}
	return Event{
		Name:       EventContactSalesSubmitted,
		DistinctID: inquiryID,
		Properties: withCoreProperties(props, CoreProperties{
			Source: "marketing_contact_sales",
		}),
	}
}

// SquadCreated fires when a workspace member or admin creates a new squad.
// `memberCount` is the number of members the squad was seeded with at
// creation time (frontend can pre-populate via the picker).
func SquadCreated(actorID, workspaceID, squadID string, memberCount int) Event {
	return Event{
		Name:        EventSquadCreated,
		DistinctID:  actorID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"squad_id":     squadID,
			"member_count": int64(memberCount),
		}, CoreProperties{
			UserID:      nonAgentUserID(actorID),
			WorkspaceID: workspaceID,
			Source:      SourceManual,
		}),
	}
}

// AutopilotCreated fires when a workspace member creates a new autopilot.
// `cadence` matches the autopilot.cadence enum (hourly/daily/weekly/...
// /webhook). triggerKind is the initial trigger type (schedule / webhook /
// manual) — when both schedule and webhook triggers are seeded, we report
// the dominant one (schedule wins).
func AutopilotCreated(actorID, workspaceID, autopilotID, cadence, triggerKind string) Event {
	return Event{
		Name:        EventAutopilotCreated,
		DistinctID:  actorID,
		WorkspaceID: workspaceID,
		Properties: withCoreProperties(map[string]any{
			"autopilot_id": autopilotID,
			"cadence":      cadence,
			"trigger_kind": triggerKind,
		}, CoreProperties{
			UserID:      nonAgentUserID(actorID),
			WorkspaceID: workspaceID,
			Source:      SourceManual,
		}),
	}
}

// WikiSearch records whether a tenanted Wiki search returned at least one
// result. The constructor intentionally cannot accept the query, paths, IDs,
// or result content, so sensitive knowledge cannot enter analytics by mistake.
func WikiSearch(scope string, resultCount int) Event {
	result := "empty"
	if resultCount > 0 {
		result = "hit"
	}
	return Event{
		Name:       EventWikiSearch,
		DistinctID: "server",
		Properties: map[string]any{"scope": scope, "result": result},
	}
}

// WikiProposalReview records the bounded human review outcome. edited only
// reports whether the reviewer changed the proposal before accepting it.
func WikiProposalReview(decision string, edited bool) Event {
	return Event{
		Name:       EventWikiProposalReview,
		DistinctID: "server",
		Properties: map[string]any{"decision": decision, "edited": edited},
	}
}

// LMWikiReview records acceptance/rejection without artifact or citation IDs.
func LMWikiReview(decision string) Event {
	return Event{
		Name:       EventLMWikiReview,
		DistinctID: "server",
		Properties: map[string]any{"decision": decision},
	}
}

func autopilotRunEvent(name, actorID, workspaceID, autopilotID, runID, cadence string, assignee AutopilotAssignee, triggerSource string, extra map[string]any) Event {
	if extra == nil {
		extra = map[string]any{}
	}
	extra["trigger_source"] = triggerSource
	extra["trigger_kind"] = triggerSource
	if cadence != "" {
		extra["cadence"] = cadence
	}
	props := withCoreProperties(extra, CoreProperties{
		UserID:         nonAgentUserID(actorID),
		WorkspaceID:    workspaceID,
		AgentID:        assignee.AgentID,
		AutopilotRunID: runID,
		Source:         SourceAutopilot,
	})
	props["autopilot_id"] = autopilotID
	if assignee.AssigneeType != "" {
		props["assignee_type"] = assignee.AssigneeType
	}
	if assignee.SquadID != "" {
		props["squad_id"] = assignee.SquadID
	}
	return Event{
		Name:        name,
		DistinctID:  actorID,
		WorkspaceID: workspaceID,
		Properties:  props,
	}
}

func withCoreProperties(props map[string]any, core CoreProperties) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	if core.UserID != "" {
		props["user_id"] = core.UserID
	}
	if core.AgentID != "" {
		props["agent_id"] = core.AgentID
	}
	if core.TaskID != "" {
		props["task_id"] = core.TaskID
	}
	if core.IssueID != "" {
		props["issue_id"] = core.IssueID
	}
	if core.ChatSessionID != "" {
		props["chat_session_id"] = core.ChatSessionID
	}
	if core.AutopilotRunID != "" {
		props["autopilot_run_id"] = core.AutopilotRunID
	}
	if core.Source != "" {
		props["source"] = core.Source
	}
	if core.RuntimeMode != "" {
		props["runtime_mode"] = core.RuntimeMode
	}
	if core.Provider != "" {
		props["provider"] = core.Provider
	}
	props["is_demo"] = core.IsDemo
	return props
}

func nonAgentUserID(distinct string) string {
	if distinct == "" || strings.Contains(distinct, ":") {
		return ""
	}
	return distinct
}

func feedbackLengthBucket(n int) string {
	switch {
	case n < 100:
		return "0-100"
	case n < 500:
		return "100-500"
	case n < 2000:
		return "500-2000"
	default:
		return "2000+"
	}
}

func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}
