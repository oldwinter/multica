import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { ListWikiPagesParams, WikiScope } from "./types";

export const wikiKeys = {
  all: (wsId: string) => ["wiki", wsId] as const,
  list: (wsId: string, params: ListWikiPagesParams) =>
    [...wikiKeys.all(wsId), "list", params.scope ?? "workspace", params.project_id ?? ""] as const,
  detail: (wsId: string, id: string) => [...wikiKeys.all(wsId), "detail", id] as const,
};

export function wikiPageListOptions(wsId: string, params: ListWikiPagesParams = {}) {
  const scope: WikiScope = params.scope ?? "workspace";
  return queryOptions({
    queryKey: wikiKeys.list(wsId, { scope, project_id: params.project_id }),
    queryFn: () => api.listWikiPages({ scope, project_id: params.project_id }),
    enabled: Boolean(wsId) && (scope !== "project" || Boolean(params.project_id)),
  });
}

export function wikiPageDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: wikiKeys.detail(wsId, id),
    queryFn: () => api.getWikiPage(id),
    enabled: Boolean(wsId && id),
  });
}
