ALTER TABLE room_memory_revision
    DROP CONSTRAINT IF EXISTS room_memory_revision_creator_type_check,
    DROP COLUMN IF EXISTS creator_id,
    DROP COLUMN IF EXISTS creator_type;
