CREATE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_workspace_scope_idx
    ON wiki_page (workspace_id, scope, updated_at DESC);
