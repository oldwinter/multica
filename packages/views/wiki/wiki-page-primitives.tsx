"use client";

import { useId } from "react";
import type { WikiPageSummary } from "@multica/core/wiki";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";

export function WikiPageList({
  pages,
  activePageId,
  onSelect,
}: {
  pages: readonly WikiPageSummary[];
  activePageId?: string;
  onSelect: (page: WikiPageSummary) => void;
}) {
  return (
    <ul className="space-y-0.5">
      {pages.map((page) => (
        <li key={page.id}>
          <button
            type="button"
            className={cn(
              "w-full rounded-md px-2.5 py-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
              page.id === activePageId
                ? "bg-muted font-medium text-foreground"
                : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
            )}
            onClick={() => onSelect(page)}
            aria-current={page.id === activePageId ? "page" : undefined}
          >
            <span className="block break-words text-body font-medium leading-snug">{page.title || page.path}</span>
            <span className="mt-0.5 block truncate font-mono text-caption opacity-80">{page.path}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}

export function WikiEditor({
  path,
  title,
  content,
  onPathChange,
  onTitleChange,
  onContentChange,
  onSave,
  onCancel,
  pending,
  create = false,
  error,
}: {
  path: string;
  title: string;
  content: string;
  onPathChange: (value: string) => void;
  onTitleChange: (value: string) => void;
  onContentChange: (value: string) => void;
  onSave: () => void;
  onCancel: () => void;
  pending: boolean;
  create?: boolean;
  error?: string | null;
}) {
  const { t } = useT("wiki");
  const editorId = useId();
  const createInvalid = create && (!title.trim() || !hasMeaningfulWikiContent(content));
  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault();
        if (!pending && path.trim() && !createInvalid) onSave();
      }}
      aria-busy={pending}
    >
      <div className="space-y-1.5">
        <label htmlFor={`${editorId}-path`} className="block text-caption text-muted-foreground">
          {t(($) => $.fields.path)}
        </label>
        <Input
          id={`${editorId}-path`}
          value={path}
          onChange={(event) => onPathChange(event.target.value)}
          placeholder={t(($) => $.fields.path_placeholder)}
          autoFocus
          aria-describedby={`${editorId}-path-hint`}
        />
        <p id={`${editorId}-path-hint`} className="break-words text-caption text-muted-foreground">
          {t(($) => $.fields.path_hint)}
        </p>
      </div>
      <div className="space-y-1.5">
        <label htmlFor={`${editorId}-title`} className="block text-caption text-muted-foreground">
          {t(($) => $.fields.title)}
        </label>
        <Input id={`${editorId}-title`} value={title} onChange={(event) => onTitleChange(event.target.value)} />
      </div>
      <div className="space-y-1.5">
        <label htmlFor={`${editorId}-content`} className="block text-caption text-muted-foreground">
          {t(($) => $.fields.content)}
        </label>
        <Textarea
          id={`${editorId}-content`}
          value={content}
          onChange={(event) => onContentChange(event.target.value)}
          className="min-h-72 resize-y font-mono text-body sm:min-h-96"
        />
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button type="submit" disabled={pending || !path.trim() || createInvalid}>
          {pending
            ? t(($) => $.states.saving)
            : create
              ? t(($) => $.actions.create)
              : t(($) => $.actions.save)}
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel}>{t(($) => $.actions.cancel)}</Button>
      </div>
      {error ? <p className="break-words text-caption text-destructive" role="alert">{error}</p> : null}
    </form>
  );
}

function hasMeaningfulWikiContent(content: string): boolean {
  const normalized = content.trim();
  return normalized.length > 0 && normalized !== "#";
}
