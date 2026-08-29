import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type {
  OfficeAgent,
  OfficeIssue,
  OfficeModel,
  OfficeSquadMembers,
  OfficeSquad,
  OfficeSubjectRef,
  OfficeWorldId,
} from "@multica/core/office";
import { useEffect } from "react";
import type { SupportedLocale } from "@multica/core/i18n";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { OfficePage } from "./index";
import type { OfficeSceneSlotProps } from "./scene-slot";

const mocks = vi.hoisted(() => ({
  model: null as OfficeModel | null,
  world: "studio" as OfficeWorldId,
  setWorld: vi.fn(),
  useOfficeModel: vi.fn(),
  resolveModel: vi.fn(),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/office", () => ({
  useOfficeModel: (input: unknown) => {
    mocks.useOfficeModel(input);
    return mocks.resolveModel(input);
  },
  useOfficeViewStore: Object.assign(
    (selector: (state: { world: OfficeWorldId }) => unknown) =>
      selector({ world: mocks.world }),
    {
      getState: () => ({ setWorld: mocks.setWorld }),
    },
  ),
  useOfficeTaskCache: () => [],
}));

vi.mock("./office-scene-bridge", () => ({
  OfficeSceneBridge: () => <div data-testid="default-office-scene" />,
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    agentDetail: (id: string) => `/acme/agents/${id}`,
    memberDetail: (id: string) => `/acme/members/${id}`,
    squadDetail: (id: string) => `/acme/squads/${id}`,
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));

const navigation: NavigationAdapter = {
  push: vi.fn(),
  replace: vi.fn(),
  back: vi.fn(),
  pathname: "/acme/office",
  searchParams: new URLSearchParams(),
  getShareableUrl: (path) => `https://multica.test${path}`,
};

const ada = {
  id: "agent-1",
  name: "Ada",
  avatarUrl: null,
  description: "Builds release tooling",
  availability: { kind: "known", value: "online" },
  workload: {
    kind: "known",
    value: "working",
    runningCount: 1,
    queuedCount: 2,
    capacity: 3,
  },
  activeIssueIds: ["issue-1"],
} satisfies OfficeAgent;

const longNameAgent = {
  id: "agent-2",
  name: "负责跨区域发布验证的超长名称智能体",
  avatarUrl: null,
  description: "",
  availability: { kind: "known", value: "offline" },
  workload: {
    kind: "known",
    value: "queued",
    runningCount: 0,
    queuedCount: 3,
    capacity: 2,
  },
  activeIssueIds: [],
} satisfies OfficeAgent;

const releaseSquad = {
  id: "squad-1",
  name: "Release Team",
  description: "Owns release confidence",
  avatarUrl: null,
  leaderAgentId: "agent-1",
  memberCount: 4,
  memberPreview: [
    { kind: "agent", id: "agent-1", role: "Lead" },
    { kind: "member", id: "member-1", role: "Reviewer" },
  ],
} satisfies OfficeSquad;

const resolvedIssue = {
  kind: "resolved",
  id: "issue-1",
  identifier: "MUL-42",
  title: "Ship the accessible office",
  status: "in_progress",
  statusCategory: "in_progress",
  assignedSquadId: "squad-1",
  executingAgentIds: ["agent-1"],
} satisfies OfficeIssue;

const readyModel = {
  kind: "ready",
  snapshot: {
    agents: [ada, longNameAgent],
    squads: [releaseSquad],
    activeIssues: [resolvedIssue],
    overflow: { agents: 0, squads: 0, activeIssues: 0 },
  },
  quality: { kind: "current", refreshing: false },
  inspector: { kind: "closed" },
  retry: vi.fn(),
} satisfies OfficeModel;

function renderOffice() {
  return renderWithI18n(
    <NavigationProvider value={navigation}>
      <OfficePage />
    </NavigationProvider>,
  );
}

const fitCamera = vi.fn();

function SceneProbe({
  selectedSquadAgentIds,
  reducedMotion,
  motionFrozen,
  onCameraControlsChange,
  onRendererFallback,
}: OfficeSceneSlotProps) {
  useEffect(() => {
    onCameraControlsChange({
      fit: fitCamera,
      zoomIn: vi.fn(),
      zoomOut: vi.fn(),
    });
    return () => onCameraControlsChange(null);
  }, [onCameraControlsChange]);
  return (
    <button
      type="button"
      data-testid="scene-probe"
      data-reduced-motion={reducedMotion}
      data-motion-frozen={motionFrozen}
      data-selected-squad-agent-ids={selectedSquadAgentIds?.join(",") ?? ""}
      onClick={onRendererFallback}
    >
      Scene probe
    </button>
  );
}

let controlledSceneProps: OfficeSceneSlotProps | null = null;

function ControlledScene(props: OfficeSceneSlotProps) {
  controlledSceneProps = props;
  return <div data-testid="controlled-office-scene" />;
}

describe("OfficePage", () => {
  beforeEach(() => {
    mocks.model = readyModel;
    mocks.world = "studio";
    mocks.setWorld.mockReset();
    mocks.useOfficeModel.mockReset();
    mocks.resolveModel.mockReset();
    mocks.resolveModel.mockImplementation(() => mocks.model);
    fitCamera.mockReset();
    controlledSceneProps = null;
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 1024,
    });
  });

  it("renders the read-only shell with the real scene bridge by default", () => {
    renderOffice();

    expect(mocks.useOfficeModel).toHaveBeenCalledWith({
      wsId: "workspace-1",
      selected: null,
    });
    expect(screen.getByRole("heading", { name: "Office" })).toBeInTheDocument();
    expect(screen.getByTestId("office-scene-slot")).toBeInTheDocument();
    expect(screen.getByTestId("default-office-scene")).toBeInTheDocument();
    expect(screen.queryByTestId("office-dom-fallback")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fit" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Zoom out" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Zoom in" })).toBeDisabled();

    for (const forbidden of [
      "Create",
      "Edit",
      "Run",
      "Cancel",
      "Chat",
      "Terminal",
    ]) {
      expect(
        screen.queryByRole("button", { name: new RegExp(forbidden, "i") }),
      ).not.toBeInTheDocument();
    }
  });

  it("persists a world only after the scene confirms installation", async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <NavigationProvider value={navigation}>
        <OfficePage SceneSlot={ControlledScene} />
      </NavigationProvider>,
    );

    await user.click(screen.getByRole("radio", { name: "Expedition" }));
    expect(screen.getByRole("radio", { name: "Expedition" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(mocks.setWorld).not.toHaveBeenCalled();

    act(() => controlledSceneProps?.onWorldReady("expedition"));
    expect(mocks.setWorld).toHaveBeenCalledWith("expedition");
  });

  it("reverts a failed world switch without changing the preference", async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <NavigationProvider value={navigation}>
        <OfficePage SceneSlot={ControlledScene} />
      </NavigationProvider>,
    );

    await user.click(screen.getByRole("radio", { name: "Expedition" }));
    act(() => controlledSceneProps?.onWorldSwitchFailure("studio"));

    expect(screen.getByRole("radio", { name: "Studio" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(mocks.setWorld).not.toHaveBeenCalled();
  });

  it("passes exact selected-Squad Agent IDs to the scene", async () => {
    const user = userEvent.setup();
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) =>
        input.selected?.kind === "squad"
          ? ({
              ...readyModel,
              inspector: {
                kind: "squad",
                squad: releaseSquad,
                members: {
                  kind: "ready",
                  members: [
                    {
                      kind: "agent",
                      id: "agent-1",
                      name: "Ada",
                      activeIssueIds: ["issue-1"],
                    },
                    {
                      kind: "member",
                      id: "member-1",
                      name: "Mina",
                      activeIssueIds: [],
                    },
                  ],
                },
              },
            } satisfies OfficeModel)
          : readyModel,
    );
    renderWithI18n(
      <NavigationProvider value={navigation}>
        <OfficePage SceneSlot={SceneProbe} />
      </NavigationProvider>,
    );

    await user.click(screen.getByRole("tab", { name: /Squads/ }));
    await user.click(
      screen.getByRole("button", { name: "Select squad Release Team" }),
    );

    expect(screen.getByTestId("scene-probe")).toHaveAttribute(
      "data-selected-squad-agent-ids",
      "agent-1",
    );
  });

  it("uses roving roster focus and restores the exact tab and row", async () => {
    const user = userEvent.setup();
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) => {
        if (input.selected?.kind !== "squad") return readyModel;
        return {
          ...readyModel,
          inspector: {
            kind: "squad",
            squad: releaseSquad,
            members: {
              kind: "ready",
              members: [
                {
                  kind: "agent",
                  id: "agent-1",
                  name: "Ada",
                  activeIssueIds: ["issue-1"],
                },
                {
                  kind: "member",
                  id: "member-1",
                  name: "Mina",
                  activeIssueIds: [],
                },
              ],
            },
          },
        } satisfies OfficeModel;
      },
    );
    renderOffice();

    const squadTab = screen.getByRole("tab", { name: /Squads/ });
    await user.click(squadTab);
    const squadRow = screen.getByRole("button", {
      name: "Select squad Release Team",
    });
    squadRow.focus();
    await user.keyboard("{Enter}");

    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Release Team" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Squad" })).toHaveAttribute(
      "href",
      "/acme/squads/squad-1",
    );
    const minaLink = screen.getByRole("link", { name: "Mina" });
    expect(minaLink).toHaveAttribute("href", "/acme/members/member-1");
    const minaRow = minaLink.closest("li");
    expect(minaRow).not.toBeNull();
    if (minaRow) expect(within(minaRow).queryByText("Online")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Back to roster" }));

    expect(screen.getByRole("tab", { name: /Squads/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("button", { name: "Select squad Release Team" })).toHaveFocus();
  });

  it("supports Arrow, Home, End, Space, and Escape without losing row focus", async () => {
    const user = userEvent.setup();
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) => {
        if (input.selected?.kind !== "agent") return readyModel;
        const agent = readyModel.snapshot.agents.find(
          (candidate) => candidate.id === input.selected?.id,
        );
        return agent
          ? ({
              ...readyModel,
              inspector: { kind: "agent", agent },
            } satisfies OfficeModel)
          : readyModel;
      },
    );
    renderOffice();

    const first = screen.getByRole("button", { name: "Select agent Ada" });
    const last = screen.getByRole("button", {
      name: "Select agent 负责跨区域发布验证的超长名称智能体",
    });
    expect(first).toHaveAttribute("tabindex", "0");
    expect(last).toHaveAttribute("tabindex", "-1");

    first.focus();
    await user.keyboard("{ArrowDown}");
    expect(last).toHaveFocus();
    await user.keyboard("{Home}");
    expect(first).toHaveFocus();
    await user.keyboard("{End}");
    expect(last).toHaveFocus();
    await user.keyboard(" ");
    expect(
      screen.getByRole("heading", {
        name: "负责跨区域发布验证的超长名称智能体",
      }),
    ).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(
      screen.getByRole("button", {
        name: "Select agent 负责跨区域发布验证的超长名称智能体",
      }),
    ).toHaveFocus();
  });

  it("uses adapter links and keeps availability separate from workload", async () => {
    const user = userEvent.setup();
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) =>
        input.selected?.kind === "agent"
          ? ({
              ...readyModel,
              inspector: {
                kind: "agent",
                agent: ada,
              },
            } satisfies OfficeModel)
          : readyModel,
    );
    renderOffice();

    const adaRow = screen.getByRole("button", { name: "Select agent Ada" });
    expect(adaRow).toHaveTextContent("Online");
    expect(adaRow).toHaveTextContent("Working");
    expect(adaRow).toHaveTextContent("1 running");
    expect(adaRow).toHaveTextContent("2 queued");
    await user.click(adaRow);

    const link = screen.getByRole("link", { name: "Open Agent" });
    expect(link).toHaveAttribute("href", "/acme/agents/agent-1");
    await user.click(link);
    expect(navigation.push).toHaveBeenCalledWith("/acme/agents/agent-1");
  });

  it("labels squad previews without giving human members agent presence", async () => {
    const user = userEvent.setup();
    renderOffice();

    await user.click(screen.getByRole("tab", { name: /Squads/ }));
    const row = screen.getByRole("button", {
      name: "Select squad Release Team",
    });
    expect(row).toHaveTextContent("Member preview");
    expect(row).toHaveTextContent("4 members");
    expect(row).toHaveTextContent("Leader: Ada");
    expect(row).toHaveTextContent("Reviewer");
    expect(row).not.toHaveTextContent("Online");
  });

  it("keeps unresolved active issues selectable and linkable without guessed details", async () => {
    const user = userEvent.setup();
    const unresolved = {
      kind: "unresolved",
      id: "opaque-issue-id",
      reason: "not-returned",
      executingAgentIds: ["agent-1"],
    } as const;
    const unresolvedModel = {
      ...readyModel,
      snapshot: {
        ...readyModel.snapshot,
        activeIssues: [unresolved],
      },
    } satisfies OfficeModel;
    mocks.model = unresolvedModel;
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) =>
        input.selected?.kind === "issue"
          ? ({
              ...unresolvedModel,
              inspector: { kind: "issue", issue: unresolved },
            } satisfies OfficeModel)
          : unresolvedModel,
    );
    renderOffice();

    await user.click(screen.getByRole("tab", { name: /Active Issues/ }));
    const row = screen.getByRole("button", {
      name: "Select issue opaque-issue-id",
    });
    expect(row).toHaveTextContent("opaque-issue-id");
    expect(row).toHaveTextContent("Issue details unavailable");
    expect(row).not.toHaveTextContent("Ship the accessible office");
    expect(row).not.toHaveTextContent("in_progress");
    await user.click(row);

    expect(screen.getByText("Issue opaque-issue-id")).toBeInTheDocument();
    expect(screen.getByText(/title and status are unavailable/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Issue" })).toHaveAttribute(
      "href",
      "/acme/issues/opaque-issue-id",
    );
    expect(screen.queryByRole("heading", { name: "Assigned Squad" })).not.toBeInTheDocument();
  });

  it("shows only model-proven executing Agents and assigned Squad relationships", async () => {
    const user = userEvent.setup();
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) =>
        input.selected?.kind === "issue"
          ? ({
              ...readyModel,
              inspector: { kind: "issue", issue: resolvedIssue },
            } satisfies OfficeModel)
          : readyModel,
    );
    renderOffice();

    await user.click(screen.getByRole("tab", { name: /Active Issues/ }));
    await user.click(screen.getByRole("button", { name: "Select issue MUL-42" }));
    expect(
      screen.getByRole("heading", { name: "Ship the accessible office" }),
    ).toBeInTheDocument();
    expect(screen.getByText("in_progress")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ada" })).toHaveAttribute(
      "href",
      "/acme/agents/agent-1",
    );
    expect(screen.getByRole("link", { name: "Release Team" })).toHaveAttribute(
      "href",
      "/acme/squads/squad-1",
    );
    expect(screen.getByRole("link", { name: "Open Issue" })).toHaveAttribute(
      "href",
      "/acme/issues/issue-1",
    );
  });

  it("presents loading, unavailable recovery, and a truthful empty state", async () => {
    const user = userEvent.setup();
    const retry = vi.fn().mockResolvedValue(undefined);
    mocks.model = { kind: "loading" };
    const loading = renderOffice();
    expect(screen.getByRole("status", { name: "Loading office" })).toBeInTheDocument();

    loading.unmount();
    mocks.model = { kind: "unavailable", retry };
    const unavailable = renderOffice();
    expect(
      screen.getByRole("heading", { name: "Office data is unavailable" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledTimes(1);

    unavailable.unmount();
    mocks.model = {
      ...readyModel,
      snapshot: {
        agents: [],
        squads: [],
        activeIssues: [],
        overflow: { agents: 0, squads: 0, activeIssues: 0 },
      },
    };
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 390,
    });
    renderOffice();
    expect(
      screen.getByRole("heading", { name: "The office is quiet" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/no active agents, squads, or issue signals/i)).toBeInTheDocument();
  });

  it("distinguishes partial, refreshing, and stale data without a Live claim", () => {
    mocks.model = {
      ...readyModel,
      quality: {
        kind: "partial",
        gaps: ["availability", "issue-briefs"],
      },
    };
    const partial = renderOffice();
    const partialNotice = screen.getByTestId("office-quality-notice");
    expect(partialNotice).toHaveTextContent("Some office details are unavailable");
    expect(partialNotice).toHaveTextContent("Agent availability");
    expect(partialNotice).toHaveTextContent("Issue details");
    expect(partialNotice).not.toHaveTextContent(/\bLive\b/);

    partial.unmount();
    mocks.model = {
      ...readyModel,
      quality: { kind: "current", refreshing: true },
    };
    const refreshing = renderOffice();
    expect(screen.getByText("Refreshing workspace facts")).toBeInTheDocument();

    refreshing.unmount();
    mocks.model = {
      ...readyModel,
      quality: { kind: "stale", gaps: ["workload"] },
    };
    renderOffice();
    const staleNotice = screen.getByTestId("office-quality-notice");
    expect(staleNotice).toHaveTextContent("Showing last-known office data");
    expect(staleNotice).toHaveTextContent("Agent workload");
    expect(staleNotice).not.toHaveTextContent(/\bLive\b/);
  });

  it("mounts the narrow scene slot, enables camera controls, and retires to the DOM fallback", async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <NavigationProvider value={navigation}>
        <OfficePage SceneSlot={SceneProbe} />
      </NavigationProvider>,
    );

    const scene = screen.getByTestId("office-scene-slot");
    expect(scene).toHaveAttribute("aria-hidden", "true");
    expect(screen.queryByTestId("office-dom-fallback")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Fit" })).toBeEnabled(),
    );
    await user.click(screen.getByRole("button", { name: "Fit" }));
    expect(fitCamera).toHaveBeenCalledTimes(1);

    await user.click(screen.getByTestId("scene-probe"));
    expect(screen.getByTestId("office-dom-fallback")).toHaveAttribute(
      "data-fallback-reason",
      "renderer",
    );
    expect(screen.getByText(/visual scene is unavailable/i)).toBeInTheDocument();
  });

  it("passes reduced motion to the scene and freezes it for partial data", async () => {
    const originalMatchMedia = window.matchMedia;
    window.matchMedia = (query: string) => ({
      matches: query === "(prefers-reduced-motion: reduce)",
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    });
    mocks.model = {
      ...readyModel,
      quality: { kind: "partial", gaps: ["workload"] },
    };
    try {
      renderWithI18n(
        <NavigationProvider value={navigation}>
          <OfficePage SceneSlot={SceneProbe} />
        </NavigationProvider>,
      );
      await waitFor(() =>
        expect(screen.getByTestId("scene-probe")).toHaveAttribute(
          "data-reduced-motion",
          "true",
        ),
      );
      expect(screen.getByTestId("scene-probe")).toHaveAttribute(
        "data-motion-frozen",
        "true",
      );
    } finally {
      window.matchMedia = originalMatchMedia;
    }
  });

  it("keeps the full roster below 768px and opens selection in a Sheet", async () => {
    const user = userEvent.setup();
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 390,
    });
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) =>
        input.selected?.kind === "agent"
          ? ({
              ...readyModel,
              inspector: {
                kind: "agent",
                agent: ada,
              },
            } satisfies OfficeModel)
          : readyModel,
    );
    renderWithI18n(
      <NavigationProvider value={navigation}>
        <OfficePage SceneSlot={SceneProbe} />
      </NavigationProvider>,
    );

    await waitFor(() =>
      expect(screen.getByTestId("office-dom-fallback")).toHaveAttribute(
        "data-fallback-reason",
        "narrow",
      ),
    );
    expect(screen.queryByTestId("scene-probe")).not.toBeInTheDocument();
    expect(screen.getByRole("tablist", { hidden: true })).toBeInTheDocument();
    const longRow = screen.getByRole("button", {
      name: "Select agent 负责跨区域发布验证的超长名称智能体",
    });
    expect(within(longRow).getByText("负责跨区域发布验证的超长名称智能体")).toHaveClass(
      "break-words",
    );
    expect(longRow).toHaveClass("min-h-11");

    await user.click(screen.getByRole("button", { name: "Select agent Ada" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("tablist", { hidden: true })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Agent" })).toHaveAttribute(
      "href",
      "/acme/agents/agent-1",
    );
  });

  it("moves the roster into a Sheet at medium widths without removing the scene", async () => {
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 900,
    });

    renderWithI18n(
      <NavigationProvider value={navigation}>
        <OfficePage SceneSlot={SceneProbe} />
      </NavigationProvider>,
    );

    expect(await screen.findByTestId("scene-probe")).toBeInTheDocument();
    const sheet = await screen.findByRole("dialog");
    expect(within(sheet).getByRole("tablist")).toBeInTheDocument();
  });

  it("shows selected Squad membership loading and local retry states", async () => {
    const user = userEvent.setup();
    const retryMembers = vi.fn().mockResolvedValue(undefined);
    let members: OfficeSquadMembers = { kind: "loading" };
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) =>
        input.selected?.kind === "squad"
          ? ({
              ...readyModel,
              inspector: {
                kind: "squad",
                squad: releaseSquad,
                members,
              },
            } satisfies OfficeModel)
          : readyModel,
    );
    renderOffice();
    await user.click(screen.getByRole("tab", { name: /Squads/ }));
    await user.click(screen.getByRole("button", { name: "Select squad Release Team" }));
    expect(screen.getByText("Loading full membership")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Back to roster" }));
    members = { kind: "unavailable", retry: retryMembers };
    await user.click(screen.getByRole("button", { name: "Select squad Release Team" }));
    expect(screen.getByText("Full membership is unavailable")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try membership again" }));
    expect(retryMembers).toHaveBeenCalledTimes(1);
  });

  it("announces only a selected subject disappearing", async () => {
    const user = userEvent.setup();
    mocks.resolveModel.mockImplementation(
      (input: { selected: OfficeSubjectRef | null }) =>
        input.selected
          ? ({
              ...readyModel,
              inspector: { kind: "missing", subject: input.selected },
            } satisfies OfficeModel)
          : readyModel,
    );
    renderOffice();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Select agent Ada" }));
    expect(screen.getByRole("status")).toHaveTextContent(
      "The selected office subject is no longer available.",
    );
  });

  const localizedCopy: readonly {
    locale: SupportedLocale;
    title: string;
    squads: RegExp;
  }[] = [
    { locale: "en", title: "Office", squads: /Squads/ },
    { locale: "zh-Hans", title: "办公室", squads: /小队/ },
    { locale: "ja", title: "オフィス", squads: /スクワッド/ },
    { locale: "ko", title: "오피스", squads: /스쿼드/ },
  ];

  it.each(localizedCopy)(
    "renders the Office namespace in $locale",
    ({ locale, title, squads }) => {
      renderWithI18n(
        <NavigationProvider value={navigation}>
          <OfficePage />
        </NavigationProvider>,
        { locale },
      );
      expect(screen.getByRole("heading", { name: title })).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: squads })).toBeInTheDocument();
    },
  );
});
