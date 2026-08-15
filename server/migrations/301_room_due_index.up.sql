CREATE INDEX CONCURRENTLY room_due_idx ON room (next_wake_at, id) WHERE status = 'active' AND schedule_interval_minutes IS NOT NULL;
