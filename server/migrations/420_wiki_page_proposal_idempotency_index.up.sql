CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_edit_proposal_idempotency_uidx ON wiki_page_edit_proposal (workspace_id, agent_id, idempotency_key);
