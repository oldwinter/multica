// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { Agent, AgentRuntime, AgentTask, Issue, Squad } from "../types";
import { OFFICE_LIMITS } from "./types";
import { buildOfficeSnapshot } from "./model";

const NOW = new Date("2026-08-29T12:00:00Z").getTime();

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    workspace_id: "ws-1",
    runtime_id: "runtime-1",
    name: "Agent One",
    description: "",
    instructions: "",
    avatar_url: null,
    runtime_mode: "local",
    runtime_config: {},
    custom_args: [],
    visibility: "workspace",
    permission_mode: "public_to",
    invocation_targets: [{ target_type: "workspace", target_id: null }],
    status: "idle",
    max_concurrent_tasks: 2,
    model: "",
    owner_id: null,
    skills: [],
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

function makeRuntime(overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
    id: "runtime-1",
    workspace_id: "ws-1",
    daemon_id: "daemon-1",
    name: "Runtime One",
    runtime_mode: "local",
    provider: "codex",
    launch_header: "",
    status: "online",
    device_info: "",
    metadata: {},
    owner_id: null,
    visibility: "private",
    last_seen_at: "2026-08-29T11:59:50Z",
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

function makeTask(overrides: Partial<AgentTask> = {}): AgentTask {
  return {
    id: "task-1",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "",
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

function makeSquad(overrides: Partial<Squad> = {}): Squad {
  return {
    id: "squad-1",
    workspace_id: "ws-1",
    name: "Squad One",
    description: "",
    instructions: "",
    avatar_url: null,
    leader_id: "agent-1",
    creator_id: "member-1",
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
    member_count: 2,
    member_preview: [
      { member_type: "agent", member_id: "agent-1", role: "leader" },
      { member_type: "member", member_id: "member-1", role: "operator" },
    ],
    ...overrides,
  };
}

function makeIssue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "Issue One",
    description: null,
    status: "in_progress",
    status_category: "in_progress",
    priority: "medium",
    assignee_type: "squad",
    assignee_id: "squad-1",
    creator_type: "member",
    creator_id: "member-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    stage: null,
    start_date: null,
    due_date: null,
    metadata: {},
    properties: {},
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

function build(overrides: Partial<Parameters<typeof buildOfficeSnapshot>[0]> = {}) {
  return buildOfficeSnapshot({
    nowMs: NOW,
    agents: [makeAgent()],
    runtimes: { kind: "available", value: [makeRuntime()] },
    tasks: { kind: "available", value: [] },
    squads: { kind: "available", value: [] },
    issueBriefs: { kind: "available", value: [] },
    limits: OFFICE_LIMITS,
    ...overrides,
  });
}

describe("buildOfficeSnapshot", () => {
  it("keeps availability and workload independent when one source is missing", () => {
    const missingRuntimes = build({
      runtimes: { kind: "unavailable" },
      tasks: { kind: "available", value: [makeTask({ status: "running" })] },
    });
    expect(missingRuntimes.agents[0]).toMatchObject({
      availability: { kind: "unknown", reason: "unavailable" },
      workload: {
        kind: "known",
        value: "working",
        runningCount: 1,
        queuedCount: 0,
      },
    });

    const missingTasks = build({
      tasks: { kind: "unavailable" },
    });
    expect(missingTasks.agents[0]).toMatchObject({
      availability: { kind: "known", value: "online" },
      workload: { kind: "unknown", reason: "unavailable" },
    });
  });

  it.each([
    ["queued", "queued"],
    ["dispatched", "queued"],
    ["waiting_local_directory", "queued"],
    ["running", "working"],
  ] as const)(
    "maps %s to %s workload and an active Issue",
    (status, workload) => {
      const snapshot = build({
        tasks: {
          kind: "available",
          value: [makeTask({ status, issue_id: "issue-1" })],
        },
        issueBriefs: { kind: "available", value: [makeIssue()] },
      });

      expect(snapshot.agents[0]?.workload).toMatchObject({
        kind: "known",
        value: workload,
        ...(workload === "queued"
          ? { queuedCount: 1, runningCount: 0 }
          : { queuedCount: 0, runningCount: 1 }),
      });
      expect(snapshot.activeIssues).toHaveLength(1);
    },
  );

  it("normalizes the empty Issue sentinel, dedupes links, and post-filters briefs", () => {
    const snapshot = build({
      agents: [makeAgent(), makeAgent({ id: "agent-2", runtime_id: "runtime-2" })],
      runtimes: {
        kind: "available",
        value: [makeRuntime(), makeRuntime({ id: "runtime-2" })],
      },
      tasks: {
        kind: "available",
        value: [
          makeTask({ id: "empty", issue_id: "" }),
          makeTask({ id: "linked-1", issue_id: "issue-1" }),
          makeTask({ id: "linked-2", agent_id: "agent-2", issue_id: "issue-1" }),
        ],
      },
      issueBriefs: {
        kind: "available",
        value: [makeIssue(), makeIssue({ id: "unrequested", identifier: "MUL-999" })],
      },
    });

    expect(snapshot.agents[0]?.activeIssueIds).toEqual(["issue-1"]);
    expect(snapshot.activeIssues).toEqual([
      expect.objectContaining({
        kind: "resolved",
        id: "issue-1",
        assignedSquadId: "squad-1",
        executingAgentIds: ["agent-1", "agent-2"],
      }),
    ]);
  });

  it("excludes archived Agents before stale runtime, task, and Issue rows can contribute", () => {
    const snapshot = build({
      agents: [makeAgent({ archived_at: "2026-08-28T00:00:00Z" })],
      tasks: {
        kind: "available",
        value: [makeTask({ status: "running", issue_id: "issue-1" })],
      },
      issueBriefs: { kind: "available", value: [makeIssue()] },
    });

    expect(snapshot.agents).toEqual([]);
    expect(snapshot.activeIssues).toEqual([]);
  });

  it("preserves one Agent's truth in every Squad preview", () => {
    const snapshot = build({
      squads: {
        kind: "available",
        value: [
          makeSquad({ id: "squad-b", name: "B" }),
          makeSquad({ id: "squad-a", name: "A" }),
        ],
      },
    });

    expect(snapshot.squads.map((squad) => squad.id)).toEqual(["squad-a", "squad-b"]);
    expect(snapshot.squads.every((squad) => squad.memberPreview[0]?.id === "agent-1")).toBe(true);
  });

  it("keeps the full sorted roster while reporting exact deterministic scene overflow", () => {
    const agents = Array.from({ length: 42 }, (_, index) =>
      makeAgent({
        id: `agent-${String(42 - index).padStart(2, "0")}`,
        runtime_id: `runtime-${index}`,
      }),
    );
    const squads = Array.from({ length: 14 }, (_, index) =>
      makeSquad({ id: `squad-${String(14 - index).padStart(2, "0")}` }),
    );
    const tasks = Array.from({ length: 50 }, (_, index) =>
      makeTask({
        id: `task-${index}`,
        agent_id: agents[index % agents.length]?.id ?? "",
        issue_id: `issue-${String(50 - index).padStart(2, "0")}`,
      }),
    );

    const snapshot = build({
      agents,
      runtimes: { kind: "available", value: [] },
      tasks: { kind: "available", value: tasks },
      squads: { kind: "available", value: squads },
      issueBriefs: { kind: "available", value: [] },
    });

    expect(snapshot.agents).toHaveLength(42);
    expect(snapshot.squads).toHaveLength(14);
    expect(snapshot.activeIssues).toHaveLength(50);
    expect(snapshot.overflow).toEqual({ agents: 2, squads: 2, activeIssues: 2 });
    expect(snapshot.agents.slice(0, OFFICE_LIMITS.agents).map((agent) => agent.id)).toEqual(
      [...snapshot.agents.slice(0, OFFICE_LIMITS.agents).map((agent) => agent.id)].sort(),
    );
  });

  it("marks Issue IDs outside the bounded brief request without hiding them", () => {
    const tasks = Array.from({ length: OFFICE_LIMITS.issueBriefs + 1 }, (_, index) =>
      makeTask({ id: `task-${index}`, issue_id: `issue-${String(index).padStart(3, "0")}` }),
    );
    const snapshot = build({
      tasks: { kind: "available", value: tasks },
      issueBriefs: { kind: "available", value: [] },
      limits: { ...OFFICE_LIMITS, activeIssues: 200 },
    });

    expect(snapshot.activeIssues.at(-1)).toMatchObject({
      kind: "unresolved",
      id: "issue-100",
      reason: "brief-limit",
    });
  });

  it("degrades a future task status to unknown workload", () => {
    const futureTask = makeTask();
    Object.defineProperty(futureTask, "status", { value: "pausing" });
    const snapshot = build({
      tasks: { kind: "available", value: [futureTask] },
    });

    expect(snapshot.agents[0]?.workload).toEqual({
      kind: "unknown",
      reason: "unavailable",
      capacity: 2,
    });
  });
});
