CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_task_attribution_identity_uidx ON skill_evolution_task_attribution (workspace_id, task_id, skill_id);
