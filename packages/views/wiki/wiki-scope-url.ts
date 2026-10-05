import { currentPath } from "../navigation/current-path";
import type { NavigationAdapter } from "../navigation/types";

export type WikiCollectionScope = "workspace" | "project";

export type WikiScopeSelection =
  | { readonly scope: "workspace"; readonly projectId: null }
  | { readonly scope: "project"; readonly projectId: string | null };

export const DEFAULT_WIKI_SCOPE: WikiCollectionScope = "workspace";
export const WIKI_SCOPE_QUERY_KEY = "scope";
export const WIKI_PROJECT_ID_QUERY_KEY = "project_id";

export function parseWikiScopeSelection(
  searchParams: URLSearchParams,
): WikiScopeSelection {
  if (searchParams.get(WIKI_SCOPE_QUERY_KEY) !== "project") {
    return { scope: DEFAULT_WIKI_SCOPE, projectId: null };
  }

  const rawProjectId = searchParams.get(WIKI_PROJECT_ID_QUERY_KEY);
  return {
    scope: "project",
    projectId: rawProjectId === null || rawProjectId === "" ? null : rawProjectId,
  };
}

/** Build a collection URL without mutating the adapter's live parameters. */
export function buildWikiScopePath(
  location: Pick<NavigationAdapter, "pathname" | "searchParams" | "hash">,
  selection: WikiScopeSelection,
): string {
  const searchParams = new URLSearchParams(location.searchParams);
  if (selection.scope === DEFAULT_WIKI_SCOPE) {
    searchParams.delete(WIKI_SCOPE_QUERY_KEY);
    searchParams.delete(WIKI_PROJECT_ID_QUERY_KEY);
  } else {
    searchParams.set(WIKI_SCOPE_QUERY_KEY, selection.scope);
    if (selection.projectId) {
      searchParams.set(WIKI_PROJECT_ID_QUERY_KEY, selection.projectId);
    } else {
      searchParams.delete(WIKI_PROJECT_ID_QUERY_KEY);
    }
  }

  return currentPath({ ...location, searchParams });
}
