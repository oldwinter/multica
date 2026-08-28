ALTER TABLE room_artifact
    DROP CONSTRAINT room_artifact_kind_check,
    ADD CONSTRAINT room_artifact_kind_check CHECK (kind IN (
        'issue', 'wiki', 'decision',
        'knowledge', 'preference', 'constraint', 'executable_procedure',
        'implementation_defect', 'unsupported'
    )) NOT VALID;
