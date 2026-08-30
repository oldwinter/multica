DO $$
DECLARE
    id_attnum SMALLINT;
    constraint_matches BOOLEAN;
    index_matches BOOLEAN;
BEGIN
    SELECT attnum::SMALLINT
    INTO id_attnum
    FROM pg_attribute
    WHERE attrelid = 'skill_evolution_task_dispatch_snapshot'::regclass
      AND attname = 'id'
      AND NOT attisdropped;

    IF id_attnum IS NULL THEN
        RAISE EXCEPTION 'skill_evolution_task_dispatch_snapshot.id is missing';
    END IF;

    SELECT c.contype = 'p'
           AND c.conkey = ARRAY[id_attnum]::SMALLINT[]
           AND c.convalidated
           AND NOT c.condeferrable
           AND NOT c.condeferred
           AND i.indnatts = 1
           AND i.indnkeyatts = 1
           AND i.indkey[0] = id_attnum
           AND i.indisunique
           AND i.indisprimary
           AND i.indisvalid
           AND i.indisready
           AND i.indislive
           AND i.indpred IS NULL
           AND i.indexprs IS NULL
           AND am.amname = 'btree'
    INTO constraint_matches
    FROM pg_constraint c
    JOIN pg_index i ON i.indexrelid = c.conindid
    JOIN pg_class index_relation ON index_relation.oid = c.conindid
    JOIN pg_am am ON am.oid = index_relation.relam
    WHERE c.conrelid = 'skill_evolution_task_dispatch_snapshot'::regclass
      AND c.conname = 'skill_evolution_task_dispatch_snapshot_pkey';

    IF FOUND THEN
        IF NOT constraint_matches THEN
            RAISE EXCEPTION 'skill_evolution_task_dispatch_snapshot_pkey exists with an incompatible definition; expected a validated non-deferrable primary key on skill_evolution_task_dispatch_snapshot(id)';
        END IF;
        RETURN;
    END IF;

    SELECT i.indrelid = 'skill_evolution_task_dispatch_snapshot'::regclass
           AND i.indnatts = 1
           AND i.indnkeyatts = 1
           AND i.indkey[0] = id_attnum
           AND i.indisunique
           AND i.indisvalid
           AND i.indisready
           AND i.indislive
           AND NOT i.indisprimary
           AND i.indpred IS NULL
           AND i.indexprs IS NULL
           AND am.amname = 'btree'
    INTO index_matches
    FROM pg_class index_relation
    JOIN pg_index i ON i.indexrelid = index_relation.oid
    JOIN pg_am am ON am.oid = index_relation.relam
    WHERE index_relation.oid = to_regclass('skill_evolution_task_dispatch_snapshot_id_uidx');

    IF NOT FOUND OR NOT index_matches THEN
        RAISE EXCEPTION 'skill_evolution_task_dispatch_snapshot_id_uidx is missing or incompatible; expected a valid unique btree index on skill_evolution_task_dispatch_snapshot(id)';
    END IF;

    ALTER TABLE skill_evolution_task_dispatch_snapshot
        ADD CONSTRAINT skill_evolution_task_dispatch_snapshot_pkey
        PRIMARY KEY USING INDEX skill_evolution_task_dispatch_snapshot_id_uidx;
END
$$;
