ALTER TABLE room ADD CONSTRAINT room_pkey PRIMARY KEY USING INDEX room_id_uidx;
ALTER TABLE room_participant ADD CONSTRAINT room_participant_pkey PRIMARY KEY USING INDEX room_participant_id_uidx;
ALTER TABLE room_entry ADD CONSTRAINT room_entry_pkey PRIMARY KEY USING INDEX room_entry_id_uidx;
ALTER TABLE room_cycle ADD CONSTRAINT room_cycle_pkey PRIMARY KEY USING INDEX room_cycle_id_uidx;
ALTER TABLE room_turn ADD CONSTRAINT room_turn_pkey PRIMARY KEY USING INDEX room_turn_id_uidx;
ALTER TABLE room_artifact ADD CONSTRAINT room_artifact_pkey PRIMARY KEY USING INDEX room_artifact_id_uidx;
