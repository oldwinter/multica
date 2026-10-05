ALTER TABLE lm_wiki_source_wiki_page
    DROP CONSTRAINT IF EXISTS lm_wiki_source_wiki_page_pkey;

ALTER TABLE lm_wiki_source_policy
    DROP CONSTRAINT IF EXISTS lm_wiki_source_policy_pkey;

ALTER TABLE wiki_page_edit_proposal
    DROP CONSTRAINT IF EXISTS wiki_page_edit_proposal_pkey;

ALTER TABLE wiki_page_revision
    DROP CONSTRAINT IF EXISTS wiki_page_revision_pkey;
