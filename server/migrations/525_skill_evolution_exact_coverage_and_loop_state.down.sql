ALTER TABLE skill_evolution_loop
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'skill_evolution_loop'
          AND column_name = 'is_enabled'
    ) THEN
        UPDATE skill_evolution_loop
        SET enabled = is_enabled
        WHERE enabled IS NULL;
    END IF;
END
$$;

UPDATE skill_evolution_loop
SET enabled = FALSE
WHERE enabled IS NULL;

ALTER TABLE skill_evolution_loop
    ALTER COLUMN enabled SET DEFAULT FALSE,
    ALTER COLUMN enabled SET NOT NULL,
    DROP COLUMN IF EXISTS is_enabled;

ALTER TABLE skill_evolution_task_attribution
    DROP COLUMN IF EXISTS feedback_covered_at;
