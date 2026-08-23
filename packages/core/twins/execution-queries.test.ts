// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  twinBindingsOptions,
  twinExecutionKeys,
  twinExecutionMetricsOptions,
  twinTaskContextOptions,
} from "./execution-queries";

describe("Twin execution query keys", () => {
  it("isolates every server-state surface by workspace", () => {
    expect(twinBindingsOptions("workspace-a").queryKey).toEqual([
      "workspaces", "workspace-a", "twin-execution", "bindings",
    ]);
    expect(twinExecutionMetricsOptions("workspace-b").queryKey).toEqual([
      "workspaces", "workspace-b", "twin-execution", "metrics",
    ]);
    expect(twinTaskContextOptions("workspace-a", "task-1").queryKey).toEqual([
      "workspaces", "workspace-a", "twin-execution", "tasks", "task-1", "context",
    ]);
    expect(twinExecutionKeys.all("workspace-a")).not.toEqual(twinExecutionKeys.all("workspace-b"));
  });

  it("does not fetch task context without both workspace and task identity", () => {
    expect(twinTaskContextOptions("", "task-1").enabled).toBe(false);
    expect(twinTaskContextOptions("workspace-a", "").enabled).toBe(false);
  });
});
