ALTER TABLE skill_evolution_proposal
    DROP CONSTRAINT IF EXISTS skill_evolution_proposal_rationale,
    DROP COLUMN IF EXISTS regression_risk,
    DROP COLUMN IF EXISTS expected_benefit,
    DROP COLUMN IF EXISTS observed_pattern;
