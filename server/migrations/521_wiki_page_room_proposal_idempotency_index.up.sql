CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS wiki_page_edit_proposal_room_idempotency_uidx ON wiki_page_edit_proposal (workspace_id, source_ref_id, idempotency_key) WHERE source_kind = 'room';
