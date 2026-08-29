// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { AgentTask } from "../types";
import {
  advanceOfficeContinuity,
  createOfficeContinuityState,
  type OfficeContinuityInput,
} from "./continuity";

function makeTask(overrides: Partial<AgentTask> = {}): AgentTask {
  return {
    id: "task-1",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "issue-1",
    status: "queued",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-08-29T11:00:00Z",
    ...overrides,
  };
}

function input(overrides: Partial<OfficeContinuityInput> = {}): OfficeContinuityInput {
  return {
    workspaceId: "ws-1",
    world: "studio",
    foreground: true,
    stale: false,
    reconnectEpoch: 0,
    recoveryEpoch: 0,
    reducedMotion: false,
    tasks: [],
    ...overrides,
  };
}

describe("Office continuity", () => {
  it("suppresses initial load, then emits queued and started once by exact task ID", () => {
    const initial = advanceOfficeContinuity(
      createOfficeContinuityState(),
      input(),
    );
    expect(initial.commit).toEqual({ mode: "replace", effects: [] });

    const queued = advanceOfficeContinuity(
      initial.state,
      input({ tasks: [makeTask()] }),
    );
    expect(queued.commit).toEqual({
      mode: "transition",
      effects: [
        {
          kind: "task-queued",
          taskId: "task-1",
          agentId: "agent-1",
          issueId: "issue-1",
        },
      ],
    });

    const started = advanceOfficeContinuity(
      queued.state,
      input({ tasks: [makeTask({ status: "running" })] }),
    );
    expect(started.commit.effects).toEqual([
      {
        kind: "task-started",
        taskId: "task-1",
        agentId: "agent-1",
        issueId: "issue-1",
      },
    ]);

    const repeated = advanceOfficeContinuity(
      started.state,
      input({ tasks: [makeTask({ status: "running" })] }),
    );
    expect(repeated.commit.effects).toEqual([]);
  });

  it.each(["completed", "failed"] as const)(
    "emits a proven %s outcome once",
    (status) => {
      const baseline = advanceOfficeContinuity(
        createOfficeContinuityState(),
        input({ tasks: [makeTask({ status: "running" })] }),
      );
      const terminal = advanceOfficeContinuity(
        baseline.state,
        input({ tasks: [makeTask({ status })] }),
      );

      expect(terminal.commit.effects).toEqual([
        expect.objectContaining({
          kind: "task-finished",
          taskId: "task-1",
          outcome: status,
        }),
      ]);
      expect(
        advanceOfficeContinuity(
          terminal.state,
          input({ tasks: [makeTask({ status })] }),
        ).commit.effects,
      ).toEqual([]);
    },
  );

  it("does not invent an outcome when an active task disappears", () => {
    const baseline = advanceOfficeContinuity(
      createOfficeContinuityState(),
      input({ tasks: [makeTask({ status: "running" })] }),
    );
    const missing = advanceOfficeContinuity(baseline.state, input());

    expect(missing.commit).toEqual({ mode: "transition", effects: [] });
  });

  it.each([
    ["workspace", { workspaceId: "ws-2" }],
    ["reconnect", { reconnectEpoch: 1 }],
    ["foreground resume", { foreground: false }],
    ["stale snapshot", { stale: true }],
    ["world switch", { world: "expedition" as const }],
    ["renderer recovery", { recoveryEpoch: 1 }],
    ["reduced motion", { reducedMotion: true }],
  ])("rebases without effects on %s input", (_name, changed) => {
    const baseline = advanceOfficeContinuity(
      createOfficeContinuityState(),
      input({ tasks: [makeTask()] }),
    );
    const rebased = advanceOfficeContinuity(
      baseline.state,
      input({ ...changed, tasks: [makeTask(), makeTask({ id: "task-2" })] }),
    );

    expect(rebased.commit).toEqual({ mode: "replace", effects: [] });
  });

  it("rebases on the first settled snapshot after returning to the foreground", () => {
    const baseline = advanceOfficeContinuity(
      createOfficeContinuityState(),
      input({ tasks: [makeTask()] }),
    );
    const hidden = advanceOfficeContinuity(
      baseline.state,
      input({ foreground: false, tasks: [makeTask()] }),
    );
    const resumed = advanceOfficeContinuity(
      hidden.state,
      input({ tasks: [makeTask(), makeTask({ id: "task-2" })] }),
    );

    expect(resumed.commit).toEqual({ mode: "replace", effects: [] });
  });

  it.each([
    ["stale", { stale: true }],
    ["reduced-motion", { reducedMotion: true }],
  ])("suppresses effects for every %s snapshot", (_name, frozen) => {
    const baseline = advanceOfficeContinuity(
      createOfficeContinuityState(),
      input(),
    );
    const firstFrozen = advanceOfficeContinuity(
      baseline.state,
      input({ ...frozen, tasks: [makeTask()] }),
    );
    const nextFrozen = advanceOfficeContinuity(
      firstFrozen.state,
      input({
        ...frozen,
        tasks: [makeTask(), makeTask({ id: "task-2" })],
      }),
    );

    expect(firstFrozen.commit).toEqual({ mode: "replace", effects: [] });
    expect(nextFrozen.commit).toEqual({ mode: "replace", effects: [] });
  });
});
