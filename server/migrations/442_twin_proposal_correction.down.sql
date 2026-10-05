DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM twin_proposal
        WHERE kind = 'correction'
           OR replaces_proposal_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'rollback blocked: Twin proposal correction history cannot be represented before migration 416';
    END IF;
END $$;

ALTER TABLE twin_proposal
    DROP CONSTRAINT IF EXISTS twin_proposal_kind_check,
    ADD CONSTRAINT twin_proposal_kind_check
        CHECK (kind IN ('initial', 'evolution', 'deposition')),
    DROP COLUMN IF EXISTS replaces_proposal_id;
