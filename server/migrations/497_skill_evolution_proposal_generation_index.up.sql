CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_proposal_generation_uidx ON skill_evolution_proposal (workspace_id, skill_id, generation_idempotency_key);
