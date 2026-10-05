import { act, render, screen } from "@testing-library/react";
import type {
  OfficeSnapshot,
  OfficeTaskObservation,
} from "@multica/core/office";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { OfficeSceneCommit } from "./scene";
import type { OfficeSceneSlotProps } from "./scene-slot";

const mocks = vi.hoisted(() => ({
  tasks: [] as OfficeTaskObservation[],
  taskFetching: false,
  taskUpdatedAt: 0,
  wsId: "workspace-1",
  reconnect: null as null | (() => void),
  sceneProps: null as null | {
    commit: OfficeSceneCommit;
    onStatus: (status: unknown) => void;
    onCameraControlsChange: (controls: unknown) => void;
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => mocks.wsId,
}));

vi.mock("@multica/core/office", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/office")>()),
  useOfficeTaskCache: () => ({
    observations: mocks.tasks,
    isFetching: mocks.taskFetching,
    dataUpdatedAt: mocks.taskUpdatedAt,
  }),
}));

vi.mock("@multica/core/realtime", () => ({
  useWSReconnect: (callback: () => void) => {
    mocks.reconnect = callback;
  },
}));

vi.mock("./scene", () => ({
  OfficeScene: (props: unknown) => {
    mocks.sceneProps = props as NonNullable<typeof mocks.sceneProps>;
    return <div data-testid="semantic-office-scene" />;
  },
}));

import { OfficeSceneBridge } from "./office-scene-bridge";

const snapshot: OfficeSnapshot = {
  agents: [],
  squads: [],
  activeIssues: [],
  overflow: { agents: 0, squads: 0, activeIssues: 0 },
};

function task(
  overrides: Partial<OfficeTaskObservation> = {},
): OfficeTaskObservation {
  return {
    taskId: "task-1",
    agentId: "agent-1",
    issueId: "issue-1",
    status: "queued-like",
    ...overrides,
  };
}

function bridgeProps(
  overrides: Partial<OfficeSceneSlotProps> = {},
): OfficeSceneSlotProps {
  return {
    snapshot,
    world: "studio",
    selected: { kind: "squad", id: "squad-1" },
    selectedSquadAgentIds: ["agent-2"],
    reducedMotion: false,
    motionFrozen: false,
    onSelect: vi.fn(),
    onCameraControlsChange: vi.fn(),
    onRendererFallback: vi.fn(),
    onWorldReady: vi.fn(),
    onWorldSwitchFailure: vi.fn(),
    ...overrides,
  };
}

function latestSceneProps() {
  if (!mocks.sceneProps) throw new Error("scene did not render");
  return mocks.sceneProps;
}

beforeEach(() => {
  mocks.tasks = [];
  mocks.taskFetching = false;
  mocks.taskUpdatedAt = 0;
  mocks.wsId = "workspace-1";
  mocks.reconnect = null;
  mocks.sceneProps = null;
  Object.defineProperty(document, "hidden", {
    configurable: true,
    value: false,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("OfficeSceneBridge", () => {
  it("builds the cold replace commit with exact selected-Squad Agent IDs", () => {
    mocks.tasks = [task()];
    render(<OfficeSceneBridge {...bridgeProps()} />);

    expect(screen.getByTestId("semantic-office-scene")).toBeInTheDocument();
    expect(latestSceneProps().commit).toMatchObject({
      world: "studio",
      snapshot,
      selected: { kind: "squad", id: "squad-1" },
      selectedSquadAgentIds: ["agent-2"],
      mode: "replace",
      effects: [],
      reducedMotion: false,
      motionFrozen: false,
    });
    expect("tasks" in latestSceneProps().commit.snapshot).toBe(false);
  });

  it("emits queued, started, and finished only for continuously observed task IDs", () => {
    const props = bridgeProps({ selected: null, selectedSquadAgentIds: [] });
    const view = render(<OfficeSceneBridge {...props} />);
    expect(latestSceneProps().commit.mode).toBe("replace");

    mocks.tasks = [task()];
    view.rerender(<OfficeSceneBridge {...props} />);
    expect(latestSceneProps().commit).toMatchObject({
      mode: "transition",
      effects: [{ kind: "task-queued", taskId: "task-1" }],
    });

    mocks.tasks = [task({ status: "running" })];
    view.rerender(<OfficeSceneBridge {...props} />);
    expect(latestSceneProps().commit.effects).toEqual([
      expect.objectContaining({ kind: "task-started", taskId: "task-1" }),
    ]);

    mocks.tasks = [task({ status: "completed" })];
    view.rerender(<OfficeSceneBridge {...props} />);
    expect(latestSceneProps().commit.effects).toEqual([
      expect.objectContaining({
        kind: "task-finished",
        taskId: "task-1",
        outcome: "completed",
      }),
    ]);
  });

  it("rebases on workspace, world, quality, foreground, recovery, and reduced motion", () => {
    const initial = bridgeProps({ selected: null, selectedSquadAgentIds: [] });
    const view = render(<OfficeSceneBridge {...initial} />);

    mocks.tasks = [task()];
    view.rerender(<OfficeSceneBridge {...initial} />);
    expect(latestSceneProps().commit.mode).toBe("transition");

    mocks.wsId = "workspace-2";
    view.rerender(<OfficeSceneBridge {...initial} />);
    expect(latestSceneProps().commit).toMatchObject({ mode: "replace", effects: [] });

    mocks.taskFetching = true;
    act(() => mocks.reconnect?.());
    view.rerender(<OfficeSceneBridge {...initial} />);
    expect(latestSceneProps().commit).toMatchObject({
      mode: "replace",
      effects: [],
      motionFrozen: true,
    });

    mocks.tasks = [
      task({ status: "running" }),
      task({ taskId: "task-2" }),
    ];
    mocks.taskFetching = false;
    mocks.taskUpdatedAt += 1;
    view.rerender(<OfficeSceneBridge {...initial} />);
    expect(latestSceneProps().commit).toMatchObject({
      mode: "replace",
      effects: [],
      motionFrozen: false,
    });

    view.rerender(<OfficeSceneBridge {...initial} world="expedition" />);
    expect(latestSceneProps().commit).toMatchObject({ mode: "replace", effects: [] });

    mocks.tasks = [task(), task({ taskId: "task-2" })];
    view.rerender(
      <OfficeSceneBridge {...initial} world="expedition" motionFrozen />,
    );
    expect(latestSceneProps().commit).toMatchObject({
      mode: "replace",
      effects: [],
      motionFrozen: true,
      reducedMotion: false,
    });

    Object.defineProperty(document, "hidden", {
      configurable: true,
      value: true,
    });
    act(() => document.dispatchEvent(new Event("visibilitychange")));
    expect(latestSceneProps().commit).toMatchObject({ mode: "replace", effects: [] });

    Object.defineProperty(document, "hidden", {
      configurable: true,
      value: false,
    });
    act(() => document.dispatchEvent(new Event("visibilitychange")));
    expect(latestSceneProps().commit).toMatchObject({ mode: "replace", effects: [] });

    act(() => latestSceneProps().onStatus({ kind: "recovering" }));
    expect(latestSceneProps().commit).toMatchObject({ mode: "replace", effects: [] });

    view.rerender(
      <OfficeSceneBridge {...initial} world="expedition" reducedMotion />,
    );
    expect(latestSceneProps().commit).toMatchObject({
      mode: "replace",
      effects: [],
      reducedMotion: true,
      motionFrozen: false,
    });
  });

  it("forwards camera controls and renderer outcomes to the DOM surface", () => {
    const props = bridgeProps();
    render(<OfficeSceneBridge {...props} />);
    const controls = {
      fit: vi.fn(),
      zoomIn: vi.fn(),
      zoomOut: vi.fn(),
    };

    act(() => latestSceneProps().onCameraControlsChange(controls));
    expect(props.onCameraControlsChange).toHaveBeenCalledWith(controls);

    act(() =>
      latestSceneProps().onStatus({ kind: "ready", world: "studio" }),
    );
    expect(props.onWorldReady).toHaveBeenCalledWith("studio");

    act(() =>
      latestSceneProps().onStatus({
        kind: "world-switch-failed",
        attemptedWorld: "expedition",
        retainedWorld: "studio",
      }),
    );
    expect(props.onWorldSwitchFailure).toHaveBeenCalledWith("studio");

    act(() =>
      latestSceneProps().onStatus({ kind: "fallback", reason: "context" }),
    );
    expect(props.onRendererFallback).toHaveBeenCalledOnce();
  });
});
