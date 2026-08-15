CREATE UNIQUE INDEX CONCURRENTLY room_turn_participant_uidx ON room_turn (cycle_id, agent_id);
