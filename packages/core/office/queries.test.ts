// @vitest-environment node

import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import type { AgentTask, Issue } from "../types";
import {
  collectOfficeIssueBriefIds,
  officeIssueBriefsOptions,
  officeKeys,
} from "./queries";

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

function makeIssue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "Issue One",
    description: null,
    status: "in_progress",
    priority: "medium",
    assignee_type: null,
    assignee_id: null,
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

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Office Issue brief query", () => {
  it("sorts, dedupes, drops sentinels and terminal tasks, and applies the bound", () => {
    const tasks = [
      makeTask({ id: "z", issue_id: "issue-z" }),
      makeTask({ id: "empty", issue_id: "" }),
      makeTask({ id: "dup", issue_id: "issue-z", status: "running" }),
      makeTask({ id: "done", issue_id: "issue-a", status: "completed" }),
      makeTask({ id: "a", issue_id: "issue-a", status: "dispatched" }),
      makeTask({ id: "b", issue_id: "issue-b", status: "waiting_local_directory" }),
    ];

    expect(collectOfficeIssueBriefIds(tasks, 2)).toEqual(["issue-a", "issue-b"]);
    expect(officeKeys.issueBriefs("ws-1", ["issue-z", "issue-a", "issue-a"])).toEqual([
      "workspaces",
      "ws-1",
      "office",
      "issue-briefs",
      ["issue-a", "issue-z"],
    ]);
  });

  it("does not request when the ID set is empty", async () => {
    const client = new ApiClient("https://api.example.test");
    setApiInstance(client);
    const listIssues = vi.spyOn(client, "listIssues");
    const options = officeIssueBriefsOptions("ws-1", []);
    const queryClient = new QueryClient();

    expect(options.enabled).toBe(false);
    await expect(queryClient.fetchQuery(options)).resolves.toEqual([]);
    expect(listIssues).not.toHaveBeenCalled();
  });

  it("uses one bounded ids POST query and post-filters unrelated returned rows", async () => {
    const unrelated = makeIssue({ id: "issue-other", identifier: "MUL-99" });
    const requested = makeIssue({ id: "issue-a", identifier: "MUL-2" });
    const client = new ApiClient("https://api.example.test");
    setApiInstance(client);
    const listIssues = vi.spyOn(client, "listIssues").mockResolvedValue({
      issues: [unrelated, requested],
      total: 2,
    });
    const queryClient = new QueryClient();

    const result = await queryClient.fetchQuery(
      officeIssueBriefsOptions("ws-1", ["issue-z", "issue-a", "issue-a"]),
    );

    expect(listIssues).toHaveBeenCalledTimes(1);
    expect(listIssues).toHaveBeenCalledWith({
      workspace_id: "ws-1",
      ids: ["issue-a", "issue-z"],
      limit: 2,
    });
    expect(result).toEqual([requested]);
  });
});
