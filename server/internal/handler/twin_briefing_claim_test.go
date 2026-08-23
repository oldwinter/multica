package handler

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type fakeTwinBriefingClaimResolver struct {
	input      TwinBriefingClaimInput
	resolution TwinBriefingClaimResolution
	err        error
}

type compilingTwinBriefingClaimResolver struct {
	input service.TwinBriefingInput
}

func (r compilingTwinBriefingClaimResolver) ResolveTwinBriefingForClaim(_ context.Context, claim TwinBriefingClaimInput) (TwinBriefingClaimResolution, error) {
	input := r.input
	input.Task.TaskID = claim.TaskID
	input.Task.WorkspaceID = claim.WorkspaceID
	input.Task.AgentID = claim.AgentID
	input.Task.ProjectID = claim.ProjectID
	input.Task.IssueID = claim.IssueID
	input.Task.RunID = claim.RunID
	input.Task.Request = claim.Request
	input.Task.Tags = append([]string(nil), claim.Tags...)
	compiled, err := service.NewTwinBriefingCompiler().Compile(input)
	return TwinBriefingClaimResolution{Compiled: compiled}, err
}

func (f *fakeTwinBriefingClaimResolver) ResolveTwinBriefingForClaim(_ context.Context, input TwinBriefingClaimInput) (TwinBriefingClaimResolution, error) {
	f.input = input
	return f.resolution, f.err
}

func twinBriefingCompilerInput() service.TwinBriefingInput {
	return service.TwinBriefingInput{
		Task: service.TwinTaskEligibility{
			TaskID:      "task-1",
			WorkspaceID: "workspace-1",
			AgentID:     "agent-1",
			ProjectID:   "project-1",
			IssueID:     "issue-1",
			RunID:       "task-1",
			Request:     "Review the release checklist",
			Eligible:    true,
			Authorized:  true,
		},
		Version: service.TwinSignedAssertionEnvelope{
			VersionID:       "version-1",
			SignatureDigest: "signature-1",
			Lifecycle:       service.TwinVersionSigned,
			Authorized:      true,
			Assertions: []service.TwinBriefingAssertion{{
				ID:          "assertion-1",
				Lifecycle:   service.TwinAssertionSigned,
				Type:        service.TwinAssertionProcedure,
				Text:        "Run the release checklist before merging.",
				CitationIDs: []string{"citation-1"},
				Applicability: service.TwinAssertionApplicability{
					WorkspaceID: "workspace-1",
				},
			}},
		},
		Policy: service.TwinEffectiveUsePolicy{
			State:     service.TwinUseEnabled,
			Scope:     service.TwinUseScopeIssue,
			ScopeID:   "issue-1",
			BindingID: "binding-1",
			Explicit:  true,
			Reason:    service.TwinPolicyExplicitBinding,
		},
	}
}

func compileTwinBriefing(t *testing.T, mutate func(*service.TwinBriefingInput)) service.TwinCompiledBriefing {
	t.Helper()
	input := twinBriefingCompilerInput()
	if mutate != nil {
		mutate(&input)
	}
	compiled, err := service.NewTwinBriefingCompiler().Compile(input)
	if err != nil {
		t.Fatalf("compile Twin briefing: %v", err)
	}
	return compiled
}

func TestTwinBriefingClaimPayloadEnabled(t *testing.T) {
	compiled := compileTwinBriefing(t, nil)
	resolver := &fakeTwinBriefingClaimResolver{resolution: TwinBriefingClaimResolution{Compiled: compiled}}
	input := TwinBriefingClaimInput{
		TaskID:               "task-1",
		WorkspaceID:          "workspace-1",
		AgentID:              "agent-1",
		ProjectID:            "project-1",
		IssueID:              "issue-1",
		RunID:                "task-1",
		Request:              "Review the release checklist",
		Tags:                 []string{"kind:issue", "project:project-1", "issue:issue-1"},
		SupportsTwinBriefing: true,
	}

	payload, attribution, err := twinBriefingClaimPayload(context.Background(), resolver, input)
	if err != nil {
		t.Fatalf("twinBriefingClaimPayload: %v", err)
	}
	if !reflect.DeepEqual(resolver.input, input) {
		t.Fatalf("resolver input = %#v, want %#v", resolver.input, input)
	}
	if payload == nil || payload.Briefing != compiled.Briefing || payload.VersionID != compiled.VersionID || payload.BriefingDigest != compiled.Digest {
		t.Fatalf("payload = %#v, want compiled briefing metadata", payload)
	}
	if attribution == nil || attribution.Briefing != compiled.Briefing || attribution.PolicyState != string(service.TwinUseEnabled) {
		t.Fatalf("attribution = %#v, want enabled exact briefing", attribution)
	}
	if len(attribution.SelectedAssertionIDs) != 1 || attribution.SelectedAssertionIDs[0] != "assertion-1" {
		t.Fatalf("selected assertions = %v, want [assertion-1]", attribution.SelectedAssertionIDs)
	}
}

func TestTwinBriefingClaimPayloadExcludedBytesAreAbsent(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*service.TwinBriefingInput)
	}{
		{name: "off", mutate: func(input *service.TwinBriefingInput) {
			input.Policy = service.TwinEffectiveUsePolicy{State: service.TwinUseOff, Reason: service.TwinPolicyNoExplicitBinding}
		}},
		{name: "preview", mutate: func(input *service.TwinBriefingInput) {
			input.Policy.State = service.TwinUsePreview
		}},
		{name: "unauthorized task", mutate: func(input *service.TwinBriefingInput) {
			input.Task.Authorized = false
		}},
		{name: "stale version", mutate: func(input *service.TwinBriefingInput) {
			input.Version.Stale = true
		}},
		{name: "unauthorized version", mutate: func(input *service.TwinBriefingInput) {
			input.Version.Authorized = false
		}},
		{name: "local-only task", mutate: func(input *service.TwinBriefingInput) {
			input.Task.LocalOnly = true
		}},
		{name: "local-only version", mutate: func(input *service.TwinBriefingInput) {
			input.Version.LocalOnly = true
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &fakeTwinBriefingClaimResolver{resolution: TwinBriefingClaimResolution{Compiled: compileTwinBriefing(t, tt.mutate)}}
			payload, attribution, err := twinBriefingClaimPayload(context.Background(), resolver, TwinBriefingClaimInput{SupportsTwinBriefing: true})
			if err != nil {
				t.Fatalf("twinBriefingClaimPayload: %v", err)
			}
			if payload != nil || attribution != nil {
				t.Fatalf("excluded briefing leaked to claim: payload=%#v attribution=%#v", payload, attribution)
			}
			encoded, err := json.Marshal(AgentTaskResponse{TwinBriefing: payload})
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			var wire map[string]any
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if _, exists := wire["twin_briefing"]; exists {
				t.Fatalf("twin_briefing bytes present in excluded response: %s", encoded)
			}
		})
	}
}

func TestTwinBriefingClaimPayloadOldDaemonFailsClosed(t *testing.T) {
	resolver := &fakeTwinBriefingClaimResolver{resolution: TwinBriefingClaimResolution{Compiled: compileTwinBriefing(t, nil)}}
	payload, attribution, err := twinBriefingClaimPayload(context.Background(), resolver, TwinBriefingClaimInput{SupportsTwinBriefing: false})
	if !errors.Is(err, ErrTwinBriefingCapabilityRequired) {
		t.Fatalf("error = %v, want ErrTwinBriefingCapabilityRequired", err)
	}
	if payload != nil || attribution != nil {
		t.Fatalf("old daemon received Twin data: payload=%#v attribution=%#v", payload, attribution)
	}
}

func TestTwinBriefingClaimPayloadResolverFailureFailsClosed(t *testing.T) {
	resolver := &fakeTwinBriefingClaimResolver{err: errors.New("compiler rejected assertion")}
	payload, attribution, err := twinBriefingClaimPayload(context.Background(), resolver, TwinBriefingClaimInput{SupportsTwinBriefing: true})
	if err == nil {
		t.Fatal("expected resolver error")
	}
	if payload != nil || attribution != nil {
		t.Fatalf("resolver failure leaked Twin data: payload=%#v attribution=%#v", payload, attribution)
	}
}

func TestTwinBriefingClaimRequestSelectsTaskRelevantAssertion(t *testing.T) {
	input := twinBriefingCompilerInput()
	input.Version.Assertions = []service.TwinBriefingAssertion{
		{
			ID: "assertion-privacy", Lifecycle: service.TwinAssertionSigned,
			Type: service.TwinAssertionConstraint, Text: "Redact customer identifiers.",
			CitationIDs:   []string{"citation-privacy"},
			Applicability: service.TwinAssertionApplicability{WorkspaceID: "workspace-1", Keywords: []string{"privacy"}},
		},
		{
			ID: "assertion-release", Lifecycle: service.TwinAssertionSigned,
			Type: service.TwinAssertionProcedure, Text: "Run the release checklist.",
			CitationIDs:   []string{"citation-release"},
			Applicability: service.TwinAssertionApplicability{WorkspaceID: "workspace-1", Keywords: []string{"release"}},
		},
	}
	resolver := compilingTwinBriefingClaimResolver{input: input}
	base := TwinBriefingClaimInput{
		TaskID: "task-1", WorkspaceID: "workspace-1", AgentID: "agent-1",
		ProjectID: "project-1", IssueID: "issue-1", RunID: "task-1",
		SupportsTwinBriefing: true,
	}
	for _, tt := range []struct {
		request string
		want    string
	}{
		{request: "Prepare the release candidate", want: "assertion-release"},
		{request: "Perform a privacy audit", want: "assertion-privacy"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			claim := base
			claim.Request = tt.request
			_, attribution, err := twinBriefingClaimPayload(context.Background(), resolver, claim)
			if err != nil {
				t.Fatalf("twinBriefingClaimPayload: %v", err)
			}
			if attribution == nil || len(attribution.SelectedAssertionIDs) != 1 || attribution.SelectedAssertionIDs[0] != tt.want {
				t.Fatalf("selected assertions = %#v, want [%s]", attribution, tt.want)
			}
		})
	}
}

func TestTwinTaskRelevanceInputIsPrioritizedAndBounded(t *testing.T) {
	summary := "durable summary"
	request, tags := twinTaskRelevanceInput(AgentTaskResponse{
		IssueID:               "issue-1",
		ProjectID:             "project-1",
		TriggerCommentID:      twinStringPtr("comment-1"),
		TriggerCommentContent: "current user request",
		TriggerSummary:        &summary,
		ThreadName:            "issue title",
	})
	if request != "current user request\ndurable summary\nissue title" {
		t.Fatalf("request = %q", request)
	}
	if !reflect.DeepEqual(tags, []string{"kind:comment", "project:project-1", "issue:issue-1"}) {
		t.Fatalf("tags = %v", tags)
	}

	request, _ = twinTaskRelevanceInput(AgentTaskResponse{ChatMessage: strings.Repeat("界", twinTaskRelevanceRequestMaxBytes)})
	if len(request) > twinTaskRelevanceRequestMaxBytes || !utf8.ValidString(request) {
		t.Fatalf("bounded request bytes=%d valid_utf8=%v", len(request), utf8.ValidString(request))
	}
}

func twinStringPtr(value string) *string {
	return &value
}

func TestFinalizeTaskClaimRequiresAtomicTwinFinalizer(t *testing.T) {
	h := &Handler{}
	_, err := h.finalizeTaskClaim(
		context.Background(),
		db.AgentTaskQueue{},
		db.CreateTaskTokenParams{},
		[]pgtype.UUID{},
		false,
		&service.TwinClaimAttribution{VersionID: "version-1"},
	)
	if err == nil {
		t.Fatal("expected a hard error instead of silently dropping Twin attribution")
	}
}
