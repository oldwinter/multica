CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_revision_workspace_skill_hash_uidx ON skill_evolution_revision (workspace_id, skill_id, bundle_hash);
