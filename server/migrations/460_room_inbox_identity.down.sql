ALTER TABLE inbox_item
    DROP COLUMN IF EXISTS room_attention_key,
    DROP COLUMN IF EXISTS room_review_identity,
    DROP COLUMN IF EXISTS room_cycle_id,
    DROP COLUMN IF EXISTS room_id;
