CREATE INDEX CONCURRENTLY room_turn_room_idx ON room_turn (room_id, created_at DESC);
