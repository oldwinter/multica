"use client";

import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import type { EvolutionStatusTone } from "./status";

function useStatusLabel(value: string): string {
  const { t } = useT("skill-evolution");
  switch (value) {
    case "observe":
      return t(($) => $.status.observe);
    case "propose":
      return t(($) => $.status.propose);
    case "paused":
      return t(($) => $.status.paused);
    case "disabled":
      return t(($) => $.status.disabled);
    case "queued":
      return t(($) => $.status.queued);
    case "running":
      return t(($) => $.status.running);
    case "ready":
      return t(($) => $.status.ready);
    case "failed":
      return t(($) => $.status.failed);
    case "stale":
      return t(($) => $.status.stale);
    case "rejected":
      return t(($) => $.status.rejected);
    case "publishing":
      return t(($) => $.status.publishing);
    case "published":
      return t(($) => $.status.published);
    case "publication_unknown":
      return t(($) => $.status.publication_unknown);
    case "pending":
      return t(($) => $.status.pending);
    case "succeeded":
      return t(($) => $.status.succeeded);
    case "passed":
      return t(($) => $.status.passed);
    case "reject":
      return t(($) => $.status.reject);
    case "publish":
      return t(($) => $.status.publish);
    case "rollback":
      return t(($) => $.status.rollback);
    case "workspace":
      return t(($) => $.status.workspace);
    case "builtin":
      return t(($) => $.status.builtin);
    case "plugin":
      return t(($) => $.status.plugin);
    case "runtime_local":
      return t(($) => $.status.runtime_local);
    case "external":
      return t(($) => $.status.external);
    default:
      return t(($) => $.status.unknown);
  }
}

export function EvolutionStatusBadge({
  value,
  tone = "neutral",
}: {
  value: string;
  tone?: EvolutionStatusTone;
}) {
  const label = useStatusLabel(value);
  return (
    <span
      data-evolution-status={value}
      className={cn(
        "inline-flex h-5 max-w-full items-center rounded-full border px-2 text-caption font-medium",
        tone === "neutral" && "border-border bg-muted/50 text-muted-foreground",
        tone === "info" && "border-brand/25 bg-brand/8 text-foreground",
        tone === "success" && "border-success/25 bg-success/10 text-success",
        tone === "warning" && "border-warning/35 bg-warning/10 text-warning-foreground",
        tone === "danger" && "border-destructive/25 bg-destructive/10 text-destructive",
      )}
      title={value}
    >
      <span className="truncate">{label}</span>
    </span>
  );
}
