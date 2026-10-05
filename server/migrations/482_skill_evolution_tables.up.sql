CREATE TABLE skill_evolution_loop (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT false,
    mode TEXT NOT NULL DEFAULT 'observe' CHECK (mode IN ('observe', 'propose', 'paused')),
    cooldown_seconds INTEGER NOT NULL DEFAULT 86400 CHECK (cooldown_seconds BETWEEN 60 AND 2592000),
    minimum_signals INTEGER NOT NULL DEFAULT 3 CHECK (minimum_signals BETWEEN 1 AND 100),
    max_evidence_refs INTEGER NOT NULL DEFAULT 20 CHECK (max_evidence_refs BETWEEN 1 AND 100),
    max_replay_samples INTEGER NOT NULL DEFAULT 5 CHECK (max_replay_samples BETWEEN 0 AND 32),
    max_cost_usd_ticks BIGINT NOT NULL DEFAULT 0 CHECK (max_cost_usd_ticks BETWEEN 0 AND 1000000000),
    policy_version TEXT NOT NULL DEFAULT 'v1' CHECK (char_length(policy_version) BETWEEN 1 AND 80),
    last_observed_at TIMESTAMPTZ NULL,
    last_proposal_at TIMESTAMPTZ NULL,
    next_eligible_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skill_evolution_revision (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('base', 'candidate', 'release')),
    ownership_class TEXT NOT NULL CHECK (ownership_class IN ('workspace', 'plugin', 'external', 'runtime_local', 'builtin', 'unknown')),
    source TEXT NOT NULL CHECK (char_length(source) BETWEEN 1 AND 80),
    bundle_hash TEXT NOT NULL CHECK (bundle_hash ~ '^sha256:[0-9a-f]{64}$'),
    metadata_digest TEXT NOT NULL CHECK (metadata_digest ~ '^sha256:[0-9a-f]{64}$'),
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 255),
    description TEXT NOT NULL CHECK (octet_length(description) <= 8192),
    primary_content TEXT NOT NULL CHECK (octet_length(primary_content) <= 1048576),
    byte_count BIGINT NOT NULL CHECK (byte_count BETWEEN 0 AND 8388608),
    support_file_count INTEGER NOT NULL CHECK (support_file_count BETWEEN 0 AND 256),
    created_by_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skill_evolution_revision_file (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    revision_id UUID NOT NULL,
    path TEXT NOT NULL CHECK (octet_length(path) BETWEEN 1 AND 1024),
    content TEXT NOT NULL CHECK (octet_length(content) <= 1048576),
    digest TEXT NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
    byte_count INTEGER NOT NULL CHECK (byte_count BETWEEN 0 AND 1048576),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skill_evolution_proposal (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    loop_id UUID NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued' CHECK (state IN (
        'queued', 'running', 'ready', 'failed', 'stale', 'rejected',
        'publishing', 'published', 'publication_unknown'
    )),
    base_revision_id UUID NOT NULL,
    candidate_revision_id UUID NULL,
    base_hash TEXT NOT NULL CHECK (base_hash ~ '^sha256:[0-9a-f]{64}$'),
    candidate_hash TEXT NULL CHECK (candidate_hash IS NULL OR candidate_hash ~ '^sha256:[0-9a-f]{64}$'),
    rationale_digest TEXT NULL CHECK (rationale_digest IS NULL OR rationale_digest ~ '^sha256:[0-9a-f]{64}$'),
    failure_reason TEXT NULL CHECK (failure_reason IS NULL OR char_length(failure_reason) <= 160),
    stale_reason TEXT NULL CHECK (stale_reason IS NULL OR char_length(stale_reason) <= 160),
    generation_idempotency_key TEXT NOT NULL CHECK (char_length(generation_idempotency_key) BETWEEN 1 AND 160),
    requested_by_id UUID NULL,
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((candidate_revision_id IS NULL) = (candidate_hash IS NULL)),
    CHECK (state NOT IN ('ready', 'rejected', 'publishing', 'published', 'publication_unknown') OR candidate_revision_id IS NOT NULL)
);

CREATE TABLE skill_evolution_evidence (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    proposal_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN (
        'task_review', 'manual_rerun', 'wiki_proposal_review',
        'room_accepted_outcome', 'twin_run_feedback', 'twin_accepted_deposition'
    )),
    source_id TEXT NOT NULL CHECK (char_length(source_id) BETWEEN 1 AND 160),
    source_revision_id TEXT NOT NULL DEFAULT '' CHECK (char_length(source_revision_id) <= 160),
    target_skill_id UUID NULL,
    source_state TEXT NOT NULL CHECK (char_length(source_state) BETWEEN 1 AND 64),
    digest TEXT NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
    eligibility TEXT NOT NULL CHECK (eligibility IN ('eligible', 'ineligible')),
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skill_evolution_evaluation (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    proposal_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('deterministic_validation', 'behavioral_replay')),
    result TEXT NOT NULL CHECK (result IN ('passed', 'failed', 'inconclusive', 'unknown')),
    adapter TEXT NOT NULL CHECK (char_length(adapter) BETWEEN 1 AND 80),
    adapter_version TEXT NOT NULL CHECK (char_length(adapter_version) BETWEEN 1 AND 80),
    policy_version TEXT NOT NULL CHECK (char_length(policy_version) BETWEEN 1 AND 80),
    result_digest TEXT NOT NULL CHECK (result_digest ~ '^sha256:[0-9a-f]{64}$'),
    safe_metrics JSONB NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(safe_metrics) = 'object' AND octet_length(safe_metrics::text) <= 16384),
    cost_usd_ticks BIGINT NULL CHECK (cost_usd_ticks IS NULL OR cost_usd_ticks BETWEEN 0 AND 1000000000),
    duration_ms BIGINT NOT NULL CHECK (duration_ms BETWEEN 0 AND 86400000),
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 160),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skill_evolution_review (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    proposal_id UUID NOT NULL,
    candidate_revision_id UUID NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('rejected', 'publish')),
    actor_id UUID NOT NULL,
    reason TEXT NULL CHECK (reason IS NULL OR char_length(reason) <= 2000),
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 160),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skill_evolution_release (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    proposal_id UUID NULL,
    source_release_id UUID NULL,
    revision_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('publish', 'rollback')),
    expected_base_hash TEXT NOT NULL CHECK (expected_base_hash ~ '^sha256:[0-9a-f]{64}$'),
    pre_hash TEXT NULL CHECK (pre_hash IS NULL OR pre_hash ~ '^sha256:[0-9a-f]{64}$'),
    post_hash TEXT NULL CHECK (post_hash IS NULL OR post_hash ~ '^sha256:[0-9a-f]{64}$'),
    outcome TEXT NOT NULL DEFAULT 'pending' CHECK (outcome IN ('pending', 'succeeded', 'failed', 'publication_unknown')),
    actor_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 160),
    error_code TEXT NULL CHECK (error_code IS NULL OR char_length(error_code) <= 160),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ NULL,
    CHECK ((kind = 'publish' AND proposal_id IS NOT NULL AND source_release_id IS NULL) OR
           (kind = 'rollback' AND source_release_id IS NOT NULL))
);

CREATE TABLE skill_evolution_task_attribution (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    task_id UUID NOT NULL,
    runtime_id UUID NOT NULL,
    skill_id UUID NOT NULL,
    revision_id UUID NOT NULL,
    manifest_version INTEGER NOT NULL CHECK (manifest_version = 1),
    source TEXT NOT NULL CHECK (char_length(source) BETWEEN 1 AND 80),
    bundle_hash TEXT NOT NULL CHECK (bundle_hash ~ '^sha256:[0-9a-f]{64}$'),
    manifest_digest TEXT NOT NULL CHECK (manifest_digest ~ '^sha256:[0-9a-f]{64}$'),
    eligibility TEXT NOT NULL CHECK (eligibility IN ('eligible', 'ineligible')),
    reason TEXT NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 160),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE task_run_review (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    task_id UUID NOT NULL,
    reviewer_id UUID NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('helpful', 'needs_correction')),
    target TEXT NOT NULL CHECK (target IN ('knowledge', 'twin_assertion', 'skill_procedure', 'product_defect')),
    skill_id UUID NULL,
    correction TEXT NULL CHECK (correction IS NULL OR octet_length(correction) <= 4096),
    reason TEXT NOT NULL CHECK (octet_length(reason) BETWEEN 1 AND 4096),
    digest TEXT NOT NULL CHECK (digest ~ '^sha256:[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((target = 'skill_procedure') = (skill_id IS NOT NULL)),
    CHECK (outcome <> 'needs_correction' OR (correction IS NOT NULL AND octet_length(correction) > 0))
);
