import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type {
  ListWikiPagesParams,
  SearchPersonalWikiPagesParams,
  SearchWikiPagesParams,
  WikiScope,
} from "./types";

export const wikiKeys = {
  all: (wsId: string) => ["wiki", wsId] as const,
  list: (wsId: string, params: ListWikiPagesParams) =>
    [...wikiKeys.all(wsId), "list", params.scope ?? "workspace", params.projectId ?? ""] as const,
  detail: (wsId: string, id: string) => [...wikiKeys.all(wsId), "detail", id] as const,
  search: (wsId: string, params: SearchWikiPagesParams) => [
    ...wikiKeys.all(wsId),
    "search",
    params.q.trim(),
    params.scope ?? "all",
    params.projectId ?? "",
  ] as const,
  revisions: (wsId: string, id: string) => [...wikiKeys.detail(wsId, id), "revisions"] as const,
  proposals: (wsId: string, id: string) => [...wikiKeys.detail(wsId, id), "proposals"] as const,
  sourcePolicy: (wsId: string) => [...wikiKeys.all(wsId), "lm-wiki-source-policy"] as const,
  readiness: (wsId: string) => [...wikiKeys.all(wsId), "knowledge-readiness"] as const,
  revision: (wsId: string, revisionId: string) => [
    ...wikiKeys.all(wsId), "revision", revisionId,
  ] as const,
};

export const personalWikiKeys = {
  all: ["personal-wiki"] as const,
  list: () => [...personalWikiKeys.all, "list"] as const,
  detail: (id: string) => [...personalWikiKeys.all, "detail", id] as const,
  search: (params: SearchPersonalWikiPagesParams) => [
    ...personalWikiKeys.all, "search", params.q.trim(),
  ] as const,
  revisions: (id: string) => [...personalWikiKeys.detail(id), "revisions"] as const,
  revision: (revisionId: string) => [
    ...personalWikiKeys.all, "revision", revisionId,
  ] as const,
};

export function wikiPageListOptions(wsId: string, params: ListWikiPagesParams = {}) {
  const scope: WikiScope = params.scope ?? "workspace";
  return queryOptions({
    queryKey: wikiKeys.list(wsId, { scope, projectId: params.projectId }),
    queryFn: () => api.listWikiPages({ scope, projectId: params.projectId }),
    enabled: Boolean(wsId) && (scope !== "project" || Boolean(params.projectId)),
  });
}

export function wikiSearchOptions(wsId: string, params: SearchWikiPagesParams) {
  const q = params.q.trim();
  return queryOptions({
    queryKey: wikiKeys.search(wsId, { ...params, q }),
    queryFn: () => api.searchWikiPages({ ...params, q }),
    enabled: Boolean(wsId) && q.length >= 2,
  });
}

export function wikiRevisionListOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: wikiKeys.revisions(wsId, id),
    queryFn: () => api.listWikiRevisions(id),
    enabled: Boolean(wsId && id),
  });
}

export function wikiRevisionDetailOptions(wsId: string, revisionId: string) {
  return queryOptions({
    queryKey: wikiKeys.revision(wsId, revisionId),
    queryFn: () => api.getWikiRevision(revisionId),
    enabled: Boolean(wsId && revisionId),
  });
}

export function personalWikiPageListOptions() {
  return queryOptions({
    queryKey: personalWikiKeys.list(),
    queryFn: () => api.listPersonalWikiPages(),
  });
}

export function personalWikiSearchOptions(params: SearchPersonalWikiPagesParams) {
  const q = params.q.trim();
  return queryOptions({
    queryKey: personalWikiKeys.search({ q }),
    queryFn: () => api.searchPersonalWikiPages({ q }),
    enabled: q.length >= 2,
  });
}

export function personalWikiPageDetailOptions(id: string) {
  return queryOptions({
    queryKey: personalWikiKeys.detail(id),
    queryFn: () => api.getPersonalWikiPage(id),
    enabled: Boolean(id),
    retry: false,
  });
}

export function personalWikiRevisionListOptions(id: string) {
  return queryOptions({
    queryKey: personalWikiKeys.revisions(id),
    queryFn: () => api.listPersonalWikiRevisions(id),
    enabled: Boolean(id),
  });
}

export function personalWikiRevisionDetailOptions(revisionId: string) {
  return queryOptions({
    queryKey: personalWikiKeys.revision(revisionId),
    queryFn: () => api.getPersonalWikiRevision(revisionId),
    enabled: Boolean(revisionId),
  });
}

export function wikiProposalListOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: wikiKeys.proposals(wsId, id),
    queryFn: () => api.listWikiProposals(id),
    enabled: Boolean(wsId && id),
  });
}

export function lmWikiSourcePolicyOptions(wsId: string) {
  return queryOptions({
    queryKey: wikiKeys.sourcePolicy(wsId),
    queryFn: () => api.getLMWikiSourcePolicy(),
    enabled: Boolean(wsId),
  });
}

export function wikiKnowledgeReadinessOptions(wsId: string) {
  return queryOptions({
    queryKey: wikiKeys.readiness(wsId),
    queryFn: () => api.getWikiKnowledgeReadiness(),
    enabled: Boolean(wsId),
  });
}

export function wikiPageDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: wikiKeys.detail(wsId, id),
    queryFn: () => api.getWikiPage(id),
    enabled: Boolean(wsId && id),
    retry: false,
  });
}
