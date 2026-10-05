import { queryOptions } from "@tanstack/react-query";
import { api } from "@/data/api";
import type { ListWikiPagesParams } from "@/data/wiki-schema";

export const wikiKeys = {
  all: (wsId: string | null) => ["wiki", wsId] as const,
  list: (wsId: string | null, params: ListWikiPagesParams) =>
    [
      ...wikiKeys.all(wsId),
      "list",
      params.scope,
      params.projectId ?? "",
    ] as const,
  search: (
    wsId: string | null,
    query: string,
    params: ListWikiPagesParams,
  ) =>
    [
      ...wikiKeys.all(wsId),
      "search",
      params.scope,
      params.projectId ?? "",
      query,
    ] as const,
  detail: (wsId: string | null, id: string) =>
    [...wikiKeys.all(wsId), "detail", id] as const,
  revisions: (wsId: string | null, id: string) =>
    [...wikiKeys.detail(wsId, id), "revisions"] as const,
  proposals: (wsId: string | null, id: string) =>
    [...wikiKeys.detail(wsId, id), "proposals"] as const,
  readiness: (wsId: string | null) =>
    [...wikiKeys.all(wsId), "knowledge-readiness"] as const,
};

export const wikiPageListOptions = (
  wsId: string | null,
  params: ListWikiPagesParams,
) =>
  queryOptions({
    queryKey: wikiKeys.list(wsId, params),
    queryFn: ({ signal }) => api.listWikiPages(params, { signal }),
    enabled:
      !!wsId && (params.scope !== "project" || Boolean(params.projectId)),
  });

export const wikiPageSearchOptions = (
  wsId: string | null,
  query: string,
  params: ListWikiPagesParams,
) => {
  const normalized = query.trim();
  return queryOptions({
    queryKey: wikiKeys.search(wsId, normalized, params),
    queryFn: ({ signal }) =>
      api.searchWikiPages(normalized, params, { signal }),
    enabled:
      !!wsId &&
      normalized.length > 0 &&
      (params.scope !== "project" || Boolean(params.projectId)),
  });
};

export const wikiPageDetailOptions = (wsId: string | null, id: string) =>
  queryOptions({
    queryKey: wikiKeys.detail(wsId, id),
    queryFn: ({ signal }) => api.getWikiPage(id, { signal }),
    enabled: !!wsId && !!id,
  });

export const wikiKnowledgeReadinessOptions = (wsId: string | null) =>
  queryOptions({
    queryKey: wikiKeys.readiness(wsId),
    queryFn: ({ signal }) => api.getWikiKnowledgeReadiness({ signal }),
    enabled: !!wsId,
  });

export const wikiPageRevisionsOptions = (wsId: string | null, id: string) =>
  queryOptions({
    queryKey: wikiKeys.revisions(wsId, id),
    queryFn: ({ signal }) => api.listWikiPageRevisions(id, { signal }),
    enabled: !!wsId && !!id,
  });

export const wikiPageProposalsOptions = (wsId: string | null, id: string) =>
  queryOptions({
    queryKey: wikiKeys.proposals(wsId, id),
    queryFn: ({ signal }) => api.listWikiPageProposals(id, { signal }),
    enabled: !!wsId && !!id,
  });
