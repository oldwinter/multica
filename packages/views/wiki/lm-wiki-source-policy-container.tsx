"use client";

import { useEffect, useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectListOptions } from "@multica/core/projects/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import { useRefreshLMWiki } from "@multica/core/twins";
import {
  lmWikiSourcePolicyOptions,
  parseLMWikiSourcePolicyStale,
  useUpdateLMWikiSourcePolicy,
  wikiKnowledgeReadinessOptions,
  wikiPageListOptions,
  wikiRevisionListOptions,
  type WikiKnowledgeNextAction,
  type WikiPageSummary,
  type WikiRevision,
} from "@multica/core/wiki";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { WikiKnowledgeMaintenanceQueue } from "./wiki-knowledge-maintenance-queue";
import { LMWikiSourcePolicyPanel } from "./lm-wiki-source-policy-panel";

export function LMWikiSourcePolicyContainer({ canManage }: { canManage: boolean }) {
  const { t } = useT("wiki");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const nav = useNavigation();
  const [saved, setSaved] = useState(false);
  const [conflictPolicyVersion, setConflictPolicyVersion] = useState<number | null>(null);
  const [maintenanceError, setMaintenanceError] = useState<string | null>(null);
  const [revisionPageIds, setRevisionPageIds] = useState<ReadonlySet<string>>(new Set());
  const policyQuery = useQuery(lmWikiSourcePolicyOptions(wsId));
  const readinessQuery = useQuery(wikiKnowledgeReadinessOptions(wsId));
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
  const refreshLMWiki = useRefreshLMWiki(wsId);
  const pageQueries = [workspacePagesQuery, personalPagesQuery, ...projectPageQueries];
  const isLoading = policyQuery.isPending || projectsQuery.isPending || pageQueries.some((query) => query.isPending);
  const isError = policyQuery.isError || projectsQuery.isError || pageQueries.some((query) => query.isError);

  const handlePolicyError = (error: unknown) => {
    const conflict = error instanceof ApiError ? parseLMWikiSourcePolicyStale(error.body) : null;
    setConflictPolicyVersion(conflict?.currentPolicy.policyVersion ?? null);
  };

  const handleMaintenanceAction = (action: WikiKnowledgeNextAction) => {
    setMaintenanceError(null);
    setConflictPolicyVersion(null);
    switch (action.kind) {
      case "pin_revision":
        if (action.revisionId) nav.push(paths.wikiRevision(action.revisionId));
        else setMaintenanceError(t(($) => $.maintenance.action_error));
        return;
      case "remove_source": {
        const readiness = readinessQuery.data;
        if (!readiness || !action.pageId) {
          setMaintenanceError(t(($) => $.maintenance.action_error));
          return;
        }
        updatePolicy.mutate({
          sourceClasses: readiness.policy.sourceClasses,
          wikiPages: readiness.policy.wikiPages.filter((selection) => selection.pageId !== action.pageId),
          remoteGenerationEnabled: readiness.policy.remoteGenerationEnabled,
          expectedPolicyVersion: readiness.policy.policyVersion,
          expectedPolicyDigest: readiness.policy.policyDigest,
        }, {
          onError: (error) => {
            handlePolicyError(error);
            setMaintenanceError(t(($) => $.maintenance.action_error));
          },
        });
        return;
      }
      case "refresh_lm_wiki":
        refreshLMWiki.mutate(undefined, {
          onError: () => setMaintenanceError(t(($) => $.maintenance.action_error)),
        });
        return;
      case "review_lm_wiki": {
        const policySlot = document.querySelector<HTMLElement>("[data-testid='lm-wiki-source-policy-slot']");
        const reviewSurface = policySlot?.nextElementSibling;
        if (reviewSurface instanceof HTMLElement) {
          reviewSurface.scrollIntoView({ behavior: "smooth", block: "start" });
          reviewSurface.focus({ preventScroll: true });
        } else {
          nav.push(paths.twins());
        }
        return;
      }
      default:
        return;
    }
  };

  return (
    <div className="space-y-8">
      <WikiKnowledgeMaintenanceQueue
        readiness={readinessQuery.data}
        pages={pages}
        isLoading={readinessQuery.isPending}
        isError={readinessQuery.isError}
        isPending={updatePolicy.isPending || refreshLMWiki.isPending}
        actionError={maintenanceError}
        onRetry={() => void readinessQuery.refetch()}
        onAction={handleMaintenanceAction}
      />
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
        conflictPolicyVersion={conflictPolicyVersion}
        onResolveConflict={() => {
          setConflictPolicyVersion(null);
          setMaintenanceError(null);
          void policyQuery.refetch();
          void readinessQuery.refetch();
        }}
        onRetry={() => {
          void policyQuery.refetch();
          void projectsQuery.refetch();
          void readinessQuery.refetch();
          for (const query of pageQueries) void query.refetch();
        }}
        onPageSelectionChange={(pageId, enabled) => {
          if (!enabled) return;
          setRevisionPageIds((current) => new Set([...current, pageId]));
        }}
        onSave={(policy) => {
          setSaved(false);
          setConflictPolicyVersion(null);
          updatePolicy.mutate(policy, {
            onSuccess: () => setSaved(true),
            onError: handlePolicyError,
          });
        }}
      />
    </div>
  );
}

export function mergeWikiPageLists(lists: readonly (readonly WikiPageSummary[])[]): WikiPageSummary[] {
  const pages = new Map<string, WikiPageSummary>();
  for (const list of lists) {
    for (const page of list) pages.set(page.id, page);
  }
  return [...pages.values()];
}
