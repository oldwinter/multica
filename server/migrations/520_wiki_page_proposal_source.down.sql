DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM wiki_page_edit_proposal WHERE source_kind = 'room'
    ) THEN
        RAISE EXCEPTION 'cannot roll back 520_wiki_page_proposal_source while Room proposals exist; back up and explicitly remove Room proposals, then rerun migrate down';
    END IF;
END
$$;

ALTER TABLE wiki_page_edit_proposal
    DROP CONSTRAINT IF EXISTS wiki_page_edit_proposal_source,
    DROP COLUMN IF EXISTS source_ref_id,
    DROP COLUMN IF EXISTS source_kind,
    ALTER COLUMN agent_id SET NOT NULL;
