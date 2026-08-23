CREATE TABLE room_recommendation_review (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    room_id UUID NOT NULL,
    memory_revision_id UUID NOT NULL,
    recommendation_key TEXT NOT NULL CHECK (recommendation_key ~ '^sha256:[0-9a-f]{64}$'),
    status TEXT NOT NULL CHECK (status IN ('approved', 'rejected')),
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    request_digest TEXT NOT NULL CHECK (request_digest ~ '^sha256:[0-9a-f]{64}$'),
    artifact_id UUID NULL,
    reviewed_by_user_id UUID NOT NULL,
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'approved') = (artifact_id IS NOT NULL))
);
