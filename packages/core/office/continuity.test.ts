// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  advanceOfficeContinuity,
  createOfficeContinuityState,
  type OfficeContinuityInput,
  type OfficeTaskObservation,
} from "./continuity";

function makeTask(
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

function input(overrides: Partial<OfficeContinuityInput> = {}): OfficeContinuityInput {
  return {
    workspaceId: "ws-1",
    world: "studio",
    foreground: true,
    stale: false,
    reconnectEpoch: 0,
    recoveryEpoch: 0,
    reducedMotion: false,
    observations: [],
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
      input({ observations: [makeTask()] }),
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
      input({ observations: [makeTask({ status: "running" })] }),
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
      input({ observations: [makeTask({ status: "running" })] }),
    );
    expect(repeated.commit.effects).toEqual([]);
  });

  it.each(["completed", "failed"] as const)(
    "emits a proven %s outcome once",
    (status) => {
      const baseline = advanceOfficeContinuity(
        createOfficeContinuityState(),
        input({ observations: [makeTask({ status: "running" })] }),
      );
      const terminal = advanceOfficeContinuity(
        baseline.state,
        input({ observations: [makeTask({ status })] }),
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
          input({ observations: [makeTask({ status })] }),
        ).commit.effects,
      ).toEqual([]);
    },
  );

  it("does not invent an outcome when an active task disappears", () => {
    const baseline = advanceOfficeContinuity(
      createOfficeContinuityState(),
      input({ observations: [makeTask({ status: "running" })] }),
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
      input({ observations: [makeTask()] }),
    );
    const rebased = advanceOfficeContinuity(
      baseline.state,
      input({
        ...changed,
        observations: [makeTask(), makeTask({ taskId: "task-2" })],
      }),
    );

    expect(rebased.commit).toEqual({ mode: "replace", effects: [] });
  });

  it("rebases on the first settled snapshot after returning to the foreground", () => {
    const baseline = advanceOfficeContinuity(
      createOfficeContinuityState(),
      input({ observations: [makeTask()] }),
    );
    const hidden = advanceOfficeContinuity(
      baseline.state,
      input({ foreground: false, observations: [makeTask()] }),
    );
    const resumed = advanceOfficeContinuity(
      hidden.state,
      input({
        observations: [makeTask(), makeTask({ taskId: "task-2" })],
      }),
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
      input({ ...frozen, observations: [makeTask()] }),
    );
    const nextFrozen = advanceOfficeContinuity(
      firstFrozen.state,
      input({
        ...frozen,
        observations: [makeTask(), makeTask({ taskId: "task-2" })],
      }),
    );

    expect(firstFrozen.commit).toEqual({ mode: "replace", effects: [] });
    expect(nextFrozen.commit).toEqual({ mode: "replace", effects: [] });
  });
});
