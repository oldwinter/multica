// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { WikiProposal } from "@multica/core/wiki";
import enWiki from "../locales/en/wiki.json";
import { WikiProposalsPanel } from "./wiki-proposals-panel";

const proposal: WikiProposal = {
  id: "proposal-1",
  pageId: "page-1",
  baseRevisionNumber: 3,
  proposedPath: "guide.md",
  proposedTitle: "Agent guide",
  proposedContent: "# Proposed content",
  contentDigest: "sha256:proposal",
  rationale: "Clarifies the release steps.",
  evidenceRefs: ["run:123"],
  agentId: "agent-1",
  sourceKind: "agent",
  sourceRefId: null,
  idempotencyKey: "proposal-key",
  status: "pending",
  reviewedById: null,
  reviewReason: null,
  reviewedAt: null,
  acceptedRevisionId: null,
  createdAt: "2026-08-23T10:00:00Z",
};

function renderPanel(overrides: Partial<React.ComponentProps<typeof WikiProposalsPanel>> = {}) {
  const onAccept = overrides.onAccept ?? vi.fn();
  const onReject = overrides.onReject ?? vi.fn();
  const view = render(
    <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
      <WikiProposalsPanel
        proposals={[proposal]}
        isLoading={false}
        isError={false}
        isPending={false}
        onRetry={vi.fn()}
        onAccept={onAccept}
        onReject={onReject}
        {...overrides}
      />
    </I18nProvider>,
  );
  return { onAccept, onReject, ...view };
}

describe("WikiProposalsPanel", () => {
  it("lets a reviewer edit, preview, and accept a proposal", () => {
    const { onAccept } = renderPanel();
    fireEvent.change(screen.getByDisplayValue("Agent guide"), { target: { value: "Reviewed guide" } });
    fireEvent.change(screen.getByDisplayValue("# Proposed content"), { target: { value: "# Reviewed content\n\n[Open issue](/issues/MUL-1)" } });
    fireEvent.click(screen.getByText("Preview"));
    expect(screen.getByText("Reviewed content")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open issue" })).toHaveAttribute(
      "href",
      expect.stringMatching(/\/issues\/MUL-1$/),
    );
    fireEvent.click(screen.getByRole("button", { name: "Accept proposal" }));
    expect(onAccept).toHaveBeenCalledWith({
      proposalId: "proposal-1",
      path: "guide.md",
      title: "Reviewed guide",
      content: "# Reviewed content\n\n[Open issue](/issues/MUL-1)",
    });
  });

  it("records an optional human rejection reason", () => {
    const { onReject } = renderPanel();
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Reason (optional)" }), { target: { value: "Unsupported" } });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    expect(onReject).toHaveBeenCalledWith("proposal-1", "Unsupported");
  });

  it("renders loading, empty, error, and stale action states", () => {
    const loading = renderPanel({ proposals: [], isLoading: true });
    expect(screen.getByText("Loading proposals…")).toBeInTheDocument();
    loading.unmount();
    const empty = renderPanel({ proposals: [] });
    expect(screen.getByText("No proposals")).toBeInTheDocument();
    empty.unmount();
    const error = renderPanel({ proposals: [], isError: true });
    expect(screen.getByText("Could not load proposals.")).toBeInTheDocument();
    error.unmount();
    renderPanel({ actionError: "This proposal is stale." });
    expect(screen.getByRole("alert")).toHaveTextContent("This proposal is stale.");
  });
});
