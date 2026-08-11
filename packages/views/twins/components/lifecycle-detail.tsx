"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, FileText } from "lucide-react";
import type { LMWikiCitation } from "@multica/core/twins";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Separator } from "@multica/ui/components/ui/separator";
import { useT } from "../../i18n";
import type { ProjectedDiff, ProjectedItem } from "./twin-workspace-types";

export function ContentList({ items, emptyLabel }: { items: readonly ProjectedItem[]; emptyLabel: string }) {
  if (items.length === 0) return <p className="text-body text-muted-foreground">{emptyLabel}</p>;
  return (
    <div className="divide-y divide-border/70">
      {items.map((item) => (
        <article key={`${item.kind}:${item.id}`} className="min-w-0 py-3 first:pt-0 last:pb-0">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <p className="min-w-0 break-words text-body font-medium text-foreground">{item.title || item.id}</p>
            {item.status ? <Badge variant="outline">{item.status}</Badge> : null}
          </div>
          {item.summary ? <p className="mt-1 break-words text-body text-muted-foreground">{item.summary}</p> : null}
          <p className="mt-1 break-all font-mono text-caption text-muted-foreground">{item.id}</p>
        </article>
      ))}
    </div>
  );
}

function DiffGroup({ label, values, tone }: { label: string; values: readonly string[]; tone: string }) {
  return (
    <div className="min-w-0 space-y-2">
      <p className={`text-label font-medium ${tone}`}>{label} ({values.length})</p>
      {values.length > 0 ? (
        <ul className="space-y-1">
          {values.map((value) => <li key={value} className="break-all font-mono text-caption text-muted-foreground">{value}</li>)}
        </ul>
      ) : <p className="text-caption text-muted-foreground">0</p>}
    </div>
  );
}

export function AssertionDiff({ diff }: { diff: ProjectedDiff }) {
  const { t } = useT("twins");
  return (
    <section className="grid gap-4 lg:grid-cols-3" aria-label={t(($) => $.diff.title)}>
      <DiffGroup label={t(($) => $.diff.added)} values={diff.added} tone="text-success" />
      <DiffGroup label={t(($) => $.diff.removed)} values={diff.removed} tone="text-destructive" />
      <DiffGroup label={t(($) => $.diff.unchanged)} values={diff.unchanged} tone="text-muted-foreground" />
    </section>
  );
}

function CitationRow({ citation }: { citation: LMWikiCitation }) {
  const { t } = useT("twins");
  const [open, setOpen] = useState(false);
  const Icon = open ? ChevronDown : ChevronRight;
  return (
    <div className="py-2 first:pt-0 last:pb-0">
      <Button
        variant="ghost"
        className="h-auto w-full min-w-0 justify-start px-1 py-1"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        aria-label={t(($) => $.citations.show)}
      >
        <Icon className="shrink-0" aria-hidden="true" />
        <span className="min-w-0 break-words text-left">{citation.label || citation.citation_key}</span>
      </Button>
      {open ? (
        <div className="ml-7 mt-2 min-w-0 space-y-1 text-caption text-muted-foreground">
          <p className="break-all font-mono">{citation.citation_key}</p>
          <p className="break-words">{citation.source_type} / {citation.locator}</p>
          <p className="break-all font-mono">{citation.source_digest}</p>
        </div>
      ) : null}
    </div>
  );
}

export function CitationList({ citations }: { citations: readonly LMWikiCitation[] }) {
  const { t } = useT("twins");
  return (
    <section className="space-y-3" aria-label={t(($) => $.citations.title)}>
      <div className="flex items-center gap-2">
        <FileText className="size-4 text-muted-foreground" aria-hidden="true" />
        <h3 className="text-title font-medium text-foreground">{t(($) => $.citations.title)}</h3>
        <Badge variant="outline">{citations.length}</Badge>
      </div>
      <Separator />
      {citations.length > 0
        ? <div className="divide-y divide-border/70">{citations.map((citation) => <CitationRow key={citation.id} citation={citation} />)}</div>
        : <p className="text-body text-muted-foreground">{t(($) => $.citations.empty)}</p>}
    </section>
  );
}
