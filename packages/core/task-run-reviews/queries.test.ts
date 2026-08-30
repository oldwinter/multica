// @vitest-environment node

import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance, type ApiClient } from "../api";
import { taskRunReviewSkillOptions } from "./queries";

afterEach(() => vi.restoreAllMocks());

describe("taskRunReviewSkillOptions", () => {
  it("offers only workspace skills and prioritizes skills assigned to the task agent", async () => {
    setApiInstance({
      listSkills: vi.fn().mockResolvedValue([
        { id: "skill-2", name: "Release notes" },
        { id: "skill-1", name: "Compatibility checks" },
      ]),
      listAgentSkills: vi.fn().mockResolvedValue([
        { id: "skill-1", name: "Compatibility checks", enabled: true },
        { id: "foreign-skill", name: "Foreign skill", enabled: true },
      ]),
    } as unknown as ApiClient);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    await expect(client.fetchQuery(
      taskRunReviewSkillOptions("workspace-1", "agent-1"),
    )).resolves.toEqual([
      { id: "skill-1", name: "Compatibility checks", assignedToTaskAgent: true },
      { id: "skill-2", name: "Release notes", assignedToTaskAgent: false },
    ]);
    client.clear();
  });

  it("keeps the authorized workspace list when agent assignments are unavailable", async () => {
    setApiInstance({
      listSkills: vi.fn().mockResolvedValue([
        { id: "skill-1", name: "Compatibility checks" },
      ]),
      listAgentSkills: vi.fn().mockRejectedValue(new Error("unavailable")),
    } as unknown as ApiClient);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    await expect(client.fetchQuery(
      taskRunReviewSkillOptions("workspace-1", "agent-1"),
    )).resolves.toEqual([
      { id: "skill-1", name: "Compatibility checks", assignedToTaskAgent: false },
    ]);
    client.clear();
  });
});
