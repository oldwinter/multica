-- Account-scoped appearance preferences. NULL across all four columns means
-- that this account has never synced an explicit choice, allowing an existing
-- local preference to be imported on first authenticated sync.
ALTER TABLE "user"
    ADD COLUMN skin TEXT,
    ADD COLUMN appearance TEXT,
    ADD COLUMN appearance_updated_at TIMESTAMPTZ,
    ADD COLUMN appearance_token_version INTEGER,
    ADD CONSTRAINT user_skin_validate
        CHECK (skin IS NULL OR skin IN ('tension', 'relay', 'field')),
    ADD CONSTRAINT user_appearance_validate
        CHECK (appearance IS NULL OR appearance IN ('system', 'light', 'dark')),
    ADD CONSTRAINT user_appearance_token_version_validate
        CHECK (appearance_token_version IS NULL OR appearance_token_version = 1),
    ADD CONSTRAINT user_appearance_preferences_complete
        CHECK (
            (skin IS NULL AND appearance IS NULL AND appearance_updated_at IS NULL AND appearance_token_version IS NULL)
            OR
            (skin IS NOT NULL AND appearance IS NOT NULL AND appearance_updated_at IS NOT NULL AND appearance_token_version IS NOT NULL)
        );
