CREATE UNIQUE INDEX CONCURRENTLY room_artifact_kind_uidx ON room_artifact (room_id, kind, idempotency_key);
