// @vitest-environment jsdom

import { act, fireEvent, render, screen, within } from "@testing-library/react";
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

vi.mock("./twin-activation-readiness", () => ({
  TwinActivationReadiness: () => null,
}));

const resources = { en: { common: enCommon, twins: enTwins, ui: enUi } };

function renderView(overrides = {}) {
  return render(
    <I18nProvider locale="en" resources={resources}>
      <WorkspaceSlugProvider slug="acme">
        <SidebarProvider>
          <TwinWorkspaceView {...lifecycleFixture()} {...overrides} />
        </SidebarProvider>
      </WorkspaceSlugProvider>
    </I18nProvider>,
  );
}

describe("TwinWorkspaceView edge states", () => {
  it("allows a stalled review to be dismissed", async () => {
    const deferred: { resolve: () => void } = { resolve: () => undefined };
    const onAcceptWiki = vi.fn(() => new Promise<void>((resolve) => {
      deferred.resolve = resolve;
    }));
    renderView({ onAcceptWiki });

    fireEvent.click(screen.getByRole("button", { name: "Accept revision" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm acceptance" }));

    expect(onAcceptWiki).toHaveBeenCalledWith("wiki-2");
    expect(screen.getByRole("button", { name: "Saving decision" })).toBeDisabled();
    const dismiss = screen.getByRole("button", { name: "Dismiss" });
    expect(dismiss).toBeEnabled();
    fireEvent.click(dismiss);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await act(async () => deferred.resolve());
  });

  it("keeps the rejection reason and displays the failure", async () => {
    const onRejectWiki = vi.fn(async () => {
      throw new Error("offline");
    });
    renderView({ onRejectWiki });

    fireEvent.click(screen.getByRole("button", { name: "Reject revision" }));
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Missing evidence" } });
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Confirm rejection" })));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Reason")).toHaveValue("Missing evidence");
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't save the decision. Try again.");
  });

  it("localizes a lifecycle timeout instead of exposing the transport message", async () => {
    const timeout = new Error("Review request timed out. Try again.");
    timeout.name = "TimeoutError";
    renderView({ onAcceptWiki: vi.fn(async () => Promise.reject(timeout)) });

    fireEvent.click(screen.getByRole("button", { name: "Accept revision" }));
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Confirm acceptance" })));

    expect(screen.getByRole("alert")).toHaveTextContent("The request took too long. Try again.");
  });

  it("ignores a dismissed request's late failure when the dialog is reopened", async () => {
    let rejectRequest: (error: Error) => void = () => undefined;
    const onAcceptWiki = vi.fn(() => new Promise<void>((_resolve, reject) => {
      rejectRequest = reject;
    }));
    renderView({ onAcceptWiki });

    fireEvent.click(screen.getByRole("button", { name: "Accept revision" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm acceptance" }));
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    await act(async () => rejectRequest(new Error("offline")));
    fireEvent.click(screen.getByRole("button", { name: "Accept revision" }));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the rejection reason limit before submission", () => {
    renderView();

    fireEvent.click(screen.getByRole("button", { name: "Reject revision" }));
    const reason = screen.getByLabelText("Reason");
    expect(reason).toHaveAttribute("maxlength", "2000");
    fireEvent.change(reason, { target: { value: "Missing source context" } });
    expect(screen.getByText("22 / 2000 characters")).toBeInTheDocument();
  });

  it("announces progress and errors without removing review context", () => {
    renderView({ wikiMutationPending: true, actionError: "Revision is stale" });

    expect(screen.getByRole("button", { name: "Saving decision" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Revision is stale");
    expect(screen.getByText("Issue 42: Review the source model")).toBeInTheDocument();
  });

  it("does not substitute the current Twin when historical detail fails", () => {
    const fixture = lifecycleFixture();
    const currentVersion = {
      ...fixture.twin.current_version,
      id: "version-2",
      version_number: 2,
    };
    renderView({
      twin: { ...fixture.twin, current_version: currentVersion },
      selectedVersionId: "version-1",
      versionDetail: null,
      actionError: "Historical version unavailable",
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.getByRole("heading", { name: "Current Twin v2" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Version v2" })).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Historical version unavailable");
  });

  it("offers a local retry when the selected Wiki revision is unavailable", () => {
    const onRetryWikiDetail = vi.fn();
    renderView({
      wikiDetail: null,
      wikiDetailState: { kind: "error" },
      onRetryWikiDetail,
    });

    expect(screen.getByTestId("twin-detail-error")).toHaveTextContent("Selection unavailable");
    expect(screen.queryByText("Couldn't save the decision. Try again.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(onRetryWikiDetail).toHaveBeenCalledOnce();
  });

  it("offers local retries for unavailable Twin proposal and version details", () => {
    const onRetryProposalDetail = vi.fn();
    const onRetryVersionDetail = vi.fn();
    renderView({
      proposalDetail: null,
      proposalDetailState: { kind: "error" },
      versionDetail: null,
      versionDetailState: { kind: "error" },
      onRetryProposalDetail,
      onRetryVersionDetail,
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    const detailErrors = screen.getAllByTestId("twin-detail-error");
    expect(detailErrors).toHaveLength(2);
    expect(screen.queryByText("No Twin proposal is available for the accepted Wiki revision.")).not.toBeInTheDocument();
    for (const detailError of detailErrors) {
      fireEvent.click(within(detailError).getByRole("button", { name: "Try again" }));
    }

    expect(onRetryProposalDetail).toHaveBeenCalledOnce();
    expect(onRetryVersionDetail).toHaveBeenCalledOnce();
  });

  it("keeps cached Twin detail visible while offering a stale retry", () => {
    const onRetryProposalDetail = vi.fn();
    renderView({
      proposalDetailState: { kind: "stale" },
      onRetryProposalDetail,
    });

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.getByText("Prefer explicit review decisions.")).toBeInTheDocument();
    expect(screen.getByTestId("twin-detail-stale")).toHaveTextContent("Showing the last known selection");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(onRetryProposalDetail).toHaveBeenCalledOnce();
  });
});
