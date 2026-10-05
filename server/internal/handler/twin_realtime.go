package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) publishTwinRealtime(eventType, workspaceID, actorID string, payload any) {
	if h.Bus == nil {
		return
	}
	h.publish(eventType, workspaceID, "member", actorID, payload)
}

func (h *Handler) publishTwinProposalChanged(workspaceID, actorID, proposalID, state, versionID string) {
	h.publishTwinRealtime(protocol.EventTwinProposalChanged, workspaceID, actorID, protocol.TwinProposalChangedPayload{
		ProposalID: proposalID, State: state, VersionID: versionID,
	})
}

func (h *Handler) publishTwinVersionChanged(workspaceID, actorID string, version db.TwinVersion) {
	h.publishTwinRealtime(protocol.EventTwinVersionChanged, workspaceID, actorID, protocol.TwinVersionChangedPayload{
		VersionID: version.ID.String(), ProposalID: version.ProposalID.String(), VersionNumber: version.VersionNumber,
	})
}

func (h *Handler) publishTwinBindingChanged(workspaceID, actorID string, binding db.TwinBinding) {
	h.publishTwinRealtime(protocol.EventTwinBindingChanged, workspaceID, actorID, protocol.TwinBindingChangedPayload{
		BindingID: binding.ID.String(), State: binding.State, TwinVersionID: binding.TwinVersionID.String(),
	})
}

func (h *Handler) publishTwinBindingDeleted(workspaceID, actorID, bindingID string) {
	h.publishTwinRealtime(protocol.EventTwinBindingChanged, workspaceID, actorID, protocol.TwinBindingChangedPayload{
		BindingID: bindingID, State: "deleted",
	})
}

func (h *Handler) publishTwinDepositionChanged(workspaceID, actorID string, deposition db.TwinDeposition, state string) {
	h.publishTwinRealtime(protocol.EventTwinDepositionChanged, workspaceID, actorID, protocol.TwinDepositionChangedPayload{
		DepositionID: deposition.ID.String(), ProposalID: deposition.ProposalID.String(),
		TaskID: deposition.TaskID.String(), BaseTwinVersionID: deposition.BaseTwinVersionID.String(), State: state,
	})
}

// A Twin proposal review may also be a deposition review. The proposal review
// transaction has already committed before this lookup runs, so a lookup or
// fanout failure must not turn a successful write into an HTTP error.
func (h *Handler) publishTwinDepositionReview(ctx context.Context, workspaceID, proposalID pgtype.UUID, actorID, state string) {
	if h.Queries == nil || h.Bus == nil {
		return
	}
	deposition, err := h.Queries.GetTwinDepositionByProposal(ctx, db.GetTwinDepositionByProposalParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		slog.WarnContext(ctx, "load reviewed Twin deposition for realtime", "proposal_id", proposalID.String(), "error", err)
		return
	}
	h.publishTwinDepositionChanged(workspaceID.String(), actorID, deposition, state)
}
