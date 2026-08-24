CREATE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_revision_page_created_idx ON wiki_page_revision (page_id, revision_number DESC);
