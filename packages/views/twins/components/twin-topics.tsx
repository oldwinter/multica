"use client";

import { ExternalLink, ListTodo } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import { Badge } from "@multica/ui/components/ui/badge";
import { buttonVariants } from "@multica/ui/components/ui/button";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import type { ProjectedTopic } from "./twin-workspace-types";

export function TwinTopics({ topics }: { topics: readonly ProjectedTopic[] }) {
  const { t } = useT("twins");
  const paths = useWorkspacePaths();
  return (
    <section className="space-y-3" aria-label={t(($) => $.topics.title)}>
      <div className="flex items-center gap-2">
        <ListTodo className="size-4 text-muted-foreground" aria-hidden="true" />
        <h3 className="text-title font-medium text-foreground">{t(($) => $.topics.title)}</h3>
        <Badge variant="outline">{topics.length}</Badge>
      </div>
      {topics.length > 0 ? (
        <div className="divide-y divide-border/70">
          {topics.map((topic) => (
            <div key={topic.id || topic.issueId} className="flex min-w-0 flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0">
                <p className="break-words text-body font-medium text-foreground">
                  {topic.issueNumber === null ? topic.title : `Issue ${topic.issueNumber}: ${topic.title}`}
                </p>
                {topic.status ? <p className="mt-1 text-caption text-muted-foreground">{topic.status}</p> : null}
              </div>
              <AppLink href={paths.issueDetail(topic.issueId)} className={buttonVariants({ variant: "ghost", size: "sm" })}>
                <ExternalLink data-icon="inline-start" />
                {t(($) => $.topics.open_issue)}
              </AppLink>
            </div>
          ))}
        </div>
      ) : <p className="text-body text-muted-foreground">{t(($) => $.topics.empty)}</p>}
    </section>
  );
}
