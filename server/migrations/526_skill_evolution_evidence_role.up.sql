ALTER TABLE skill_evolution_evidence
    ADD COLUMN IF NOT EXISTS evidence_role TEXT;

UPDATE skill_evolution_evidence
SET evidence_role = 'synthesis'
WHERE evidence_role IS NULL;

ALTER TABLE skill_evolution_evidence
    ALTER COLUMN evidence_role SET DEFAULT 'synthesis',
    ALTER COLUMN evidence_role SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'skill_evolution_evidence'::regclass
          AND conname = 'skill_evolution_evidence_role'
    ) THEN
        ALTER TABLE skill_evolution_evidence
            ADD CONSTRAINT skill_evolution_evidence_role
            CHECK (evidence_role IN ('synthesis', 'held_out_replay'));
    END IF;
END
$$;
