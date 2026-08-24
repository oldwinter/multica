CREATE INDEX CONCURRENTLY IF NOT EXISTS room_cycle_phase_idx ON room_cycle (workspace_id, room_id, phase, created_at DESC);
