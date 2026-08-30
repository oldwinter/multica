CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_review_idempotency_uidx ON skill_evolution_review (workspace_id, proposal_id, idempotency_key);
