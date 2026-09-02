"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowLeft,
  BookLock,
  FileClock,
  History,
  Plus,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { paths } from "@multica/core/paths";
import {
  personalWikiPageDetailOptions,
  personalWikiPageListOptions,
  personalWikiRevisionListOptions,
  personalWikiSearchOptions,
  useCreatePersonalWikiPage,
  useDeletePersonalWikiPage,
  useRestorePersonalWikiRevision,
  useUpdatePersonalWikiPage,
  type WikiRevisionConflict,
} from "@multica/core/wiki";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { RichContent } from "../rich-content";
import { wikiConflict } from "./wiki-conflict";
import { WikiHistoryDialog } from "./wiki-history-dialog";
import { WikiEditor, WikiPageList } from "./wiki-page-primitives";
import {
  WikiMasterDetail,
  wikiDestructiveActionClassName,
  wikiInteractionRegionClassName,
  type WikiNarrowDetailRole,
} from "./wiki-ui-contract";

export interface PersonalWikiRoutePaths {
  list: () => string;
  page: (id: string) => string;
  revision: (revisionId: string) => string;
}

const GLOBAL_PERSONAL_WIKI_PATHS: PersonalWikiRoutePaths = {
  list: paths.personalWiki,
  page: paths.personalWikiPage,
  revision: paths.personalWikiRevision,
};

export function PersonalWikiPageView({
  pageId,
  routePaths = GLOBAL_PERSONAL_WIKI_PATHS,
}: {
  pageId?: string;
  routePaths?: PersonalWikiRoutePaths;
}) {
  const { t } = useT("wiki");
  const nav = useNavigation();
  const [searchText, setSearchText] = useState("");
  const [creating, setCreating] = useState(false);
  const [draftPath, setDraftPath] = useState("notes.md");
  const [draftTitle, setDraftTitle] = useState("");
  const [draftContent, setDraftContent] = useState("# ");
  const [editMode, setEditMode] = useState(false);
  const [editPath, setEditPath] = useState("");
  const [editTitle, setEditTitle] = useState("");
  const [editContent, setEditContent] = useState("");
  const [expectedRevision, setExpectedRevision] = useState(0);
  const [mergeBaseContent, setMergeBaseContent] = useState<string | null>(null);
  const [conflict, setConflict] = useState<WikiRevisionConflict | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const normalizedSearch = searchText.trim();
  const isSearching = normalizedSearch.length >= 2;
  const listQuery = useQuery(personalWikiPageListOptions());
  const searchQuery = useQuery(personalWikiSearchOptions({ q: searchText }));
  const detailQuery = useQuery(personalWikiPageDetailOptions(pageId ?? ""));
  const revisionsQuery = useQuery(personalWikiRevisionListOptions(pageId ?? ""));
  const createMutation = useCreatePersonalWikiPage();
  const updateMutation = useUpdatePersonalWikiPage(pageId ?? "");
  const deleteMutation = useDeletePersonalWikiPage();
  const restoreMutation = useRestorePersonalWikiRevision(pageId ?? "");
  const selected = detailQuery.data?.id ? detailQuery.data : undefined;
  const visiblePages = isSearching ? searchQuery.data ?? [] : listQuery.data ?? [];

  useEffect(() => {
    setCreating(false);
    setEditMode(false);
    setMergeBaseContent(null);
    setConflict(null);
    setActionError(null);
  }, [pageId]);

  const startEdit = () => {
    if (!selected) return;
    setEditPath(selected.path);
    setEditTitle(selected.title);
    setEditContent(selected.content);
    setExpectedRevision(selected.currentRevisionNumber);
    setMergeBaseContent(null);
    setActionError(null);
    setEditMode(true);
  };

  const submitUpdate = () => {
    if (!selected) return;
    setActionError(null);
    updateMutation.mutate({
      expectedRevisionNumber: expectedRevision,
      path: editPath,
      title: editTitle,
      content: editContent,
    }, {
      onSuccess: () => {
        setEditMode(false);
        setMergeBaseContent(null);
      },
      onError: (error) => {
        const stale = wikiConflict(error);
        if (stale) setConflict(stale);
        else setActionError(t(($) => $.states.action_error));
      },
    });
  };

  const reloadConflict = async () => {
    const result = await detailQuery.refetch();
    if (result.data?.id) {
      setEditPath(result.data.path);
      setEditTitle(result.data.title);
      setEditContent(result.data.content);
      setExpectedRevision(result.data.currentRevisionNumber);
    }
    setConflict(null);
    setMergeBaseContent(null);
  };

  const beginMerge = async () => {
    const result = await detailQuery.refetch();
    if (result.data?.id) {
      setMergeBaseContent(result.data.content);
      setExpectedRevision(result.data.currentRevisionNumber);
    } else if (conflict) {
      setExpectedRevision(conflict.currentRevisionNumber);
    }
    setConflict(null);
  };

  const createPage = () => {
    setActionError(null);
    createMutation.mutate({
      path: draftPath,
      title: draftTitle || undefined,
      content: draftContent,
    }, {
      onSuccess: (page) => {
        setCreating(false);
        setDraftPath("notes.md");
        setDraftTitle("");
        setDraftContent("# ");
        nav.push(routePaths.page(page.id));
      },
      onError: () => setActionError(t(($) => $.states.action_error)),
    });
  };

  const narrowDetailRole: WikiNarrowDetailRole =
    !creating && !pageId ? "collection-echo" : "required";

  return (
    <main
      className={cn(
        "min-h-0 flex-1 overflow-y-auto bg-page-canvas lg:overflow-hidden",
        wikiInteractionRegionClassName,
      )}
      data-testid="personal-wiki-page"
      data-wiki-interaction-region
    >
      <div className="mx-auto flex min-h-full w-full max-w-7xl flex-col gap-4 px-3 py-4 sm:px-6 sm:py-5 lg:px-8">
        <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 items-start gap-2">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="mt-0.5 shrink-0"
              onClick={nav.back}
              title={t(($) => $.actions.back)}
            >
              <ArrowLeft />
              <span className="sr-only">{t(($) => $.actions.back)}</span>
            </Button>
            <div className="min-w-0 space-y-1">
              <div className="flex items-center gap-2 text-caption text-muted-foreground">
                <BookLock className="size-3.5" aria-hidden="true" />
                <span>{t(($) => $.personal.eyebrow)}</span>
              </div>
              <h1 className="break-words text-display-sm font-medium text-foreground">
                {t(($) => $.personal.title)}
              </h1>
              <p className="max-w-2xl break-words text-body text-muted-foreground">
                {t(($) => $.personal.description)}
              </p>
            </div>
          </div>
          <Button onClick={() => setCreating(true)}>
            <Plus data-icon="inline-start" />
            {t(($) => $.actions.new_page)}
          </Button>
        </header>

        <div className="relative w-full sm:max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
          <Input
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
            onKeyDown={(event) => event.key === "Escape" && setSearchText("")}
            aria-label={t(($) => $.personal.search_label)}
            placeholder={t(($) => $.personal.search_placeholder)}
            className="pl-9 pr-9 max-lg:pr-12"
          />
          {searchText ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="absolute right-1 top-1/2 -translate-y-1/2"
              onClick={() => setSearchText("")}
            >
              <X />
              <span className="sr-only">{t(($) => $.search.clear)}</span>
            </Button>
          ) : null}
        </div>

        <WikiMasterDetail
          detailRole={narrowDetailRole}
          collection={
            normalizedSearch.length === 1 ? (
              <p className="px-1 py-4 text-body text-muted-foreground">{t(($) => $.search.minimum)}</p>
            ) : isSearching && searchQuery.isPending ? (
              <p className="text-body text-muted-foreground" role="status">{t(($) => $.search.loading)}</p>
            ) : isSearching && searchQuery.isError ? (
              <p className="text-body text-destructive" role="alert">{t(($) => $.search.error)}</p>
            ) : !isSearching && listQuery.isPending ? (
              <p className="text-body text-muted-foreground" role="status">{t(($) => $.states.loading)}</p>
            ) : !isSearching && listQuery.isError ? (
              <p className="text-body text-destructive" role="alert">{t(($) => $.states.error)}</p>
            ) : visiblePages.length === 0 ? (
              <div className="space-y-1 px-1 py-4">
                <p className="break-words text-body font-medium text-foreground">
                  {isSearching ? t(($) => $.search.empty) : t(($) => $.personal.empty_title)}
                </p>
                {!isSearching ? (
                  <p className="break-words text-caption text-muted-foreground">
                    {t(($) => $.personal.empty_description)}
                  </p>
                ) : null}
              </div>
            ) : (
              <WikiPageList
                pages={visiblePages}
                activePageId={pageId}
                onSelect={(page) => nav.push(routePaths.page(page.id))}
              />
            )
          }
          detail={
            <>
            {actionError && !editMode ? (
              <p className="mb-3 break-words text-body text-destructive" role="alert">{actionError}</p>
            ) : null}
            {creating ? (
              <WikiEditor
                path={draftPath}
                title={draftTitle}
                content={draftContent}
                onPathChange={setDraftPath}
                onTitleChange={setDraftTitle}
                onContentChange={setDraftContent}
                onSave={createPage}
                onCancel={() => setCreating(false)}
                pending={createMutation.isPending}
                create
                error={actionError}
              />
            ) : !pageId ? (
              <div className="flex min-h-64 flex-col items-center justify-center gap-2 text-center">
                <p className="break-words text-body font-medium text-foreground">{t(($) => $.personal.empty_title)}</p>
                <p className="max-w-md break-words text-caption text-muted-foreground">{t(($) => $.personal.empty_description)}</p>
              </div>
            ) : detailQuery.isPending ? (
              <p className="text-body text-muted-foreground" role="status">{t(($) => $.states.loading)}</p>
            ) : detailQuery.isError || !selected ? (
              <div className="space-y-3">
                <p className="text-body text-destructive" role="alert">{t(($) => $.states.error)}</p>
                <Button variant="outline" onClick={() => detailQuery.refetch()}>{t(($) => $.actions.retry)}</Button>
              </div>
            ) : editMode ? (
              <div className="space-y-4">
                {mergeBaseContent !== null ? (
                  <div className="rounded-md border border-warning/40 bg-warning/10 p-3" role="status">
                    <p className="break-words text-body font-medium text-foreground">{t(($) => $.conflict.merge_title)}</p>
                    <p className="break-words text-caption text-muted-foreground">{t(($) => $.conflict.merge_description)}</p>
                    <details className="mt-2">
                      <summary className="cursor-pointer text-caption font-medium text-foreground">{t(($) => $.conflict.server_version)}</summary>
                      <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-words rounded-md bg-surface p-2 font-mono text-caption text-foreground">{mergeBaseContent}</pre>
                    </details>
                  </div>
                ) : null}
                <WikiEditor
                  path={editPath}
                  title={editTitle}
                  content={editContent}
                  onPathChange={setEditPath}
                  onTitleChange={setEditTitle}
                  onContentChange={setEditContent}
                  onSave={submitUpdate}
                  onCancel={() => { setEditMode(false); setMergeBaseContent(null); }}
                  pending={updateMutation.isPending}
                  error={actionError}
                />
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0 space-y-1">
                    <h2 className="break-words text-title-lg font-medium text-foreground">{selected.title || selected.path}</h2>
                    <p className="break-all font-mono text-caption text-muted-foreground">{selected.path}</p>
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
                      <span>{t(($) => $.meta.revision, { number: selected.currentRevisionNumber })}</span>
                      <span aria-hidden="true">·</span>
                      <span>{t(($) => $.meta.provenance, { source: selected.lastSourceKind, actor: selected.lastActorType })}</span>
                      <span className="break-all font-mono">{selected.contentDigest}</span>
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" onClick={() => nav.push(routePaths.revision(selected.currentRevisionId))}>
                      <FileClock data-icon="inline-start" />
                      {t(($) => $.personal.stable_revision)}
                    </Button>
                    <Button variant="outline" onClick={() => { setActionError(null); setHistoryOpen(true); }}>
                      <History data-icon="inline-start" />
                      {t(($) => $.history.action)}
                    </Button>
                    <Button variant="outline" onClick={startEdit}>{t(($) => $.actions.edit)}</Button>
                    <Button variant="ghost" onClick={() => { setActionError(null); setDeleteOpen(true); }}>
                      <Trash2 data-icon="inline-start" />
                      {t(($) => $.actions.delete)}
                    </Button>
                  </div>
                </div>
                <RichContent
                  content={selected.content || t(($) => $.history.empty_content)}
                  className="prose prose-sm dark:prose-invert max-w-none break-words"
                />
              </div>
            )}
            </>
          }
        />
      </div>

      <WikiHistoryDialog
        open={historyOpen}
        onOpenChange={setHistoryOpen}
        revisions={revisionsQuery.data ?? []}
        currentRevisionNumber={selected?.currentRevisionNumber ?? 0}
        isLoading={revisionsQuery.isPending}
        isError={revisionsQuery.isError}
        isRestoring={restoreMutation.isPending}
        actionError={actionError}
        onRetry={() => revisionsQuery.refetch()}
        onRestore={(revisionId) => {
          if (!selected) return;
          setActionError(null);
          restoreMutation.mutate({
            revisionId,
            expectedRevisionNumber: selected.currentRevisionNumber,
          }, {
            onSuccess: () => setHistoryOpen(false),
            onError: (error) => {
              const stale = wikiConflict(error);
              if (stale) setConflict(stale);
              else setActionError(t(($) => $.states.action_error));
            },
          });
        }}
      />

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent
          className={wikiInteractionRegionClassName}
          data-wiki-interaction-region
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.delete_dialog.title)}</AlertDialogTitle>
            <AlertDialogDescription>{t(($) => $.personal.delete_description)}</AlertDialogDescription>
            {actionError ? (
              <p className="break-words text-body text-destructive" role="alert">{actionError}</p>
            ) : null}
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t(($) => $.actions.cancel)}</AlertDialogCancel>
            <AlertDialogAction
              className={wikiDestructiveActionClassName}
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => {
                if (!selected) return;
                setActionError(null);
                deleteMutation.mutate(selected.id, {
                  onSuccess: () => nav.push(routePaths.list()),
                  onError: () => setActionError(t(($) => $.states.action_error)),
                });
              }}
            >
              {t(($) => $.actions.delete)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={Boolean(conflict)} onOpenChange={(open) => !open && setConflict(null)}>
        <AlertDialogContent
          className={wikiInteractionRegionClassName}
          data-wiki-interaction-region
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.conflict.title)}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.conflict.description, { number: conflict?.currentRevisionNumber ?? 0 })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => void beginMerge()}>{t(($) => $.conflict.merge)}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void reloadConflict()}>{t(($) => $.conflict.reload)}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </main>
  );
}
