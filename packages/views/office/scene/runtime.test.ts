import { describe, expect, it } from "vitest";
import type { OfficeSceneCommit, OfficeScenePlan } from "./contracts";
import { OfficeSceneRuntime } from "./runtime";

function commit(overrides: Partial<OfficeSceneCommit> = {}): OfficeSceneCommit {
  return {
    world: "studio",
    snapshot: {
      agents: [],
      squads: [],
      activeIssues: [],
      overflow: { agents: 0, squads: 0, activeIssues: 0 },
    },
    selected: null,
    selectedSquadAgentIds: [],
    mode: "replace",
    effects: [],
    reducedMotion: false,
    motionFrozen: false,
    ...overrides,
  };
}

class RuntimePort {
  plans = [] as OfficeScenePlan[];
  played: string[][] = [];
  pauses = 0;
  resumes = 0;
  rebuilds = 0;
  cancels = 0;
  destroys = 0;
  fits = 0;
  zoomIns = 0;
  zoomOuts = 0;
  rejectInstall = false;
  rejectRebuild = false;
  contextLossHandler = () => {};

  async installWorld() {
    if (this.rejectInstall) throw new Error("asset failure");
  }

  apply(plan: OfficeScenePlan) {
    this.plans.push(plan);
  }

  cancelEffects() {
    this.cancels += 1;
  }

  playEffects(effects: readonly { readonly taskId: string }[]) {
    this.played.push(effects.map((effect) => effect.taskId));
  }

  pause() {
    this.pauses += 1;
  }

  resume() {
    this.resumes += 1;
  }

  fit() {
    this.fits += 1;
  }

  zoomIn() {
    this.zoomIns += 1;
  }

  zoomOut() {
    this.zoomOuts += 1;
  }

  async rebuild() {
    this.rebuilds += 1;
    if (this.rejectRebuild) throw new Error("rebuild failure");
  }

  onContextLoss(handler: () => void) {
    this.contextLossHandler = handler;
    return () => {
      this.contextLossHandler = () => {};
    };
  }

  destroy() {
    this.destroys += 1;
  }
}

class IntersectionObserverHarness {
  callback: IntersectionObserverCallback | null = null;
  disconnects = 0;

  create = (callback: IntersectionObserverCallback): IntersectionObserver => {
    this.callback = callback;
    return {
      disconnect: () => {
        this.disconnects += 1;
      },
      observe: () => {},
      takeRecords: () => [],
      unobserve: () => {},
      root: null,
      rootMargin: "0px",
      thresholds: [0],
    };
  };

  setVisible(target: Element, visible: boolean) {
    this.callback?.(
      [
        {
          isIntersecting: visible,
          target,
        } as IntersectionObserverEntry,
      ],
      {} as IntersectionObserver,
    );
  }
}

function setDocumentHidden(hidden: boolean) {
  Object.defineProperty(document, "hidden", {
    configurable: true,
    value: hidden,
  });
  document.dispatchEvent(new Event("visibilitychange"));
}

describe("Office scene browser lifecycle", () => {
  it("forwards camera controls to the renderer port", () => {
    setDocumentHidden(false);
    const port = new RuntimePort();
    const runtime = new OfficeSceneRuntime({
      host: document.createElement("div"),
      port,
      onStatus: () => {},
      createIntersectionObserver: null,
    });

    runtime.fit();
    runtime.zoomIn();
    runtime.zoomOut();

    expect(port.fits).toBe(1);
    expect(port.zoomIns).toBe(1);
    expect(port.zoomOuts).toBe(1);
    runtime.destroy();
  });

  it("pauses while hidden or offscreen and resumes from the latest replace state", async () => {
    setDocumentHidden(false);
    const port = new RuntimePort();
    const intersections = new IntersectionObserverHarness();
    const host = document.createElement("div");
    const runtime = new OfficeSceneRuntime({
      host,
      port,
      onStatus: () => {},
      createIntersectionObserver: intersections.create,
    });
    runtime.reconcile(commit());
    await runtime.whenIdle();

    setDocumentHidden(true);
    runtime.reconcile(
      commit({
        mode: "transition",
        effects: [
          { kind: "task-queued", taskId: "hidden-task", agentId: "agent-a", issueId: null },
        ],
      }),
    );
    await runtime.whenIdle();
    setDocumentHidden(false);
    await runtime.whenIdle();
    intersections.setVisible(host, false);
    intersections.setVisible(host, true);
    await runtime.whenIdle();

    expect(port.played).toEqual([]);
    expect(port.pauses).toBe(2);
    expect(port.resumes).toBe(3);
    expect(port.cancels).toBeGreaterThanOrEqual(3);
    runtime.destroy();
  });

  it("rebuilds once from the latest commit, then retires on repeated context loss", async () => {
    setDocumentHidden(false);
    const port = new RuntimePort();
    const statuses = [] as string[];
    const runtime = new OfficeSceneRuntime({
      host: document.createElement("div"),
      port,
      onStatus: (status) => statuses.push(
        status.kind === "fallback" ? `${status.kind}:${status.reason}` : status.kind,
      ),
      createIntersectionObserver: null,
    });
    runtime.reconcile(commit());
    await runtime.whenIdle();

    port.contextLossHandler();
    await runtime.whenIdle();
    expect(port.rebuilds).toBe(1);
    expect(statuses).toContain("recovering");
    expect(statuses.at(-1)).toBe("ready");

    port.contextLossHandler();
    await runtime.whenIdle();
    expect(port.rebuilds).toBe(1);
    expect(port.destroys).toBe(1);
    expect(statuses.at(-1)).toBe("fallback:context");
    runtime.destroy();
    expect(port.destroys).toBe(1);
  });

  it("reports asset and recovery fallback without removing the host DOM", async () => {
    setDocumentHidden(false);
    const host = document.createElement("div");
    host.append(document.createElement("span"));
    const assetPort = new RuntimePort();
    assetPort.rejectInstall = true;
    const assetStatuses = [] as string[];
    const assetRuntime = new OfficeSceneRuntime({
      host,
      port: assetPort,
      onStatus: (status) => assetStatuses.push(
        status.kind === "fallback" ? status.reason : status.kind,
      ),
      createIntersectionObserver: null,
    });
    assetRuntime.reconcile(commit());
    await assetRuntime.whenIdle();
    expect(assetStatuses.at(-1)).toBe("asset");
    expect(host.firstElementChild).not.toBeNull();
    assetRuntime.destroy();

    const recoveryPort = new RuntimePort();
    recoveryPort.rejectRebuild = true;
    const recoveryStatuses = [] as string[];
    const recoveryRuntime = new OfficeSceneRuntime({
      host,
      port: recoveryPort,
      onStatus: (status) => recoveryStatuses.push(
        status.kind === "fallback" ? status.reason : status.kind,
      ),
      createIntersectionObserver: null,
    });
    recoveryRuntime.reconcile(commit());
    await recoveryRuntime.whenIdle();
    recoveryPort.contextLossHandler();
    await recoveryRuntime.whenIdle();
    expect(recoveryStatuses.at(-1)).toBe("context");
    expect(host.firstElementChild).not.toBeNull();
  });

  it("disconnects observers and listeners during idempotent disposal", () => {
    setDocumentHidden(false);
    const port = new RuntimePort();
    const intersections = new IntersectionObserverHarness();
    const runtime = new OfficeSceneRuntime({
      host: document.createElement("div"),
      port,
      onStatus: () => {},
      createIntersectionObserver: intersections.create,
    });
    runtime.destroy();
    runtime.destroy();
    const pauseCount = port.pauses;
    setDocumentHidden(true);

    expect(port.destroys).toBe(1);
    expect(intersections.disconnects).toBe(1);
    expect(port.pauses).toBe(pauseCount);
  });
});
