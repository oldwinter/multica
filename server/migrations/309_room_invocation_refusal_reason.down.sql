UPDATE room_cycle
SET refusal_reason = NULL
WHERE refusal_reason = 'invocation_not_allowed';

ALTER TABLE room_cycle
    DROP CONSTRAINT room_cycle_refusal_reason_check,
    ADD CONSTRAINT room_cycle_refusal_reason_check CHECK (
        refusal_reason IS NULL OR refusal_reason IN (
            'room_paused',
            'room_archived',
            'budget_exhausted',
            'cycle_active',
            'no_targets'
        )
    );
