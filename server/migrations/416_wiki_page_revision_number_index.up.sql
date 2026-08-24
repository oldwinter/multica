CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_revision_page_number_uidx ON wiki_page_revision (page_id, revision_number);
