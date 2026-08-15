CREATE TABLE room (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 160),
    instructions TEXT NOT NULL DEFAULT '' CHECK (char_length(instructions) <= 20000),
    created_by_user_id UUID NOT NULL,
    facilitator_agent_id UUID NOT NULL,
    facilitator_squad_id UUID NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    daily_turn_limit INTEGER NULL CHECK (daily_turn_limit IS NULL OR daily_turn_limit > 0),
    schedule_interval_minutes INTEGER NULL CHECK (schedule_interval_minutes IS NULL OR schedule_interval_minutes BETWEEN 5 AND 10080),
    next_wake_at TIMESTAMPTZ NULL,
    active_cycle_id UUID NULL,
    memory JSONB NOT NULL DEFAULT '{"summary":"","facts":[],"decisions":[],"open_questions":[]}'::jsonb
        CHECK (jsonb_typeof(memory) = 'object' AND octet_length(memory::text) <= 262144),
    memory_version BIGINT NOT NULL DEFAULT 0 CHECK (memory_version >= 0),
    last_entry_ordinal BIGINT NOT NULL DEFAULT 0 CHECK (last_entry_ordinal >= 0),
    last_cycle_sequence BIGINT NOT NULL DEFAULT 0 CHECK (last_cycle_sequence >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE room_participant (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    participant_type TEXT NOT NULL CHECK (participant_type IN ('member', 'agent')),
    participant_id UUID NOT NULL,
    role TEXT NOT NULL DEFAULT 'participant' CHECK (role IN ('facilitator', 'participant', 'observer')),
    source_squad_id UUID NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ NULL
);

CREATE TABLE room_entry (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    cycle_id UUID NULL,
    turn_id UUID NULL,
    ordinal BIGINT NOT NULL CHECK (ordinal > 0),
    entry_type TEXT NOT NULL DEFAULT 'message' CHECK (entry_type IN ('message', 'result', 'system')),
    author_type TEXT NOT NULL CHECK (author_type IN ('member', 'agent', 'system')),
    author_id UUID NULL,
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 100000),
    mentions JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(mentions) = 'array' AND octet_length(mentions::text) <= 65536),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE room_cycle (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    sequence BIGINT NOT NULL CHECK (sequence > 0),
    source TEXT NOT NULL CHECK (source IN ('message', 'mention', 'manual', 'schedule', 'agent')),
    wake_key TEXT NOT NULL CHECK (char_length(wake_key) BETWEEN 1 AND 200),
    triggering_entry_id UUID NULL,
    status TEXT NOT NULL CHECK (status IN ('refused', 'queued', 'running', 'completed', 'failed', 'cancelled')),
    refusal_reason TEXT NULL CHECK (refusal_reason IS NULL OR refusal_reason IN ('room_paused', 'room_archived', 'budget_exhausted', 'cycle_active', 'no_targets')),
    planned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL
);

CREATE TABLE room_turn (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    cycle_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    squad_id UUID NULL,
    status TEXT NOT NULL CHECK (status IN ('refused', 'queued', 'dispatched', 'running', 'completed', 'failed', 'cancelled')),
    refusal_reason TEXT NULL,
    session_id TEXT NULL,
    work_dir TEXT NULL,
    result JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL
);

CREATE TABLE room_artifact (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    cycle_id UUID NULL,
    turn_id UUID NULL,
    entry_id UUID NULL,
    kind TEXT NOT NULL CHECK (kind IN ('issue', 'wiki', 'decision')),
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    target_id UUID NULL,
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 300),
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 200000),
    rationale TEXT NULL CHECK (rationale IS NULL OR char_length(rationale) <= 20000),
    source_digest TEXT NOT NULL CHECK (source_digest ~ '^sha256:[0-9a-f]{64}$'),
    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE agent_task_queue ADD COLUMN room_turn_id UUID NULL;
