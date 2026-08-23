ALTER TABLE wiki_page
    ADD COLUMN current_revision_number BIGINT NOT NULL DEFAULT 1 CHECK (current_revision_number > 0),
    ADD COLUMN current_revision_id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN content_digest TEXT NOT NULL DEFAULT 'sha256:0000000000000000000000000000000000000000000000000000000000000000'
        CHECK (content_digest ~ '^sha256:[0-9a-f]{64}$'),
    ADD COLUMN last_source_kind TEXT NOT NULL DEFAULT 'human'
        CHECK (last_source_kind IN ('human', 'room_promotion', 'agent_proposal', 'restore', 'system')),
    ADD COLUMN last_actor_type TEXT NOT NULL DEFAULT 'member'
        CHECK (last_actor_type IN ('member', 'agent', 'system')),
    ADD COLUMN last_actor_id UUID;

UPDATE wiki_page
SET content_digest = 'sha256:' || encode(sha256(convert_to(content, 'UTF8')), 'hex'),
    last_actor_id = created_by;

CREATE TABLE wiki_page_revision (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID,
    owner_user_id UUID,
    page_id UUID NOT NULL,
    revision_number BIGINT NOT NULL CHECK (revision_number > 0),
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    content_digest TEXT NOT NULL CHECK (content_digest ~ '^sha256:[0-9a-f]{64}$'),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('member', 'agent', 'system')),
    actor_id UUID,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('human', 'room_promotion', 'agent_proposal', 'restore', 'system')),
    source_ref_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT wiki_page_revision_tenant CHECK (
        (workspace_id IS NOT NULL AND owner_user_id IS NULL)
        OR (workspace_id IS NULL AND owner_user_id IS NOT NULL)
    )
);

INSERT INTO wiki_page_revision (
    id, workspace_id, owner_user_id, page_id, revision_number, path, title, content,
    content_digest, actor_type, actor_id, source_kind, created_at
)
SELECT current_revision_id, workspace_id, owner_user_id, id, 1, path, title, content,
       content_digest, 'member', created_by, 'human', created_at
FROM wiki_page;

CREATE TABLE wiki_page_edit_proposal (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    page_id UUID NOT NULL,
    base_revision_number BIGINT NOT NULL CHECK (base_revision_number > 0),
    proposed_path TEXT NOT NULL,
    proposed_title TEXT NOT NULL,
    proposed_content TEXT NOT NULL,
    content_digest TEXT NOT NULL CHECK (content_digest ~ '^sha256:[0-9a-f]{64}$'),
    rationale TEXT NOT NULL CHECK (char_length(rationale) <= 8000),
    evidence_refs JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(evidence_refs) = 'array' AND octet_length(evidence_refs::text) <= 65536),
    agent_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
    reviewed_by_id UUID,
    review_reason TEXT CHECK (review_reason IS NULL OR char_length(review_reason) <= 2000),
    reviewed_at TIMESTAMPTZ,
    accepted_revision_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT wiki_page_edit_proposal_review CHECK (
        (status = 'pending' AND reviewed_by_id IS NULL AND reviewed_at IS NULL AND accepted_revision_id IS NULL)
        OR (status = 'rejected' AND reviewed_by_id IS NOT NULL AND reviewed_at IS NOT NULL AND accepted_revision_id IS NULL)
        OR (status = 'accepted' AND reviewed_by_id IS NOT NULL AND reviewed_at IS NOT NULL AND accepted_revision_id IS NOT NULL)
    )
);

CREATE TABLE lm_wiki_source_policy (
    workspace_id UUID NOT NULL,
    policy_version BIGINT NOT NULL DEFAULT 1 CHECK (policy_version > 0),
    source_classes JSONB NOT NULL CHECK (jsonb_typeof(source_classes) = 'array'),
    remote_generation_enabled BOOLEAN NOT NULL DEFAULT false,
    updated_by_id UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE lm_wiki_source_wiki_page (
    workspace_id UUID NOT NULL,
    page_id UUID NOT NULL,
    revision_id UUID NOT NULL,
    revision_number BIGINT NOT NULL CHECK (revision_number > 0),
    selected_by_id UUID NOT NULL,
    selected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE lm_wiki_revision
    DROP CONSTRAINT lm_wiki_revision_schema_version_check,
    ALTER COLUMN schema_version SET DEFAULT 2,
    ADD COLUMN source_policy_version BIGINT NOT NULL DEFAULT 0 CHECK (source_policy_version >= 0),
    ADD COLUMN source_policy_digest TEXT NOT NULL
        DEFAULT 'sha256:0000000000000000000000000000000000000000000000000000000000000000'
        CHECK (source_policy_digest ~ '^sha256:[0-9a-f]{64}$'),
    ADD COLUMN remote_generation_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD CONSTRAINT lm_wiki_revision_schema_version_check CHECK (schema_version IN (1, 2));

ALTER TABLE lm_wiki_citation
    DROP CONSTRAINT lm_wiki_citation_source_type_check,
    ADD CONSTRAINT lm_wiki_citation_source_type_check CHECK (
        source_type IN ('issue', 'project', 'project_resource', 'autopilot_run', 'wiki_page_revision')
    );
