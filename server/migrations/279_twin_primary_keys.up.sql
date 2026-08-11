ALTER TABLE twin_proposal ADD CONSTRAINT twin_proposal_pkey PRIMARY KEY USING INDEX twin_proposal_id_uidx;
ALTER TABLE twin_proposal_review ADD CONSTRAINT twin_proposal_review_pkey PRIMARY KEY USING INDEX twin_proposal_review_id_uidx;
ALTER TABLE twin_version ADD CONSTRAINT twin_version_pkey PRIMARY KEY USING INDEX twin_version_id_uidx;
