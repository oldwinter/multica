CREATE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_revision_skill_created_idx ON skill_evolution_revision (workspace_id, skill_id, created_at DESC, id DESC);
