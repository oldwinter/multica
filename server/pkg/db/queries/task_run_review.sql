-- name: LoadTaskRunReviewTask :one
SELECT
    task.id AS task_id,
    agent.workspace_id,
    task.agent_id,
    task.status
FROM agent_task_queue task
JOIN agent
  ON agent.id = task.agent_id
 AND agent.workspace_id = sqlc.arg(workspace_id)
WHERE task.id = sqlc.arg(task_id);

-- name: CreateTaskRunReview :one
INSERT INTO task_run_review (
    id, workspace_id, task_id, reviewer_id, outcome, target, skill_id,
    correction, reason, digest, created_at
)
SELECT
    sqlc.arg(id), sqlc.arg(workspace_id), task.id, sqlc.arg(reviewer_id),
    sqlc.arg(outcome), sqlc.arg(target), sqlc.narg(skill_id)::uuid,
    sqlc.narg(correction)::text, sqlc.arg(reason), sqlc.arg(digest),
    sqlc.arg(created_at)
FROM agent_task_queue task
JOIN agent
  ON agent.id = task.agent_id
 AND agent.workspace_id = sqlc.arg(workspace_id)
JOIN member reviewer
  ON reviewer.workspace_id = sqlc.arg(workspace_id)
 AND reviewer.user_id = sqlc.arg(reviewer_id)
WHERE task.id = sqlc.arg(task_id)
  AND task.completed_at IS NOT NULL
  AND task.status IN ('completed', 'failed', 'cancelled')
  AND (
      (sqlc.arg(target)::text = 'skill_procedure' AND EXISTS (
          SELECT 1 FROM skill
          WHERE skill.workspace_id = sqlc.arg(workspace_id)
            AND skill.id = sqlc.narg(skill_id)::uuid
      ))
      OR (sqlc.arg(target)::text <> 'skill_procedure' AND sqlc.narg(skill_id)::uuid IS NULL)
  )
RETURNING task_run_review.*;

-- name: LoadTaskRunReview :one
SELECT * FROM task_run_review
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: ListTaskRunReviewRecords :many
SELECT
    id, workspace_id, task_id, reviewer_id, outcome, target, skill_id,
    digest, created_at
FROM task_run_review
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (
      sqlc.narg(after_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: LoadManualRerun :one
SELECT
    rerun.id AS task_id,
    rerun_agent.workspace_id,
    source.id AS source_task_id,
    source_agent.workspace_id AS source_workspace_id,
    rerun.agent_id,
    source.agent_id AS source_agent_id,
    rerun.issue_id,
    source.issue_id AS source_issue_id,
    rerun.chat_session_id,
    source.chat_session_id AS source_chat_session_id,
    rerun.status,
    source.status AS source_status,
    rerun.rerun_of_task_id,
    rerun.retry_of_task_id,
    rerun.originator_user_id,
    rerun.originator_source,
    rerun.created_at
FROM agent_task_queue rerun
JOIN agent rerun_agent
  ON rerun_agent.id = rerun.agent_id
 AND rerun_agent.workspace_id = sqlc.arg(workspace_id)
JOIN agent_task_queue source
  ON source.id = rerun.rerun_of_task_id
JOIN agent source_agent
  ON source_agent.id = source.agent_id
 AND source_agent.workspace_id = sqlc.arg(workspace_id)
WHERE rerun.id = sqlc.arg(task_id)
  AND rerun.rerun_of_task_id IS NOT NULL
  AND rerun.retry_of_task_id IS NULL
  AND rerun.originator_user_id IS NOT NULL
  AND rerun.originator_source = 'direct_human'
  AND source.status IN ('completed', 'failed', 'cancelled')
  AND rerun.agent_id = source.agent_id
  AND rerun.issue_id IS NOT DISTINCT FROM source.issue_id
  AND rerun.chat_session_id IS NOT DISTINCT FROM source.chat_session_id;

-- name: ListManualReruns :many
SELECT
    rerun.id AS task_id,
    rerun_agent.workspace_id,
    source.id AS source_task_id,
    source_agent.workspace_id AS source_workspace_id,
    rerun.agent_id,
    source.agent_id AS source_agent_id,
    rerun.issue_id,
    source.issue_id AS source_issue_id,
    rerun.chat_session_id,
    source.chat_session_id AS source_chat_session_id,
    rerun.status,
    source.status AS source_status,
    rerun.rerun_of_task_id,
    rerun.retry_of_task_id,
    rerun.originator_user_id,
    rerun.originator_source,
    rerun.created_at
FROM agent_task_queue rerun
JOIN agent rerun_agent
  ON rerun_agent.id = rerun.agent_id
 AND rerun_agent.workspace_id = sqlc.arg(workspace_id)
JOIN agent_task_queue source
  ON source.id = rerun.rerun_of_task_id
JOIN agent source_agent
  ON source_agent.id = source.agent_id
 AND source_agent.workspace_id = sqlc.arg(workspace_id)
WHERE rerun.rerun_of_task_id IS NOT NULL
  AND rerun.retry_of_task_id IS NULL
  AND rerun.originator_user_id IS NOT NULL
  AND rerun.originator_source = 'direct_human'
  AND source.status IN ('completed', 'failed', 'cancelled')
  AND rerun.agent_id = source.agent_id
  AND rerun.issue_id IS NOT DISTINCT FROM source.issue_id
  AND rerun.chat_session_id IS NOT DISTINCT FROM source.chat_session_id
  AND (
      sqlc.narg(after_created_at)::timestamptz IS NULL
      OR (rerun.created_at, rerun.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY rerun.created_at DESC, rerun.id DESC
LIMIT sqlc.arg(page_size);
