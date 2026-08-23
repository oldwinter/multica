ALTER TABLE twin_binding ADD CONSTRAINT twin_binding_pkey PRIMARY KEY USING INDEX twin_binding_id_uidx;
ALTER TABLE twin_task_attribution ADD CONSTRAINT twin_task_attribution_pkey PRIMARY KEY USING INDEX twin_task_attribution_id_uidx;
ALTER TABLE twin_run_feedback ADD CONSTRAINT twin_run_feedback_pkey PRIMARY KEY USING INDEX twin_run_feedback_id_uidx;
ALTER TABLE twin_deposition ADD CONSTRAINT twin_deposition_pkey PRIMARY KEY USING INDEX twin_deposition_id_uidx;
