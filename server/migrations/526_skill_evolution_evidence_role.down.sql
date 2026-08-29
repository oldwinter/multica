ALTER TABLE skill_evolution_evidence
    DROP CONSTRAINT IF EXISTS skill_evolution_evidence_role,
    DROP COLUMN IF EXISTS evidence_role;
