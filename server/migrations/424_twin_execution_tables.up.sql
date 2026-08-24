CREATE TABLE twin_binding (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    scope_type TEXT NOT NULL CHECK (scope_type IN ('workspace', 'agent', 'project', 'issue')),
    scope_id UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('off', 'preview', 'enabled')),
    twin_version_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (scope_type <> 'workspace' OR scope_id = workspace_id)
);

CREATE TABLE twin_task_attribution (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    task_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    runtime_id UUID NOT NULL,
    task_dispatched_at TIMESTAMPTZ NOT NULL,
    twin_version_id UUID NOT NULL,
    briefing TEXT NOT NULL CHECK (octet_length(briefing) <= 8192),
    briefing_digest TEXT NOT NULL CHECK (briefing_digest ~ '^sha256:[0-9a-f]{64}$'),
    assertion_ids JSONB NOT NULL CHECK (jsonb_typeof(assertion_ids) = 'array' AND octet_length(assertion_ids::text) <= 16384),
    citation_keys JSONB NOT NULL CHECK (jsonb_typeof(citation_keys) = 'array' AND octet_length(citation_keys::text) <= 32768),
    policy_scope_type TEXT NOT NULL CHECK (policy_scope_type IN ('workspace', 'agent', 'project', 'issue', 'one_off')),
    policy_scope_id UUID NOT NULL,
    policy_state TEXT NOT NULL CHECK (policy_state = 'enabled'),
    compiler_version TEXT NOT NULL CHECK (char_length(compiler_version) BETWEEN 1 AND 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (policy_scope_type <> 'workspace' OR policy_scope_id = workspace_id),
    CHECK (policy_scope_type <> 'one_off' OR policy_scope_id = task_id)
);

CREATE TABLE twin_run_feedback (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    task_id UUID NOT NULL,
    rating TEXT NOT NULL CHECK (rating IN ('helped', 'irrelevant', 'mismatch')),
    note TEXT NULL CHECK (note IS NULL OR char_length(note) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE twin_deposition (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    task_id UUID NOT NULL,
    base_twin_version_id UUID NOT NULL,
    proposal_id UUID NOT NULL,
    evidence_digest TEXT NOT NULL CHECK (evidence_digest ~ '^sha256:[0-9a-f]{64}$'),
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'accepted', 'rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
