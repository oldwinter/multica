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

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: { children: React.ReactNode; href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
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

describe("TwinWorkspaceView", () => {
  it("uses the shared mobile page header and reserves localized workspace clearance", () => {
    renderView();

    expect(screen.getByRole("button", { name: "Toggle Sidebar" })).toHaveClass("md:hidden");
    expect(screen.getByRole("main")).toHaveAttribute("data-twin-copy");
    expect(screen.getByRole("main").firstElementChild).toHaveClass("border-b");
    expect(screen.getByTestId("twin-workspace-content")).toHaveClass("pb-chat-launcher");
    expect(screen.getByTestId("twin-workspace-content")).not.toHaveClass("pe-chat-launcher");
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
      .toHaveClass("pe-chat-launcher", "sm:pe-0");
    expect(screen.getByText(/^Added assertions/)).toBeInTheDocument();
    expect(screen.getAllByText("assertion-new")).toHaveLength(2);
    expect(screen.getByText(/^Removed assertions/)).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Assertion changes" })).toHaveClass("lg:grid-cols-3");
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

  it("keeps history readable for members while hiding lifecycle mutations", () => {
    renderView({ canManageWiki: false, canManageTwin: false });

    expect(screen.getByText("Read-only access")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Accept revision" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.getByText("Current Twin v1")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Sign off proposal" })).not.toBeInTheDocument();
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

});
