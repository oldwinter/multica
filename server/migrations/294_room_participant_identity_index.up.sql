CREATE UNIQUE INDEX CONCURRENTLY room_participant_identity_uidx ON room_participant (room_id, participant_type, participant_id) WHERE left_at IS NULL;
