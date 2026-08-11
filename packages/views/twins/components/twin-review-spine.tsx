"use client";

import { Check, Circle, LoaderCircle } from "lucide-react";
import type { TwinReviewStep, TwinReviewStepId, TwinReviewStepState } from "@multica/core/twins";
import { Badge } from "@multica/ui/components/ui/badge";
import { useT } from "../../i18n";

function stepIcon(state: TwinReviewStepState) {
  if (state === "complete") return Check;
  if (state === "current") return LoaderCircle;
  return Circle;
}

export function TwinReviewSpine({ steps }: { steps: readonly TwinReviewStep[] }) {
  const { t } = useT("twins");
  if (steps.length === 0) return null;

  function stepLabel(id: TwinReviewStepId): string {
    switch (id) {
      case "import": return t(($) => $.review.steps.import);
      case "generate": return t(($) => $.review.steps.generate);
      case "topic": return t(($) => $.review.steps.topic);
      case "coordinate": return t(($) => $.review.steps.coordinate);
      case "accept": return t(($) => $.review.steps.accept);
      case "deposition": return t(($) => $.review.steps.deposition);
    }
  }

  function stateLabel(state: TwinReviewStepState): string {
    switch (state) {
      case "complete": return t(($) => $.review.states.complete);
      case "current": return t(($) => $.review.states.current);
      case "upcoming": return t(($) => $.review.states.upcoming);
    }
  }

  return (
    <section
      className="space-y-3 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]"
      aria-label={t(($) => $.review.title)}
    >
      <div>
        <h2 className="text-title font-medium text-foreground">{t(($) => $.review.title)}</h2>
        <p className="mt-1 text-body text-muted-foreground">{t(($) => $.review.description)}</p>
      </div>
      <ol className="grid overflow-hidden rounded-md border border-border sm:grid-cols-2 lg:grid-cols-3">
        {steps.map((step) => {
          const Icon = stepIcon(step.state);
          return (
            <li
              key={step.id}
              className="flex min-w-0 items-center gap-3 border-b border-border p-3 last:border-b-0 sm:border-r lg:[&:nth-child(3n)]:border-r-0"
              data-state={step.state}
              data-testid="twin-review-step"
            >
              <Icon
                className={step.state === "current" ? "size-4 shrink-0 text-brand motion-reduce:animate-none" : "size-4 shrink-0 text-muted-foreground"}
                aria-hidden="true"
              />
              <span className="min-w-0 flex-1 text-body font-medium text-foreground">{stepLabel(step.id)}</span>
              <Badge variant="outline" className="shrink-0">{stateLabel(step.state)}</Badge>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
