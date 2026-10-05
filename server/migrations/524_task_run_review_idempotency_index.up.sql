CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS task_run_review_idempotency_uidx ON task_run_review (workspace_id, task_id, reviewer_id, idempotency_key);
