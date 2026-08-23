"use client";

import { useEffect, useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  lmWikiSourcePolicyOptions,
  useUpdateLMWikiSourcePolicy,
  wikiPageListOptions,
  wikiRevisionListOptions,
  type WikiPageSummary,
  type WikiRevision,
} from "@multica/core/wiki";
import { useT } from "../i18n";
import { LMWikiSourcePolicyPanel } from "./lm-wiki-source-policy-panel";

export function LMWikiSourcePolicyContainer({ canManage }: { canManage: boolean }) {
  const { t } = useT("wiki");
  const wsId = useWorkspaceId();
  const [saved, setSaved] = useState(false);
  const [revisionPageIds, setRevisionPageIds] = useState<ReadonlySet<string>>(new Set());
  const policyQuery = useQuery(lmWikiSourcePolicyOptions(wsId));
  const projectsQuery = useQuery(projectListOptions(wsId));
  const workspacePagesQuery = useQuery(wikiPageListOptions(wsId, { scope: "workspace" }));
  const personalPagesQuery = useQuery(wikiPageListOptions(wsId, { scope: "user" }));
  const projectIds = (projectsQuery.data ?? []).map((project) => project.id);
  const projectPageQueries = useQueries({
    queries: projectIds.map((projectId) => wikiPageListOptions(wsId, {
      scope: "project",
      projectId: projectId,
    })),
  });
  const pages = useMemo(() => mergeWikiPageLists([
    workspacePagesQuery.data ?? [],
    ...projectPageQueries.map((query) => query.data ?? []),
    personalPagesQuery.data ?? [],
  ]), [workspacePagesQuery.data, projectPageQueries, personalPagesQuery.data]);
  useEffect(() => {
    const selected = policyQuery.data?.wikiPages.map((page) => page.pageId) ?? [];
    setRevisionPageIds((current) => new Set([...current, ...selected]));
  }, [policyQuery.data]);
  const revisionPages = pages.filter((page) => page.scope !== "user" && revisionPageIds.has(page.id));
  const revisionQueries = useQueries({
    queries: revisionPages.map((page) => wikiRevisionListOptions(wsId, page.id)),
  });
  const revisionsByPage = useMemo(() => Object.fromEntries(
    revisionPages.map((page, index) => [page.id, revisionQueries[index]?.data ?? []]),
  ) as Record<string, readonly WikiRevision[]>, [revisionPages, revisionQueries]);
  const updatePolicy = useUpdateLMWikiSourcePolicy(wsId);
  const pageQueries = [workspacePagesQuery, personalPagesQuery, ...projectPageQueries];
  const isLoading = policyQuery.isPending || projectsQuery.isPending || pageQueries.some((query) => query.isPending);
  const isError = policyQuery.isError || projectsQuery.isError || pageQueries.some((query) => query.isError);

  return (
    <LMWikiSourcePolicyPanel
      policy={policyQuery.data ?? null}
      pages={pages}
      revisionsByPage={revisionsByPage}
      canManage={canManage}
      canManageRemoteGeneration={canManage}
      isLoading={isLoading}
      isError={isError}
      isSaving={updatePolicy.isPending}
      saved={saved}
      errorMessage={updatePolicy.isError ? t(($) => $.source_policy.save_error) : null}
      onRetry={() => {
        void policyQuery.refetch();
        void projectsQuery.refetch();
        for (const query of pageQueries) void query.refetch();
      }}
      onPageSelectionChange={(pageId, enabled) => {
        if (!enabled) return;
        setRevisionPageIds((current) => new Set([...current, pageId]));
      }}
      onSave={(policy) => {
        setSaved(false);
        updatePolicy.mutate(policy, { onSuccess: () => setSaved(true) });
      }}
    />
  );
}

export function mergeWikiPageLists(lists: readonly (readonly WikiPageSummary[])[]): WikiPageSummary[] {
  const pages = new Map<string, WikiPageSummary>();
  for (const list of lists) {
    for (const page of list) pages.set(page.id, page);
  }
  return [...pages.values()];
}
