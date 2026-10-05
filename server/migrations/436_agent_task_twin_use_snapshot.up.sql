ALTER TABLE agent_task_queue
    ADD COLUMN twin_use_state TEXT,
    ADD COLUMN twin_version_id UUID,
    ADD CONSTRAINT agent_task_queue_twin_use_snapshot_check CHECK (
        (twin_use_state IS NULL AND twin_version_id IS NULL)
        OR (twin_use_state = 'off' AND twin_version_id IS NULL)
        OR (twin_use_state IN ('preview', 'enabled') AND twin_version_id IS NOT NULL)
    );
