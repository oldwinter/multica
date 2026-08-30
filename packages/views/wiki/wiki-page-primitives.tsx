"use client";

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
              "w-full rounded-md px-2 py-1.5 text-left transition-colors",
              page.id === activePageId
                ? "bg-muted font-medium text-foreground"
                : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
            )}
            onClick={() => onSelect(page)}
          >
            <span className="block break-words text-body font-medium">{page.title || page.path}</span>
            <span className="block truncate font-mono text-caption opacity-80">{page.path}</span>
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
  const createInvalid = create && (!title.trim() || !hasMeaningfulWikiContent(content));
  return (
    <div className="space-y-3">
      <label className="block space-y-1.5 text-caption text-muted-foreground">
        <span>{t(($) => $.fields.path)}</span>
        <Input value={path} onChange={(event) => onPathChange(event.target.value)} placeholder="index.md" autoFocus />
        <span className="block break-words">{t(($) => $.fields.path_hint)}</span>
      </label>
      <label className="block space-y-1.5 text-caption text-muted-foreground">
        <span>{t(($) => $.fields.title)}</span>
        <Input value={title} onChange={(event) => onTitleChange(event.target.value)} />
      </label>
      <label className="block space-y-1.5 text-caption text-muted-foreground">
        <span>{t(($) => $.fields.content)}</span>
        <Textarea
          value={content}
          onChange={(event) => onContentChange(event.target.value)}
          className="min-h-72 font-mono text-body sm:min-h-96"
        />
      </label>
      <div className="flex flex-wrap gap-2">
        <Button onClick={onSave} disabled={pending || !path.trim() || createInvalid}>
          {pending
            ? t(($) => $.states.saving)
            : create
              ? t(($) => $.actions.create)
              : t(($) => $.actions.save)}
        </Button>
        <Button variant="ghost" onClick={onCancel}>{t(($) => $.actions.cancel)}</Button>
      </div>
      {error ? <p className="text-caption text-destructive" role="alert">{error}</p> : null}
    </div>
  );
}

function hasMeaningfulWikiContent(content: string): boolean {
  const normalized = content.trim();
  return normalized.length > 0 && normalized !== "#";
}
