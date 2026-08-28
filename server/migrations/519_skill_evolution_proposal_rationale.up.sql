ALTER TABLE skill_evolution_proposal
    ADD COLUMN observed_pattern TEXT NULL CHECK (observed_pattern IS NULL OR octet_length(observed_pattern) BETWEEN 1 AND 2048),
    ADD COLUMN expected_benefit TEXT NULL CHECK (expected_benefit IS NULL OR octet_length(expected_benefit) BETWEEN 1 AND 2048),
    ADD COLUMN regression_risk TEXT NULL CHECK (regression_risk IS NULL OR octet_length(regression_risk) BETWEEN 1 AND 2048),
    ADD CONSTRAINT skill_evolution_proposal_rationale CHECK (
        (observed_pattern IS NULL AND expected_benefit IS NULL AND regression_risk IS NULL)
        OR
        (observed_pattern IS NOT NULL AND expected_benefit IS NOT NULL AND regression_risk IS NOT NULL AND rationale_digest IS NOT NULL)
    );
