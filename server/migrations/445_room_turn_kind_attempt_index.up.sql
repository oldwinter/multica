CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS room_turn_kind_attempt_uidx ON room_turn (cycle_id, turn_kind, agent_id, attempt);
