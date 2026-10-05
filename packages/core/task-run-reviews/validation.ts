import type {
  CreateTaskRunReviewInput,
  TaskRunReviewOutcome,
  TaskRunReviewTarget,
} from "./types";

export const MAX_TASK_RUN_REVIEW_TEXT_BYTES = 4096;

function hasDisallowedControlCharacter(value: string): boolean {
  return Array.from(value).some((character) => {
    const codePoint = character.codePointAt(0);
    if (codePoint === undefined) return false;
    if (codePoint >= 0x7f && codePoint <= 0x9f) return true;
    return codePoint <= 0x1f && codePoint !== 0x09 && codePoint !== 0x0a && codePoint !== 0x0d;
  });
}

export interface TaskRunReviewDraft {
  readonly outcome: TaskRunReviewOutcome | null;
  readonly target: TaskRunReviewTarget | null;
  readonly skillId: string;
  readonly correction: string;
  readonly reason: string;
}

export interface TaskRunReviewDraftErrors {
  outcome?: "required";
  target?: "required";
  skillId?: "required";
  correction?: "required" | "invalid" | "too_long";
  reason?: "required" | "invalid" | "too_long";
}

function reviewTextError(
  value: string,
  required: boolean,
): "required" | "invalid" | "too_long" | undefined {
  const trimmed = value.trim();
  if (required && trimmed.length === 0) return "required";
  if (hasDisallowedControlCharacter(trimmed)) return "invalid";
  if (new TextEncoder().encode(trimmed).byteLength > MAX_TASK_RUN_REVIEW_TEXT_BYTES) {
    return "too_long";
  }
  return undefined;
}

export function validateTaskRunReviewDraft(
  draft: TaskRunReviewDraft,
): TaskRunReviewDraftErrors {
  const errors: TaskRunReviewDraftErrors = {};
  if (!draft.outcome) errors.outcome = "required";
  if (!draft.target) errors.target = "required";
  if (draft.target === "skill_procedure" && !draft.skillId) {
    errors.skillId = "required";
  }
  const correctionError = reviewTextError(
    draft.correction,
    draft.outcome === "needs_correction",
  );
  if (correctionError) errors.correction = correctionError;
  const reasonError = reviewTextError(draft.reason, true);
  if (reasonError) errors.reason = reasonError;
  return errors;
}

export function taskRunReviewInput(
  draft: TaskRunReviewDraft & { readonly idempotencyKey: string },
): CreateTaskRunReviewInput | null {
  if (Object.keys(validateTaskRunReviewDraft(draft)).length > 0 || !draft.outcome || !draft.target) {
    return null;
  }
  return {
    outcome: draft.outcome,
    target: draft.target,
    ...(draft.target === "skill_procedure" ? { skillId: draft.skillId } : {}),
    ...(draft.outcome === "needs_correction"
      ? { correction: draft.correction.trim() }
      : {}),
    reason: draft.reason.trim(),
    idempotencyKey: draft.idempotencyKey,
  };
}
