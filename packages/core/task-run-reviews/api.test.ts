// @vitest-environment node

import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";

const responseReview = {
  id: "review-1",
  workspace_id: "workspace-1",
  task_id: "task-1",
  reviewer_id: "user-1",
  outcome: "needs_correction",
  target: "skill_procedure",
  skill_id: "skill-1",
  correction: "Keep the fallback branch intact.",
  reason: "The change removed a required compatibility path.",
  digest: "a".repeat(64),
  created_at: "2026-08-29T10:00:00Z",
};

function jsonResponse(value: unknown, status = 201): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("Task run review ApiClient contract", () => {
  it("posts the exact Go JSON tags and parses the response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(responseReview));
    vi.stubGlobal("fetch", fetchMock);

    const review = await new ApiClient("https://api.example.test")
      .createTaskRunReview("task/1", {
        outcome: "needs_correction",
        target: "skill_procedure",
        skillId: "skill-1",
        correction: "Keep the fallback branch intact.",
        reason: "The change removed a required compatibility path.",
        idempotencyKey: "review-key-1",
      });

    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.example.test/api/tasks/task%2F1/review",
      expect.objectContaining({ method: "POST" }),
    );
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      outcome: "needs_correction",
      target: "skill_procedure",
      skill_id: "skill-1",
      correction: "Keep the fallback branch intact.",
      reason: "The change removed a required compatibility path.",
      idempotency_key: "review-key-1",
    });
    expect(review).toMatchObject({
      id: "review-1",
      taskId: "task-1",
    });
  });

  it("omits target-specific fields for helpful non-skill feedback", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      ...responseReview,
      outcome: "helpful",
      target: "knowledge",
      skill_id: undefined,
      correction: undefined,
    }));
    vi.stubGlobal("fetch", fetchMock);

    await new ApiClient("https://api.example.test").createTaskRunReview("task-1", {
      outcome: "helpful",
      target: "knowledge",
      reason: "The cited source made the answer verifiable.",
      idempotencyKey: "review-key-2",
    });

    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      outcome: "helpful",
      target: "knowledge",
      reason: "The cited source made the answer verifiable.",
      idempotency_key: "review-key-2",
    });
  });

  it("rejects a malformed success response instead of reporting a false success", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ id: "review-1" })));

    await expect(new ApiClient("https://api.example.test").createTaskRunReview("task-1", {
      outcome: "helpful",
      target: "knowledge",
      reason: "The cited source made the answer verifiable.",
      idempotencyKey: "review-key-2",
    })).rejects.toThrow("Invalid task run review response");
  });
});
