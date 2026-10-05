ALTER TABLE inbox_item
    ADD COLUMN IF NOT EXISTS room_id UUID,
    ADD COLUMN IF NOT EXISTS room_cycle_id UUID,
    ADD COLUMN IF NOT EXISTS room_review_identity TEXT,
    ADD COLUMN IF NOT EXISTS room_attention_key TEXT;
