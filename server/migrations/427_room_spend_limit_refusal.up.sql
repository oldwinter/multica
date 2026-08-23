ALTER TABLE room_cycle
    DROP CONSTRAINT room_cycle_refusal_reason_check,
    ADD CONSTRAINT room_cycle_refusal_reason_check CHECK (
        refusal_reason IS NULL OR refusal_reason IN (
            'room_paused',
            'room_archived',
            'budget_exhausted',
            'cycle_active',
            'no_targets',
            'invocation_not_allowed',
            'spend_limit_unsupported'
        )
    );
