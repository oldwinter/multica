export type WikiScope = "workspace" | "project" | "user";

export interface WikiPageSummary {
  id: string;
  /** Null for cross-workspace personal pages (scope=user). */
  workspace_id: string | null;
  scope: WikiScope;
  project_id: string | null;
  owner_user_id: string | null;
  path: string;
  title: string;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface WikiPage extends WikiPageSummary {
  content: string;
}

export interface ListWikiPagesParams {
  scope?: WikiScope;
  project_id?: string;
}

export interface CreateWikiPageInput {
  scope: WikiScope;
  project_id?: string;
  path: string;
  title?: string;
  content?: string;
}

export interface UpdateWikiPageInput {
  path?: string;
  title?: string;
  content?: string;
}
