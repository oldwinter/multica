CREATE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_search_fts_idx ON wiki_page USING gin (to_tsvector('simple', title || ' ' || path || ' ' || content));
