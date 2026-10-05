CREATE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_proposal_skill_created_idx ON skill_evolution_proposal (workspace_id, skill_id, created_at DESC, id DESC);
