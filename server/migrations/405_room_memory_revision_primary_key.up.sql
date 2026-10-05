DO $$
DECLARE
    id_attnum SMALLINT;
    constraint_matches BOOLEAN;
BEGIN
    SELECT attnum::SMALLINT
    INTO id_attnum
    FROM pg_attribute
    WHERE attrelid = 'room_memory_revision'::regclass
      AND attname = 'id'
      AND NOT attisdropped;

    IF id_attnum IS NULL THEN
        RAISE EXCEPTION 'room_memory_revision.id is missing';
    END IF;

    SELECT c.contype = 'p'
           AND c.conkey = ARRAY[id_attnum]::SMALLINT[]
           AND c.convalidated
           AND NOT c.condeferrable
           AND NOT c.condeferred
           AND i.indisunique
           AND i.indisvalid
           AND i.indisready
           AND i.indislive
           AND i.indpred IS NULL
           AND i.indexprs IS NULL
    INTO constraint_matches
    FROM pg_constraint c
    JOIN pg_index i ON i.indexrelid = c.conindid
    WHERE c.conrelid = 'room_memory_revision'::regclass
      AND c.conname = 'room_memory_revision_pkey';

    IF FOUND THEN
        IF NOT constraint_matches THEN
            RAISE EXCEPTION 'room_memory_revision_pkey exists with an incompatible definition; expected a validated non-deferrable primary key on room_memory_revision(id)';
        END IF;
        RETURN;
    END IF;

    ALTER TABLE room_memory_revision
        ADD CONSTRAINT room_memory_revision_pkey
        PRIMARY KEY USING INDEX room_memory_revision_id_uidx;
END
$$;
