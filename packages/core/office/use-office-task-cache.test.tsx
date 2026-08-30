// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { useOfficeTaskCache } from "./use-office-task-cache";

function wrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("useOfficeTaskCache", () => {
  it("projects only safe semantic observations and omits unsupported rows", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify([
            {
              id: "task-queued",
              agent_id: "agent-1",
              issue_id: "issue-1",
              status: "queued",
              result: { private: "result" },
              error: "private error",
              trigger_summary: "private prompt summary",
              work_dir: "/home/private/worktree",
            },
            {
              id: "task-waiting",
              agent_id: "agent-2",
              issue_id: "",
              status: "waiting_local_directory",
            },
            {
              id: "task-running",
              agent_id: "agent-3",
              issue_id: "issue-3",
              status: "running",
            },
            {
              id: "task-completed",
              agent_id: "agent-4",
              issue_id: "issue-4",
              status: "completed",
            },
            {
              id: "task-failed",
              agent_id: "agent-5",
              issue_id: "issue-5",
              status: "failed",
            },
            {
              id: "task-cancelled",
              agent_id: "agent-6",
              issue_id: "issue-6",
              status: "cancelled",
              result: "private cancelled result",
            },
            {
              id: "task-future",
              agent_id: "agent-7",
              issue_id: "issue-7",
              status: "pausing",
              handoff_note: "private future payload",
            },
          ]),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );
    setApiInstance(new ApiClient("https://api.example.test"));
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const { result } = renderHook(() => useOfficeTaskCache("ws-1"), {
      wrapper: wrapper(queryClient),
    });

    await waitFor(() =>
      expect(result.current.observations).toEqual([
        {
          taskId: "task-queued",
          agentId: "agent-1",
          issueId: "issue-1",
          status: "queued-like",
        },
        {
          taskId: "task-waiting",
          agentId: "agent-2",
          issueId: null,
          status: "queued-like",
        },
        {
          taskId: "task-running",
          agentId: "agent-3",
          issueId: "issue-3",
          status: "running",
        },
        {
          taskId: "task-completed",
          agentId: "agent-4",
          issueId: "issue-4",
          status: "completed",
        },
        {
          taskId: "task-failed",
          agentId: "agent-5",
          issueId: "issue-5",
          status: "failed",
        },
      ]),
    );
    expect(JSON.stringify(result.current)).not.toContain("private");
  });
});
