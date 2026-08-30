// @vitest-environment node

import { describe, expect, it } from "vitest";
import { TaskRunReviewSchema } from "./schemas";

const wireReview = {
  id: "review-1",
  workspace_id: "workspace-1",
  task_id: "task-1",
  reviewer_id: "user-1",
  outcome: "needs_correction",
  target: "skill_procedure",
  skill_id: "skill-1",
  correction: "Keep the fallback branch intact.",
  reason: "The change removed the compatibility path used by Desktop.",
  digest: "a".repeat(64),
  created_at: "2026-08-29T10:00:00Z",
};

describe("TaskRunReviewSchema", () => {
  it("parses the strict wire response into camelCase", () => {
    expect(TaskRunReviewSchema.parse(wireReview)).toEqual({
      id: "review-1",
      workspaceId: "workspace-1",
      taskId: "task-1",
      reviewerId: "user-1",
      outcome: "needs_correction",
      target: "skill_procedure",
      skillId: "skill-1",
      correction: "Keep the fallback branch intact.",
      reason: "The change removed the compatibility path used by Desktop.",
      digest: "a".repeat(64),
      createdAt: "2026-08-29T10:00:00Z",
    });
  });

  it("keeps future enum values compatible without weakening required fields", () => {
    expect(TaskRunReviewSchema.parse({
      ...wireReview,
      outcome: "partially_helpful",
      target: "runtime_policy",
      skill_id: null,
      correction: null,
    })).toMatchObject({
      outcome: "unknown",
      target: "unknown",
      skillId: null,
      correction: null,
    });

    expect(TaskRunReviewSchema.safeParse({
      ...wireReview,
      digest: undefined,
    }).success).toBe(false);
  });
});
