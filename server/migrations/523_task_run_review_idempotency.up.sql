ALTER TABLE task_run_review
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

UPDATE task_run_review
SET idempotency_key = 'legacy:' || id::text
WHERE idempotency_key IS NULL;

ALTER TABLE task_run_review
    ALTER COLUMN idempotency_key SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'task_run_review_idempotency_key_length_chk'
          AND conrelid = 'task_run_review'::regclass
    ) THEN
        ALTER TABLE task_run_review
            ADD CONSTRAINT task_run_review_idempotency_key_length_chk
            CHECK (octet_length(idempotency_key) BETWEEN 1 AND 200);
    END IF;
END
$$;
