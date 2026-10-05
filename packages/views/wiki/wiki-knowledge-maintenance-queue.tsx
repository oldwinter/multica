"use client";

import { AlertTriangle, ArrowRight, RefreshCw, Wrench } from "lucide-react";
import type {
  WikiKnowledgeMaintenanceItem,
  WikiKnowledgeNextAction,
  WikiKnowledgeReadiness,
  WikiPageSummary,
} from "@multica/core/wiki";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";

export interface WikiKnowledgeMaintenanceQueueProps {
  readiness?: WikiKnowledgeReadiness;
  pages: readonly WikiPageSummary[];
  isLoading: boolean;
  isError: boolean;
  isPending: boolean;
  actionError?: string | null;
  onRetry: () => void;
  onAction: (action: WikiKnowledgeNextAction) => void;
}

export function WikiKnowledgeMaintenanceQueue({
  readiness,
  pages,
  isLoading,
  isError,
  isPending,
  actionError,
  onRetry,
  onAction,
}: WikiKnowledgeMaintenanceQueueProps) {
  const { t } = useT("wiki");
  const pageNames = new Map(pages.map((page) => [page.id, page.title || page.path]));

  return (
    <section className="space-y-3" aria-labelledby="wiki-knowledge-maintenance-title" data-testid="wiki-knowledge-maintenance">
      <header className="space-y-1">
        <h2 id="wiki-knowledge-maintenance-title" className="flex items-center gap-2 text-title-sm font-medium text-foreground">
          <Wrench className="size-4" aria-hidden="true" />
          {t(($) => $.maintenance.title)}
        </h2>
        <p className="max-w-3xl break-words text-body text-muted-foreground">{t(($) => $.maintenance.description)}</p>
      </header>

      {isLoading ? (
        <p className="py-4 text-body text-muted-foreground" role="status">{t(($) => $.maintenance.loading)}</p>
      ) : isError || !readiness ? (
        <div className="flex flex-wrap items-center gap-2 py-3">
          <p className="text-body text-destructive" role="alert">{t(($) => $.maintenance.error)}</p>
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            <RefreshCw data-icon="inline-start" />
            {t(($) => $.actions.retry)}
          </Button>
        </div>
      ) : readiness.maintenanceItems.length === 0 ? (
        <p className="border-y border-surface-border py-4 text-body text-muted-foreground">{t(($) => $.maintenance.empty)}</p>
      ) : (
        <ul className="divide-y divide-surface-border border-y border-surface-border">
          {readiness.maintenanceItems.map((item) => (
            <MaintenanceItem
              key={item.id}
              item={item}
              pageName={item.pageId ? pageNames.get(item.pageId) : undefined}
              canManage={readiness.canManage}
              isPending={isPending}
              onAction={onAction}
            />
          ))}
        </ul>
      )}

      {readiness?.truncated ? (
        <p className="text-caption text-muted-foreground">{t(($) => $.maintenance.truncated)}</p>
      ) : null}
      {actionError ? <p className="text-body text-destructive" role="alert">{actionError}</p> : null}
    </section>
  );
}

function MaintenanceItem({
  item,
  pageName,
  canManage,
  isPending,
  onAction,
}: {
  item: WikiKnowledgeMaintenanceItem;
  pageName?: string;
  canManage: boolean;
  isPending: boolean;
  onAction: (action: WikiKnowledgeNextAction) => void;
}) {
  const { t } = useT("wiki");
  const actionable = item.nextAction.kind !== "none";
  return (
    <li className="flex min-w-0 flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 space-y-1">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <AlertTriangle className="size-4 shrink-0 text-warning" aria-hidden="true" />
          <p className="break-words text-body font-medium text-foreground">{maintenanceKindLabel(item, t)}</p>
          <Badge variant={item.severity === "high" ? "destructive" : "outline"}>
            {item.severity === "high" ? t(($) => $.maintenance.high) : t(($) => $.maintenance.warning)}
          </Badge>
        </div>
        {pageName || item.pageId ? (
          <p className="break-all text-caption text-muted-foreground">{pageName ?? item.pageId}</p>
        ) : null}
        <p className="break-words text-caption text-muted-foreground">
          {t(($) => $.maintenance.policy_version, { version: item.policyVersion })}
        </p>
      </div>
      {actionable ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0"
          disabled={!canManage || isPending}
          onClick={() => onAction(item.nextAction)}
        >
          <ArrowRight data-icon="inline-start" />
          {maintenanceActionLabel(item.nextAction, t)}
        </Button>
      ) : null}
    </li>
  );
}

function maintenanceKindLabel(
  item: WikiKnowledgeMaintenanceItem,
  t: ReturnType<typeof useT<"wiki">>["t"],
): string {
  switch (item.kind) {
    case "source_newer_revision": return t(($) => $.maintenance.kinds.source_newer_revision);
    case "source_deleted": return t(($) => $.maintenance.kinds.source_deleted);
    case "source_excluded": return t(($) => $.maintenance.kinds.source_excluded);
    case "policy_stale": return t(($) => $.maintenance.kinds.policy_stale);
    case "lm_wiki_review_pending": return t(($) => $.maintenance.kinds.lm_wiki_review_pending);
    default: return item.kind;
  }
}

function maintenanceActionLabel(
  action: WikiKnowledgeNextAction,
  t: ReturnType<typeof useT<"wiki">>["t"],
): string {
  switch (action.kind) {
    case "pin_revision": return t(($) => $.maintenance.actions.pin_revision);
    case "remove_source": return t(($) => $.maintenance.actions.remove_source);
    case "refresh_lm_wiki": return t(($) => $.maintenance.actions.refresh_lm_wiki);
    case "review_lm_wiki": return t(($) => $.maintenance.actions.review_lm_wiki);
    default: return t(($) => $.maintenance.actions.none);
  }
}
