DELETE FROM wiki_page_edit_proposal WHERE source_kind = 'room';

ALTER TABLE wiki_page_edit_proposal
    DROP CONSTRAINT IF EXISTS wiki_page_edit_proposal_source,
    DROP COLUMN IF EXISTS source_ref_id,
    DROP COLUMN IF EXISTS source_kind,
    ALTER COLUMN agent_id SET NOT NULL;
