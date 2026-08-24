ALTER TABLE room_cycle
    ADD COLUMN cost_limit_ticks BIGINT NULL
        CHECK (cost_limit_ticks IS NULL OR cost_limit_ticks > 0);
