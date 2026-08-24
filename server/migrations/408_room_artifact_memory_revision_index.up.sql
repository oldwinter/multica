CREATE INDEX CONCURRENTLY IF NOT EXISTS room_artifact_memory_revision_idx ON room_artifact (memory_revision_id) WHERE memory_revision_id IS NOT NULL;
