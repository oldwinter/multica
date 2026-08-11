CREATE TABLE lm_wiki_revision (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    revision_number BIGINT NOT NULL CHECK (revision_number > 0),
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    source_digest TEXT NOT NULL CHECK (source_digest ~ '^sha256:[0-9a-f]{64}$'),
    content JSONB NOT NULL CHECK (jsonb_typeof(content) = 'object' AND octet_length(content::text) <= 2097152),
    trigger_kind TEXT NOT NULL CHECK (trigger_kind IN ('manual', 'scheduled')),
    requested_by_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE lm_wiki_citation (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    revision_id UUID NOT NULL,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    citation_key TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('issue', 'project', 'project_resource', 'autopilot_run')),
    source_id UUID NOT NULL,
    source_updated_at TIMESTAMPTZ NULL,
    locator TEXT NOT NULL,
    label TEXT NOT NULL,
    safe_metadata JSONB NOT NULL CHECK (jsonb_typeof(safe_metadata) = 'object' AND octet_length(safe_metadata::text) <= 65536),
    source_digest TEXT NOT NULL CHECK (source_digest ~ '^sha256:[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE lm_wiki_review (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    revision_id UUID NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('accepted', 'rejected')),
    reviewer_id UUID NOT NULL,
    reason TEXT NULL CHECK (reason IS NULL OR char_length(reason) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
