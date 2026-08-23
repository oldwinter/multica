package handler

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/featureflag"
)

func TestHandlerWiresModelBackedTwinGenerator(t *testing.T) {
	h := New(
		db.New(testPool), testPool, realtime.NewHub(), events.New(),
		service.NewEmailService(), nil, nil, analytics.NoopClient{},
		Config{LLMBaseURL: "http://127.0.0.1:1"},
	)
	if _, ok := h.TwinService.ProposalGenerator.(*service.ModelTwinProposalGenerator); !ok {
		t.Fatalf("Twin generator = %T, want *service.ModelTwinProposalGenerator", h.TwinService.ProposalGenerator)
	}
	if h.TwinBriefingResolver != h {
		t.Fatalf("Twin briefing resolver = %T, want handler", h.TwinBriefingResolver)
	}
	if h.TwinTaskClaimFinalizer != h.TaskService {
		t.Fatalf("Twin task claim finalizer = %T, want TaskService", h.TwinTaskClaimFinalizer)
	}
}

func TestResolveTwinBriefingForClaimKillSwitchSkipsInjection(t *testing.T) {
	provider := featureflag.NewStaticProvider()
	provider.Set("twin_execution", featureflag.Rule{Default: false})
	h := &Handler{FeatureFlags: featureflag.NewService(provider)}

	resolution, err := h.ResolveTwinBriefingForClaim(context.Background(), service.TwinBriefingClaimInput{
		// Deliberately invalid identifiers prove the disabled path does not read
		// policy state or touch persistence before returning.
		WorkspaceID: "not-a-uuid",
	})
	if err != nil {
		t.Fatalf("ResolveTwinBriefingForClaim() error = %v", err)
	}
	if resolution.Compiled.Inject || resolution.Compiled.Briefing != "" {
		t.Fatalf("disabled Twin resolution = %+v, want no injected bytes", resolution.Compiled)
	}
	if resolution.Compiled.PolicyDecision.State != service.TwinUseOff {
		t.Fatalf("disabled policy state = %q, want %q", resolution.Compiled.PolicyDecision.State, service.TwinUseOff)
	}
}
