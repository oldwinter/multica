package skillevolution

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidState      = errors.New("invalid skill evolution state")
	ErrInvalidTransition = errors.New("invalid skill evolution state transition")
)

type TransitionError struct {
	Machine string
	From    string
	To      string
	Cause   error
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("%s transition %q -> %q: %v", e.Machine, e.From, e.To, e.Cause)
}

func (e *TransitionError) Unwrap() error { return e.Cause }

var loopTransitions = map[LoopMode]map[LoopMode]struct{}{
	LoopModeObserve: {LoopModePropose: {}, LoopModePaused: {}},
	LoopModePropose: {LoopModeObserve: {}, LoopModePaused: {}},
	LoopModePaused:  {LoopModeObserve: {}, LoopModePropose: {}},
}

var proposalTransitions = map[ProposalState]map[ProposalState]struct{}{
	ProposalStateQueued: {
		ProposalStateRunning: {},
		ProposalStateFailed:  {},
		ProposalStateStale:   {},
	},
	ProposalStateRunning: {
		ProposalStateReady:  {},
		ProposalStateFailed: {},
		ProposalStateStale:  {},
	},
	ProposalStateReady: {
		ProposalStatePublishing: {},
		ProposalStateRejected:   {},
		ProposalStateStale:      {},
	},
	ProposalStatePublishing: {
		// A known failed attempt may safely return to reviewable readiness.
		ProposalStateReady:              {},
		ProposalStatePublished:          {},
		ProposalStatePublicationUnknown: {},
	},
	ProposalStateFailed:             {},
	ProposalStateStale:              {},
	ProposalStateRejected:           {},
	ProposalStatePublished:          {},
	ProposalStatePublicationUnknown: {},
}

var releaseTransitions = map[ReleaseOutcome]map[ReleaseOutcome]struct{}{
	ReleaseOutcomePending: {
		ReleaseOutcomeSucceeded:          {},
		ReleaseOutcomeFailed:             {},
		ReleaseOutcomePublicationUnknown: {},
	},
	ReleaseOutcomeSucceeded:          {},
	ReleaseOutcomeFailed:             {},
	ReleaseOutcomePublicationUnknown: {},
}

func CanTransitionLoopMode(from, to LoopMode) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	_, ok := loopTransitions[from][to]
	return ok
}

func ValidateLoopModeTransition(from, to LoopMode) error {
	if !from.Valid() || !to.Valid() {
		return &TransitionError{Machine: "loop", From: string(from), To: string(to), Cause: ErrInvalidState}
	}
	if !CanTransitionLoopMode(from, to) {
		return &TransitionError{Machine: "loop", From: string(from), To: string(to), Cause: ErrInvalidTransition}
	}
	return nil
}

func CanTransitionProposal(from, to ProposalState) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	_, ok := proposalTransitions[from][to]
	return ok
}

func ValidateProposalTransition(from, to ProposalState) error {
	if !from.Valid() || !to.Valid() {
		return &TransitionError{Machine: "proposal", From: string(from), To: string(to), Cause: ErrInvalidState}
	}
	if !CanTransitionProposal(from, to) {
		return &TransitionError{Machine: "proposal", From: string(from), To: string(to), Cause: ErrInvalidTransition}
	}
	return nil
}

func CanTransitionRelease(from, to ReleaseOutcome) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	_, ok := releaseTransitions[from][to]
	return ok
}

func ValidateReleaseTransition(from, to ReleaseOutcome) error {
	if !from.Valid() || !to.Valid() {
		return &TransitionError{Machine: "release", From: string(from), To: string(to), Cause: ErrInvalidState}
	}
	if !CanTransitionRelease(from, to) {
		return &TransitionError{Machine: "release", From: string(from), To: string(to), Cause: ErrInvalidTransition}
	}
	return nil
}
