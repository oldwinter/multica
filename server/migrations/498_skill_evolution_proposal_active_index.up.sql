CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_proposal_active_uidx ON skill_evolution_proposal (workspace_id, skill_id) WHERE state IN ('queued', 'running', 'ready', 'publishing');
