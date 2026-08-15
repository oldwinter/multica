CREATE INDEX CONCURRENTLY room_workspace_idx ON room (workspace_id, status, updated_at DESC);
