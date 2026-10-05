CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_evaluation_idempotency_uidx ON skill_evolution_evaluation (workspace_id, proposal_id, idempotency_key);
