ALTER TABLE room_memory_revision
    DROP COLUMN review_request_digest,
    DROP COLUMN review_idempotency_key;

ALTER TABLE room_cycle DROP COLUMN cancel_idempotency_key;

ALTER TABLE room_turn DROP COLUMN idempotency_key;
