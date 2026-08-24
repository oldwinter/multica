CREATE UNIQUE INDEX CONCURRENTLY twin_task_attribution_claim_uidx ON twin_task_attribution (workspace_id, task_id, runtime_id, task_dispatched_at);
