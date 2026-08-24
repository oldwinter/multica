DO $$
DECLARE
    id_attnum SMALLINT;
    constraint_matches BOOLEAN;
BEGIN
    SELECT attnum::SMALLINT
    INTO id_attnum
    FROM pg_attribute
    WHERE attrelid = 'room_recommendation_review'::regclass
      AND attname = 'id'
      AND NOT attisdropped;

    IF id_attnum IS NULL THEN
        RAISE EXCEPTION 'room_recommendation_review.id is missing';
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
    WHERE c.conrelid = 'room_recommendation_review'::regclass
      AND c.conname = 'room_recommendation_review_pkey';

    IF FOUND THEN
        IF NOT constraint_matches THEN
            RAISE EXCEPTION 'room_recommendation_review_pkey exists with an incompatible definition; expected a validated non-deferrable primary key on room_recommendation_review(id)';
        END IF;
        RETURN;
    END IF;

    ALTER TABLE room_recommendation_review
        ADD CONSTRAINT room_recommendation_review_pkey
        PRIMARY KEY USING INDEX room_recommendation_review_id_uidx;
END
$$;
