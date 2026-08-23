CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS room_memory_review_key_uidx ON room_memory_revision (room_id, cycle_id, review_idempotency_key) WHERE review_idempotency_key IS NOT NULL;
