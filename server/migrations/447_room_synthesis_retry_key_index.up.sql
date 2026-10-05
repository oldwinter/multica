CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS room_synthesis_retry_key_uidx ON room_turn (cycle_id, idempotency_key) WHERE turn_kind = 'synthesis' AND idempotency_key IS NOT NULL;
