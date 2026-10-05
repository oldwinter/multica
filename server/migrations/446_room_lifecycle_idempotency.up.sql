ALTER TABLE room_turn
    ADD COLUMN idempotency_key TEXT NULL
        CHECK (idempotency_key IS NULL OR char_length(idempotency_key) BETWEEN 1 AND 200);

ALTER TABLE room_cycle
    ADD COLUMN cancel_idempotency_key TEXT NULL
        CHECK (cancel_idempotency_key IS NULL OR char_length(cancel_idempotency_key) BETWEEN 1 AND 200);

ALTER TABLE room_memory_revision
    ADD COLUMN review_idempotency_key TEXT NULL
        CHECK (review_idempotency_key IS NULL OR char_length(review_idempotency_key) BETWEEN 1 AND 200),
    ADD COLUMN review_request_digest TEXT NULL
        CHECK (review_request_digest IS NULL OR review_request_digest ~ '^sha256:[0-9a-f]{64}$');
