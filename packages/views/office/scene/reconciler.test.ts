// @vitest-environment node

import { describe, expect, it } from "vitest";
import type {
  OfficeAgent,
  OfficeIssue,
  OfficeSnapshot,
  OfficeSquad,
} from "@multica/core/office";
import type {
  OfficeSceneCommit,
  OfficeScenePlan,
  OfficeScenePort,
} from "./contracts";
import { OfficeSceneController } from "./reconciler";

function makeAgent(id: string): OfficeAgent {
  return {
    id,
    name: `private-name-${id}`,
    avatarUrl: null,
    description: `private-description-${id}`,
    availability: { kind: "known", value: "online" },
    workload: {
      kind: "known",
      value: "idle",
      runningCount: 0,
      queuedCount: 0,
      capacity: 1,
    },
    activeIssueIds: [],
  };
}

function makeSquad(id: string): OfficeSquad {
  return {
    id,
    name: `private-squad-${id}`,
    description: "private-squad-description",
    avatarUrl: null,
    leaderAgentId: "agent-a",
    memberCount: 2,
    memberPreview: [
      { kind: "agent", id: "agent-a", role: "leader" },
      { kind: "agent", id: "agent-b", role: "member" },
    ],
  };
}

function makeIssue(id: string): OfficeIssue {
  return {
    kind: "resolved",
    id,
    identifier: "OFF-1",
    title: "private issue title",
    status: "In Progress",
    statusCategory: "in_progress",
    assignedSquadId: "squad-a",
    executingAgentIds: ["agent-a", "agent-b"],
  };
}

function snapshot(): OfficeSnapshot {
  return {
    agents: [makeAgent("agent-a"), makeAgent("agent-b")],
    squads: [makeSquad("squad-a")],
    activeIssues: [makeIssue("issue-a")],
    overflow: { agents: 3, squads: 2, activeIssues: 1 },
  };
}

function commit(
  overrides: Partial<OfficeSceneCommit> = {},
): OfficeSceneCommit {
  return {
    world: "studio",
    snapshot: snapshot(),
    selected: null,
    selectedSquadAgentIds: [],
    mode: "replace",
    effects: [],
    reducedMotion: false,
    ...overrides,
  };
}

class RecordingPort implements OfficeScenePort {
  readonly installed = [] as string[];
  readonly plans = [] as OfficeScenePlan[];
  readonly effects: string[][] = [];
  cancelCount = 0;
  destroyCount = 0;
  failWorld: string | null = null;

  async installWorld(pack: { readonly id: string }) {
    if (pack.id === this.failWorld) throw new Error("asset load failed");
    this.installed.push(pack.id);
  }

  apply(plan: OfficeScenePlan) {
    this.plans.push(plan);
  }

  cancelEffects() {
    this.cancelCount += 1;
  }

  playEffects(effects: readonly { readonly taskId: string }[]) {
    this.effects.push(effects.map((effect) => effect.taskId));
  }

  pause() {}

  resume() {}

  async rebuild() {}

  onContextLoss() {
    return () => {};
  }

  destroy() {
    this.destroyCount += 1;
  }
}

describe("Office scene reconciliation", () => {
  it("is idempotent by semantic commit and suppresses every replace effect", async () => {
    const port = new RecordingPort();
    const statuses = [] as string[];
    const controller = new OfficeSceneController({
      port,
      onStatus: (status) => statuses.push(status.kind),
    });
    const replacement = commit({
      effects: [
        { kind: "task-queued", taskId: "task-1", agentId: "agent-a", issueId: null },
      ],
    });

    controller.reconcile(replacement);
    await controller.whenIdle();
    controller.reconcile(structuredClone(replacement));
    await controller.whenIdle();

    expect(port.installed).toEqual(["studio"]);
    expect(port.plans).toHaveLength(1);
    expect(port.cancelCount).toBe(1);
    expect(port.effects).toEqual([]);
    expect(statuses).toEqual(["ready"]);
  });

  it("plays only transition effects, caps the pool at 16, and removes motion when reduced", async () => {
    const port = new RecordingPort();
    const controller = new OfficeSceneController({ port, onStatus: () => {} });
    controller.reconcile(commit());
    await controller.whenIdle();
    const effects = Array.from({ length: 20 }, (_, index) => ({
      kind: "task-queued" as const,
      taskId: `task-${index}`,
      agentId: "agent-a",
      issueId: null,
    }));

    controller.reconcile(commit({ mode: "transition", effects }));
    await controller.whenIdle();
    controller.reconcile(
      commit({ mode: "transition", effects, reducedMotion: true }),
    );
    await controller.whenIdle();

    expect(port.effects).toHaveLength(1);
    expect(port.effects[0]).toHaveLength(16);
    expect(port.cancelCount).toBe(2);
  });

  it("highlights exact selected Squad members without moving stations", async () => {
    const port = new RecordingPort();
    const controller = new OfficeSceneController({ port, onStatus: () => {} });
    controller.reconcile(commit());
    await controller.whenIdle();
    const before = port.plans.at(-1);
    controller.reconcile(
      commit({
        mode: "transition",
        selected: { kind: "squad", id: "squad-a" },
        selectedSquadAgentIds: ["agent-b"],
      }),
    );
    await controller.whenIdle();
    const after = port.plans.at(-1);

    const beforeAgents = before?.entities.filter((entity) => entity.kind === "agent");
    const afterAgents = after?.entities.filter((entity) => entity.kind === "agent");
    expect(afterAgents?.find((entity) => entity.id === "agent-b")?.highlighted).toBe(true);
    expect(afterAgents?.find((entity) => entity.id === "agent-a")?.highlighted).toBe(false);
    expect(afterAgents?.map(({ id, anchor }) => ({ id, anchor }))).toEqual(
      beforeAgents?.map(({ id, anchor }) => ({ id, anchor })),
    );
  });

  it("models one Issue with Agent links and its authoritative Squad assignment", async () => {
    const port = new RecordingPort();
    const controller = new OfficeSceneController({ port, onStatus: () => {} });
    controller.reconcile(
      commit({ selected: { kind: "issue", id: "issue-a" } }),
    );
    await controller.whenIdle();
    const plan = port.plans.at(-1);

    expect(plan?.entities.filter((entity) => entity.kind === "issue")).toHaveLength(1);
    expect(plan?.links).toEqual(
      expect.arrayContaining([
        { from: "issue:issue-a", to: "agent:agent-a", kind: "execution" },
        { from: "issue:issue-a", to: "agent:agent-b", kind: "execution" },
        { from: "issue:issue-a", to: "squad:squad-a", kind: "assignment" },
      ]),
    );
    expect(JSON.stringify(plan)).not.toContain("private-");
  });

  it("loads a new pack before switching, keeps the old pack on failure, and disposes once", async () => {
    const port = new RecordingPort();
    const statuses = [] as string[];
    const controller = new OfficeSceneController({
      port,
      onStatus: (status) => statuses.push(status.kind),
    });
    controller.reconcile(commit());
    await controller.whenIdle();
    controller.reconcile(commit({ world: "expedition", mode: "transition" }));
    await controller.whenIdle();
    const expeditionPlan = port.plans.at(-1);
    port.failWorld = "studio";
    controller.reconcile(commit({ world: "studio", mode: "transition" }));
    await controller.whenIdle();

    expect(port.installed).toEqual(["studio", "expedition"]);
    expect(port.plans.at(-1)).toBe(expeditionPlan);
    expect(controller.currentWorld).toBe("expedition");
    expect(statuses.at(-1)).toBe("ready");

    controller.destroy();
    controller.destroy();
    expect(port.destroyCount).toBe(1);
  });
});
