// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { workspaceKeys } from "../workspace/queries";
import type {
  Agent,
  AgentRuntime,
  AgentTask,
  Issue,
  MemberWithUser,
  Squad,
  SquadMemberStatus,
} from "../types";
import type { OfficeSubjectRef } from "./types";
import { useOfficeModel } from "./use-office-model";
import { useOfficeTaskCache } from "./use-office-task-cache";

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

function makeRuntime(): AgentRuntime {
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
    last_seen_at: new Date().toISOString(),
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
  };
}

function makeTask(): AgentTask {
  return {
    id: "task-1",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "issue-1",
    status: "running",
    priority: 0,
    dispatched_at: "2026-08-29T11:00:00Z",
    started_at: "2026-08-29T11:01:00Z",
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-08-29T11:00:00Z",
  };
}

function makeIssue(): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "Issue One",
    description: null,
    status: "in_progress",
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
  };
}

function makeSquad(): Squad {
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
    member_preview: [],
  };
}

function makeMember(): MemberWithUser {
  return {
    id: "member-1",
    workspace_id: "ws-1",
    user_id: "user-1",
    role: "member",
    created_at: "2026-08-01T00:00:00Z",
    name: "Human One",
    email: "human@example.test",
    avatar_url: null,
  };
}

function wrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function mockBaseQueries(options: { issueBriefs?: Issue[] } = {}) {
  const client = new ApiClient("https://api.example.test");
  setApiInstance(client);
  const spies = {
    agents: vi.spyOn(client, "listAgents").mockResolvedValue([makeAgent()]),
    runtimes: vi.spyOn(client, "listRuntimes").mockResolvedValue([makeRuntime()]),
    tasks: vi.spyOn(client, "getAgentTaskSnapshot").mockResolvedValue([makeTask()]),
    squads: vi.spyOn(client, "listSquads").mockResolvedValue([makeSquad()]),
    issues: vi.spyOn(client, "listIssues").mockResolvedValue({
      issues: options.issueBriefs ?? [makeIssue()],
      total: options.issueBriefs?.length ?? 1,
    }),
  };
  return { client, ...spies };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useOfficeModel", () => {
  it("keeps a complete cached snapshot but marks it stale after refresh failure", async () => {
    const spies = mockBaseQueries();
    spies.agents.mockRejectedValueOnce(new Error("refresh failed"));
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(workspaceKeys.agents("ws-1"), [makeAgent()]);

    const { result } = renderHook(
      () => useOfficeModel({ wsId: "ws-1", selected: null }),
      { wrapper: wrapper(queryClient) },
    );

    await waitFor(() => {
      if (result.current.kind !== "ready") return false;
      expect(result.current.snapshot.agents[0]?.availability.kind).toBe("known");
      expect(result.current.snapshot.agents[0]?.workload.kind).toBe("known");
      expect(result.current.snapshot.squads).toHaveLength(1);
      expect(result.current.snapshot.activeIssues[0]?.kind).toBe("resolved");
      expect(result.current.quality.kind).toBe("stale");
      return true;
    });
  });

  it("subscribes to four warm base queries and one deduped Issue batch", async () => {
    const spies = mockBaseQueries();
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(
      () => ({
        model: useOfficeModel({ wsId: "ws-1", selected: null }),
        sceneTasks: useOfficeTaskCache("ws-1"),
      }),
      { wrapper: wrapper(queryClient) },
    );

    await waitFor(() => expect(result.current.model.kind).toBe("ready"));
    expect(result.current.sceneTasks.observations).toEqual([
      {
        taskId: "task-1",
        agentId: "agent-1",
        issueId: "issue-1",
        status: "running",
      },
    ]);
    expect(result.current.sceneTasks.isFetching).toBe(false);
    expect(spies.agents).toHaveBeenCalledTimes(1);
    expect(spies.runtimes).toHaveBeenCalledTimes(1);
    expect(spies.tasks).toHaveBeenCalledTimes(1);
    expect(spies.squads).toHaveBeenCalledTimes(1);
    expect(spies.issues).toHaveBeenCalledTimes(1);
    expect(spies.issues).toHaveBeenCalledWith({
      workspace_id: "ws-1",
      ids: ["issue-1"],
      limit: 1,
    });
  });

  it("uses projection-only Agent selection and selected-Squad-only status", async () => {
    const { client } = mockBaseQueries();
    const status = vi.spyOn(client, "getSquadMemberStatus").mockResolvedValue({
      members: [
        {
          member_type: "agent",
          member_id: "agent-1",
          status: "working",
          active_issues: [],
          last_active_at: null,
        },
      ],
    });
    const members = vi.spyOn(client, "listMembers").mockResolvedValue([makeMember()]);
    const detail = vi.spyOn(client, "getIssue").mockResolvedValue(makeIssue());
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const initialProps: { selected: OfficeSubjectRef | null } = {
      selected: { kind: "agent", id: "agent-1" },
    };
    const { result, rerender } = renderHook(
      ({ selected }) => useOfficeModel({ wsId: "ws-1", selected }),
      { initialProps, wrapper: wrapper(queryClient) },
    );

    await waitFor(() =>
      expect(result.current.kind === "ready" && result.current.inspector.kind).toBe(
        "agent",
      ),
    );
    expect(status).not.toHaveBeenCalled();
    expect(members).not.toHaveBeenCalled();
    expect(detail).not.toHaveBeenCalled();

    rerender({ selected: { kind: "squad", id: "squad-1" } });
    await waitFor(() => expect(status).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(result.current.kind === "ready" && result.current.inspector.kind).toBe(
        "squad",
      ),
    );
    expect(members).not.toHaveBeenCalled();
  });

  it("loads human names only when selected Squad status contains humans", async () => {
    const { client } = mockBaseQueries();
    const futureMemberType: SquadMemberStatus = {
      member_type: "member",
      member_id: "member-1",
      status: null,
      active_issues: [],
      last_active_at: null,
    };
    Object.defineProperty(futureMemberType, "member_type", {
      value: "future-member-type",
    });
    vi.spyOn(client, "getSquadMemberStatus").mockResolvedValue({
      members: [
        {
          member_type: "member",
          member_id: "member-1",
          status: null,
          active_issues: [],
          last_active_at: null,
        },
        futureMemberType,
      ],
    });
    const members = vi.spyOn(client, "listMembers").mockResolvedValue([makeMember()]);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { result } = renderHook(
      () =>
        useOfficeModel({
          wsId: "ws-1",
          selected: { kind: "squad", id: "squad-1" },
        }),
      { wrapper: wrapper(queryClient) },
    );

    await waitFor(() => expect(members).toHaveBeenCalledTimes(1));
    await waitFor(() => {
      if (result.current.kind !== "ready") return false;
      if (result.current.inspector.kind !== "squad") return false;
      return result.current.inspector.members.kind === "ready";
    });
    if (result.current.kind !== "ready") throw new Error("model not ready");
    if (result.current.inspector.kind !== "squad") throw new Error("squad not selected");
    if (result.current.inspector.members.kind !== "ready") throw new Error("members not ready");
    expect(result.current.inspector.members.members[0]?.name).toBe("Human One");
    expect(result.current.inspector.members.members[1]).toMatchObject({
      kind: "unknown",
      name: null,
    });
  });

  it("enables existing Issue detail only for a selected unresolved active Issue", async () => {
    const { client } = mockBaseQueries({ issueBriefs: [] });
    const detail = vi.spyOn(client, "getIssue").mockResolvedValue(makeIssue());
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { result } = renderHook(
      () =>
        useOfficeModel({
          wsId: "ws-1",
          selected: { kind: "issue", id: "issue-1" },
        }),
      { wrapper: wrapper(queryClient) },
    );

    await waitFor(() => expect(detail).toHaveBeenCalledTimes(1));
    await waitFor(() => {
      if (result.current.kind !== "ready") return false;
      return result.current.inspector.kind === "issue";
    });
    expect(detail).toHaveBeenCalledWith("issue-1");
  });

  it("does not start Issue detail while the selected Issue brief is loading", async () => {
    const { client } = mockBaseQueries();
    const detail = vi.spyOn(client, "getIssue").mockResolvedValue(makeIssue());
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { result } = renderHook(
      () =>
        useOfficeModel({
          wsId: "ws-1",
          selected: { kind: "issue", id: "issue-1" },
        }),
      { wrapper: wrapper(queryClient) },
    );

    await waitFor(() => {
      if (result.current.kind !== "ready") return false;
      if (result.current.inspector.kind !== "issue") return false;
      return result.current.inspector.issue.kind === "resolved";
    });
    expect(detail).not.toHaveBeenCalled();
  });
});
