CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_project_path_uidx
    ON wiki_page (workspace_id, project_id, path)
    WHERE scope = 'project';
