package analytics

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRuntimeReadyOmitsUnmeasuredDuration(t *testing.T) {
	ev := RuntimeReady("user-1", "workspace-1", "runtime-1", "daemon-1", "codex", 0)
	if _, ok := ev.Properties["ready_duration_ms"]; ok {
		t.Fatalf("ready_duration_ms should be omitted until it is measured")
	}

	ev = RuntimeReady("user-1", "workspace-1", "runtime-1", "daemon-1", "codex", 123)
	if got := ev.Properties["ready_duration_ms"]; got != int64(123) {
		t.Fatalf("ready_duration_ms = %v, want 123", got)
	}
}

func TestFailedEventsUseWillRetry(t *testing.T) {
	runEv := AutopilotRunFailed("user-1", "workspace-1", "autopilot-1", "run-1", "manual", AutopilotAssignee{AgentID: "agent-1", AssigneeType: "agent"}, "manual", "task failed", "task_error", false, 10)
	if got := runEv.Properties["will_retry"]; got != false {
		t.Fatalf("autopilot will_retry = %v, want false", got)
	}
	if _, ok := runEv.Properties["recoverable"]; ok {
		t.Fatalf("autopilot failure should not emit recoverable")
	}
}

func TestIsMetricsOnly(t *testing.T) {
	// As of MUL-4127, PostHog is retired for server-side product analytics:
	// every server-side event is Prometheus-only and must not ship to PostHog.
	for _, name := range []string{
		// runtime / autopilot execution-lifecycle telemetry
		EventRuntimeRegistered, EventRuntimeReady, EventRuntimeFailed, EventRuntimeOffline,
		EventAutopilotRunStarted, EventAutopilotRunCompleted, EventAutopilotRunFailed,
		// product-behaviour events (now DB + Grafana only)
		EventSignup, EventWorkspaceCreated, EventIssueCreated, EventIssueExecuted,
		EventChatMessageSent, EventTeamInviteSent, EventTeamInviteAccepted,
		EventOnboardingStarted, EventOnboardingQuestionnaireSubmit, EventOnboardingSourceSubmit,
		EventAgentCreated,
		EventOnboardingCompleted, EventCloudWaitlistJoined, EventFeedbackSubmitted,
		EventContactSalesSubmitted, EventSquadCreated, EventAutopilotCreated,
		// Twin execution lifecycle remains metrics-only and never reaches PostHog.
		EventTwinProposalGeneration, EventTwinSignOff,
		EventTwinBriefingCompilation, EventTwinBriefingUse,
		EventTwinRunFeedback, EventTwinTaskRevision, EventTwinDepositionReview,
	} {
		if !IsMetricsOnly(name) {
			t.Errorf("IsMetricsOnly(%q) = false, want true (server events stay out of PostHog since MUL-4127)", name)
		}
	}
	// A name that isn't a declared server event is not metrics-only.
	if IsMetricsOnly("$exception") {
		t.Errorf("IsMetricsOnly(%q) = true, want false (frontend-only event)", "$exception")
	}
}

func TestTwinMetricContractsArePrivacySafe(t *testing.T) {
	context := TwinMetricContext{UserID: "user-1", WorkspaceID: "workspace-1", TaskID: "task-1"}
	tests := []struct {
		name    string
		event   Event
		allowed []string
	}{
		{
			name: "proposal generation",
			event: TwinProposalGeneration(TwinProposalGenerationMetric{
				Context: context, Kind: TwinProposalKindInitial, State: TwinGenerationStateSucceeded,
				AssertionCount: 3, CitationCount: 2, UnsupportedAssertionCount: 1, LatencyMS: 42,
			}),
			allowed: []string{"assertion_count", "citation_count", "kind", "latency_ms", "state", "task_id", "unsupported_assertion_count", "user_id", "workspace_id"},
		},
		{
			name: "sign off",
			event: TwinSignOff(TwinSignOffMetric{
				Context: context, Kind: TwinProposalKindInitial, Decision: TwinReviewDecisionSigned,
				AssertionCount: 3, CitationCount: 2,
			}),
			allowed: []string{"assertion_count", "citation_count", "decision", "kind", "task_id", "user_id", "workspace_id"},
		},
		{
			name: "briefing compilation",
			event: TwinBriefingCompilation(TwinBriefingCompilationMetric{
				Context: context, State: TwinCompilationStateCompiled, Scope: TwinPolicyScopeIssue,
				ExclusionCode: TwinExclusionNone, AssertionCount: 3, CitationCount: 2,
				ExclusionCount: 1, ByteCount: 512, TokenCount: 128, LatencyMS: 7,
			}),
			allowed: []string{"assertion_count", "byte_count", "citation_count", "exclusion_code", "exclusion_count", "latency_ms", "scope", "state", "task_id", "token_count", "user_id", "workspace_id"},
		},
		{
			name: "briefing use",
			event: TwinBriefingUse(TwinBriefingUseMetric{
				Context: context, State: TwinUseStateInjected, Scope: TwinPolicyScopeAgent,
				ExclusionCode: TwinExclusionNone, AssertionCount: 3, CitationCount: 2,
				ByteCount: 512, TokenCount: 128,
			}),
			allowed: []string{"assertion_count", "byte_count", "citation_count", "exclusion_code", "scope", "state", "task_id", "token_count", "user_id", "workspace_id"},
		},
		{
			name:    "run feedback",
			event:   TwinRunFeedback(TwinRunFeedbackMetric{Context: context, Rating: TwinFeedbackRatingHelped}),
			allowed: []string{"rating", "task_id", "user_id", "workspace_id"},
		},
		{
			name: "task revision",
			event: TwinTaskRevision(TwinTaskRevisionMetric{
				Context: context, Decision: TwinTaskRevisionRequestedRevision,
				Kind: TwinTaskRevisionKindInstructionRelated, RevisionCount: 1,
			}),
			allowed: []string{"decision", "kind", "revision_count", "task_id", "user_id", "workspace_id"},
		},
		{
			name: "deposition review",
			event: TwinDepositionReview(TwinDepositionReviewMetric{
				Context: context, Decision: TwinReviewDecisionAccepted,
				AssertionCount: 1, CitationCount: 1,
			}),
			allowed: []string{"assertion_count", "citation_count", "decision", "task_id", "user_id", "workspace_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsMetricsOnly(tt.event.Name) {
				t.Fatalf("%s must remain metrics-only", tt.event.Name)
			}
			got := make([]string, 0, len(tt.event.Properties))
			for key := range tt.event.Properties {
				got = append(got, key)
			}
			sort.Strings(got)
			sort.Strings(tt.allowed)
			if !reflect.DeepEqual(got, tt.allowed) {
				t.Fatalf("properties = %v, want privacy allowlist %v", got, tt.allowed)
			}
		})
	}
}

func TestTwinMetricInputTypesCannotCarrySensitivePayloads(t *testing.T) {
	metricTypes := []reflect.Type{
		reflect.TypeOf(TwinMetricContext{}),
		reflect.TypeOf(TwinProposalGenerationMetric{}),
		reflect.TypeOf(TwinSignOffMetric{}),
		reflect.TypeOf(TwinBriefingCompilationMetric{}),
		reflect.TypeOf(TwinBriefingUseMetric{}),
		reflect.TypeOf(TwinRunFeedbackMetric{}),
		reflect.TypeOf(TwinTaskRevisionMetric{}),
		reflect.TypeOf(TwinDepositionReviewMetric{}),
	}
	forbidden := []string{
		"content", "text", "prompt", "briefing", "path", "credential", "secret",
		"assertionid", "assertionids", "citationid", "citationids", "rawcitation",
	}
	for _, typ := range metricTypes {
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).Name == "Context" && typ.Field(i).Type == reflect.TypeOf(TwinMetricContext{}) {
				continue
			}
			field := strings.ToLower(typ.Field(i).Name)
			for _, fragment := range forbidden {
				if strings.Contains(field, fragment) {
					t.Fatalf("%s.%s exposes forbidden analytics payload fragment %q", typ.Name(), typ.Field(i).Name, fragment)
				}
			}
		}
	}
}

func TestTwinMetricDimensionsRejectArbitraryStringsAndNegativeNumbers(t *testing.T) {
	event := TwinBriefingCompilation(TwinBriefingCompilationMetric{
		State:          TwinCompilationState("raw briefing text"),
		Scope:          TwinPolicyScope("/Users/private/repo"),
		ExclusionCode:  TwinExclusionCode("token=secret"),
		AssertionCount: -1,
		CitationCount:  -2,
		ExclusionCount: -3,
		ByteCount:      -4,
		TokenCount:     -5,
		LatencyMS:      -6,
	})
	for _, key := range []string{"state", "scope", "exclusion_code"} {
		if got := event.Properties[key]; got != "unknown" {
			t.Errorf("%s = %v, want unknown", key, got)
		}
	}
	for _, key := range []string{"assertion_count", "citation_count", "exclusion_count", "byte_count", "token_count", "latency_ms"} {
		switch got := event.Properties[key].(type) {
		case int:
			if got != 0 {
				t.Errorf("%s = %d, want 0", key, got)
			}
		case int64:
			if got != 0 {
				t.Errorf("%s = %d, want 0", key, got)
			}
		default:
			t.Errorf("%s has unexpected type %T", key, got)
		}
	}
}

func TestOnboardingSourceSubmittedSetOnlyWhenAnswered(t *testing.T) {
	answered := OnboardingSourceSubmitted("u1", []string{"search"}, false, false)
	if answered.Properties["source_skipped"] != false {
		t.Fatalf("answered: source_skipped = %v, want false", answered.Properties["source_skipped"])
	}
	if answered.Set == nil || answered.Set["source"] == nil {
		t.Fatalf("answered: expected $set source, got %v", answered.Set)
	}

	declined := OnboardingSourceSubmitted("u1", nil, true, false)
	if declined.Properties["source_skipped"] != true {
		t.Fatalf("declined: source_skipped = %v, want true", declined.Properties["source_skipped"])
	}
	if declined.Set != nil {
		t.Fatalf("declined: a skip has nothing to mirror — expected nil Set, got %v", declined.Set)
	}
	// nil slice must normalize to [] so property types stay stable.
	// (Key is acquisition_source — plain "source" is the event-source
	// dimension stamped by core properties.)
	if src, ok := declined.Properties["acquisition_source"].([]string); !ok || src == nil {
		t.Fatalf("declined: acquisition_source property = %#v, want empty []string", declined.Properties["acquisition_source"])
	}
}
