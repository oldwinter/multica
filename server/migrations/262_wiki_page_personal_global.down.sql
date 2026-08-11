-- Re-tenant personal pages is lossy if multiple workspaces existed; assign a
-- placeholder is not possible without a workspace. Leave rows with NULL
-- workspace_id blocked by restoring NOT NULL only after operators re-home them.
ALTER TABLE wiki_page
    DROP CONSTRAINT IF EXISTS wiki_page_scope_keys;

ALTER TABLE wiki_page
    ADD CONSTRAINT wiki_page_scope_keys CHECK (
        (scope = 'workspace' AND project_id IS NULL AND owner_user_id IS NULL)
        OR (scope = 'project' AND project_id IS NOT NULL AND owner_user_id IS NULL)
        OR (scope = 'user' AND project_id IS NULL AND owner_user_id IS NOT NULL)
    );
