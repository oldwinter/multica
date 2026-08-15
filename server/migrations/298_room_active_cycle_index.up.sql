CREATE UNIQUE INDEX CONCURRENTLY room_active_cycle_uidx ON room_cycle (room_id) WHERE status IN ('queued', 'running');
