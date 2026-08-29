ALTER TABLE skill_evolution_task_attribution
    DROP COLUMN IF EXISTS task_dispatched_at,
    DROP COLUMN IF EXISTS dispatch_snapshot_id;

DROP TABLE IF EXISTS skill_evolution_task_dispatch_snapshot;
