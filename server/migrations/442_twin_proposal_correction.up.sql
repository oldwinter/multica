ALTER TABLE twin_proposal
    ADD COLUMN IF NOT EXISTS replaces_proposal_id UUID;

ALTER TABLE twin_proposal
    DROP CONSTRAINT IF EXISTS twin_proposal_kind_check,
    ADD CONSTRAINT twin_proposal_kind_check
        CHECK (kind IN ('initial', 'evolution', 'deposition', 'correction'));
