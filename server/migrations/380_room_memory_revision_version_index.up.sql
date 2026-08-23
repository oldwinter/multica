CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS room_memory_revision_version_uidx ON room_memory_revision (room_id, version);
