ALTER TABLE room
    ADD COLUMN objective TEXT NOT NULL DEFAULT '' CHECK (char_length(objective) <= 4000),
    ADD COLUMN success_criteria JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(success_criteria) = 'array' AND octet_length(success_criteria::text) <= 65536),
    ADD COLUMN stop_conditions JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(stop_conditions) = 'array' AND octet_length(stop_conditions::text) <= 65536),
    ADD COLUMN template_id TEXT NULL CHECK (template_id IS NULL OR char_length(template_id) BETWEEN 1 AND 80),
    ADD COLUMN max_cost_ticks BIGINT NULL CHECK (max_cost_ticks IS NULL OR max_cost_ticks > 0),
    ADD COLUMN accepted_memory_revision_id UUID NULL,
    ADD COLUMN last_memory_revision_version BIGINT NOT NULL DEFAULT 0 CHECK (last_memory_revision_version >= 0),
    ADD COLUMN capability_version INTEGER NOT NULL DEFAULT 1 CHECK (capability_version BETWEEN 1 AND 2);

ALTER TABLE room_cycle
    ADD COLUMN phase TEXT NOT NULL DEFAULT 'gathering'
        CHECK (phase IN ('gathering', 'synthesizing', 'awaiting_review', 'completed', 'failed', 'cancelled', 'refused')),
    ADD COLUMN synthesis_error JSONB NULL
        CHECK (synthesis_error IS NULL OR (jsonb_typeof(synthesis_error) = 'object' AND octet_length(synthesis_error::text) <= 16384)),
    ADD COLUMN synthesis_turn_id UUID NULL,
    ADD COLUMN memory_revision_id UUID NULL,
    ADD COLUMN expected_max_turns INTEGER NOT NULL DEFAULT 0 CHECK (expected_max_turns >= 0);

UPDATE room_cycle
SET phase = CASE status
    WHEN 'completed' THEN 'completed'
    WHEN 'failed' THEN 'failed'
    WHEN 'cancelled' THEN 'cancelled'
    WHEN 'refused' THEN 'refused'
    ELSE 'gathering'
END;

ALTER TABLE room_turn
    ADD COLUMN turn_kind TEXT NOT NULL DEFAULT 'participant'
        CHECK (turn_kind IN ('participant', 'synthesis')),
    ADD COLUMN attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt > 0);

ALTER TABLE room_artifact
    ADD COLUMN memory_revision_id UUID NULL,
    ADD COLUMN citation_entry_ids JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(citation_entry_ids) = 'array' AND octet_length(citation_entry_ids::text) <= 65536);

CREATE TABLE room_memory_revision (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    cycle_id UUID NOT NULL,
    synthesis_turn_id UUID NOT NULL,
    version BIGINT NOT NULL CHECK (version > 0),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    synthesis JSONB NOT NULL
        CHECK (jsonb_typeof(synthesis) = 'object' AND octet_length(synthesis::text) <= 262144),
    digest TEXT NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
    review_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (review_status IN ('pending', 'accepted', 'rejected', 'corrected')),
    reviewed_by_user_id UUID NULL,
    reviewed_at TIMESTAMPTZ NULL,
    corrected_from_revision_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((review_status = 'pending') = (reviewed_at IS NULL)),
    CHECK ((reviewed_at IS NULL) = (reviewed_by_user_id IS NULL))
);
