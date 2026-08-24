ALTER TABLE room_memory_revision
    ADD COLUMN creator_type TEXT NULL,
    ADD COLUMN creator_id UUID NULL;

UPDATE room_memory_revision AS revision
SET creator_type = 'agent',
    creator_id = turn.agent_id
FROM room_turn AS turn
WHERE revision.corrected_from_revision_id IS NULL
  AND turn.id = revision.synthesis_turn_id
  AND turn.workspace_id = revision.workspace_id
  AND turn.room_id = revision.room_id;

UPDATE room_memory_revision AS correction
SET creator_type = 'member',
    creator_id = original.reviewed_by_user_id
FROM room_memory_revision AS original
WHERE correction.corrected_from_revision_id = original.id
  AND correction.workspace_id = original.workspace_id
  AND correction.room_id = original.room_id
  AND original.reviewed_by_user_id IS NOT NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM room_memory_revision
        WHERE creator_type IS NULL OR creator_id IS NULL
    ) THEN
        RAISE EXCEPTION 'room_memory_revision creator backfill is incomplete';
    END IF;
END $$;

ALTER TABLE room_memory_revision
    ALTER COLUMN creator_type SET NOT NULL,
    ALTER COLUMN creator_id SET NOT NULL,
    ADD CONSTRAINT room_memory_revision_creator_type_check
        CHECK (creator_type IN ('member', 'agent'));
