ALTER TABLE lm_wiki_citation
    DROP CONSTRAINT lm_wiki_citation_source_type_check,
    ADD CONSTRAINT lm_wiki_citation_source_type_check CHECK (
        source_type IN ('issue', 'project', 'project_resource', 'autopilot_run')
    );

ALTER TABLE lm_wiki_revision
    DROP CONSTRAINT lm_wiki_revision_schema_version_check,
    ALTER COLUMN schema_version SET DEFAULT 1,
    DROP COLUMN IF EXISTS remote_generation_enabled,
    DROP COLUMN IF EXISTS source_policy_digest,
    DROP COLUMN IF EXISTS source_policy_version,
    ADD CONSTRAINT lm_wiki_revision_schema_version_check CHECK (schema_version = 1);

DROP TABLE IF EXISTS lm_wiki_source_wiki_page;
DROP TABLE IF EXISTS lm_wiki_source_policy;
DROP TABLE IF EXISTS wiki_page_edit_proposal;
DROP TABLE IF EXISTS wiki_page_revision;

ALTER TABLE wiki_page
    DROP COLUMN IF EXISTS last_actor_id,
    DROP COLUMN IF EXISTS last_actor_type,
    DROP COLUMN IF EXISTS last_source_kind,
    DROP COLUMN IF EXISTS content_digest,
    DROP COLUMN IF EXISTS current_revision_id,
    DROP COLUMN IF EXISTS current_revision_number;
