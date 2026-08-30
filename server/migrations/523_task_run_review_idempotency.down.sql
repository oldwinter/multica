ALTER TABLE task_run_review
    DROP CONSTRAINT IF EXISTS task_run_review_idempotency_key_length_chk,
    DROP COLUMN IF EXISTS idempotency_key;
