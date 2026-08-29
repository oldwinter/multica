export type TaskRunReviewOutcome = "helpful" | "needs_correction";

export type TaskRunReviewTarget =
  | "knowledge"
  | "twin_assertion"
  | "skill_procedure"
  | "product_defect";

export interface CreateTaskRunReviewInput {
  readonly outcome: TaskRunReviewOutcome;
  readonly target: TaskRunReviewTarget;
  readonly skillId?: string;
  readonly correction?: string;
  readonly reason: string;
  readonly idempotencyKey: string;
}

export interface TaskRunReview {
  readonly id: string;
  readonly workspaceId: string;
  readonly taskId: string;
  readonly reviewerId: string;
  readonly outcome: TaskRunReviewOutcome | "unknown";
  readonly target: TaskRunReviewTarget | "unknown";
  readonly skillId: string | null;
  readonly correction: string | null;
  readonly reason: string;
  readonly digest: string;
  readonly createdAt: string;
}

export interface TaskRunReviewSkillOption {
  readonly id: string;
  readonly name: string;
  readonly assignedToTaskAgent: boolean;
}
