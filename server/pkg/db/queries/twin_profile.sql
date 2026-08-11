-- Twin profile metadata is workspace-scoped. Raw evidence, local artifacts,
-- paths, logs, and secrets are intentionally not represented here.

-- name: GetTwinProfileByWorkspace :one
SELECT id,
       workspace_id,
       name,
       state,
       review_digest,
       source_count,
       assertion_count,
       skill_count,
       rule_count,
       assertions,
       topics,
       review_steps,
       created_at,
       updated_at
FROM twin_profile
WHERE workspace_id = $1;

-- name: UpsertSignedTwinProfile :one
INSERT INTO twin_profile (
    workspace_id,
    name,
    state,
    review_digest,
    review_steps
) VALUES (
    sqlc.arg(workspace_id),
    'Workspace Twin',
    'signed-off',
    sqlc.arg(review_digest),
    sqlc.arg(review_steps)::jsonb
)
ON CONFLICT (workspace_id) DO UPDATE
SET state = EXCLUDED.state,
    review_digest = EXCLUDED.review_digest,
    review_steps = EXCLUDED.review_steps,
    updated_at = now()
RETURNING *;
