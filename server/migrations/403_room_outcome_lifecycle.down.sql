DROP TABLE room_memory_revision;

ALTER TABLE room_artifact
    DROP COLUMN citation_entry_ids,
    DROP COLUMN memory_revision_id;

ALTER TABLE room_turn
    DROP COLUMN attempt,
    DROP COLUMN turn_kind;

ALTER TABLE room_cycle
    DROP COLUMN expected_max_turns,
    DROP COLUMN memory_revision_id,
    DROP COLUMN synthesis_turn_id,
    DROP COLUMN synthesis_error,
    DROP COLUMN phase;

ALTER TABLE room
    DROP COLUMN capability_version,
    DROP COLUMN last_memory_revision_version,
    DROP COLUMN accepted_memory_revision_id,
    DROP COLUMN max_cost_ticks,
    DROP COLUMN template_id,
    DROP COLUMN stop_conditions,
    DROP COLUMN success_criteria,
    DROP COLUMN objective;
