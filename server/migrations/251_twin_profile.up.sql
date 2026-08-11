CREATE TABLE twin_profile (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    name TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending-signoff'
        CHECK (state IN ('invalid', 'pending-signoff', 'signed-off')),
    review_digest TEXT NOT NULL DEFAULT '',
    source_count BIGINT NOT NULL DEFAULT 0,
    assertion_count BIGINT NOT NULL DEFAULT 0,
    skill_count BIGINT NOT NULL DEFAULT 0,
    rule_count BIGINT NOT NULL DEFAULT 0,
    assertions JSONB NOT NULL DEFAULT '[]'::jsonb,
    topics JSONB NOT NULL DEFAULT '[]'::jsonb,
    review_steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
