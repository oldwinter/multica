ALTER TABLE twin_proposal
    DROP CONSTRAINT twin_proposal_kind_check,
    DROP CONSTRAINT twin_proposal_schema_version_check,
    ADD CONSTRAINT twin_proposal_kind_check CHECK (kind IN ('initial', 'evolution', 'deposition')),
    ADD CONSTRAINT twin_proposal_schema_version_check CHECK (schema_version IN (1, 2));

ALTER TABLE twin_version
    DROP CONSTRAINT twin_version_schema_version_check,
    ADD CONSTRAINT twin_version_schema_version_check CHECK (schema_version IN (1, 2));
