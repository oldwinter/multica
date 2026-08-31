import { currentPath } from "../../navigation/current-path";
import type { NavigationAdapter } from "../../navigation/types";

export type TwinWorkspaceTab = "wiki" | "twin" | "use";

export const DEFAULT_TWIN_WORKSPACE_TAB: TwinWorkspaceTab = "wiki";
export const TWIN_WORKSPACE_TAB_QUERY_KEY = "tab";

export function isTwinWorkspaceTab(value: unknown): value is TwinWorkspaceTab {
  return value === "wiki" || value === "twin" || value === "use";
}

export function parseTwinWorkspaceTab(value: string | null): TwinWorkspaceTab {
  return isTwinWorkspaceTab(value) ? value : DEFAULT_TWIN_WORKSPACE_TAB;
}

/** Build a same-page tab URL without mutating the adapter's live parameters. */
export function buildTwinWorkspaceTabPath(
  location: Pick<NavigationAdapter, "pathname" | "searchParams" | "hash">,
  tab: TwinWorkspaceTab,
): string {
  const searchParams = new URLSearchParams(location.searchParams);
  if (tab === DEFAULT_TWIN_WORKSPACE_TAB) {
    searchParams.delete(TWIN_WORKSPACE_TAB_QUERY_KEY);
  } else {
    searchParams.set(TWIN_WORKSPACE_TAB_QUERY_KEY, tab);
  }

  return currentPath({ ...location, searchParams });
}
