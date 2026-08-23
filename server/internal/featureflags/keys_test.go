package featureflags

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/pkg/featureflag"
)

func TestResourceLabelsCompatDecisionStaysEnabled(t *testing.T) {
	flags := EvaluateFrontendPublicFlags(context.Background(), nil)
	if !flags[resourceLabelsCompat] {
		t.Fatal("resource labels must stay enabled for installed clients")
	}
}

// MUL-5345: hang stack capture is gone from this build, but v0.4.13–v0.4.18 are
// installed and still hold a debugger channel open on every renderer whenever
// this key arrives as `true`. Those clients are fail-closed on absence, so NOT
// publishing the key is what disarms them — re-adding it would put a flag flip
// back within reach of a fleet that can no longer produce a usable stack.
func TestDesktopHangStackCaptureIsNotPublished(t *testing.T) {
	flags := EvaluateFrontendPublicFlags(context.Background(), nil)
	if _, published := flags["desktop_hang_stack_capture"]; published {
		t.Fatal("hang stack capture must stay unpublished so installed clients keep their debugger channels closed")
	}
}

func TestAgentBuilderCompatDecisionStaysEnabled(t *testing.T) {
	flags := EvaluateFrontendPublicFlags(context.Background(), nil)
	if !flags[agentBuilderCompat] {
		t.Fatal("agent builder must stay enabled for installed clients")
	}
}

func TestAgentSkillTogglesCompatDecisionStaysEnabled(t *testing.T) {
	flags := EvaluateFrontendPublicFlags(context.Background(), nil)
	if !flags[agentSkillTogglesCompat] {
		t.Fatal("agent skill toggles must stay enabled for installed v0.4.0 clients")
	}
}

func TestPluginsV1DefaultsOff(t *testing.T) {
	flags := EvaluateFrontendPublicFlags(context.Background(), nil)
	if flags[PluginsV1] {
		t.Fatal("plugins_v1 must stay disabled unless explicitly enabled")
	}
}

func TestPluginSubFlagsAreNotPublished(t *testing.T) {
	flags := EvaluateFrontendPublicFlags(context.Background(), nil)
	for _, retired := range []string{"private_plugins_v1", "remote_mcp_plugins_v1"} {
		if _, published := flags[retired]; published {
			t.Fatalf("retired Plugin sub-flag %q must not be published", retired)
		}
	}
}

func TestTwinExecutionDefaultsOnAndIsFrontendPublic(t *testing.T) {
	ctx := context.Background()
	if !TwinExecutionEnabled(ctx, nil) {
		t.Fatal("twin_execution must default on without a provider")
	}
	flags := EvaluateFrontendPublicFlags(ctx, nil)
	if enabled, published := flags[TwinExecution]; !published || !enabled {
		t.Fatalf("twin_execution public decision = (%v, published=%v), want (true, true)", enabled, published)
	}
}

func TestTwinExecutionOpsKillSwitchOverridesDefault(t *testing.T) {
	t.Setenv("FF_TWIN_EXECUTION", "false")
	t.Setenv(featureflag.EnvFlagFile, "")

	flags, err := featureflag.NewServiceFromEnv()
	if err != nil {
		t.Fatalf("NewServiceFromEnv: %v", err)
	}
	ctx := context.Background()
	if TwinExecutionEnabled(ctx, flags) {
		t.Fatal("FF_TWIN_EXECUTION=false must disable new Twin execution activity")
	}
	if EvaluateFrontendPublicFlags(ctx, flags)[TwinExecution] {
		t.Fatal("frontend-public decision must expose the Ops kill switch")
	}
}
