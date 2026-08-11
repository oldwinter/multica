CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_workspace_path_uidx
    ON wiki_page (workspace_id, path)
    WHERE scope = 'workspace';
