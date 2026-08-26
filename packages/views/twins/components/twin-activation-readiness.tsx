"use client";

import { useQuery } from "@tanstack/react-query";
import { AlertCircle, ArrowRight, CheckCircle2, Circle, Route } from "lucide-react";
import {
  twinActivationReadinessOptions,
  type TwinActivationActionKey,
  type TwinActivationBlocker,
  type TwinActivationInspectionLink,
  type TwinActivationStage,
} from "@multica/core/twins";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";

export type TwinWorkspaceTab = "wiki" | "twin" | "use";

export function TwinActivationReadiness({
  wsId,
  onNavigate,
}: {
  wsId: string;
  onNavigate: (target: TwinWorkspaceTab) => void;
}) {
  const { t } = useT("twins");
  const readiness = useQuery(twinActivationReadinessOptions(wsId));

  if (readiness.isPending) {
    return <Skeleton className="h-28 w-full" aria-label={t(($) => $.use.activation_loading)} />;
  }
  if (!readiness.data) return null;

  const action = readiness.data.nextAction;
  return (
    <section className="border-y border-border/70 py-4" aria-labelledby="twin-next-action-title" data-testid="twin-activation-readiness">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <Route className="size-4 text-muted-foreground" aria-hidden="true" />
            <h2 id="twin-next-action-title" className="text-title font-medium text-foreground">{t(($) => $.use.next_action_title)}</h2>
            <Badge variant={readiness.data.ready ? "default" : "outline"}>
              {readiness.data.ready ? t(($) => $.use.activation_ready) : t(($) => $.use.activation_in_progress)}
            </Badge>
          </div>
          <p className="text-body text-muted-foreground">{activationReason(action.key, t)}</p>
          <p className="text-caption text-muted-foreground">
            {action.responsibleRole === "owner_admin" ? t(($) => $.use.role_owner_admin) : t(($) => $.use.role_member)}
          </p>
          {readiness.data.blockers.length > 0 ? (
            <ul className="space-y-1" aria-label={t(($) => $.use.activation_blockers)}>
              {readiness.data.blockers.map((blocker, index) => (
                <li key={`${blocker.kind}:${index}`} className="flex items-center gap-1.5 text-caption text-muted-foreground">
                  <AlertCircle className="size-3.5 shrink-0" aria-hidden="true" />
                  <span>{activationBlockerLabel(blocker, t)}</span>
                </li>
              ))}
            </ul>
          ) : null}
          <ol className="flex flex-wrap gap-x-4 gap-y-1" aria-label={t(($) => $.use.activation_stages)}>
            {readiness.data.stages.map((stage) => (
              <li key={stage.key} className="flex items-center gap-1.5 text-caption text-muted-foreground">
                {stage.complete
                  ? <CheckCircle2 className="size-3.5 text-success" aria-hidden="true" />
                  : <Circle className="size-3.5" aria-hidden="true" />}
                <span>{stageLabel(stage, t)}</span>
              </li>
            ))}
          </ol>
          <nav className="flex flex-wrap gap-1" aria-label={t(($) => $.use.inspection_links)}>
            {readiness.data.inspectionLinks.filter((link) => link.target !== action.target).map((link) => (
              <Button key={link.key} variant="ghost" size="sm" className="h-7 px-2" onClick={() => onNavigate(link.target)}>
                {inspectionLinkLabel(link, t)}
              </Button>
            ))}
          </nav>
        </div>
        <Button
          data-testid="twin-primary-action"
          className="shrink-0 self-start lg:self-center"
          disabled={!action.canAct}
          onClick={() => onNavigate(action.target)}
        >
          {activationActionLabel(action.key, t)}
          <ArrowRight data-icon="inline-end" aria-hidden="true" />
        </Button>
      </div>
    </section>
  );
}

function inspectionLinkLabel(link: TwinActivationInspectionLink, t: Translate): string {
  switch (link.key) {
    case "evidence_history": return t(($) => $.use.inspect_evidence);
    case "twin_history": return t(($) => $.use.inspect_twin);
    case "execution_evidence": return t(($) => $.use.inspect_execution);
  }
}

type Translate = ReturnType<typeof useT<"twins">>["t"];

function activationBlockerLabel(blocker: TwinActivationBlocker, t: Translate): string {
  switch (blocker.kind) {
    case "kill_switch": return t(($) => $.use.blocker_kill_switch);
    case "missing_capability": return t(($) => $.use.blocker_missing_capability);
    case "missing_state": return t(($) => $.use.blocker_missing_state);
    case "stale_version": return t(($) => $.use.blocker_stale_version);
    case "review_gate": return t(($) => $.use.blocker_review_gate);
    case "exclusion": return t(($) => $.use.blocker_exclusion);
  }
}

function activationActionLabel(key: TwinActivationActionKey, t: Translate): string {
  switch (key) {
    case "inspect_disabled": return t(($) => $.use.action_inspect_disabled);
    case "configure_source": return t(($) => $.use.action_configure_source);
    case "review_evidence": return t(($) => $.use.action_review_evidence);
    case "refresh_evidence": return t(($) => $.use.action_refresh_evidence);
    case "review_twin": return t(($) => $.use.action_review_twin);
    case "generate_twin": return t(($) => $.use.action_generate_twin);
    case "compile_preview": return t(($) => $.use.action_compile_preview);
    case "configure_binding": return t(($) => $.use.action_configure_binding);
    case "run_with_twin": return t(($) => $.use.action_run_with_twin);
    case "review_run": return t(($) => $.use.action_review_run);
    case "review_deposition": return t(($) => $.use.action_review_deposition);
    case "monitor_effectiveness": return t(($) => $.use.action_monitor_effectiveness);
  }
}

function activationReason(key: TwinActivationActionKey, t: Translate): string {
  switch (key) {
    case "inspect_disabled": return t(($) => $.use.reason_inspect_disabled);
    case "configure_source": return t(($) => $.use.reason_configure_source);
    case "review_evidence": return t(($) => $.use.reason_review_evidence);
    case "refresh_evidence": return t(($) => $.use.reason_refresh_evidence);
    case "review_twin": return t(($) => $.use.reason_review_twin);
    case "generate_twin": return t(($) => $.use.reason_generate_twin);
    case "compile_preview": return t(($) => $.use.reason_compile_preview);
    case "configure_binding": return t(($) => $.use.reason_configure_binding);
    case "run_with_twin": return t(($) => $.use.reason_run_with_twin);
    case "review_run": return t(($) => $.use.reason_review_run);
    case "review_deposition": return t(($) => $.use.reason_review_deposition);
    case "monitor_effectiveness": return t(($) => $.use.reason_monitor_effectiveness);
  }
}

function stageLabel(stage: TwinActivationStage, t: Translate): string {
  switch (stage.key) {
    case "source_policy": return t(($) => $.use.stage_source_policy);
    case "evidence": return t(($) => $.use.stage_evidence);
    case "signed_twin": return t(($) => $.use.stage_signed_twin);
    case "preview": return t(($) => $.use.stage_preview);
    case "binding": return t(($) => $.use.stage_binding);
    case "attributed_run": return t(($) => $.use.stage_run);
    case "feedback": return t(($) => $.use.stage_feedback);
    case "deposition": return t(($) => $.use.stage_deposition);
  }
}
