ALTER TABLE agent_task_queue
    DROP CONSTRAINT IF EXISTS agent_task_queue_twin_use_snapshot_check,
    DROP COLUMN IF EXISTS twin_version_id,
    DROP COLUMN IF EXISTS twin_use_state;
