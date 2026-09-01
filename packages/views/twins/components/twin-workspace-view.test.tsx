// @vitest-environment jsdom

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { SidebarProvider } from "@multica/ui/components/ui/sidebar";
import { describe, expect, it, vi } from "vitest";
import enCommon from "../../locales/en/common.json";
import enTwins from "../../locales/en/twins.json";
import enUi from "../../locales/en/ui.json";
import { TwinWorkspaceView } from "./twin-workspace-view";
import { lifecycleFixture } from "./twin-workspace-view.test-fixture";
import type { NavigationAdapter } from "../../navigation";

const navigationHarness = vi.hoisted(() => ({
  current: null as NavigationAdapter | null,
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: { children: React.ReactNode; href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
  useOptionalNavigation: () => navigationHarness.current,
}));

vi.mock("./twin-activation-readiness", () => ({
  TwinActivationReadiness: () => null,
}));

vi.mock("./twin-use-panel", () => ({
  TwinUsePanel: () => null,
}));

const resources = { en: { common: enCommon, twins: enTwins, ui: enUi } };

function renderView(overrides = {}) {
  const fixture = lifecycleFixture();
  return render(
    <I18nProvider locale="en" resources={resources}>
      <WorkspaceSlugProvider slug="acme">
        <SidebarProvider>
          <TwinWorkspaceView {...fixture} {...overrides} />
        </SidebarProvider>
      </WorkspaceSlugProvider>
    </I18nProvider>,
  );
}

function makeNavigation(search = "", hash = "#comment-7"): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/twins",
    searchParams: new URLSearchParams(search),
    hash,
    getShareableUrl: (path) => `https://multica.test${path}`,
  };
}

describe("TwinWorkspaceView", () => {
  it("restores the selected tab from the navigation URL", () => {
    navigationHarness.current = makeNavigation("tab=use");
    try {
      renderView();
      expect(screen.getByRole("tab", { name: "Use" })).toHaveAttribute("aria-selected", "true");
    } finally {
      navigationHarness.current = null;
    }
  });

  it("replaces the URL when a tab changes and preserves unrelated location state", () => {
    const navigation = makeNavigation("view=activity&filter=open", "#evidence-7");
    navigationHarness.current = navigation;
    try {
      renderView();
      fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));

      expect(navigation.replace).toHaveBeenCalledWith(
        "/acme/twins?view=activity&filter=open&tab=twin#evidence-7",
      );
      expect(navigation.push).not.toHaveBeenCalled();
      expect(navigation.searchParams.toString()).toBe("view=activity&filter=open");
    } finally {
      navigationHarness.current = null;
    }
  });

  it("fails closed to LM Wiki for an unknown URL tab without rewriting it", () => {
    const navigation = makeNavigation("tab=unknown");
    navigationHarness.current = navigation;
    try {
      renderView();
      expect(screen.getByRole("tab", { name: "LM Wiki" })).toHaveAttribute("aria-selected", "true");
      expect(navigation.replace).not.toHaveBeenCalled();
    } finally {
      navigationHarness.current = null;
    }
  });

  it("uses the shared mobile page header and owns narrow interaction plus launcher clearance", () => {
    renderView();

    expect(screen.getByRole("button", { name: "Toggle left sidebar" })).toHaveClass("xl:hidden");
    const root = screen.getByRole("main");
    const content = screen.getByTestId("twin-workspace-content");
    expect(root).toHaveAttribute("data-twin-copy");
    expect(root).toHaveClass("pe-chat-launcher", "overflow-y-auto");
    expect(root.firstElementChild).toHaveClass("border-b");
    expect(content).toHaveAttribute("data-twin-interaction-region");
    expect(content).toHaveClass(
      "pb-chat-launcher",
      "max-lg:[&_button:not([data-slot=switch]):not([data-slot=checkbox])]:min-h-11",
      "max-lg:[&_button:not([data-slot=switch]):not([data-slot=checkbox])]:min-w-11",
      "max-lg:[&_[data-slot=input]]:min-h-11",
      "max-lg:[&_[data-slot=select-trigger]]:min-h-11",
      "max-lg:[&_[data-slot=tabs-list]]:min-h-11",
      "max-lg:[&_[data-slot=switch]]:after:-inset-y-[13px]",
      "max-lg:[&_[data-slot=checkbox]]:after:-inset-x-[14px]",
      "max-lg:[&_[data-slot=checkbox]]:after:-inset-y-[14px]",
    );
    expect(content).not.toHaveClass("pe-chat-launcher");
  });

  it("keeps the main landmark configurable for a dashboard shell", () => {
    const { container } = renderView({ rootElement: "div" });
    expect(container.querySelector("[data-twin-workspace]")?.tagName).toBe("DIV");
  });

  it.each([
    ["loading", "Loading LM Wiki and Twin"],
    ["error", "Review workspace unavailable"],
  ])("renders the %s overview state", (state, label) => {
    const onRetry = vi.fn();
    renderView({ state, onRetry });

    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(screen.getByText(label)).toBeInTheDocument();
    if (state === "error") {
      fireEvent.click(screen.getByRole("button", { name: "Try again" }));
      expect(onRetry).toHaveBeenCalledOnce();
    }
  });

  it("reviews a pending Wiki revision with history, content, citations, and a reason", async () => {
    const onAcceptWiki = vi.fn(async () => undefined);
    const onRejectWiki = vi.fn(async () => undefined);
    const accepted = renderView({ onAcceptWiki, onRejectWiki });

    expect(screen.getByRole("tab", { name: "LM Wiki" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Pending review")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Wiki revision" })).toBeInTheDocument();
    expect(screen.getByText("Issue 42: Review the source model")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show citation" }));
    expect(screen.getByText("Issue #42: Review the source model")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Accept revision" }));
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Confirm acceptance" })));
    expect(onAcceptWiki).toHaveBeenCalledWith("wiki-2");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    accepted.unmount();
    renderView({ onAcceptWiki, onRejectWiki });

    fireEvent.click(screen.getByRole("button", { name: "Reject revision" }));
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Source is incomplete" } });
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Confirm rejection" })));
    expect(onRejectWiki).toHaveBeenCalledWith("wiki-2", "Source is incomplete");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("shows current Twin, proposal history, assertion diff, and sign-off", async () => {
    const onAcceptTwin = vi.fn(async () => undefined);
    const onRejectTwin = vi.fn(async () => undefined);
    const accepted = renderView({ onAcceptTwin, onRejectTwin });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.getByText("Current Twin v1")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Twin proposal" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Twin version" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Twin version" }).closest("label")?.parentElement)
      .not.toHaveClass("pe-chat-launcher", "sm:pe-0");
    expect(screen.getByText(/^Added assertions/)).toBeInTheDocument();
    expect(screen.getAllByText("assertion-new")).toHaveLength(2);
    expect(screen.getByText(/^Removed assertions/)).toBeInTheDocument();
    expect(screen.getByText(/^Changed assertions/)).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Assertion changes" })).toHaveClass("xl:grid-cols-4");
    expect(screen.getByRole("region", { name: "Assertion changes" })).not.toHaveClass("sm:grid-cols-3");
    expect(screen.getByRole("link", { name: "Open issue" })).toHaveAttribute("href", "/acme/issues/issue-42");

    fireEvent.click(screen.getByRole("button", { name: "Sign off proposal" }));
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Confirm sign-off" })));
    expect(onAcceptTwin).toHaveBeenCalledWith("proposal-2");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    accepted.unmount();
    renderView({ onAcceptTwin, onRejectTwin });
    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));

    fireEvent.click(screen.getByRole("button", { name: "Reject proposal" }));
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Needs narrower evidence" } });
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Confirm rejection" })));
    expect(onRejectTwin).toHaveBeenCalledWith("proposal-2", "Needs narrower evidence");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("links Wiki citations to their immutable revision", () => {
    const fixture = lifecycleFixture();
    const revisionCitation = {
      ...fixture.proposalDetail.citations[0],
      id: "citation-wiki-revision",
      citation_key: "wiki_page_revision:revision-7",
      source_type: "wiki_page_revision",
      source_id: "revision-7",
      locator: "wiki-pages/page-1/revisions/revision-7",
      label: "Wiki: Focused verification",
    };
    renderView({ proposalDetail: { ...fixture.proposalDetail, citations: [revisionCitation] } });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    fireEvent.click(screen.getByText(revisionCitation.label).closest("button")!);

    expect(screen.getByRole("link", { name: revisionCitation.locator }))
      .toHaveAttribute("href", "/acme/wiki/revisions/revision-7");
  });

  it("renders the persisted six-step Twin review spine and its data states", () => {
    renderView();

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    const spine = screen.getByRole("region", { name: "Twin review progress" });
    expect(within(spine).getAllByTestId("twin-review-step")).toHaveLength(6);
    expect(within(spine).getByText("Import workspace sources")).toBeInTheDocument();
    expect(within(spine).getByText("Coordinate execution").closest("li")).toHaveAttribute("data-state", "current");
    expect(within(spine).getByText("Deposition").closest("li")).toHaveAttribute("data-state", "upcoming");
  });

  it("keeps the current Twin identity accurate while inspecting an older version", () => {
    const fixture = lifecycleFixture();
    const currentVersion = {
      ...fixture.twin.current_version,
      id: "version-2",
      version_number: 2,
      proposal_id: "proposal-2",
    };
    renderView({
      twin: {
        ...fixture.twin,
        current_version: currentVersion,
        versions: [currentVersion, ...fixture.twin.versions],
      },
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.getByRole("heading", { name: "Current Twin v2" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Version v1" })).toBeInTheDocument();
  });

  it("does not offer to reopen a rejected immutable proposal", () => {
    const fixture = lifecycleFixture();
    const rejected = {
      ...fixture.twin.proposals[0],
      review: {
        id: "review-2",
        decision: "rejected",
        reviewer_id: "member-1",
        reason: "Needs different evidence",
        created_at: "2026-08-11T08:10:00Z",
      },
    };
    renderView({
      wiki: {
        ...fixture.wiki,
        accepted_revision: fixture.wiki.latest_revision,
        pending_revision: null,
      },
      twin: { ...fixture.twin, pending_proposal: null, proposals: [rejected] },
      proposalDetail: { ...fixture.proposalDetail, proposal: rejected },
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.getByText("rejected")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Build proposal" })).not.toBeInTheDocument();
  });

  it("fails closed when a Wiki review decision is unknown", () => {
    const fixture = lifecycleFixture();
    const unknownRevision = {
      ...fixture.wiki.pending_revision!,
      review: {
        id: "review-future",
        decision: "deferred_by_policy",
        reviewer_id: "member-1",
        reason: "Held by a newer policy",
        created_at: "2026-08-11T08:10:00Z",
      },
    };
    renderView({
      wiki: { ...fixture.wiki, pending_revision: unknownRevision, latest_revision: unknownRevision, revisions: [unknownRevision] },
      wikiDetail: { ...fixture.wikiDetail, revision: unknownRevision },
    });

    expect(screen.getByText("Unknown review state")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Accept revision" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Reject revision" })).not.toBeInTheDocument();
  });

  it("does not offer Twin sign-off for an unknown proposal kind", () => {
    const fixture = lifecycleFixture();
    const unknownProposal = { ...fixture.twin.proposals[0], kind: "future_kind" };
    renderView({
      twin: { ...fixture.twin, pending_proposal: unknownProposal, proposals: [unknownProposal] },
      proposalDetail: { ...fixture.proposalDetail, proposal: unknownProposal },
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.queryByRole("button", { name: "Sign off proposal" })).not.toBeInTheDocument();
  });

  it("keeps history readable for members while hiding lifecycle mutations", () => {
    renderView({ canManageWiki: false, canManageTwin: false });

    expect(screen.getByText("Read-only access")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Accept revision" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.getByText("Current Twin v1")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Sign off proposal" })).not.toBeInTheDocument();
  });

  it("renders the LM Wiki source-policy slot in the evidence workflow", () => {
    renderView({ sourcePolicyPanel: <div>Exact source policy</div> });

    expect(screen.getByTestId("lm-wiki-source-policy-slot")).toHaveTextContent("Exact source policy");
  });

  it("keeps first-run refresh explicit for managers and unavailable to members", () => {
    const onRefreshWiki = vi.fn();
    const fixture = lifecycleFixture();
    const emptyWiki = {
      ...fixture.wiki,
      latest_revision: null,
      accepted_revision: null,
      pending_revision: null,
      revisions: [],
    };
    const manager = renderView({ wiki: emptyWiki, wikiDetail: null, onRefreshWiki });

    expect(screen.getByText("Refresh Wiki to compile the first evidence revision.")).toBeInTheDocument();
    expect(screen.queryByText("Pending review")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Refresh Wiki" }));
    expect(onRefreshWiki).toHaveBeenCalledOnce();
    manager.unmount();

    renderView({ wiki: emptyWiki, wikiDetail: null, canManageWiki: false });
    expect(screen.queryByRole("button", { name: "Refresh Wiki" })).not.toBeInTheDocument();
    expect(screen.getByText("No Wiki revision yet. An owner or admin can start the first refresh.")).toBeInTheDocument();
  });

  it("keeps the empty Twin Builder focused on its next prerequisite", () => {
    const fixture = lifecycleFixture();
    renderView({
      wiki: {
        ...fixture.wiki,
        latest_revision: null,
        accepted_revision: null,
        pending_revision: null,
        revisions: [],
      },
      wikiDetail: null,
      twin: {
        ...fixture.twin,
        current_version: null,
        pending_proposal: null,
        proposals: [],
        versions: [],
      },
      proposalDetail: null,
      versionDetail: null,
      selectedRevisionId: "",
      selectedProposalId: "",
      selectedVersionId: "",
      reviewSteps: [],
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.queryByRole("combobox", { name: "Twin proposal" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Twin version" })).not.toBeInTheDocument();
    expect(screen.getByText("Accept a Wiki revision to start Twin Builder.")).toBeInTheDocument();
  });

  it("marks stable Wiki and Twin destinations as programmatically focusable", () => {
    renderView({ sourcePolicyPanel: <p>Source policy controls</p> });

    for (const destination of [
      "wiki-overview",
      "wiki-source-policy",
      "wiki-evidence",
    ]) {
      expect(document.querySelector(`[data-twin-destination="${destination}"]`))
        .toHaveAttribute("tabindex", "-1");
    }

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    for (const destination of ["twin-overview", "twin-history"]) {
      expect(document.querySelector(`[data-twin-destination="${destination}"]`))
        .toHaveAttribute("tabindex", "-1");
    }
  });

  it("shows only sanitized deposition evidence and never renders raw metadata", () => {
    const fixture = lifecycleFixture();
    renderView({
      proposalDetail: {
        ...fixture.proposalDetail,
        proposal: { ...fixture.proposalDetail.proposal, kind: "deposition" },
        run_evidence: {
          taskId: "00000000-0000-4000-8000-000000000010",
          baseTwinVersionId: "00000000-0000-4000-8000-000000000011",
          evidenceDigest: `sha256:${"a".repeat(64)}`,
          taskStatus: "completed",
          completedAt: "2026-08-11T09:00:00Z",
          feedbackRating: "helped",
          safeMetadata: {
            raw_output: "must-never-render",
            local_path: "/home/private/worktree",
          },
        },
      },
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    const evidence = screen.getByRole("region", { name: "Sanitized run evidence" });
    expect(evidence).toBeInTheDocument();
    expect(within(evidence).getAllByText("completed")).not.toHaveLength(0);
    expect(within(evidence).getByText("helped")).toBeInTheDocument();
    expect(screen.queryByText("must-never-render")).not.toBeInTheDocument();
    expect(screen.queryByText("/home/private/worktree")).not.toBeInTheDocument();
  });

  it("lets a manager create an edited replacement for a pending deposition", async () => {
    const fixture = lifecycleFixture();
    const onEditDeposition = vi.fn(async () => undefined);
    renderView({
      onEditDeposition,
      proposalDetail: {
        ...fixture.proposalDetail,
        proposal: { ...fixture.proposalDetail.proposal, kind: "deposition" },
        run_evidence: {
          taskId: "00000000-0000-4000-8000-000000000010",
          baseTwinVersionId: "00000000-0000-4000-8000-000000000011",
          evidenceDigest: `sha256:${"a".repeat(64)}`,
          taskStatus: "completed",
          completedAt: "2026-08-11T09:00:00Z",
          feedbackRating: "helped",
          safeMetadata: {},
        },
      },
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit deposition" }));
    const editor = screen.getByLabelText("Assertions JSON");
    fireEvent.change(editor, { target: { value: "{invalid" } });
    expect(screen.getByText("Enter valid JSON.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create replacement" })).toBeDisabled();

    const editedAssertions = [{
      id: "assertion-new",
      type: "quality_bar",
      text: "始终记录明确且可复核的评审结论",
      applicability: { workspace_id: fixture.wsId, keywords: ["review"] },
      evidence_citations: ["issue:42"],
      confidence: 0.95,
    }];
    fireEvent.change(editor, { target: { value: JSON.stringify(editedAssertions, null, 2) } });
    expect(screen.getByText("Changes are ready for review.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Create replacement" }));

    await waitFor(() => expect(onEditDeposition).toHaveBeenCalledWith("proposal-2", editedAssertions));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("creates a validated correction before signing a normal proposal", async () => {
    const onCorrectTwin = vi.fn(async () => undefined);
    renderView({ onCorrectTwin });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit proposal" }));
    const editor = screen.getByLabelText("Assertions JSON");
    fireEvent.change(editor, { target: { value: "[]" } });
    expect(screen.getByText("Changes are ready for validation.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Create correction" }));

    await waitFor(() => expect(onCorrectTwin).toHaveBeenCalledWith("proposal-2", []));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("keeps deposition editing unavailable to read-only members", () => {
    const fixture = lifecycleFixture();
    renderView({
      canManageTwin: false,
      proposalDetail: {
        ...fixture.proposalDetail,
        proposal: { ...fixture.proposalDetail.proposal, kind: "deposition" },
        run_evidence: {
          taskId: "00000000-0000-4000-8000-000000000010",
          baseTwinVersionId: "00000000-0000-4000-8000-000000000011",
          evidenceDigest: `sha256:${"a".repeat(64)}`,
          taskStatus: "completed",
          completedAt: "2026-08-11T09:00:00Z",
          feedbackRating: null,
          safeMetadata: {},
        },
      },
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.queryByRole("button", { name: "Edit deposition" })).not.toBeInTheDocument();
  });

});
