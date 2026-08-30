"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { BookOpenText, Check, Copy, FileClock, FileDiff, History, Plus, Search, Trash2, X } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { paths as appPaths, useWorkspacePaths } from "@multica/core/paths";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  useAcceptWikiProposal,
  useCreateWikiPage,
  useDeleteWikiPage,
  useRejectWikiProposal,
  useRestoreWikiRevision,
  useUpdateWikiPage,
  wikiPageDetailOptions,
  wikiPageListOptions,
  wikiProposalListOptions,
  wikiRevisionListOptions,
  wikiSearchOptions,
  type WikiPageSummary,
  type WikiRevisionConflict,
  type WikiScope,
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
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { RichContent } from "../rich-content";
import { WikiHistoryDialog } from "./wiki-history-dialog";
import { WikiProposalsPanel } from "./wiki-proposals-panel";
import { WikiEditor, WikiPageList } from "./wiki-page-primitives";
import { wikiConflict } from "./wiki-conflict";
import { WorkspaceWikiKnowledgeActivation } from "./wiki-knowledge-activation";

export function WikiPageView({
  pageId,
  personalWikiPath = appPaths.personalWiki(),
}: {
  pageId?: string;
  personalWikiPath?: string;
}) {
  const { t } = useT("wiki");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const nav = useNavigation();

  const [scope, setScope] = useState<WikiScope>("workspace");
  const [projectId, setProjectId] = useState("");
  const [searchText, setSearchText] = useState("");
  const [creating, setCreating] = useState(false);
  const [draftPath, setDraftPath] = useState("index.md");
  const [draftTitle, setDraftTitle] = useState("");
  const [draftContent, setDraftContent] = useState("# ");
  const [editMode, setEditMode] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editContent, setEditContent] = useState("");
  const [editPath, setEditPath] = useState("");
  const [expectedRevision, setExpectedRevision] = useState(0);
  const [conflict, setConflict] = useState<WikiRevisionConflict | null>(null);
  const [mergeBaseContent, setMergeBaseContent] = useState<string | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [detailTab, setDetailTab] = useState("document");
  const [actionError, setActionError] = useState<string | null>(null);
  const [proposalError, setProposalError] = useState<string | null>(null);
  const [citationCopied, setCitationCopied] = useState(false);

  const projectsQuery = useQuery(projectListOptions(wsId));
  const listQuery = useQuery(wikiPageListOptions(wsId, {
    scope,
    projectId: scope === "project" ? projectId || undefined : undefined,
  }));
  const detailQuery = useQuery(wikiPageDetailOptions(wsId, pageId ?? ""));
  const revisionsQuery = useQuery(wikiRevisionListOptions(wsId, pageId ?? ""));
  const proposalsQuery = useQuery(wikiProposalListOptions(wsId, pageId ?? ""));
  const searchQuery = useQuery(wikiSearchOptions(wsId, { q: searchText }));

  const createMutation = useCreateWikiPage(wsId);
  const updateMutation = useUpdateWikiPage(wsId, pageId ?? "");
  const deleteMutation = useDeleteWikiPage(wsId);
  const restoreMutation = useRestoreWikiRevision(wsId, pageId ?? "");
  const acceptProposalMutation = useAcceptWikiProposal(wsId, pageId ?? "");
  const rejectProposalMutation = useRejectWikiProposal(wsId, pageId ?? "");

  const selected = detailQuery.data?.id ? detailQuery.data : undefined;
  const pages = listQuery.data ?? [];
  const normalizedSearch = searchText.trim();
  const isSearching = normalizedSearch.length >= 2;
  const searchResults = searchQuery.data ?? [];
  const pendingProposalCount = (proposalsQuery.data ?? []).filter((proposal) => proposal.status === "pending").length;
  const citationKey = selected ? `wiki_page_revision:${selected.currentRevisionId}` : "";

  useEffect(() => {
    if (!selected) return;
    setScope(selected.scope);
    setProjectId(selected.scope === "project" ? selected.projectId ?? "" : "");
  }, [selected?.id, selected?.scope, selected?.projectId]);

  useEffect(() => {
    setCreating(false);
    setEditMode(false);
    setConflict(null);
    setMergeBaseContent(null);
    setActionError(null);
    setProposalError(null);
    setDetailTab("document");
    setCitationCopied(false);
  }, [pageId]);

  const projectOptions = useMemo(
    () => (Array.isArray(projectsQuery.data) ? projectsQuery.data : []),
    [projectsQuery.data],
  );
  const projectSelectItems = useMemo(
    () => projectOptions.map((project) => ({ value: project.id, label: project.title || project.id })),
    [projectOptions],
  );
  const groupedSearchResults = useMemo(() => {
    const groups: Record<WikiScope, WikiPageSummary[]> = { workspace: [], project: [], user: [] };
    for (const page of searchResults) groups[page.scope].push(page);
    return groups;
  }, [searchResults]);

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
      scope,
      projectId: scope === "project" ? projectId : undefined,
      path: draftPath,
      title: draftTitle || undefined,
      content: draftContent,
    }, {
      onSuccess: (page) => {
        setCreating(false);
        setDraftPath("index.md");
        setDraftTitle("");
        setDraftContent("# ");
        nav.push(paths.wikiPage(page.id));
      },
      onError: () => setActionError(t(($) => $.states.action_error)),
    });
  };

  return (
    <main className="pe-chat-launcher min-h-0 flex-1 overflow-hidden bg-page-canvas" data-testid="wiki-page">
      <div className="mx-auto flex h-full w-full max-w-7xl flex-col gap-4 px-3 py-4 sm:px-6 sm:py-5 lg:px-8">
        <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-1">
            <div className="flex items-center gap-2 text-caption text-muted-foreground">
              <BookOpenText className="size-3.5" aria-hidden="true" />
              <span>{t(($) => $.page.eyebrow)}</span>
            </div>
            <h1 className="break-words text-display-sm font-medium text-foreground">{t(($) => $.page.title)}</h1>
            <p className="max-w-2xl break-words text-body text-muted-foreground">{t(($) => $.page.description)}</p>
          </div>
          <Button onClick={() => setCreating(true)} disabled={scope === "project" && !projectId}>
            <Plus data-icon="inline-start" />
            {t(($) => $.actions.new_page)}
          </Button>
        </header>

        <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
          <Tabs
            value={scope}
            onValueChange={(value) => {
              if (value === "user") {
                nav.push(personalWikiPath);
                return;
              }
              setScope(value as WikiScope);
              setCreating(false);
              if (pageId) nav.push(paths.wiki());
            }}
          >
            <TabsList variant="line" className="max-w-full overflow-x-auto">
              <TabsTrigger value="workspace">{t(($) => $.scopes.workspace)}</TabsTrigger>
              <TabsTrigger value="project">{t(($) => $.scopes.project)}</TabsTrigger>
              <TabsTrigger value="user">{t(($) => $.scopes.user)}</TabsTrigger>
            </TabsList>
          </Tabs>

          <div className="relative w-full xl:max-w-md">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <Input
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
              onKeyDown={(event) => event.key === "Escape" && setSearchText("")}
              aria-label={t(($) => $.search.label)}
              placeholder={t(($) => $.search.placeholder)}
              className="pl-9 pr-9"
            />
            {searchText ? (
              <Button type="button" variant="ghost" size="icon-sm" className="absolute right-1 top-1/2 -translate-y-1/2" onClick={() => setSearchText("")}>
                <X />
                <span className="sr-only">{t(($) => $.search.clear)}</span>
              </Button>
            ) : null}
          </div>
        </div>

        {scope === "user" ? <p className="text-caption text-muted-foreground">{t(($) => $.scope_hints.user)}</p> : null}
        {scope === "project" ? (
          <label className="max-w-sm space-y-1.5 text-caption text-muted-foreground">
            <span>{t(($) => $.fields.project)}</span>
            <Select items={projectSelectItems} value={projectId || null} onValueChange={(value) => setProjectId(value ?? "")}>
              <SelectTrigger className="w-full"><SelectValue placeholder={t(($) => $.empty.pick_project)} /></SelectTrigger>
              <SelectContent>
                {projectSelectItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}
              </SelectContent>
            </Select>
          </label>
        ) : null}

        <div className="grid min-h-0 flex-1 grid-rows-[minmax(10rem,35dvh)_minmax(20rem,1fr)] gap-4 lg:grid-cols-[minmax(14rem,18rem)_minmax(0,1fr)] lg:grid-rows-1">
          <aside className="min-h-0 overflow-y-auto rounded-lg border border-surface-border bg-surface p-3 shadow-[var(--surface-shadow)]">
            {normalizedSearch.length === 1 ? (
              <p className="px-1 py-4 text-body text-muted-foreground">{t(($) => $.search.minimum)}</p>
            ) : isSearching ? (
              <WikiSearchResults groups={groupedSearchResults} isLoading={searchQuery.isPending} isError={searchQuery.isError} activePageId={pageId} onSelect={(id) => nav.push(paths.wikiPage(id))} />
            ) : listQuery.isLoading ? (
              <p className="text-body text-muted-foreground" role="status">{t(($) => $.states.loading)}</p>
            ) : listQuery.isError ? (
              <p className="text-body text-destructive" role="alert">{t(($) => $.states.error)}</p>
            ) : scope === "project" && !projectId ? (
              <p className="text-body text-muted-foreground">{t(($) => $.empty.pick_project)}</p>
            ) : pages.length === 0 ? (
              <div className="space-y-1 px-1 py-4">
                <p className="text-body font-medium text-foreground">{t(($) => $.empty.title)}</p>
                <p className="text-caption text-muted-foreground">{t(($) => $.empty.description)}</p>
              </div>
            ) : <WikiPageList pages={pages} activePageId={pageId} onSelect={(id) => nav.push(paths.wikiPage(id))} />}
          </aside>

          <section className="min-h-0 overflow-y-auto rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
            {creating ? (
              <WikiEditor path={draftPath} title={draftTitle} content={draftContent} onPathChange={setDraftPath} onTitleChange={setDraftTitle} onContentChange={setDraftContent} onSave={createPage} onCancel={() => setCreating(false)} pending={createMutation.isPending} create error={actionError} />
            ) : !pageId ? (
              <div className="flex min-h-64 flex-col items-center justify-center gap-2 text-center">
                <p className="text-body font-medium text-foreground">{t(($) => $.empty.title)}</p>
                <p className="max-w-md text-caption text-muted-foreground">{t(($) => $.empty.description)}</p>
              </div>
            ) : detailQuery.isLoading ? (
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
                    <p className="text-body font-medium text-foreground">{t(($) => $.conflict.merge_title)}</p>
                    <p className="text-caption text-muted-foreground">{t(($) => $.conflict.merge_description)}</p>
                    <details className="mt-2">
                      <summary className="cursor-pointer text-caption font-medium text-foreground">{t(($) => $.conflict.server_version)}</summary>
                      <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-words rounded-md bg-surface p-2 font-mono text-caption text-foreground">{mergeBaseContent}</pre>
                    </details>
                  </div>
                ) : null}
                <WikiEditor path={editPath} title={editTitle} content={editContent} onPathChange={setEditPath} onTitleChange={setEditTitle} onContentChange={setEditContent} onSave={submitUpdate} onCancel={() => { setEditMode(false); setMergeBaseContent(null); }} pending={updateMutation.isPending} error={actionError} />
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
                      {selected.contentDigest ? <span className="break-all font-mono">{selected.contentDigest}</span> : null}
                      <span className="inline-flex min-w-0 items-center gap-1">
                        <code className="break-all">{citationKey}</code>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          title={citationCopied ? t(($) => $.meta.copied) : t(($) => $.meta.copy_citation)}
                          onClick={() => {
                            if (!navigator.clipboard?.writeText) {
                              setActionError(t(($) => $.states.action_error));
                              return;
                            }
                            void navigator.clipboard.writeText(citationKey)
                              .then(() => setCitationCopied(true))
                              .catch(() => setActionError(t(($) => $.states.action_error)));
                          }}
                        >
                          {citationCopied ? <Check /> : <Copy />}
                          <span className="sr-only">{citationCopied ? t(($) => $.meta.copied) : t(($) => $.meta.copy_citation)}</span>
                        </Button>
                      </span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => nav.push(paths.wikiRevision(selected.currentRevisionId))}
                      >
                        <FileClock data-icon="inline-start" />
                        {t(($) => $.revision.open_stable)}
                      </Button>
                    </div>
                  </div>
                  <div className="flex min-w-0 flex-col items-start gap-2 sm:items-end">
                    <WorkspaceWikiKnowledgeActivation target={{
                      pageId: selected.id,
                      revisionId: selected.currentRevisionId,
                      revisionNumber: selected.currentRevisionNumber,
                      title: selected.title,
                      path: selected.path,
                      contentDigest: selected.contentDigest,
                      sourceKind: selected.lastSourceKind,
                      actorType: selected.lastActorType,
                    }} />
                    <div className="flex flex-wrap gap-2">
                      <Button variant="outline" onClick={() => { setActionError(null); setHistoryOpen(true); }}><History data-icon="inline-start" />{t(($) => $.history.action)}</Button>
                      <Button variant="outline" onClick={startEdit}>{t(($) => $.actions.edit)}</Button>
                      <Button variant="ghost" onClick={() => { setActionError(null); setDeleteOpen(true); }}><Trash2 data-icon="inline-start" />{t(($) => $.actions.delete)}</Button>
                    </div>
                  </div>
                </div>

                <Tabs value={detailTab} onValueChange={setDetailTab}>
                  <TabsList variant="line">
                    <TabsTrigger value="document">{t(($) => $.tabs.document)}</TabsTrigger>
                    <TabsTrigger value="proposals"><FileDiff className="size-3.5" aria-hidden="true" />{t(($) => $.tabs.proposals)}{pendingProposalCount > 0 ? <Badge variant="secondary">{pendingProposalCount}</Badge> : null}</TabsTrigger>
                  </TabsList>
                  <TabsContent value="document" className="pt-4">
                    <article className="prose prose-sm dark:prose-invert max-w-none break-words">
                      <RichContent
                        content={selected.content || t(($) => $.history.empty_content)}
                        density="document"
                      />
                    </article>
                  </TabsContent>
                  <TabsContent value="proposals" className="pt-4">
                    <WikiProposalsPanel
                      proposals={proposalsQuery.data ?? []}
                      isLoading={proposalsQuery.isPending}
                      isError={proposalsQuery.isError}
                      isPending={acceptProposalMutation.isPending || rejectProposalMutation.isPending}
                      actionError={proposalError}
                      onRetry={() => proposalsQuery.refetch()}
                      onAccept={(input) => {
                        setProposalError(null);
                        acceptProposalMutation.mutate({ proposalId: input.proposalId, expectedRevisionNumber: selected.currentRevisionNumber, path: input.path, title: input.title, content: input.content }, {
                          onError: (error) => setProposalError(wikiConflict(error) ? t(($) => $.proposals.stale) : t(($) => $.states.action_error)),
                        });
                      }}
                      onReject={(proposalId, reason) => {
                        setProposalError(null);
                        rejectProposalMutation.mutate({ proposalId: proposalId, reason }, { onError: () => setProposalError(t(($) => $.states.action_error)) });
                      }}
                    />
                  </TabsContent>
                </Tabs>
              </div>
            )}
          </section>
        </div>
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
          restoreMutation.mutate({ revisionId: revisionId, expectedRevisionNumber: selected.currentRevisionNumber }, {
            onSuccess: () => setHistoryOpen(false),
            onError: (error) => setActionError(wikiConflict(error) ? t(($) => $.conflict.stale_restore) : t(($) => $.states.action_error)),
          });
        }}
      />

      <AlertDialog open={deleteOpen} onOpenChange={(open) => { setDeleteOpen(open); if (open) setActionError(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>{t(($) => $.delete_dialog.title)}</AlertDialogTitle><AlertDialogDescription>{t(($) => $.delete_dialog.description)}</AlertDialogDescription></AlertDialogHeader>
          {actionError ? <p className="text-body text-destructive" role="alert">{actionError}</p> : null}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>{t(($) => $.actions.cancel)}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={deleteMutation.isPending || !pageId} onClick={() => {
              if (!pageId) return;
              deleteMutation.mutate(pageId, { onSuccess: () => nav.push(paths.wiki()), onError: () => setActionError(t(($) => $.states.action_error)) });
            }}>{t(($) => $.actions.delete)}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={Boolean(conflict)} onOpenChange={(open) => !open && setConflict(null)}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>{t(($) => $.conflict.title)}</AlertDialogTitle><AlertDialogDescription>{t(($) => $.conflict.description, { number: conflict?.currentRevisionNumber ?? 0 })}</AlertDialogDescription></AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => void beginMerge()}>{t(($) => $.conflict.merge)}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void reloadConflict()}>{t(($) => $.conflict.reload)}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </main>
  );
}

function WikiSearchResults({ groups, isLoading, isError, activePageId, onSelect }: { groups: Record<WikiScope, WikiPageSummary[]>; isLoading: boolean; isError: boolean; activePageId?: string; onSelect: (id: string) => void }) {
  const { t } = useT("wiki");
  if (isLoading) return <p className="text-body text-muted-foreground" role="status">{t(($) => $.search.loading)}</p>;
  if (isError) return <p className="text-body text-destructive" role="alert">{t(($) => $.search.error)}</p>;
  const count = groups.workspace.length + groups.project.length + groups.user.length;
  if (count === 0) return <p className="px-1 py-4 text-body text-muted-foreground">{t(($) => $.search.empty)}</p>;
  return (
    <div className="space-y-4">
      {(["workspace", "project", "user"] as const).map((scope) => groups[scope].length > 0 ? (
        <section key={scope} aria-labelledby={`wiki-search-${scope}`}>
          <h2 id={`wiki-search-${scope}`} className="mb-1 px-2 text-caption font-medium text-muted-foreground">{t(($) => $.scopes[scope])}</h2>
          <WikiPageList pages={groups[scope]} activePageId={activePageId} onSelect={onSelect} />
        </section>
      ) : null)}
    </div>
  );
}

export { WikiPageView as WikiPage };
export { LMWikiSourcePolicyPanel } from "./lm-wiki-source-policy-panel";
export type { LMWikiSourcePolicyPanelProps } from "./lm-wiki-source-policy-panel";
export {
  PersonalWikiKnowledgeActivation,
  WorkspaceWikiKnowledgeActivation,
} from "./wiki-knowledge-activation";
export { PersonalWikiPageView } from "./personal-wiki-page";
export type { PersonalWikiRoutePaths } from "./personal-wiki-page";
export {
  ImmutableWikiRevision,
  PersonalWikiRevisionView,
  WorkspaceWikiRevisionView,
} from "./wiki-revision-view";
