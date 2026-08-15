CREATE UNIQUE INDEX CONCURRENTLY room_entry_turn_result_uidx ON room_entry (turn_id) WHERE turn_id IS NOT NULL AND entry_type = 'result';
