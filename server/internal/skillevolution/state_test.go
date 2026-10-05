package skillevolution

import (
	"errors"
	"testing"
)

func TestLoopModeTransitionTableIsClosed(t *testing.T) {
	states := []LoopMode{LoopModeObserve, LoopModePropose, LoopModePaused}
	for _, from := range states {
		for _, to := range states {
			want := from != to
			if got := CanTransitionLoopMode(from, to); got != want {
				t.Errorf("CanTransitionLoopMode(%q, %q) = %v, want %v", from, to, got, want)
			}
		}
	}
	if err := ValidateLoopModeTransition(LoopMode("future"), LoopModeObserve); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("unknown mode error = %v, want ErrInvalidState", err)
	}
}

func TestProposalTransitionTable(t *testing.T) {
	allowed := map[[2]ProposalState]bool{
		{ProposalStateQueued, ProposalStateRunning}:                true,
		{ProposalStateQueued, ProposalStateFailed}:                 true,
		{ProposalStateQueued, ProposalStateStale}:                  true,
		{ProposalStateRunning, ProposalStateReady}:                 true,
		{ProposalStateRunning, ProposalStateFailed}:                true,
		{ProposalStateRunning, ProposalStateStale}:                 true,
		{ProposalStateReady, ProposalStatePublishing}:              true,
		{ProposalStateReady, ProposalStateRejected}:                true,
		{ProposalStateReady, ProposalStateStale}:                   true,
		{ProposalStatePublishing, ProposalStateReady}:              true,
		{ProposalStatePublishing, ProposalStateStale}:              true,
		{ProposalStatePublishing, ProposalStatePublished}:          true,
		{ProposalStatePublishing, ProposalStatePublicationUnknown}: true,
	}
	states := []ProposalState{
		ProposalStateQueued, ProposalStateRunning, ProposalStateReady,
		ProposalStateFailed, ProposalStateStale, ProposalStateRejected,
		ProposalStatePublishing, ProposalStatePublished, ProposalStatePublicationUnknown,
	}
	for _, from := range states {
		for _, to := range states {
			if got := CanTransitionProposal(from, to); got != allowed[[2]ProposalState{from, to}] {
				t.Errorf("CanTransitionProposal(%q, %q) = %v", from, to, got)
			}
		}
	}
	for _, terminal := range []ProposalState{
		ProposalStateFailed, ProposalStateStale, ProposalStateRejected,
		ProposalStatePublished, ProposalStatePublicationUnknown,
	} {
		if !terminal.Terminal() {
			t.Errorf("%q must be terminal", terminal)
		}
	}
	if err := ValidateProposalTransition(ProposalStateReady, ProposalStatePublished); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("skipped publishing transition error = %v, want ErrInvalidTransition", err)
	}
}

func TestReleaseTransitionTableIsTerminal(t *testing.T) {
	for _, terminal := range []ReleaseOutcome{
		ReleaseOutcomeSucceeded, ReleaseOutcomeFailed, ReleaseOutcomePublicationUnknown,
	} {
		if !CanTransitionRelease(ReleaseOutcomePending, terminal) {
			t.Errorf("pending must transition to %q", terminal)
		}
		if !terminal.Terminal() {
			t.Errorf("%q must be terminal", terminal)
		}
		if CanTransitionRelease(terminal, ReleaseOutcomePending) {
			t.Errorf("terminal release %q transitioned back to pending", terminal)
		}
	}
}
