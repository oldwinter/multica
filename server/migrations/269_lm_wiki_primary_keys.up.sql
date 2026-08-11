ALTER TABLE lm_wiki_revision ADD CONSTRAINT lm_wiki_revision_pkey PRIMARY KEY USING INDEX lm_wiki_revision_id_uidx;
ALTER TABLE lm_wiki_citation ADD CONSTRAINT lm_wiki_citation_pkey PRIMARY KEY USING INDEX lm_wiki_citation_id_uidx;
ALTER TABLE lm_wiki_review ADD CONSTRAINT lm_wiki_review_pkey PRIMARY KEY USING INDEX lm_wiki_review_id_uidx;
