// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  MAX_TASK_RUN_REVIEW_TEXT_BYTES,
  taskRunReviewInput,
  validateTaskRunReviewDraft,
} from "./validation";

describe("task run review draft validation", () => {
  it("requires correction and an existing skill selection for their matching choices", () => {
    expect(validateTaskRunReviewDraft({
      outcome: "needs_correction",
      target: "skill_procedure",
      skillId: "",
      correction: "",
      reason: "The task skipped the documented validation step.",
    })).toEqual({ skillId: "required", correction: "required" });
  });

  it("enforces the service's UTF-8 byte limit", () => {
    const oversized = "界".repeat(Math.floor(MAX_TASK_RUN_REVIEW_TEXT_BYTES / 3) + 1);
    expect(validateTaskRunReviewDraft({
      outcome: "helpful",
      target: "knowledge",
      skillId: "",
      correction: "",
      reason: oversized,
    }).reason).toBe("too_long");
  });

  it("rejects control characters while allowing ordinary multiline text", () => {
    expect(validateTaskRunReviewDraft({
      outcome: "helpful",
      target: "knowledge",
      skillId: "",
      correction: "",
      reason: "Evidence\u0085hidden content",
    }).reason).toBe("invalid");
    expect(validateTaskRunReviewDraft({
      outcome: "helpful",
      target: "knowledge",
      skillId: "",
      correction: "",
      reason: "First observation\nSecond observation",
    }).reason).toBeUndefined();
  });

  it("trims values and emits only fields valid for the selected target and outcome", () => {
    expect(taskRunReviewInput({
      outcome: "helpful",
      target: "product_defect",
      skillId: "unrelated-skill",
      correction: "not applicable",
      reason: "  The run exposed a reproducible crash in the product.  ",
      idempotencyKey: "review-key-1",
    })).toEqual({
      outcome: "helpful",
      target: "product_defect",
      reason: "The run exposed a reproducible crash in the product.",
      idempotencyKey: "review-key-1",
    });
  });
});
