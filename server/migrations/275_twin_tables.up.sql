CREATE TABLE twin_proposal (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('initial', 'evolution')),
    source_wiki_revision_id UUID NOT NULL,
    base_twin_version_id UUID NULL,
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    content JSONB NOT NULL CHECK (jsonb_typeof(content) = 'object' AND octet_length(content::text) <= 2097152),
    content_digest TEXT NOT NULL CHECK (content_digest ~ '^sha256:[0-9a-f]{64}$'),
    requested_by_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE twin_proposal_review (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    proposal_id UUID NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('accepted', 'rejected')),
    reviewer_id UUID NOT NULL,
    reason TEXT NULL CHECK (reason IS NULL OR char_length(reason) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE twin_version (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    version_number BIGINT NOT NULL CHECK (version_number > 0),
    proposal_id UUID NOT NULL,
    source_wiki_revision_id UUID NOT NULL,
    prior_version_id UUID NULL,
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    content JSONB NOT NULL CHECK (jsonb_typeof(content) = 'object' AND octet_length(content::text) <= 2097152),
    content_digest TEXT NOT NULL CHECK (content_digest ~ '^sha256:[0-9a-f]{64}$'),
    signed_off_by_id UUID NOT NULL,
    signed_off_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
