-- Personal (scope=user) wiki pages are cross-workspace: workspace_id is NULL
-- and uniqueness is (owner_user_id, path). Workspace/project pages stay
-- tenanted by workspace_id.

ALTER TABLE wiki_page
    ALTER COLUMN workspace_id DROP NOT NULL;

ALTER TABLE wiki_page
    DROP CONSTRAINT IF EXISTS wiki_page_scope_keys;

-- Collapse any same-owner+path personal rows across workspaces before the
-- global unique index lands (keep the oldest id).
DELETE FROM wiki_page a
USING wiki_page b
WHERE a.scope = 'user'
  AND b.scope = 'user'
  AND a.owner_user_id = b.owner_user_id
  AND a.path = b.path
  AND a.id > b.id;

UPDATE wiki_page
SET workspace_id = NULL
WHERE scope = 'user'
  AND workspace_id IS NOT NULL;

ALTER TABLE wiki_page
    ADD CONSTRAINT wiki_page_scope_keys CHECK (
        (scope = 'workspace' AND workspace_id IS NOT NULL AND project_id IS NULL AND owner_user_id IS NULL)
        OR (scope = 'project' AND workspace_id IS NOT NULL AND project_id IS NOT NULL AND owner_user_id IS NULL)
        OR (scope = 'user' AND workspace_id IS NULL AND project_id IS NULL AND owner_user_id IS NOT NULL)
    );
