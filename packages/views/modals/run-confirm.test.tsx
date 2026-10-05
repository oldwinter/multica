import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, screen, fireEvent, waitFor } from "@testing-library/react";
import { buildIssueStatusCatalog } from "@multica/core/issue-statuses";
import {
  configureShortcutPlatform,
  createShortcutChord,
  useShortcutStore,
} from "@multica/core/shortcuts";
import { RunConfirmModal } from "./run-confirm";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-test" }));
vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: [{ id: "squad-1", leader_id: "agent-1" }] }),
}));
vi.mock("@multica/core/workspace/queries", () => ({ squadListOptions: () => ({}) }));
vi.mock("@multica/core/issue-statuses/hooks", () => ({
  useIssueStatuses: () =>
    buildIssueStatusCatalog([
      {
        id: "rework",
        workspace_id: "ws-test",
        key: "rework",
        name: "Rework",
        description: "",
        category: "unstarted",
        color: "#22c55e",
        is_system: false,
        position: 0,
        archived_at: null,
        created_at: "",
        updated_at: "",
      },
    ]),
}));

const mockUpdate = vi.fn().mockResolvedValue({ id: "issue-1" });
const mockBatch = vi.fn().mockResolvedValue({ updated: 2 });
vi.mock("@multica/core/issues/mutations", () => ({
  useUpdateIssue: () => ({ mutateAsync: mockUpdate }),
  useBatchUpdateIssues: () => ({ mutateAsync: mockBatch }),
}));

const mockPreview = vi.fn();
let previewData: Record<string, unknown> | undefined;
vi.mock("@multica/core/twins", () => ({
  usePreviewTwinBriefing: () => ({
    mutateAsync: mockPreview,
  }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Walt" }),
}));

vi.mock("../i18n", () => ({
  useT: () => ({
    t: (
      sel: (x: Record<string, Record<string, string>>) => string,
      vars?: Record<string, unknown>,
    ) => {
      // Resolve the accessor against a flat label map so assertions can target
      // text, then interpolate {{name}} / {{count}} the way i18next would — the
      // headline substitutes the assignee name and the batch count.
      const labels = {
        run_confirm: {
          title_assign: "Confirm assignment?",
          assign_single: "assign to {{name}}",
          assign_batch: "assign {{count}} to {{name}}",
          confirm_assign: "Confirm assignment",
          dont_start: "Don't start yet",
          toast_failed: "failed",
          twin_title: "Twin for this run",
          twin_description: "review exact context",
          twin_loading: "compiling",
          twin_error: "unavailable",
          twin_effective: "effective {{state}} v{{version}}",
          twin_off: "Off",
          twin_preview: "Preview",
          twin_enabled: "Enabled",
          twin_no_version: "signed version required",
          twin_budget: "{{bytes}} bytes {{tokens}} tokens",
          twin_version_id: "Exact Twin version",
          twin_version_digest: "Version digest",
          title_promote: "Start work now?",
          promote_single: "move to {{status}}, {{name}} starts",
          confirm_promote: "Move and start",
        },
        // useStatusLabel resolves BUILT-IN keys through i18n and custom ones
        // through the catalog, so the promote headline needs both sources.
        status: { todo: "Todo" },
      };
      return sel(labels).replace(/\{\{(\w+)\}\}/g, (_m, k) => String(vars?.[k] ?? ""));
    },
  }),
}));

// Keep the ui primitives as light DOM so the logic is what's under test.
vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  // Keeps the real Popup's prop passthrough, which the send chord binds to.
  DialogContent: ({ children, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
    <div data-testid="dialog-content" {...props}>{children}</div>
  ),
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));
vi.mock("@multica/ui/components/ui/button", () => ({
  Button: ({ children, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props}>{children}</button>
  ),
}));
vi.mock("@multica/ui/components/ui/spinner", () => ({
  Spinner: () => <span data-testid="spinner" />,
}));
// vi.hoisted: vi.mock factories run before module-level consts initialize.
// Only error is used now — completion is silent (no result toast).
const mockToast = vi.hoisted(() => ({ error: vi.fn(), success: vi.fn() }));
vi.mock("sonner", () => ({ toast: mockToast }));

beforeEach(() => {
  mockUpdate.mockClear().mockResolvedValue({ id: "issue-1" });
  mockBatch.mockClear().mockResolvedValue({ updated: 2 });
  mockToast.error.mockClear();
  mockToast.success.mockClear();
  previewData = {
    policy: { state: "off", reason: "one_off" },
    twinVersion: null,
    briefing: "",
    byteCount: 0,
    tokenCount: 0,
  };
  mockPreview.mockReset().mockImplementation(async () => previewData);
  // The real shortcut store drives both the submit chord and the keycap hint,
  // and jsdom's platform follows the host OS — pin it so the chord is ⌘+Enter
  // everywhere, not Ctrl+Enter on a Linux CI runner.
  configureShortcutPlatform("macos");
  useShortcutStore.setState({ overrides: {} });
});

afterEach(() => {
  configureShortcutPlatform(null);
  useShortcutStore.setState({ overrides: {} });
});

const confirmButton = () => screen.getByRole("button", { name: "Confirm assignment" });
const waitForPreview = () => waitFor(() => expect(confirmButton()).not.toBeDisabled());
const dialog = () => screen.getByTestId("dialog-content");

const single = {
  issueIds: ["issue-1"],
  mode: "assign" as const,
  assigneeType: "agent" as const,
  assigneeId: "agent-1",
  request: "Fix login",
  projectId: "project-1",
};

// Promoting a parked issue out of backlog starts the run on its own, so it
// confirms through this same dialog — one behaviour for built-in `todo` and
// every custom Todo-category status alike (MUL-6463).
const promote = {
  issueIds: ["issue-1"],
  mode: "promote" as const,
  status: "rework",
  assigneeType: "agent" as const,
  assigneeId: "agent-1",
  request: "Fix login",
  projectId: "project-1",
};

describe("RunConfirmModal", () => {
  it("blocks submission while the Twin preview is pending", async () => {
    mockPreview.mockReturnValue(new Promise(() => undefined));
    const { container } = render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    expect(screen.getByTestId("spinner")).toBeInTheDocument();
    expect(confirmButton()).toBeDisabled();
    // Headline reads across elements — the assignee name is bolded in place.
    expect(container.textContent).toContain("assign to Walt");
    await waitFor(() => expect(mockPreview).toHaveBeenCalledWith({
      agentId: "agent-1",
      projectId: "project-1",
      issueId: "issue-1",
      request: "Fix login",
      oneOffState: "off",
    }));
  });

  it("single assign sends the assignee change with the reviewed Twin state", async () => {
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    await waitForPreview();
    fireEvent.click(confirmButton());
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    expect(mockUpdate).toHaveBeenCalledWith({
      id: "issue-1",
      assignee_type: "agent",
      assignee_id: "agent-1",
      twin_use: { state: "off" },
    });
    expect(mockBatch).not.toHaveBeenCalled();
  });

  it("completes silently on success — closes with no result toast", async () => {
    // Final scope: the dialog only confirms the assignment. The assignee and any
    // run surface through the issue's normal updates, so submit adds no toast.
    const onClose = vi.fn();
    render(<RunConfirmModal onClose={onClose} data={single} />);
    await waitForPreview();
    fireEvent.click(confirmButton());
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockToast.success).not.toHaveBeenCalled();
    expect(mockToast.error).not.toHaveBeenCalled();
  });

  it("'暂不开始' sends suppress_run alongside the assignee change", async () => {
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    await waitForPreview();
    fireEvent.click(screen.getByText("Don't start yet"));
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    expect(mockUpdate).toHaveBeenCalledWith({
      id: "issue-1",
      assignee_type: "agent",
      assignee_id: "agent-1",
      suppress_run: true,
    });
    expect(mockToast.success).not.toHaveBeenCalled();
  });

  it("promote sends the status change with no assignee fields", async () => {
    // The owner is already on the issue: re-sending it would turn a status
    // write into an assignee write on the server's side of the predicate.
    render(<RunConfirmModal onClose={vi.fn()} data={promote} />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Move and start" })).not.toBeDisabled());
    fireEvent.click(screen.getByRole("button", { name: "Move and start" }));
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    expect(mockUpdate).toHaveBeenCalledWith({
      id: "issue-1",
      status: "rework",
      twin_use: { state: "off" },
    });
  });

  it("promote's 'don't start yet' still moves the issue, without the run", async () => {
    // The status change is the point; suppress_run is the only difference. This
    // is the one way to leave backlog WITHOUT waking the agent.
    render(<RunConfirmModal onClose={vi.fn()} data={promote} />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Move and start" })).not.toBeDisabled());
    fireEvent.click(screen.getByText("Don't start yet"));
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    expect(mockUpdate).toHaveBeenCalledWith({
      id: "issue-1",
      status: "rework",
      suppress_run: true,
    });
  });

  it("promote names the target status the way the workspace named it", () => {
    // A custom status is only recognisable by its catalog name; built-ins keep
    // resolving through i18n so a zh workspace never reads "In Progress".
    const { container, rerender } = render(<RunConfirmModal onClose={vi.fn()} data={promote} />);
    expect(screen.getByText("Start work now?")).toBeInTheDocument();
    expect(container.textContent).toContain("move to Rework, Walt starts");

    rerender(<RunConfirmModal onClose={vi.fn()} data={{ ...promote, status: "todo" }} />);
    expect(container.textContent).toContain("move to Todo, Walt starts");
  });

  it("batch assign (N ids) applies via batchUpdate", async () => {
    const { container } = render(
      <RunConfirmModal onClose={vi.fn()} data={{ ...single, issueIds: ["i1", "i2"] }} />,
    );
    expect(container.textContent).toContain("assign 2 to Walt");
    expect(screen.queryByText("Twin for this run")).not.toBeInTheDocument();
    fireEvent.click(confirmButton());
    await waitFor(() => expect(mockBatch).toHaveBeenCalledTimes(1));
    expect(mockBatch).toHaveBeenCalledWith({
      ids: ["i1", "i2"],
      updates: { assignee_type: "agent", assignee_id: "agent-1" },
    });
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(mockToast.success).not.toHaveBeenCalled();
  });

  it.each(["enabled", "preview"] as const)(
    "recompiles default-off as one-off %s and queues the exact returned version",
    async (state) => {
      const versionId = `version-${state}`;
      const digest = `sha256:${(state === "enabled" ? "a" : "b").repeat(64)}`;
      const signedPreview = {
        policy: { state, reason: "one_off" },
        twinVersion: { id: versionId, versionNumber: 7, contentDigest: digest },
        briefing: `briefing-${state}`,
        byteCount: 37,
        tokenCount: 9,
      };
      mockPreview.mockImplementation(async (input: { oneOffState?: string }) => (
        input.oneOffState === "off" ? previewData : signedPreview
      ));
      render(<RunConfirmModal onClose={vi.fn()} data={single} />);

      await waitForPreview();
      expect(screen.getByRole("button", { name: "Off" })).toHaveAttribute("aria-pressed", "true");
      expect(screen.getByRole("button", { name: "Enabled" })).not.toBeDisabled();
      fireEvent.click(screen.getByRole("button", { name: state === "enabled" ? "Enabled" : "Preview" }));

      await waitFor(() => expect(mockPreview).toHaveBeenLastCalledWith({
        agentId: "agent-1",
        projectId: "project-1",
        issueId: "issue-1",
        request: "Fix login",
        oneOffState: state,
      }));
      expect(await screen.findByText(`briefing-${state}`)).toBeInTheDocument();
      expect(screen.queryByText(versionId)).not.toBeInTheDocument();
      expect(screen.getAllByText(/v7/).length).toBeGreaterThan(0);
      expect(screen.getByText(digest)).toBeInTheDocument();
      await waitForPreview();
      fireEvent.click(confirmButton());

      await waitFor(() => expect(mockUpdate).toHaveBeenCalledWith({
        id: "issue-1",
        assignee_type: "agent",
        assignee_id: "agent-1",
        twin_use: { state, twin_version_id: versionId },
      }));
    },
  );

  it("ignores a stale one-off response that resolves after the latest mode", async () => {
    let resolveEnabled!: (value: Record<string, unknown>) => void;
    let resolvePreview!: (value: Record<string, unknown>) => void;
    const enabledPromise = new Promise<Record<string, unknown>>((resolve) => { resolveEnabled = resolve; });
    const previewPromise = new Promise<Record<string, unknown>>((resolve) => { resolvePreview = resolve; });
    mockPreview.mockImplementation((input: { oneOffState?: string }) => {
      if (input.oneOffState === "enabled") return enabledPromise;
      if (input.oneOffState === "preview") return previewPromise;
      return Promise.resolve(previewData);
    });
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    await waitForPreview();

    fireEvent.click(screen.getByRole("button", { name: "Enabled" }));
    await waitFor(() => expect(mockPreview).toHaveBeenCalledWith(expect.objectContaining({ oneOffState: "enabled" })));
    expect(confirmButton()).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Preview" }));
    await waitFor(() => expect(mockPreview).toHaveBeenCalledWith(expect.objectContaining({ oneOffState: "preview" })));

    await act(async () => resolvePreview({
      policy: { state: "preview", reason: "one_off" },
      twinVersion: { id: "version-preview", versionNumber: 8, contentDigest: `sha256:${"b".repeat(64)}` },
      briefing: "latest-preview",
      byteCount: 30,
      tokenCount: 8,
    }));
    expect(await screen.findByText("latest-preview")).toBeInTheDocument();
    await act(async () => resolveEnabled({
      policy: { state: "enabled", reason: "one_off" },
      twinVersion: { id: "version-stale", versionNumber: 7, contentDigest: `sha256:${"a".repeat(64)}` },
      briefing: "stale-enabled",
      byteCount: 20,
      tokenCount: 5,
    }));

    expect(screen.getByText("latest-preview")).toBeInTheDocument();
    expect(screen.queryByText("stale-enabled")).not.toBeInTheDocument();
    fireEvent.click(confirmButton());
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledWith(expect.objectContaining({
      twin_use: { state: "preview", twin_version_id: "version-preview" },
    })));
  });

  it("does not attach a Twin snapshot when the run is suppressed", async () => {
    previewData = {
      policy: { state: "enabled", reason: "workspace_binding" },
      twinVersion: { id: "version-1", versionNumber: 7 },
      briefing: "briefing",
      byteCount: 8,
      tokenCount: 2,
    };
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    await waitForPreview();
    fireEvent.click(screen.getByText("Don't start yet"));

    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(mockUpdate.mock.calls[0]![0]).not.toHaveProperty("twin_use");
  });

  it("blocks enabled submission when recompilation returns no signed version", async () => {
    previewData = {
      policy: { state: "enabled", reason: "stale_binding" },
      twinVersion: null,
      briefing: "",
      byteCount: 0,
      tokenCount: 0,
    };
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);

    await waitForPreview();
    fireEvent.click(screen.getByRole("button", { name: "Enabled" }));
    await waitFor(() => expect(mockPreview).toHaveBeenCalledWith(expect.objectContaining({ oneOffState: "enabled" })));
    expect(confirmButton()).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Off" }));
    await waitForPreview();
    fireEvent.click(confirmButton());
    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(mockUpdate.mock.calls[0]![0].twin_use).toEqual({ state: "off" });
  });

  // --- Send chord (MUL-5694) ------------------------------------------------
  // The chord is bound on the dialog, not on a single control, so it confirms
  // wherever focus happens to be.

  it("confirms on the send chord typed anywhere in the dialog", async () => {
    const onClose = vi.fn();
    render(<RunConfirmModal onClose={onClose} data={single} />);
    await waitForPreview();
    fireEvent.keyDown(dialog(), { key: "Enter", metaKey: true });
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    expect(mockUpdate).toHaveBeenCalledWith({
      id: "issue-1",
      assignee_type: "agent",
      assignee_id: "agent-1",
      twin_use: { state: "off" },
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("confirms from a focused footer button, which the chord cannot activate", async () => {
    // Chromium fires no click for ⌘/Ctrl+Enter on a focused button, so without
    // the dialog handling it there the chord is simply dead. The dialog focuses
    // its first tabbable child, which is now a footer button.
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    await waitForPreview();
    fireEvent.keyDown(screen.getByText("Don't start yet"), { key: "Enter", metaKey: true });
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    // The primary action, not the button the caret happened to sit on.
    const payload = mockUpdate.mock.calls[0]![0];
    expect(payload.suppress_run).toBeUndefined();
    expect(payload.twin_use).toEqual({ state: "off" });
  });

  it("yields to a focused button when send is remapped to plain Enter", () => {
    // A bare Enter DOES activate a focused button, so confirming here as well
    // would double-write — and on "Don't start yet" the two would disagree.
    useShortcutStore.setState({ overrides: { send: createShortcutChord("Enter") } });
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    fireEvent.keyDown(screen.getByText("Don't start yet"), { key: "Enter" });
    fireEvent.keyDown(confirmButton(), { key: "Enter" });
    expect(mockUpdate).not.toHaveBeenCalled();
  });

  it("submits once for a held chord, and never for an IME's committing Enter", async () => {
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    await waitForPreview();
    fireEvent.keyDown(dialog(), { key: "Enter", metaKey: true, isComposing: true });
    fireEvent.keyDown(dialog(), { key: "Enter", metaKey: true, repeat: true });
    expect(mockUpdate).not.toHaveBeenCalled();
    fireEvent.keyDown(dialog(), { key: "Enter", metaKey: true });
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
  });

  it("follows a remapped send chord instead of hardcoding ⌘+Enter", async () => {
    useShortcutStore.setState({ overrides: { send: createShortcutChord("Enter") } });
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    await waitForPreview();
    fireEvent.keyDown(dialog(), { key: "Enter", metaKey: true });
    expect(mockUpdate).not.toHaveBeenCalled();
    fireEvent.keyDown(dialog(), { key: "Enter" });
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
  });

  it("shows the chord on the confirm button without renaming it", () => {
    render(<RunConfirmModal onClose={vi.fn()} data={single} />);
    // Decorative: discoverable next to the label, absent from the a11y name —
    // `confirmButton()` resolving by that exact name is the assertion.
    expect(
      confirmButton().querySelector('[data-slot="shortcut-keycaps"]'),
    ).toBeInTheDocument();
  });

  it("keeps the dialog open and surfaces the error when the write fails", async () => {
    const onClose = vi.fn();
    mockUpdate.mockRejectedValue(new Error("boom"));
    render(<RunConfirmModal onClose={onClose} data={single} />);
    await waitForPreview();
    fireEvent.click(confirmButton());
    await waitFor(() => expect(mockToast.error).toHaveBeenCalledWith("boom"));
    expect(onClose).not.toHaveBeenCalled();
    expect(mockToast.success).not.toHaveBeenCalled();
  });
});
