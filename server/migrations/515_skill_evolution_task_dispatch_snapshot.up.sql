CREATE TABLE skill_evolution_task_dispatch_snapshot (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    task_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    runtime_id UUID NOT NULL,
    task_dispatched_at TIMESTAMPTZ NOT NULL,
    contract_version INTEGER NOT NULL CHECK (contract_version = 1),
    identities JSONB NOT NULL CHECK (
        jsonb_typeof(identities) = 'array'
        AND jsonb_array_length(identities) BETWEEN 1 AND 512
        AND octet_length(identities::text) <= 131072
    ),
    identity_count INTEGER NOT NULL CHECK (
        identity_count BETWEEN 1 AND 512
        AND identity_count = jsonb_array_length(identities)
    ),
    identities_digest TEXT NOT NULL CHECK (identities_digest ~ '^sha256:[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE skill_evolution_task_attribution
    ADD COLUMN dispatch_snapshot_id UUID NULL,
    ADD COLUMN task_dispatched_at TIMESTAMPTZ NULL;
