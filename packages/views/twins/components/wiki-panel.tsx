"use client";

import { useState } from "react";
import { CheckCircle2, Clock3, RefreshCw, XCircle } from "lucide-react";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Separator } from "@multica/ui/components/ui/separator";
import { useT } from "../../i18n";
import { diffWikiContent, projectWikiContent } from "./content-projection";
import { AssertionDiff, CitationList, ContentList } from "./lifecycle-detail";
import { WikiRevisionSelector } from "./lifecycle-selectors";
import { ReviewDialog } from "./review-dialog";
import type { TwinWorkspaceProps } from "./twin-workspace-types";
import { DetailStateNotice } from "./workspace-state";

function reviewState(decision: string | undefined): "accepted" | "rejected" | "pending" {
  if (decision === "accepted") return "accepted";
  if (decision === "rejected") return "rejected";
  return "pending";
}

export function WikiPanel(props: TwinWorkspaceProps) {
  const { t } = useT("twins");
  const [dialog, setDialog] = useState<"accept-wiki" | "reject-wiki" | null>(null);
  const revision = props.wikiDetail?.revision ?? null;
  const state = reviewState(revision?.review?.decision);
  const acceptedContent = props.wiki.accepted_revision?.content ?? null;
  const items = revision ? projectWikiContent(revision.content) : [];
  const diff = revision ? diffWikiContent(revision.content, acceptedContent) : { added: [], removed: [], unchanged: [] };
  const StateIcon = state === "accepted" ? CheckCircle2 : state === "rejected" ? XCircle : Clock3;

  return (
    <div className="space-y-6">
      <section className="flex flex-col gap-4 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)] sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <StateIcon className="size-4 text-muted-foreground" aria-hidden="true" />
            <h2 className="text-title font-medium text-foreground">{t(($) => $.wiki.title)}</h2>
            {revision ? <Badge variant="outline">{t(($) => $.status[state])}</Badge> : null}
          </div>
          <p className="text-body text-muted-foreground">{t(($) => $.wiki.description)}</p>
          {revision ? <p className="break-all font-mono text-caption text-muted-foreground">{revision.source_digest}</p> : null}
        </div>
        {props.canManageWiki ? (
          <Button variant="outline" disabled={props.wikiMutationPending} onClick={props.onRefreshWiki}>
            <RefreshCw data-icon="inline-start" />
            {props.wikiMutationPending ? t(($) => $.actions.refreshing) : t(($) => $.actions.refresh_wiki)}
          </Button>
        ) : null}
      </section>

      {props.sourcePolicyPanel ? (
        <section className="border-t border-surface-border pt-6" data-testid="lm-wiki-source-policy-slot">
          {props.sourcePolicyPanel}
        </section>
      ) : null}

      {props.wiki.revisions.length > 0 ? (
        <WikiRevisionSelector revisions={props.wiki.revisions} value={props.selectedRevisionId} onChange={props.onSelectRevision} disabled={props.detailLoading} />
      ) : (
        <p className="text-body text-muted-foreground">
          {props.canManageWiki
            ? t(($) => $.wiki.first_run_manager)
            : t(($) => $.wiki.first_run_member)}
        </p>
      )}

      <DetailStateNotice state={props.wikiDetailState} onRetry={props.onRetryWikiDetail} />

      {revision ? (
        <section className="space-y-5 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <h3 className="text-title font-medium text-foreground">{t(($) => $.wiki.revision_title, { number: revision.revision_number })}</h3>
              <p className="break-words text-caption text-muted-foreground">{revision.created_at}</p>
            </div>
            {props.canManageWiki && state === "pending" ? (
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" disabled={props.wikiMutationPending} onClick={() => setDialog("reject-wiki")}>{t(($) => $.actions.reject_revision)}</Button>
                <Button variant="brand" disabled={props.wikiMutationPending} onClick={() => setDialog("accept-wiki")}>
                  {props.wikiMutationPending ? t(($) => $.actions.saving) : t(($) => $.actions.accept_revision)}
                </Button>
              </div>
            ) : null}
          </div>
          <Separator />
          <ContentList items={items} emptyLabel={t(($) => $.wiki.empty_content)} />
          <Separator />
          <AssertionDiff diff={diff} />
          <Separator />
          <CitationList citations={props.wikiDetail?.citations ?? []} />
        </section>
      ) : null}

      <ReviewDialog open={dialog !== null} kind={dialog ?? "accept-wiki"} pending={props.wikiMutationPending} onOpenChange={(open) => !open && setDialog(null)} onConfirm={(reason) => {
        if (!revision) return Promise.resolve();
        if (dialog === "reject-wiki") return props.onRejectWiki(revision.id, reason);
        return props.onAcceptWiki(revision.id);
      }} />
    </div>
  );
}
