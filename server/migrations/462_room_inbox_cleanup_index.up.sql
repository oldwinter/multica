CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_inbox_room_cleanup ON inbox_item (workspace_id, room_id, room_cycle_id) WHERE room_id IS NOT NULL AND archived = false;
