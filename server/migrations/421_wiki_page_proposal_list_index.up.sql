CREATE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_edit_proposal_page_status_idx ON wiki_page_edit_proposal (workspace_id, page_id, status, created_at DESC);
