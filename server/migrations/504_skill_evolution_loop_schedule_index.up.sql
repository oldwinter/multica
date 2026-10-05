CREATE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_loop_schedule_idx ON skill_evolution_loop (next_eligible_at, workspace_id, id) WHERE enabled AND mode = 'propose';
