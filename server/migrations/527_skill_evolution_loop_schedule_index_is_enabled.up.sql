CREATE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_loop_schedule_is_enabled_idx ON skill_evolution_loop (next_eligible_at, workspace_id, id) WHERE is_enabled AND mode IN ('observe', 'propose');
