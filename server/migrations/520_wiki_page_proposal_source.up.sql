ALTER TABLE wiki_page_edit_proposal
    ALTER COLUMN agent_id DROP NOT NULL,
    ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'agent' CHECK (source_kind IN ('agent', 'room')),
    ADD COLUMN source_ref_id UUID NULL,
    ADD CONSTRAINT wiki_page_edit_proposal_source CHECK (
        (source_kind = 'agent' AND agent_id IS NOT NULL AND source_ref_id IS NULL)
        OR
        (source_kind = 'room' AND agent_id IS NULL AND source_ref_id IS NOT NULL)
    );
