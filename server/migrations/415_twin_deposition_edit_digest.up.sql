ALTER TABLE twin_deposition
    ADD COLUMN IF NOT EXISTS edited_assertions_digest TEXT NOT NULL DEFAULT '';
