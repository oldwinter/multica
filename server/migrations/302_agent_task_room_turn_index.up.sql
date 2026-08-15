CREATE UNIQUE INDEX CONCURRENTLY agent_task_room_turn_attempt_uidx ON agent_task_queue (room_turn_id, attempt) WHERE room_turn_id IS NOT NULL;
