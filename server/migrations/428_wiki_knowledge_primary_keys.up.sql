ALTER TABLE wiki_page_revision
    ADD CONSTRAINT wiki_page_revision_pkey
    PRIMARY KEY USING INDEX wiki_page_revision_id_uidx;

ALTER TABLE wiki_page_edit_proposal
    ADD CONSTRAINT wiki_page_edit_proposal_pkey
    PRIMARY KEY USING INDEX wiki_page_edit_proposal_id_uidx;

ALTER TABLE lm_wiki_source_policy
    ADD CONSTRAINT lm_wiki_source_policy_pkey
    PRIMARY KEY USING INDEX lm_wiki_source_policy_workspace_uidx;

ALTER TABLE lm_wiki_source_wiki_page
    ADD CONSTRAINT lm_wiki_source_wiki_page_pkey
    PRIMARY KEY USING INDEX lm_wiki_source_wiki_page_identity_uidx;
