CREATE TABLE wiki_page (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    scope TEXT NOT NULL
        CHECK (scope IN ('workspace', 'project', 'user')),
    project_id UUID,
    owner_user_id UUID,
    path TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT wiki_page_scope_keys CHECK (
        (scope = 'workspace' AND project_id IS NULL AND owner_user_id IS NULL)
        OR (scope = 'project' AND project_id IS NOT NULL AND owner_user_id IS NULL)
        OR (scope = 'user' AND project_id IS NULL AND owner_user_id IS NOT NULL)
    )
);
