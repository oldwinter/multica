"use client";

import {
  AlertTriangle,
  Brain,
  Check,
  Clock3,
  FileText,
  LoaderCircle,
  ShieldCheck,
} from "lucide-react";
import type { TwinOverview, TwinState } from "@multica/core/twins";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@multica/ui/components/ui/empty";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../../navigation";
import type { TwinCopy, TwinViewState } from "./twin-workspace-types";

export function StatePanel({
  state,
  copy,
  onRetry,
  emptyHref,
}: {
  state: Exclude<TwinViewState, "ready">;
  copy: TwinCopy;
  onRetry: () => void;
  emptyHref: string;
}) {
  const isLoading = state === "loading";
  const isError = state === "error";
  const title = isLoading
    ? copy.states.loading
    : isError
      ? copy.states.errorTitle
      : copy.states.emptyTitle;
  const description = isLoading
    ? copy.description
    : isError
      ? copy.states.errorDescription
      : copy.states.emptyDescription;

  if (!isLoading) {
    return (
      <section
        className="flex min-h-72 items-center justify-center rounded-lg border border-dashed border-surface-border bg-surface px-6 py-12 shadow-[var(--surface-shadow)]"
        role={isError ? "alert" : undefined}
      >
        <Empty className="flex-none border-0 p-0">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              {isError ? <AlertTriangle className="text-destructive" /> : <Brain className="text-muted-foreground" />}
            </EmptyMedia>
            <EmptyTitle>{title}</EmptyTitle>
            <EmptyDescription>{description}</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            {isError ? (
              <Button variant="outline" onClick={onRetry}>
                {copy.actions.tryAgain}
              </Button>
            ) : (
              <AppLink href={emptyHref} className={buttonVariants({ variant: "brandSubtle" })}>
                {copy.actions.connectEvidence}
              </AppLink>
            )}
          </EmptyContent>
        </Empty>
      </section>
    );
  }

  return (
    <section
      className="flex min-h-72 items-center justify-center rounded-lg border border-surface-border bg-surface px-6 py-12 shadow-[var(--surface-shadow)]"
      role="status"
      aria-busy="true"
    >
      <div className="flex flex-col items-center gap-3 text-center">
        <LoaderCircle className="size-5 animate-spin text-muted-foreground" aria-hidden="true" />
        <p className="text-body text-muted-foreground">{title}</p>
        <p className="text-caption text-muted-foreground">{description}</p>
      </div>
    </section>
  );
}

const statusKeyFor = (state: TwinState): keyof TwinCopy["status"] => {
  if (state === "pending-signoff") return "pending";
  if (state === "signed-off") return "signedOff";
  return "invalid";
};

export function StatusBanner({
  data,
  copy,
  onReview,
}: {
  data: TwinOverview;
  copy: TwinCopy;
  onReview: () => void;
}) {
  const status = copy.status[statusKeyFor(data.state)];
  const isPending = data.state === "pending-signoff";
  const Icon = data.state === "signed-off" ? ShieldCheck : data.state === "invalid" ? AlertTriangle : Clock3;
  const tone = data.state === "signed-off"
    ? "border-success/30 bg-success/10"
    : data.state === "invalid"
      ? "border-destructive/30 bg-destructive/10"
      : "border-warning/30 bg-warning/10";
  const iconTone = data.state === "signed-off"
    ? "text-success"
    : data.state === "invalid"
      ? "text-destructive"
      : "text-warning";

  return (
    <section className={cn("rounded-lg border p-4 shadow-[var(--surface-shadow)]", tone)} aria-label={status.label}>
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-background/60">
            <Icon className={cn("size-4", iconTone)} aria-hidden="true" />
          </div>
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-label font-medium text-foreground">{status.title}</p>
              <Badge variant="outline" className="bg-background/40">{status.label}</Badge>
            </div>
            <p className="max-w-2xl text-body text-muted-foreground">{status.description}</p>
          </div>
        </div>
        {isPending ? (
          <Button variant="outline" className="self-start md:self-auto" onClick={onReview}>
            <FileText data-icon="inline-start" />
            {copy.actions.reviewProfile}
          </Button>
        ) : null}
      </div>
    </section>
  );
}

export function SummaryMetrics({ data, copy }: { data: TwinOverview; copy: TwinCopy }) {
  const metrics = [
    { label: copy.summary.sources, value: data.sourceCount },
    { label: copy.summary.assertions, value: data.assertionCount },
    { label: copy.summary.skills, value: data.skillCount },
    { label: copy.summary.rules, value: data.ruleCount },
  ];

  return (
    <section aria-label={copy.summary.lastReviewed}>
      <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-surface-border bg-surface-border shadow-[var(--surface-shadow)] sm:grid-cols-4">
        {metrics.map((metric) => (
          <div key={metric.label} className="bg-surface px-4 py-3">
            <dt className="text-caption text-muted-foreground">{metric.label}</dt>
            <dd className="mt-1 text-title-lg font-medium text-foreground">{metric.value}</dd>
          </div>
        ))}
      </dl>
      <p className="mt-2 text-caption text-muted-foreground">
        {copy.summary.lastReviewed}: {data.updatedAt}
      </p>
    </section>
  );
}

export function ReviewPath({ data, copy }: { data: TwinOverview; copy: TwinCopy }) {
  return (
    <section className="rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
      <div className="space-y-1">
        <h2 className="text-title font-medium text-foreground">{copy.review.title}</h2>
        <p className="max-w-xl text-body text-muted-foreground">{copy.review.description}</p>
      </div>
      <ol className="mt-5 space-y-4">
        {data.reviewSteps.map((step, index) => {
          const stepCopy = copy.review.steps[step.id];
          const stepLabel = stepCopy?.label ?? step.id;
          const stepDescription = stepCopy?.description ?? "";
          const isComplete = step.state === "complete";
          const isCurrent = step.state === "current";
          return (
            <li key={step.id} className="relative flex gap-3">
              {index < data.reviewSteps.length - 1 ? (
                <span className="absolute left-3 top-7 h-[calc(100%+0.75rem)] w-px bg-border" aria-hidden="true" />
              ) : null}
              <span
                className={cn(
                  "relative z-[1] flex size-6 shrink-0 items-center justify-center rounded-full border bg-surface text-caption",
                  isComplete && "border-success/40 bg-success/10 text-success",
                  isCurrent && "border-brand/40 bg-brand/10 text-brand",
                  !isComplete && !isCurrent && "border-border text-muted-foreground",
                )}
              >
                {isComplete ? <Check className="size-3.5" aria-hidden="true" /> : <span>{index + 1}</span>}
              </span>
              <div className="min-w-0 flex-1 pt-0.5">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-body font-medium text-foreground">{stepLabel}</p>
                  <span className="text-caption text-muted-foreground">{copy.stateLabels[step.state]}</span>
                </div>
                <p className="text-caption text-muted-foreground">{stepDescription}</p>
              </div>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
