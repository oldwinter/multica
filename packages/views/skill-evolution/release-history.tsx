"use client";

import type { SkillEvolutionOverview } from "@multica/core/skill-evolution";
import { GitCommitHorizontal, History, RotateCcw } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { releaseStatusTone } from "./status";
import { EvolutionStatusBadge } from "./status-badge";

type Release = SkillEvolutionOverview["releases"][number];
type Revision = SkillEvolutionOverview["revisions"][number];

function formatDate(value: string | null | undefined, locale: string, fallback: string) {
  if (!value) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function Hash({ value }: { value: string | null | undefined }) {
  const { t } = useT("skill-evolution");
  return (
    <span className="truncate font-mono text-micro" title={value ?? undefined}>
      {value ? value.slice(0, 12) : t(($) => $.page.not_available)}
    </span>
  );
}

function ReleaseRow({
  release,
  canRollback,
  rollbackPending,
  onRollback,
}: {
  release: Release;
  canRollback: boolean;
  rollbackPending: boolean;
  onRollback: (release: Release) => void;
}) {
  const { t, i18n } = useT("skill-evolution");
  return (
    <li className="px-3 py-3">
      <div className="flex min-w-0 items-start gap-2">
        <GitCommitHorizontal className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5">
            <EvolutionStatusBadge value={release.kind} />
            <EvolutionStatusBadge
              value={release.outcome}
              tone={releaseStatusTone(release.outcome)}
            />
          </div>
          <div className="mt-2 flex min-w-0 items-center gap-1 text-muted-foreground">
            <Hash value={release.preHash} />
            <span aria-hidden="true">-&gt;</span>
            <Hash value={release.postHash} />
          </div>
          <div className="mt-1 text-caption text-muted-foreground">
            {formatDate(release.completedAt ?? release.createdAt, i18n.language, t(($) => $.page.not_available))}
          </div>
          {release.errorCode ? (
            <div className="mt-1 break-words text-caption text-destructive">
              {t(($) => $.history.error_code, { code: release.errorCode })}
            </div>
          ) : null}
        </div>
        {release.outcome === "succeeded" ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            disabled={!canRollback || rollbackPending}
            onClick={() => onRollback(release)}
            aria-label={t(($) => $.actions.rollback)}
            title={
              canRollback
                ? t(($) => $.actions.rollback)
                : t(($) => $.permissions.publish_required)
            }
          >
            <RotateCcw aria-hidden="true" />
          </Button>
        ) : null}
      </div>
    </li>
  );
}

function RevisionRow({ revision }: { revision: Revision }) {
  const { t, i18n } = useT("skill-evolution");
  return (
    <li className="px-3 py-3">
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 truncate font-mono text-caption" title={revision.bundleHash}>
          {revision.bundleHash.slice(0, 12)}
        </span>
        <span className="shrink-0 font-mono text-micro text-muted-foreground">
          {revision.kind}
        </span>
      </div>
      <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-caption text-muted-foreground">
        <span>{t(($) => $.history.revision_files, { count: revision.supportFileCount })}</span>
        <span>{t(($) => $.history.revision_size, { count: revision.byteCount })}</span>
        <span>{formatDate(revision.createdAt, i18n.language, t(($) => $.page.not_available))}</span>
      </div>
    </li>
  );
}

export function ReleaseHistory({
  releases,
  revisions,
  canRollback,
  rollbackPending,
  onRollback,
}: {
  releases: readonly Release[];
  revisions: readonly Revision[];
  canRollback: boolean;
  rollbackPending: boolean;
  onRollback: (release: Release) => void;
}) {
  const { t } = useT("skill-evolution");
  return (
    <div className="divide-y">
      <section aria-labelledby="evolution-release-history" className="px-4 py-5 sm:px-6">
        <div className="flex items-center gap-2">
          <History className="size-4 text-muted-foreground" aria-hidden="true" />
          <h2 id="evolution-release-history" className="text-title-sm font-medium">
            {t(($) => $.history.title)}
          </h2>
        </div>
        {releases.length > 0 ? (
          <ol className="mt-3 divide-y overflow-hidden rounded-md border">
            {releases.map((release) => (
              <ReleaseRow
                key={release.id}
                release={release}
                canRollback={canRollback}
                rollbackPending={rollbackPending}
                onRollback={onRollback}
              />
            ))}
          </ol>
        ) : (
          <p className="mt-3 text-caption text-muted-foreground">
            {t(($) => $.states.no_releases)}
          </p>
        )}
      </section>

      <section aria-labelledby="evolution-revision-history" className="px-4 py-5 sm:px-6">
        <h2 id="evolution-revision-history" className="text-title-sm font-medium">
          {t(($) => $.history.revisions)}
        </h2>
        {revisions.length > 0 ? (
          <ol className="mt-3 divide-y overflow-hidden rounded-md border">
            {revisions.map((revision) => (
              <RevisionRow key={revision.id} revision={revision} />
            ))}
          </ol>
        ) : (
          <p className="mt-3 text-caption text-muted-foreground">
            {t(($) => $.states.no_revisions)}
          </p>
        )}
      </section>
    </div>
  );
}
