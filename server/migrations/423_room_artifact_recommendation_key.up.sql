ALTER TABLE room_artifact
    ADD COLUMN recommendation_key TEXT NULL
        CHECK (recommendation_key IS NULL OR recommendation_key ~ '^sha256:[0-9a-f]{64}$');
