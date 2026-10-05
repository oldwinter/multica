import { z } from "zod";
import type {
  TaskRunReview,
  TaskRunReviewOutcome,
  TaskRunReviewTarget,
} from "./types";

function compatibleEnum<T extends string>(knownValues: readonly T[]) {
  const known = new Set<string>(knownValues);
  return z.string().transform((value): T | "unknown" =>
    known.has(value) ? value as T : "unknown",
  );
}

const TaskRunReviewWireSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  task_id: z.string(),
  reviewer_id: z.string(),
  outcome: compatibleEnum<TaskRunReviewOutcome>(["helpful", "needs_correction"]),
  target: compatibleEnum<TaskRunReviewTarget>([
    "knowledge",
    "twin_assertion",
    "skill_procedure",
    "product_defect",
  ]),
  skill_id: z.string().nullish().transform((value) => value ?? null),
  correction: z.string().nullish().transform((value) => value ?? null),
  reason: z.string(),
  digest: z.string(),
  created_at: z.string(),
}).loose();

export const TaskRunReviewSchema = TaskRunReviewWireSchema.transform(
  (wire): TaskRunReview => ({
    id: wire.id,
    workspaceId: wire.workspace_id,
    taskId: wire.task_id,
    reviewerId: wire.reviewer_id,
    outcome: wire.outcome,
    target: wire.target,
    skillId: wire.skill_id,
    correction: wire.correction,
    reason: wire.reason,
    digest: wire.digest,
    createdAt: wire.created_at,
  }),
);
