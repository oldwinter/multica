CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_user_path_uidx
    ON wiki_page (owner_user_id, path)
    WHERE scope = 'user';
