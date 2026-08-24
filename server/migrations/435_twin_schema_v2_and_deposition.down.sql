DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM twin_proposal WHERE kind = 'deposition' OR schema_version = 2)
       OR EXISTS (SELECT 1 FROM twin_version WHERE schema_version = 2) THEN
        RAISE EXCEPTION 'cannot downgrade Twin schema while deposition or schema-v2 artifacts exist';
    END IF;
END $$;

ALTER TABLE twin_proposal
    DROP CONSTRAINT twin_proposal_kind_check,
    DROP CONSTRAINT twin_proposal_schema_version_check,
    ADD CONSTRAINT twin_proposal_kind_check CHECK (kind IN ('initial', 'evolution')),
    ADD CONSTRAINT twin_proposal_schema_version_check CHECK (schema_version = 1);

ALTER TABLE twin_version
    DROP CONSTRAINT twin_version_schema_version_check,
    ADD CONSTRAINT twin_version_schema_version_check CHECK (schema_version = 1);
