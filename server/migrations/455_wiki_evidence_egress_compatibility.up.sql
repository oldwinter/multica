-- Migration 414 was published under an earlier filename before these evidence
-- egress columns were added to that migration in place. Repair databases that
-- applied the original shape without replaying the rest of the Wiki DDL.
ALTER TABLE lm_wiki_source_policy
    ADD COLUMN IF NOT EXISTS remote_generation_enabled BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE lm_wiki_revision
    ADD COLUMN IF NOT EXISTS source_policy_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS source_policy_digest TEXT NOT NULL
        DEFAULT 'sha256:0000000000000000000000000000000000000000000000000000000000000000',
    ADD COLUMN IF NOT EXISTS remote_generation_enabled BOOLEAN NOT NULL DEFAULT false;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'lm_wiki_revision'::regclass
          AND conname = 'lm_wiki_revision_source_policy_version_check'
    ) THEN
        ALTER TABLE lm_wiki_revision
            ADD CONSTRAINT lm_wiki_revision_source_policy_version_check
            CHECK (source_policy_version >= 0) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'lm_wiki_revision'::regclass
          AND conname = 'lm_wiki_revision_source_policy_digest_check'
    ) THEN
        ALTER TABLE lm_wiki_revision
            ADD CONSTRAINT lm_wiki_revision_source_policy_digest_check
            CHECK (source_policy_digest ~ '^sha256:[0-9a-f]{64}$') NOT VALID;
    END IF;
END
$$;

ALTER TABLE lm_wiki_revision
    VALIDATE CONSTRAINT lm_wiki_revision_source_policy_version_check;

ALTER TABLE lm_wiki_revision
    VALIDATE CONSTRAINT lm_wiki_revision_source_policy_digest_check;
