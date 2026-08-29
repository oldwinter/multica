package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/featureflag"
)

func TestTwinModelAndExecutionWritesRejectMachineCredentialsBeforeBodyDecode(t *testing.T) {
	h := &Handler{}
	for _, actorSource := range []string{"task_token", "cloud_pat"} {
		for name, handler := range map[string]http.HandlerFunc{
			"proposal":   h.CreateTwinProposal,
			"correction": h.CorrectTwinProposal,
			"binding":    h.UpsertTwinExecutionBinding,
			"deposition": h.CreateTwinExecutionDeposition,
			"feedback":   h.UpsertTwinExecutionFeedback,
			"pause":      h.PauseTwinExecution,
		} {
			t.Run(actorSource+"/"+name, func(t *testing.T) {
				req, err := http.NewRequest(http.MethodPost, "/api/twins", bytes.NewBufferString("not-json"))
				if err != nil {
					t.Fatalf("create request: %v", err)
				}
				req.Header.Set("X-Actor-Source", actorSource)
				testutil.Call(t, handler, req).Want(http.StatusForbidden)
			})
		}
	}
}

func TestTwinProposalGenerationHonorsKillSwitchBeforeBodyDecode(t *testing.T) {
	provider := featureflag.NewStaticProvider()
	provider.Set(featureflags.TwinExecution, featureflag.Rule{Default: false})
	h := &Handler{FeatureFlags: featureflag.NewService(provider)}
	req, err := http.NewRequest(http.MethodPost, "/api/twins/proposals", bytes.NewBufferString("not-json"))
	if err != nil {
		t.Fatalf("create proposal request: %v", err)
	}
	response := testutil.Call(t, h.CreateTwinProposal, req).Want(http.StatusForbidden)
	if response.Map()["code"] != "twin_execution_disabled" {
		t.Fatalf("kill switch response = %s", response.Text())
	}
}

func TestBatchIssueUpdateRejectsTwinUseBeforeMutation(t *testing.T) {
	h := &Handler{}
	req, err := http.NewRequest(http.MethodPatch, "/api/issues/batch", bytes.NewBufferString(`{
		"issue_ids":["11111111-1111-4111-8111-111111111111"],
		"updates":{"twin_use":{"state":"enabled","twin_version_id":"22222222-2222-4222-8222-222222222222"}}
	}`))
	if err != nil {
		t.Fatalf("create batch request: %v", err)
	}
	response := testutil.Call(t, h.BatchUpdateIssues, req).Want(http.StatusBadRequest)
	if !strings.Contains(response.Text(), "single issue") {
		t.Fatalf("batch Twin response = %s", response.Text())
	}
}

func TestMapTwinCompiledBriefingMatchesFrozenWireContract(t *testing.T) {
	bindingID := "11111111-1111-4111-8111-111111111111"
	scopeID := "22222222-2222-4222-8222-222222222222"
	versionID := "33333333-3333-4333-8333-333333333333"
	digest := "sha256:" + string(bytes.Repeat([]byte{'a'}, 64))
	preview := service.TwinExecutionBriefingPreview{
		Version: &service.TwinExecutionVersionReference{ID: versionID, VersionNumber: 2, ContentDigest: digest},
		Briefing: service.TwinCompiledBriefing{
			Briefing: "bounded", Digest: digest, SelectedAssertionIDs: []string{"assertion:a"},
			CitationIDs: []string{"issue:a"}, CompilerVersion: service.TwinBriefingCompilerVersion,
			ByteCount: 7, TokenCount: 7, Inject: true,
			PolicyDecision: service.TwinEffectiveUsePolicy{
				State: service.TwinUseEnabled, Scope: service.TwinUseScopeIssue,
				ScopeID: scopeID, BindingID: bindingID, Explicit: true,
				Reason: service.TwinPolicyExplicitBinding,
			},
			Exclusions: []service.TwinBriefingExclusion{{Code: service.TwinBriefingOverBudget}},
		},
	}
	raw, err := json.Marshal(mapTwinCompiledBriefing(preview))
	if err != nil {
		t.Fatalf("marshal briefing response: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode briefing response: %v", err)
	}
	for _, key := range []string{"policy", "twin_version", "briefing", "briefing_digest", "assertion_ids", "citation_keys", "compiler_version", "byte_count", "token_count", "inject", "exclusion_reasons"} {
		if _, exists := payload[key]; !exists {
			t.Fatalf("briefing response missing %q: %s", key, raw)
		}
	}
	for _, forbidden := range []string{"version", "digest", "compiler", "exclusions"} {
		if _, exists := payload[forbidden]; exists {
			t.Fatalf("briefing response contains stale key %q: %s", forbidden, raw)
		}
	}
}

func TestMapTwinTaskContextMatchesFrozenWireContract(t *testing.T) {
	taskID := "11111111-1111-4111-8111-111111111111"
	digest := "sha256:" + string(bytes.Repeat([]byte{'b'}, 64))
	context := service.TwinExecutionTaskContext{
		TaskID: taskID,
		Attribution: &service.TwinExecutionAttribution{
			TwinVersionID: "22222222-2222-4222-8222-222222222222", VersionNumber: 3,
			VersionDigest: digest, Briefing: "bounded", BriefingDigest: digest,
			Assertions: []service.TwinExecutionAssertion{{ID: "assertion:a"}}, CitationKeys: []string{"issue:a"},
			PolicyScopeType: "issue", PolicyScopeID: "33333333-3333-4333-8333-333333333333",
			PolicyState: "enabled", CompilerVersion: "compiler-v1", ByteCount: 7, TokenCount: 7,
		},
		Assertions:  []service.TwinExecutionAssertion{{ID: "assertion:a", CitationKeys: []string{"issue:a"}}},
		Citations:   []service.TwinExecutionCitation{{Key: "issue:a", Label: "Issue A", SourceType: "issue", Locator: "MUL-1"}},
		Depositions: []db.TwinDeposition{},
	}
	raw, err := json.Marshal(mapTwinExecutionTaskContext(context))
	if err != nil {
		t.Fatalf("marshal task context: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	for _, key := range []string{"task_id", "attribution", "feedback", "depositions", "assertions", "citations"} {
		if _, exists := payload[key]; !exists {
			t.Fatalf("task context missing %q: %s", key, raw)
		}
	}
	if payload["task_id"] != taskID {
		t.Fatalf("task id = %#v", payload["task_id"])
	}
}

func TestTwinExecutionErrorStatusMatrix(t *testing.T) {
	h := &Handler{}
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "disabled", err: service.ErrTwinExecutionDisabled, status: http.StatusForbidden},
		{name: "not found", err: service.ErrTwinExecutionNotFound, status: http.StatusNotFound},
		{name: "conflict", err: service.ErrTwinExecutionConflict, status: http.StatusConflict},
		{name: "unsupported", err: service.ErrTwinExecutionUnsupportedVersion, status: http.StatusUnprocessableEntity},
		{name: "unavailable", err: service.ErrTwinDepositionUnavailable, status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := testutil.Call(t, func(w http.ResponseWriter, _ *http.Request) { h.writeTwinExecutionError(w, test.err) }, mustTwinExecutionRequest(t)).Want(test.status)
			if response.Map()["code"] == "" {
				t.Fatalf("missing structured error code: %s", response.Text())
			}
		})
	}
}

func mustTwinExecutionRequest(t *testing.T) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "/api/twins", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	return request
}
