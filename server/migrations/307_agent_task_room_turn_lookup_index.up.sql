CREATE INDEX CONCURRENTLY agent_task_room_turn_idx ON agent_task_queue (room_turn_id, attempt DESC, created_at DESC) WHERE room_turn_id IS NOT NULL;
