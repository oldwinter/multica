CREATE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_task_attribution_task_idx ON skill_evolution_task_attribution (workspace_id, task_id, created_at, id);
