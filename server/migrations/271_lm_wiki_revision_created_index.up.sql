CREATE INDEX CONCURRENTLY lm_wiki_revision_workspace_created_idx ON lm_wiki_revision (workspace_id, created_at DESC);
