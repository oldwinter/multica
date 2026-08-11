"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import { BookOpenText, Plus, Trash2 } from "lucide-react";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  wikiKeys,
  wikiPageDetailOptions,
  wikiPageListOptions,
  type WikiScope,
} from "@multica/core/wiki";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useNavigation } from "../navigation";
import { useT } from "../i18n";

export function WikiPageView({ pageId }: { pageId?: string }) {
  const { t } = useT("wiki");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const nav = useNavigation();
  const queryClient = useQueryClient();

  const [scope, setScope] = useState<WikiScope>("workspace");
  const [projectId, setProjectId] = useState<string>("");
  const [creating, setCreating] = useState(false);
  const [draftPath, setDraftPath] = useState("index.md");
  const [draftTitle, setDraftTitle] = useState("");
  const [draftContent, setDraftContent] = useState("# ");
  const [editMode, setEditMode] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editContent, setEditContent] = useState("");
  const [editPath, setEditPath] = useState("");

  const projectsQuery = useQuery(projectListOptions(wsId));
  const listQuery = useQuery(
    wikiPageListOptions(wsId, {
      scope,
      project_id: scope === "project" ? projectId || undefined : undefined,
    }),
  );
  const detailQuery = useQuery(wikiPageDetailOptions(wsId, pageId ?? ""));

  const pages = listQuery.data ?? [];
  const selected = detailQuery.data;

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
  };

  const createMutation = useMutation({
    mutationFn: () =>
      api.createWikiPage({
        scope,
        project_id: scope === "project" ? projectId : undefined,
        path: draftPath,
        title: draftTitle || undefined,
        content: draftContent,
      }),
    onSuccess: async (page) => {
      await invalidate();
      setCreating(false);
      setDraftPath("index.md");
      setDraftTitle("");
      setDraftContent("# ");
      nav.push(paths.wikiPage(page.id));
    },
  });

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!pageId) throw new Error("missing page");
      return api.updateWikiPage(pageId, {
        path: editPath,
        title: editTitle,
        content: editContent,
      });
    },
    onSuccess: async () => {
      await invalidate();
      setEditMode(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => {
      if (!pageId) throw new Error("missing page");
      return api.deleteWikiPage(pageId);
    },
    onSuccess: async () => {
      await invalidate();
      nav.push(paths.wiki());
    },
  });

  const startEdit = () => {
    if (!selected) return;
    setEditPath(selected.path);
    setEditTitle(selected.title);
    setEditContent(selected.content);
    setEditMode(true);
  };

  const projectOptions = useMemo(
    () => (Array.isArray(projectsQuery.data) ? projectsQuery.data : []),
    [projectsQuery.data],
  );
  const projectSelectItems = useMemo(
    () => projectOptions.map((p) => ({ value: p.id, label: p.title || p.id })),
    [projectOptions],
  );

  return (
    <main className="pe-chat-launcher min-h-0 flex-1 overflow-hidden bg-page-canvas" data-testid="wiki-page">
      <div className="mx-auto flex h-full w-full max-w-6xl flex-col gap-4 px-4 py-5 sm:px-6 lg:px-8">
        <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-caption text-muted-foreground">
              <BookOpenText className="size-3.5" aria-hidden="true" />
              <span>{t(($) => $.page.eyebrow)}</span>
            </div>
            <h1 className="text-display-sm font-medium text-foreground">{t(($) => $.page.title)}</h1>
            <p className="max-w-2xl text-body text-muted-foreground">{t(($) => $.page.description)}</p>
          </div>
          <Button
            onClick={() => setCreating(true)}
            disabled={scope === "project" && !projectId}
          >
            <Plus data-icon="inline-start" />
            {t(($) => $.actions.new_page)}
          </Button>
        </header>

        <Tabs
          value={scope}
          onValueChange={(value) => {
            setScope(value as WikiScope);
            setCreating(false);
            if (!pageId) return;
            nav.push(paths.wiki());
          }}
        >
          <TabsList variant="line">
            <TabsTrigger value="workspace">{t(($) => $.scopes.workspace)}</TabsTrigger>
            <TabsTrigger value="project">{t(($) => $.scopes.project)}</TabsTrigger>
            <TabsTrigger value="user">{t(($) => $.scopes.user)}</TabsTrigger>
          </TabsList>
        </Tabs>

        {scope === "user" ? (
          <p className="text-caption text-muted-foreground">{t(($) => $.scope_hints.user)}</p>
        ) : null}

        {scope === "project" ? (
          <div className="max-w-sm">
            <label className="mb-1.5 block text-caption text-muted-foreground">
              {t(($) => $.fields.project)}
            </label>
            <Select
              items={projectSelectItems}
              value={projectId || null}
              onValueChange={(v) => setProjectId(v ?? "")}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t(($) => $.empty.pick_project)} />
              </SelectTrigger>
              <SelectContent>
                {projectSelectItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        ) : null}

        <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(14rem,18rem)_minmax(0,1fr)]">
          <aside className="min-h-0 overflow-y-auto rounded-lg border border-surface-border bg-surface p-3 shadow-[var(--surface-shadow)]">
            {listQuery.isLoading ? (
              <p className="text-body text-muted-foreground">{t(($) => $.states.loading)}</p>
            ) : listQuery.isError ? (
              <p className="text-body text-destructive">{t(($) => $.states.error)}</p>
            ) : scope === "project" && !projectId ? (
              <p className="text-body text-muted-foreground">{t(($) => $.empty.pick_project)}</p>
            ) : pages.length === 0 ? (
              <div className="space-y-1 px-1 py-4">
                <p className="text-body font-medium text-foreground">{t(($) => $.empty.title)}</p>
                <p className="text-caption text-muted-foreground">{t(($) => $.empty.description)}</p>
              </div>
            ) : (
              <ul className="space-y-0.5">
                {pages.map((page) => {
                  const active = page.id === pageId;
                  return (
                    <li key={page.id}>
                      <button
                        type="button"
                        className={cn(
                          "w-full rounded-md px-2 py-1.5 text-left transition-colors",
                          active ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                        )}
                        onClick={() => nav.push(paths.wikiPage(page.id))}
                      >
                        <div className="truncate text-body font-medium">{page.title || page.path}</div>
                        <div className="truncate font-mono text-caption opacity-80">{page.path}</div>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </aside>

          <section className="min-h-0 overflow-y-auto rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
            {creating ? (
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <label className="text-caption text-muted-foreground">{t(($) => $.fields.path)}</label>
                  <Input value={draftPath} onChange={(e) => setDraftPath(e.target.value)} placeholder="index.md" />
                  <p className="text-caption text-muted-foreground">{t(($) => $.fields.path_hint)}</p>
                </div>
                <div className="space-y-1.5">
                  <label className="text-caption text-muted-foreground">{t(($) => $.fields.title)}</label>
                  <Input value={draftTitle} onChange={(e) => setDraftTitle(e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <label className="text-caption text-muted-foreground">{t(($) => $.fields.content)}</label>
                  <Textarea
                    value={draftContent}
                    onChange={(e) => setDraftContent(e.target.value)}
                    className="min-h-72 font-mono text-body"
                  />
                </div>
                <div className="flex gap-2">
                  <Button
                    onClick={() => createMutation.mutate()}
                    disabled={createMutation.isPending || !draftPath.trim()}
                  >
                    {t(($) => $.actions.create)}
                  </Button>
                  <Button variant="ghost" onClick={() => setCreating(false)}>
                    {t(($) => $.actions.cancel)}
                  </Button>
                </div>
                {createMutation.isError ? (
                  <p className="text-caption text-destructive">{(createMutation.error as Error).message}</p>
                ) : null}
              </div>
            ) : !pageId ? (
              <div className="flex min-h-64 flex-col items-center justify-center gap-2 text-center">
                <p className="text-body font-medium text-foreground">{t(($) => $.empty.title)}</p>
                <p className="max-w-md text-caption text-muted-foreground">{t(($) => $.empty.description)}</p>
              </div>
            ) : detailQuery.isLoading ? (
              <p className="text-body text-muted-foreground">{t(($) => $.states.loading)}</p>
            ) : detailQuery.isError || !selected?.id ? (
              <p className="text-body text-destructive">{t(($) => $.states.error)}</p>
            ) : editMode ? (
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <label className="text-caption text-muted-foreground">{t(($) => $.fields.path)}</label>
                  <Input value={editPath} onChange={(e) => setEditPath(e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <label className="text-caption text-muted-foreground">{t(($) => $.fields.title)}</label>
                  <Input value={editTitle} onChange={(e) => setEditTitle(e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <label className="text-caption text-muted-foreground">{t(($) => $.fields.content)}</label>
                  <Textarea
                    value={editContent}
                    onChange={(e) => setEditContent(e.target.value)}
                    className="min-h-96 font-mono text-body"
                  />
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button onClick={() => updateMutation.mutate()} disabled={updateMutation.isPending}>
                    {updateMutation.isPending ? t(($) => $.states.saving) : t(($) => $.actions.save)}
                  </Button>
                  <Button variant="ghost" onClick={() => setEditMode(false)}>
                    {t(($) => $.actions.cancel)}
                  </Button>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0 space-y-1">
                    <h2 className="text-title-lg font-medium text-foreground">{selected.title || selected.path}</h2>
                    <p className="font-mono text-caption text-muted-foreground">{selected.path}</p>
                  </div>
                  <div className="flex gap-2">
                    <Button variant="outline" onClick={startEdit}>
                      {t(($) => $.actions.edit)}
                    </Button>
                    <Button
                      variant="ghost"
                      onClick={() => {
                        if (window.confirm(t(($) => $.delete_confirm))) {
                          deleteMutation.mutate();
                        }
                      }}
                    >
                      <Trash2 data-icon="inline-start" />
                      {t(($) => $.actions.delete)}
                    </Button>
                  </div>
                </div>
                <article className="prose prose-sm dark:prose-invert max-w-none">
                  <ReactMarkdown>{selected.content || "_Empty page_"}</ReactMarkdown>
                </article>
              </div>
            )}
          </section>
        </div>
      </div>
    </main>
  );
}

export { WikiPageView as WikiPage };
