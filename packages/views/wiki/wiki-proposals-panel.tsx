"use client";

import { useEffect, useMemo, useState } from "react";
import { Bot, Check, FileDiff, X } from "lucide-react";
import type { WikiProposal } from "@multica/core/wiki";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { RichContent } from "../rich-content";
import {
  wikiDestructiveActionClassName,
  wikiInteractionRegionClassName,
} from "./wiki-ui-contract";

interface ProposalReviewInput {
  proposalId: string;
  path: string;
  title: string;
  content: string;
}

interface WikiProposalsPanelProps {
  proposals: readonly WikiProposal[];
  isLoading: boolean;
  isError: boolean;
  isPending: boolean;
  actionError?: string | null;
  onRetry: () => void;
  onAccept: (input: ProposalReviewInput) => void;
  onReject: (proposalId: string, reason: string) => void;
}

export function WikiProposalsPanel({
  proposals,
  isLoading,
  isError,
  isPending,
  actionError,
  onRetry,
  onAccept,
  onReject,
}: WikiProposalsPanelProps) {
  const { t } = useT("wiki");
  const ordered = useMemo(
    () => [...proposals].sort((a, b) => Number(b.status === "pending") - Number(a.status === "pending")),
    [proposals],
  );
  const [selectedId, setSelectedId] = useState("");
  const [path, setPath] = useState("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [rejectOpen, setRejectOpen] = useState(false);
  const [reason, setReason] = useState("");

  useEffect(() => {
    if (!ordered.some((proposal) => proposal.id === selectedId)) {
      setSelectedId(ordered[0]?.id ?? "");
    }
  }, [ordered, selectedId]);

  const selected = ordered.find((proposal) => proposal.id === selectedId);
  useEffect(() => {
    if (!selected) return;
    setPath(selected.proposedPath);
    setTitle(selected.proposedTitle);
    setContent(selected.proposedContent);
  }, [selected]);

  if (isLoading) {
    return <p className="py-10 text-center text-body text-muted-foreground" role="status">{t(($) => $.proposals.loading)}</p>;
  }
  if (isError) {
    return (
      <div className="space-y-3 py-10 text-center">
        <p className="text-body text-destructive" role="alert">{t(($) => $.proposals.error)}</p>
        <Button variant="outline" onClick={onRetry}>{t(($) => $.actions.retry)}</Button>
      </div>
    );
  }
  if (ordered.length === 0) {
    return (
      <div className="flex min-h-64 flex-col items-center justify-center gap-2 text-center">
        <FileDiff className="size-5 text-muted-foreground" aria-hidden="true" />
        <p className="text-body font-medium text-foreground">{t(($) => $.proposals.empty_title)}</p>
        <p className="max-w-md text-caption text-muted-foreground">{t(($) => $.proposals.empty_description)}</p>
      </div>
    );
  }

  const canReview = selected?.status === "pending";
  return (
    <div className="grid min-h-0 gap-4 lg:grid-cols-[minmax(12rem,16rem)_minmax(0,1fr)]">
      <nav className="max-h-56 overflow-y-auto lg:max-h-none" aria-label={t(($) => $.proposals.list_label)}>
        <ul className="space-y-1">
          {ordered.map((proposal) => (
            <li key={proposal.id}>
              <button
                type="button"
                className={cn(
                  "w-full rounded-md px-2 py-2 text-left transition-colors",
                  proposal.id === selectedId
                    ? "bg-muted font-medium text-foreground"
                    : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                )}
                onClick={() => setSelectedId(proposal.id)}
              >
                <span className="flex items-center justify-between gap-2">
                  <span className="truncate text-body">{proposal.proposedTitle || proposal.proposedPath}</span>
                  <Badge variant={proposal.status === "pending" ? "secondary" : "outline"}>{proposal.status}</Badge>
                </span>
                <span className="mt-1 block truncate text-caption">
                  {t(($) => $.proposals.base_revision, { number: proposal.baseRevisionNumber })}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </nav>

      {selected ? (
        <section className="min-w-0 space-y-4" aria-label={t(($) => $.proposals.review_title)}>
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="flex items-center gap-2 text-body font-medium text-foreground">
                <Bot className="size-4" aria-hidden="true" />
                {t(($) => $.proposals.review_title)}
              </p>
              <p className="break-words text-caption text-muted-foreground">{selected.rationale || t(($) => $.proposals.no_rationale)}</p>
            </div>
            <Badge variant="outline">{selected.status}</Badge>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <label className="space-y-1.5 text-caption text-muted-foreground">
              <span>{t(($) => $.fields.path)}</span>
              <Input value={path} onChange={(event) => setPath(event.target.value)} disabled={!canReview} />
            </label>
            <label className="space-y-1.5 text-caption text-muted-foreground">
              <span>{t(($) => $.fields.title)}</span>
              <Input value={title} onChange={(event) => setTitle(event.target.value)} disabled={!canReview} />
            </label>
          </div>
          <label className="block space-y-1.5 text-caption text-muted-foreground">
            <span>{t(($) => $.fields.content)}</span>
            <Textarea
              value={content}
              onChange={(event) => setContent(event.target.value)}
              disabled={!canReview}
              className="min-h-56 font-mono text-body"
            />
          </label>

          <details className="rounded-md border border-surface-border px-3 py-2">
            <summary className="cursor-pointer text-body font-medium text-foreground">{t(($) => $.actions.preview)}</summary>
            <RichContent
              content={content || t(($) => $.history.empty_content)}
              className="prose prose-sm dark:prose-invert mt-3 max-w-none break-words"
            />
          </details>

          {selected.evidenceRefs.length > 0 ? (
            <div>
              <p className="text-caption font-medium text-foreground">{t(($) => $.proposals.evidence)}</p>
              <ul className="mt-1 space-y-1 font-mono text-caption text-muted-foreground">
                {selected.evidenceRefs.map((reference) => <li key={reference} className="break-all">{reference}</li>)}
              </ul>
            </div>
          ) : null}

          {actionError ? <p className="text-body text-destructive" role="alert">{actionError}</p> : null}
          {canReview ? (
            <div className="flex flex-wrap gap-2">
              <Button
                disabled={isPending || !path.trim()}
                onClick={() => onAccept({ proposalId: selected.id, path, title, content })}
              >
                <Check data-icon="inline-start" />
                {t(($) => $.proposals.accept)}
              </Button>
              <Button variant="outline" disabled={isPending} onClick={() => setRejectOpen(true)}>
                <X data-icon="inline-start" />
                {t(($) => $.proposals.reject)}
              </Button>
            </div>
          ) : selected.reviewReason ? (
            <p className="text-caption text-muted-foreground">
              {t(($) => $.proposals.review_reason, { reason: selected.reviewReason })}
            </p>
          ) : null}
        </section>
      ) : null}

      <Dialog open={rejectOpen} onOpenChange={setRejectOpen}>
        <DialogContent
          className={wikiInteractionRegionClassName}
          data-wiki-interaction-region
        >
          <DialogHeader>
            <DialogTitle>{t(($) => $.proposals.reject_title)}</DialogTitle>
            <DialogDescription>{t(($) => $.proposals.reject_description)}</DialogDescription>
          </DialogHeader>
          <label className="space-y-1.5 text-caption text-muted-foreground">
            <span>{t(($) => $.proposals.reason)}</span>
            <Textarea value={reason} onChange={(event) => setReason(event.target.value)} autoFocus />
          </label>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRejectOpen(false)}>{t(($) => $.actions.cancel)}</Button>
            <Button
              className={wikiDestructiveActionClassName}
              variant="destructive"
              disabled={isPending}
              onClick={() => {
                if (!selected) return;
                onReject(selected.id, reason);
                setRejectOpen(false);
                setReason("");
              }}
            >
              {t(($) => $.proposals.reject)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
