CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_release_idempotency_uidx ON skill_evolution_release (workspace_id, idempotency_key);
