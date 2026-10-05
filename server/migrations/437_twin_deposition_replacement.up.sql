ALTER TABLE twin_deposition
    ADD COLUMN IF NOT EXISTS replaces_proposal_id UUID;
