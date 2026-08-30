ALTER TABLE skill_evolution_task_attribution
    ADD COLUMN IF NOT EXISTS feedback_covered_at TIMESTAMPTZ NULL;

UPDATE skill_evolution_task_attribution
SET eligibility = 'ineligible',
    reason = 'dispatch_proof_missing'
WHERE eligibility = 'eligible'
  AND (dispatch_snapshot_id IS NULL OR task_dispatched_at IS NULL);

ALTER TABLE skill_evolution_loop
    ADD COLUMN IF NOT EXISTS is_enabled BOOLEAN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'skill_evolution_loop'
          AND column_name = 'enabled'
    ) THEN
        UPDATE skill_evolution_loop
        SET is_enabled = enabled
        WHERE is_enabled IS NULL;
    END IF;
END
$$;

UPDATE skill_evolution_loop
SET is_enabled = FALSE
WHERE is_enabled IS NULL;

ALTER TABLE skill_evolution_loop
    ALTER COLUMN is_enabled SET DEFAULT FALSE,
    ALTER COLUMN is_enabled SET NOT NULL,
    DROP COLUMN IF EXISTS enabled;
