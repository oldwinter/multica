"use client";

import { useState } from "react";
import { ArrowUpRight } from "lucide-react";
import type { TwinOverview } from "@multica/core/twins";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../../navigation";
import type { TwinCopy } from "./twin-workspace-types";

export function EvidenceSnapshot({ data, copy }: { data: TwinOverview; copy: TwinCopy }) {
  const [expandedId, setExpandedId] = useState<string | null>(null);
  return (
    <section className="rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
      <div className="space-y-1">
        <h2 className="text-title font-medium text-foreground">{copy.evidence.title}</h2>
        <p className="max-w-2xl text-body text-muted-foreground">{copy.evidence.description}</p>
      </div>
      <ul className="mt-4 divide-y divide-border">
        {data.assertions.map((assertion) => {
          const expanded = expandedId === assertion.id;
          return (
            <li key={assertion.id} className="py-3 first:pt-0 last:pb-0">
              <div className="flex items-start gap-3">
                <span className={cn("mt-1.5 size-2 shrink-0 rounded-full", assertion.reviewed ? "bg-success" : "bg-warning")} aria-hidden="true" />
                <div className="min-w-0 flex-1">
                  <p className="text-body text-foreground">{assertion.text}</p>
                  <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-caption text-muted-foreground">
                    <span>{assertion.sourceCount} {copy.evidence.sourceCount}</span>
                    <span>{assertion.reviewed ? copy.stateLabels.complete : copy.stateLabels.current}</span>
                  </div>
                  <p
                    id={`${assertion.id}-evidence`}
                    hidden={!expanded}
                    className="mt-2 animate-in fade-in-0 slide-in-from-top-1 rounded-md bg-muted/50 px-3 py-2 font-mono text-caption text-muted-foreground duration-200 motion-reduce:animate-none"
                  >
                    {assertion.sourceRefs.join(" · ")} · {data.reviewDigest}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label={`${copy.evidence.viewDetail}: ${assertion.text}`}
                  aria-expanded={expanded}
                  aria-controls={`${assertion.id}-evidence`}
                  onClick={() => setExpandedId(expanded ? null : assertion.id)}
                >
                  {copy.evidence.viewDetail}
                </Button>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

export function TopicList({
  data,
  copy,
  links,
}: {
  data: TwinOverview;
  copy: TwinCopy;
  links: { issues: string };
}) {
  return (
    <section className="rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
      <div className="space-y-1">
        <h2 className="text-title font-medium text-foreground">{copy.topics.title}</h2>
        <p className="max-w-2xl text-body text-muted-foreground">{copy.topics.description}</p>
      </div>
      {data.topics.length === 0 ? (
        <p className="mt-6 rounded-md border border-dashed border-surface-border px-4 py-6 text-center text-body text-muted-foreground">{copy.topics.empty}</p>
      ) : (
        <ul className="mt-4 divide-y divide-border">
          {data.topics.map((topic) => (
            <li key={topic.id} className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline">{copy.topicStates[topic.state]}</Badge>
                  <span className="font-mono text-caption text-muted-foreground">{topic.issueIdentifier}</span>
                </div>
                <p className="text-body font-medium text-foreground">{topic.title}</p>
                <p className="text-caption text-muted-foreground">{topic.owner} · {topic.updatedAt}</p>
              </div>
              <AppLink
                href={`${links.issues}/${encodeURIComponent(topic.issueIdentifier)}`}
                className={buttonVariants({ variant: "ghost", size: "sm" })}
              >
                {copy.topics.openIssue}
                <ArrowUpRight data-icon="inline-end" />
              </AppLink>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
