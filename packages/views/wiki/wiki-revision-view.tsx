"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import { ArrowLeft, Check, Copy, FileLock2 } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { paths, useWorkspacePaths } from "@multica/core/paths";
import {
  personalWikiRevisionDetailOptions,
  wikiRevisionDetailOptions,
  type WikiRevision,
} from "@multica/core/wiki";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";

interface ImmutableWikiRevisionProps {
  revision?: WikiRevision;
  isPending: boolean;
  isError: boolean;
  onRetry: () => void;
  onBack: () => void;
  citationPrefix: "wiki_page_revision" | "personal_wiki_revision";
  personal: boolean;
}

export function ImmutableWikiRevision({
  revision,
  isPending,
  isError,
  onRetry,
  onBack,
  citationPrefix,
  personal,
}: ImmutableWikiRevisionProps) {
  const { t } = useT("wiki");
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState(false);
  const citationKey = revision?.id ? `${citationPrefix}:${revision.id}` : "";
  const createdAt = revision?.createdAt ? new Date(revision.createdAt) : null;
  const createdAtLabel = createdAt && !Number.isNaN(createdAt.getTime())
    ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(createdAt)
    : revision?.createdAt ?? "";

  return (
    <main className="min-h-0 flex-1 overflow-y-auto bg-page-canvas" data-testid="wiki-revision-page">
      <div className="mx-auto w-full max-w-5xl px-3 py-4 sm:px-6 sm:py-6 lg:px-8">
        <header className="flex min-w-0 items-start gap-2 border-b border-surface-border pb-4">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="mt-0.5 shrink-0"
            onClick={onBack}
            title={t(($) => $.actions.back)}
          >
            <ArrowLeft />
            <span className="sr-only">{t(($) => $.actions.back)}</span>
          </Button>
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2 text-caption text-muted-foreground">
              <FileLock2 className="size-3.5" aria-hidden="true" />
              <span>{personal ? t(($) => $.revision.personal_eyebrow) : t(($) => $.revision.shared_eyebrow)}</span>
              <Badge variant="secondary">{t(($) => $.revision.read_only)}</Badge>
            </div>
            <h1 className="break-words text-display-sm font-medium text-foreground">
              {revision?.title || revision?.path || t(($) => $.revision.title)}
            </h1>
            <p className="break-words text-body text-muted-foreground">{t(($) => $.revision.description)}</p>
          </div>
        </header>

        {isPending ? (
          <p className="py-12 text-body text-muted-foreground" role="status">{t(($) => $.revision.loading)}</p>
        ) : isError || !revision?.id ? (
          <div className="space-y-3 py-12">
            <p className="break-words text-body text-destructive" role="alert">{t(($) => $.revision.error)}</p>
            <Button variant="outline" onClick={onRetry}>{t(($) => $.actions.retry)}</Button>
          </div>
        ) : (
          <div className="min-w-0 py-5">
            <dl className="grid min-w-0 gap-4 border-b border-surface-border pb-5 sm:grid-cols-2">
              <div className="min-w-0">
                <dt className="text-caption text-muted-foreground">{t(($) => $.revision.citation)}</dt>
                <dd className="mt-1 flex min-w-0 items-center gap-1">
                  <code className="min-w-0 break-all text-caption text-foreground">{citationKey}</code>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="shrink-0"
                    title={copied ? t(($) => $.meta.copied) : t(($) => $.meta.copy_citation)}
                    onClick={() => {
                      setCopyError(false);
                      if (!navigator.clipboard?.writeText) {
                        setCopyError(true);
                        return;
                      }
                      void navigator.clipboard.writeText(citationKey)
                        .then(() => setCopied(true))
                        .catch(() => setCopyError(true));
                    }}
                  >
                    {copied ? <Check /> : <Copy />}
                    <span className="sr-only">{copied ? t(($) => $.meta.copied) : t(($) => $.meta.copy_citation)}</span>
                  </Button>
                </dd>
              </div>
              <div className="min-w-0">
                <dt className="text-caption text-muted-foreground">{t(($) => $.revision.digest)}</dt>
                <dd className="mt-1 break-all font-mono text-caption text-foreground">{revision.contentDigest}</dd>
              </div>
              <div className="min-w-0">
                <dt className="text-caption text-muted-foreground">{t(($) => $.revision.revision_number)}</dt>
                <dd className="mt-1 text-body text-foreground">{revision.revisionNumber}</dd>
              </div>
              <div className="min-w-0">
                <dt className="text-caption text-muted-foreground">{t(($) => $.revision.provenance)}</dt>
                <dd className="mt-1 break-words text-body text-foreground">
                  {t(($) => $.meta.provenance, { source: revision.sourceKind, actor: revision.actorType })}
                </dd>
              </div>
              <div className="min-w-0 sm:col-span-2">
                <dt className="text-caption text-muted-foreground">{t(($) => $.fields.path)}</dt>
                <dd className="mt-1 break-all font-mono text-caption text-foreground">{revision.path}</dd>
              </div>
              <div className="min-w-0 sm:col-span-2">
                <dt className="text-caption text-muted-foreground">{t(($) => $.revision.created_at)}</dt>
                <dd className="mt-1 break-words text-body text-foreground">
                  <time dateTime={revision.createdAt}>{createdAtLabel}</time>
                </dd>
              </div>
            </dl>
            {copyError ? (
              <p className="mt-3 break-words text-caption text-destructive" role="alert">{t(($) => $.revision.copy_error)}</p>
            ) : null}
            <article className="prose prose-sm dark:prose-invert mt-6 max-w-none break-words">
              <ReactMarkdown>{revision.content || t(($) => $.history.empty_content)}</ReactMarkdown>
            </article>
          </div>
        )}
      </div>
    </main>
  );
}

export function WorkspaceWikiRevisionView({ revisionId }: { revisionId: string }) {
  const wsId = useWorkspaceId();
  const workspacePaths = useWorkspacePaths();
  const query = useQuery(wikiRevisionDetailOptions(wsId, revisionId));
  const nav = useNavigation();
  return (
    <ImmutableWikiRevision
      revision={query.data}
      isPending={query.isPending}
      isError={query.isError}
      onRetry={() => void query.refetch()}
      onBack={() => nav.push(workspacePaths.wiki())}
      citationPrefix="wiki_page_revision"
      personal={false}
    />
  );
}

export function PersonalWikiRevisionView({
  revisionId,
  listPath = paths.personalWiki(),
}: {
  revisionId: string;
  listPath?: string;
}) {
  const query = useQuery(personalWikiRevisionDetailOptions(revisionId));
  const nav = useNavigation();
  return (
    <ImmutableWikiRevision
      revision={query.data}
      isPending={query.isPending}
      isError={query.isError}
      onRetry={() => void query.refetch()}
      onBack={() => nav.push(listPath)}
      citationPrefix="personal_wiki_revision"
      personal
    />
  );
}
