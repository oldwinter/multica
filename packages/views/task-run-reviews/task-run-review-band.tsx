"use client";

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  CheckCircle2,
  Loader2,
  MessageSquareText,
  Send,
  ThumbsDown,
  ThumbsUp,
} from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import type { AgentTask } from "@multica/core/types/agent";
import {
  MAX_TASK_RUN_REVIEW_TEXT_BYTES,
  taskRunReviewInput,
  taskRunReviewSkillOptions,
  useCreateTaskRunReview,
  validateTaskRunReviewDraft,
  type CreateTaskRunReviewInput,
  type TaskRunReviewSkillOption,
  type TaskRunReviewOutcome,
  type TaskRunReviewTarget,
} from "@multica/core/task-run-reviews";
import { createSafeId } from "@multica/core/utils";
import { useT } from "../i18n";

export function TaskRunReviewSlot({
  wsId,
  task,
}: {
  wsId: string;
  task: AgentTask;
}) {
  const terminal = ["completed", "failed", "cancelled"].includes(task.status);
  const mutation = useCreateTaskRunReview(wsId, task.id);
  const skillsQuery = useQuery({
    ...taskRunReviewSkillOptions(wsId, task.agent_id),
    enabled: terminal && !!task.agent_id,
  });
  if (!terminal) return null;
  return (
    <TaskRunReviewBand
      key={task.id}
      task={task}
      onSubmit={mutation.mutateAsync}
      onReset={mutation.reset}
      pending={mutation.isPending}
      submitted={mutation.isSuccess}
      submitError={mutation.isError}
      skills={skillsQuery.data ?? []}
      skillsPending={skillsQuery.isFetching}
      skillsError={skillsQuery.isError}
    />
  );
}

export function TaskRunReviewBand({
  task,
  onSubmit,
  onReset,
  pending = false,
  submitted = false,
  submitError = false,
  skills = [],
  skillsPending = false,
  skillsError = false,
}: {
  task: AgentTask;
  onSubmit?: (input: CreateTaskRunReviewInput) => Promise<unknown>;
  onReset?: () => void;
  pending?: boolean;
  submitted?: boolean;
  submitError?: boolean;
  skills?: readonly TaskRunReviewSkillOption[];
  skillsPending?: boolean;
  skillsError?: boolean;
}) {
  const { t } = useT("agents");
  const terminal = ["completed", "failed", "cancelled"].includes(task.status);
  const [expanded, setExpanded] = useState(false);
  const [outcome, setOutcome] = useState<TaskRunReviewOutcome | null>(null);
  const [target, setTarget] = useState<TaskRunReviewTarget | null>(null);
  const [skillId, setSkillId] = useState("");
  const [correction, setCorrection] = useState("");
  const [reason, setReason] = useState("");
  const [attempted, setAttempted] = useState(false);
  const idempotencyKeyRef = useRef<string | null>(null);
  idempotencyKeyRef.current ??= createSafeId();
  const openButtonRef = useRef<HTMLButtonElement>(null);
  const helpfulButtonRef = useRef<HTMLButtonElement>(null);
  const targetRef = useRef<HTMLSelectElement>(null);
  const skillRef = useRef<HTMLSelectElement>(null);
  const correctionRef = useRef<HTMLTextAreaElement>(null);
  const reasonRef = useRef<HTMLTextAreaElement>(null);
  const successRef = useRef<HTMLDivElement>(null);

  const draft = { outcome, target, skillId, correction, reason };
  const errors = validateTaskRunReviewDraft(draft);
  const reasonBytes = new TextEncoder().encode(reason.trim()).byteLength;
  const correctionBytes = new TextEncoder().encode(correction.trim()).byteLength;
  const skillSelectionUnavailable = target === "skill_procedure" &&
    (skillsPending || skillsError || skills.length === 0);

  useEffect(() => {
    if (expanded) helpfulButtonRef.current?.focus();
  }, [expanded]);

  useEffect(() => {
    if (submitted) successRef.current?.focus();
  }, [submitted]);

  if (!terminal || !onSubmit) return null;

  const clearSubmitError = () => {
    if (submitError) onReset?.();
  };

  const closeForm = () => {
    setExpanded(false);
    setAttempted(false);
    onReset?.();
    requestAnimationFrame(() => openButtonRef.current?.focus());
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setAttempted(true);
    const input = taskRunReviewInput({
      ...draft,
      idempotencyKey: idempotencyKeyRef.current!,
    });
    if (!input) {
      if (errors.outcome) helpfulButtonRef.current?.focus();
      else if (errors.target) targetRef.current?.focus();
      else if (errors.skillId) skillRef.current?.focus();
      else if (errors.correction) correctionRef.current?.focus();
      else if (errors.reason) reasonRef.current?.focus();
      return;
    }
    void onSubmit(input).catch(() => undefined);
  };

  const fieldError = (
    error: "required" | "invalid" | "too_long" | undefined,
  ) => {
    switch (error) {
      case "required":
        return t(($) => $.transcript.review.error_required);
      case "invalid":
        return t(($) => $.transcript.review.error_invalid);
      case "too_long":
        return t(($) => $.transcript.review.error_too_long);
      default:
        return null;
    }
  };

  return (
    <section
      className="shrink-0 border-b bg-muted/20 px-4 py-2.5"
      aria-labelledby="task-run-review-title"
      onKeyDown={(event) => {
        if (event.key === "Escape" && expanded && !pending) {
          event.preventDefault();
          event.stopPropagation();
          closeForm();
        }
      }}
    >
      <div className="flex min-w-0 items-center gap-2">
        <h3 id="task-run-review-title" className="flex items-center gap-1.5 text-body font-medium">
          <MessageSquareText className="size-4 text-brand" />
          {t(($) => $.transcript.review.title)}
        </h3>
        {!expanded && !submitted ? (
          <Button ref={openButtonRef} type="button" size="sm" variant="outline" className="ml-auto" onClick={() => setExpanded(true)}>
            {t(($) => $.transcript.review.open)}
          </Button>
        ) : null}
      </div>

      {submitted ? (
        <div ref={successRef} role="status" tabIndex={-1} className="mt-2 flex items-center gap-1.5 text-caption text-success outline-none">
          <CheckCircle2 className="size-3.5" />
          {t(($) => $.transcript.review.success)}
        </div>
      ) : expanded ? (
        <form className="mt-2 grid gap-2.5" onSubmit={handleSubmit} noValidate>
          <fieldset>
            <legend className="mb-1 text-caption font-medium">{t(($) => $.transcript.review.outcome_label)}</legend>
            <div className="inline-flex rounded-md border bg-background p-0.5">
              <Button ref={helpfulButtonRef} type="button" size="sm" variant={outcome === "helpful" ? "secondary" : "ghost"} aria-pressed={outcome === "helpful"} onClick={() => { clearSubmitError(); setOutcome("helpful"); }}>
                <ThumbsUp className="size-3.5" />
                {t(($) => $.transcript.review.outcome_helpful)}
              </Button>
              <Button type="button" size="sm" variant={outcome === "needs_correction" ? "secondary" : "ghost"} aria-pressed={outcome === "needs_correction"} onClick={() => { clearSubmitError(); setOutcome("needs_correction"); }}>
                <ThumbsDown className="size-3.5" />
                {t(($) => $.transcript.review.outcome_needs_correction)}
              </Button>
            </div>
            {attempted && errors.outcome ? <p className="mt-1 text-caption text-destructive" role="alert">{t(($) => $.transcript.review.outcome_required)}</p> : null}
          </fieldset>

          <div className="grid gap-2 sm:grid-cols-2">
            <label className="grid gap-1 text-caption font-medium">
              {t(($) => $.transcript.review.target_label)}
              <select
                ref={targetRef}
                value={target ?? ""}
                aria-label={t(($) => $.transcript.review.target_label)}
                aria-invalid={attempted && !!errors.target}
                className="h-8 min-w-0 rounded-md border border-input bg-background px-2 text-body outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                onChange={(event) => {
                  clearSubmitError();
                  const next = event.target.value;
                  setTarget(next === "knowledge" || next === "twin_assertion" || next === "skill_procedure" || next === "product_defect" ? next : null);
                }}
              >
                <option value="">{t(($) => $.transcript.review.target_placeholder)}</option>
                <option value="knowledge">{t(($) => $.transcript.review.target_knowledge)}</option>
                <option value="twin_assertion">{t(($) => $.transcript.review.target_twin_assertion)}</option>
                <option value="skill_procedure">{t(($) => $.transcript.review.target_skill_procedure)}</option>
                <option value="product_defect">{t(($) => $.transcript.review.target_product_defect)}</option>
              </select>
              {attempted && errors.target ? <span className="text-destructive" role="alert">{t(($) => $.transcript.review.target_required)}</span> : null}
            </label>

            {target === "skill_procedure" ? (
              <label className="grid gap-1 text-caption font-medium">
                {t(($) => $.transcript.review.skill_label)}
                <select
                  ref={skillRef}
                  value={skillId}
                  disabled={skillsPending || skillsError || skills.length === 0}
                  aria-label={t(($) => $.transcript.review.skill_label)}
                  aria-invalid={attempted && !!errors.skillId}
                  className="h-8 min-w-0 rounded-md border border-input bg-background px-2 text-body outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50"
                  onChange={(event) => { clearSubmitError(); setSkillId(event.target.value); }}
                >
                  <option value="">{skillsPending ? t(($) => $.transcript.review.skill_loading) : t(($) => $.transcript.review.skill_placeholder)}</option>
                  {skills.map((skill) => (
                    <option key={skill.id} value={skill.id}>
                      {skill.name}{skill.assignedToTaskAgent ? ` · ${t(($) => $.transcript.review.skill_assigned)}` : ""}
                    </option>
                  ))}
                </select>
                {skillsError ? (
                  <span className="text-destructive" role="alert">{t(($) => $.transcript.review.skill_load_failed)}</span>
                ) : !skillsPending && skills.length === 0 ? (
                  <span className="text-muted-foreground">{t(($) => $.transcript.review.skill_empty)}</span>
                ) : attempted && errors.skillId ? (
                  <span className="text-destructive" role="alert">{t(($) => $.transcript.review.skill_required)}</span>
                ) : null}
              </label>
            ) : null}
          </div>

          {outcome === "needs_correction" ? (
            <label className="grid gap-1 text-caption font-medium">
              {t(($) => $.transcript.review.correction_label)}
              <Textarea ref={correctionRef} value={correction} rows={2} aria-label={t(($) => $.transcript.review.correction_label)} aria-invalid={attempted && !!errors.correction} placeholder={t(($) => $.transcript.review.correction_placeholder)} className="min-h-16 resize-y" onChange={(event) => { clearSubmitError(); setCorrection(event.target.value); }} />
              <span className="flex justify-between gap-2 font-normal text-muted-foreground">
                <span className="text-destructive" role={attempted && errors.correction ? "alert" : undefined}>{attempted ? fieldError(errors.correction) : null}</span>
                <span>{t(($) => $.transcript.review.byte_count, { count: correctionBytes, max: MAX_TASK_RUN_REVIEW_TEXT_BYTES })}</span>
              </span>
            </label>
          ) : null}

          <label className="grid gap-1 text-caption font-medium">
            {t(($) => $.transcript.review.reason_label)}
            <Textarea ref={reasonRef} value={reason} rows={2} aria-label={t(($) => $.transcript.review.reason_label)} aria-invalid={attempted && !!errors.reason} placeholder={t(($) => $.transcript.review.reason_placeholder)} className="min-h-16 resize-y" onChange={(event) => { clearSubmitError(); setReason(event.target.value); }} />
            <span className="flex justify-between gap-2 font-normal text-muted-foreground">
              <span className="text-destructive" role={attempted && errors.reason ? "alert" : undefined}>{attempted ? fieldError(errors.reason) : null}</span>
              <span>{t(($) => $.transcript.review.byte_count, { count: reasonBytes, max: MAX_TASK_RUN_REVIEW_TEXT_BYTES })}</span>
            </span>
          </label>

          {submitError ? <p className="text-caption text-destructive" role="alert">{t(($) => $.transcript.review.submit_failed)}</p> : null}

          <div className="flex items-center justify-end gap-1.5">
            <Button type="button" size="sm" variant="ghost" disabled={pending} onClick={closeForm}>{t(($) => $.transcript.review.cancel)}</Button>
            <Button type="submit" size="sm" disabled={pending || skillSelectionUnavailable}>
              {pending ? <Loader2 className="size-3.5 animate-spin" /> : <Send className="size-3.5" />}
              {pending ? t(($) => $.transcript.review.submitting) : submitError ? t(($) => $.transcript.review.retry) : t(($) => $.transcript.review.submit)}
            </Button>
          </div>
        </form>
      ) : null}
    </section>
  );
}
