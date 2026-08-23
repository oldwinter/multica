-- name: ListTwinBindings :many
SELECT *
FROM twin_binding
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY scope_type, scope_id;

-- name: GetTwinExecutionMetrics :one
SELECT
    (SELECT count(*) FROM twin_task_attribution WHERE twin_task_attribution.workspace_id = sqlc.arg(workspace_id)) AS attributed_runs,
    (SELECT count(*) FROM twin_run_feedback WHERE twin_run_feedback.workspace_id = sqlc.arg(workspace_id)) AS feedback_total,
    (SELECT count(*) FILTER (WHERE rating = 'helped') FROM twin_run_feedback WHERE twin_run_feedback.workspace_id = sqlc.arg(workspace_id)) AS feedback_helped,
    (SELECT count(*) FILTER (WHERE rating = 'irrelevant') FROM twin_run_feedback WHERE twin_run_feedback.workspace_id = sqlc.arg(workspace_id)) AS feedback_irrelevant,
    (SELECT count(*) FILTER (WHERE rating = 'mismatch') FROM twin_run_feedback WHERE twin_run_feedback.workspace_id = sqlc.arg(workspace_id)) AS feedback_mismatch,
    (SELECT count(*) FROM twin_deposition WHERE twin_deposition.workspace_id = sqlc.arg(workspace_id)) AS depositions_total,
    (SELECT count(*) FILTER (WHERE state = 'pending') FROM twin_deposition WHERE twin_deposition.workspace_id = sqlc.arg(workspace_id)) AS depositions_pending,
    (SELECT count(*) FILTER (WHERE state = 'accepted') FROM twin_deposition WHERE twin_deposition.workspace_id = sqlc.arg(workspace_id)) AS depositions_accepted,
    (SELECT count(*) FILTER (WHERE state = 'rejected') FROM twin_deposition WHERE twin_deposition.workspace_id = sqlc.arg(workspace_id)) AS depositions_rejected,
    (SELECT count(*) FILTER (WHERE state = 'off') FROM twin_binding WHERE twin_binding.workspace_id = sqlc.arg(workspace_id)) AS bindings_off,
    (SELECT count(*) FILTER (WHERE state = 'preview') FROM twin_binding WHERE twin_binding.workspace_id = sqlc.arg(workspace_id)) AS bindings_preview,
    (SELECT count(*) FILTER (WHERE state = 'enabled') FROM twin_binding WHERE twin_binding.workspace_id = sqlc.arg(workspace_id)) AS bindings_enabled;

-- name: UpsertTwinBinding :one
INSERT INTO twin_binding (
    workspace_id, scope_type, scope_id, state, twin_version_id
)
SELECT sqlc.arg(workspace_id), sqlc.arg(scope_type), sqlc.arg(scope_id),
       sqlc.arg(state), version.id
FROM twin_version version
WHERE version.workspace_id = sqlc.arg(workspace_id)
  AND version.id = sqlc.arg(twin_version_id)
  AND (
      (sqlc.arg(scope_type)::text = 'workspace' AND sqlc.arg(scope_id)::uuid = sqlc.arg(workspace_id)::uuid)
      OR (sqlc.arg(scope_type)::text = 'agent' AND EXISTS (
          SELECT 1 FROM agent
          WHERE agent.workspace_id = sqlc.arg(workspace_id)
            AND agent.id = sqlc.arg(scope_id)
      ))
      OR (sqlc.arg(scope_type)::text = 'project' AND EXISTS (
          SELECT 1 FROM project
          WHERE project.workspace_id = sqlc.arg(workspace_id)
            AND project.id = sqlc.arg(scope_id)
          FOR KEY SHARE
      ))
      OR (sqlc.arg(scope_type)::text = 'issue' AND EXISTS (
          SELECT 1 FROM issue
          WHERE issue.workspace_id = sqlc.arg(workspace_id)
            AND issue.id = sqlc.arg(scope_id)
          FOR KEY SHARE
      ))
  )
ON CONFLICT (workspace_id, scope_type, scope_id) DO UPDATE
SET state = EXCLUDED.state,
    twin_version_id = EXCLUDED.twin_version_id,
    updated_at = now()
RETURNING *;

-- name: DeleteTwinBinding :execrows
DELETE FROM twin_binding
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: DeleteTwinBindingsForScope :exec
-- Project and issue deletion lock the scope row FOR UPDATE before calling this
-- in the same transaction. The matching FOR KEY SHARE in UpsertTwinBinding
-- prevents a validated binding from committing after this sweep.
DELETE FROM twin_binding
WHERE workspace_id = sqlc.arg(workspace_id)
  AND scope_type = sqlc.arg(scope_type)
  AND scope_id = sqlc.arg(scope_id);

-- name: CreateTwinTaskAttributionForClaim :one
WITH inserted AS (
    INSERT INTO twin_task_attribution (
        workspace_id, task_id, agent_id, runtime_id, task_dispatched_at,
        twin_version_id, briefing, briefing_digest, assertion_ids,
        citation_keys, policy_scope_type, policy_scope_id, policy_state,
        compiler_version
    )
    SELECT sqlc.arg(workspace_id), sqlc.arg(task_id), sqlc.arg(agent_id),
           sqlc.arg(runtime_id), sqlc.arg(task_dispatched_at), version.id,
           sqlc.arg(briefing),
           sqlc.arg(briefing_digest), sqlc.arg(assertion_ids),
           sqlc.arg(citation_keys), sqlc.arg(policy_scope_type),
           sqlc.arg(policy_scope_id), sqlc.arg(policy_state),
           sqlc.arg(compiler_version)
    FROM twin_version version
    WHERE version.id = sqlc.arg(twin_version_id)
      AND version.workspace_id = sqlc.arg(workspace_id)
      AND (
          (sqlc.arg(policy_scope_type)::text = 'workspace' AND sqlc.arg(policy_scope_id)::uuid = sqlc.arg(workspace_id)::uuid)
          OR (sqlc.arg(policy_scope_type)::text = 'one_off' AND sqlc.arg(policy_scope_id)::uuid = sqlc.arg(task_id)::uuid)
          OR (sqlc.arg(policy_scope_type)::text = 'agent' AND sqlc.arg(policy_scope_id)::uuid = sqlc.arg(agent_id)::uuid)
          OR sqlc.arg(policy_scope_type)::text IN ('project', 'issue')
      )
    ON CONFLICT (workspace_id, task_id, runtime_id, task_dispatched_at) DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT attribution.*
FROM twin_task_attribution attribution
WHERE attribution.workspace_id = sqlc.arg(workspace_id)
  AND attribution.task_id = sqlc.arg(task_id)
  AND attribution.agent_id = sqlc.arg(agent_id)
  AND attribution.runtime_id = sqlc.arg(runtime_id)
  AND attribution.task_dispatched_at = sqlc.arg(task_dispatched_at)
  AND attribution.twin_version_id = sqlc.arg(twin_version_id)
  AND attribution.briefing = sqlc.arg(briefing)
  AND attribution.briefing_digest = sqlc.arg(briefing_digest)
  AND attribution.assertion_ids = sqlc.arg(assertion_ids)
  AND attribution.citation_keys = sqlc.arg(citation_keys)
  AND attribution.policy_scope_type = sqlc.arg(policy_scope_type)
  AND attribution.policy_scope_id = sqlc.arg(policy_scope_id)
  AND attribution.policy_state = sqlc.arg(policy_state)
  AND attribution.compiler_version = sqlc.arg(compiler_version)
LIMIT 1;

-- name: GetTwinTaskAttributionByClaim :one
SELECT *
FROM twin_task_attribution
WHERE workspace_id = sqlc.arg(workspace_id)
  AND task_id = sqlc.arg(task_id)
  AND runtime_id = sqlc.arg(runtime_id)
  AND task_dispatched_at = sqlc.arg(task_dispatched_at);

-- name: GetTwinTaskAttribution :one
SELECT *
FROM twin_task_attribution
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: ListTwinTaskAttributions :many
SELECT *
FROM twin_task_attribution
WHERE workspace_id = sqlc.arg(workspace_id)
  AND task_id = sqlc.arg(task_id)
ORDER BY created_at DESC, id DESC;

-- name: UpsertTwinRunFeedback :one
INSERT INTO twin_run_feedback (workspace_id, task_id, rating, note)
SELECT sqlc.arg(workspace_id), sqlc.arg(task_id), sqlc.arg(rating), sqlc.narg(note)
WHERE EXISTS (
    SELECT 1
    FROM twin_task_attribution attribution
    WHERE attribution.workspace_id = sqlc.arg(workspace_id)
      AND attribution.task_id = sqlc.arg(task_id)
)
ON CONFLICT (workspace_id, task_id) DO UPDATE
SET rating = EXCLUDED.rating,
    note = EXCLUDED.note,
    updated_at = now()
RETURNING *;

-- name: GetTwinRunFeedback :one
SELECT *
FROM twin_run_feedback
WHERE workspace_id = sqlc.arg(workspace_id)
  AND task_id = sqlc.arg(task_id);

-- name: LinkTwinDeposition :one
WITH inserted AS (
    INSERT INTO twin_deposition (
        workspace_id, task_id, base_twin_version_id, proposal_id,
        replaces_proposal_id, evidence_digest, edited_assertions_digest, state
    )
    SELECT sqlc.arg(workspace_id), sqlc.arg(task_id), base.id, proposal.id,
           sqlc.narg(replaces_proposal_id), sqlc.arg(evidence_digest),
           sqlc.arg(edited_assertions_digest), 'pending'
    FROM twin_version base
    JOIN twin_proposal proposal
      ON proposal.workspace_id = base.workspace_id
     AND proposal.id = sqlc.arg(proposal_id)
     AND proposal.base_twin_version_id = base.id
    WHERE base.workspace_id = sqlc.arg(workspace_id)
      AND base.id = sqlc.arg(base_twin_version_id)
      AND EXISTS (
          SELECT 1
          FROM twin_task_attribution attribution
          WHERE attribution.workspace_id = sqlc.arg(workspace_id)
            AND attribution.task_id = sqlc.arg(task_id)
            AND attribution.twin_version_id = base.id
      )
    ON CONFLICT (
        workspace_id, task_id, base_twin_version_id,
        (COALESCE(replaces_proposal_id, '00000000-0000-0000-0000-000000000000'::uuid)),
        evidence_digest
    ) DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT deposition.*
FROM twin_deposition deposition
WHERE deposition.workspace_id = sqlc.arg(workspace_id)
  AND deposition.task_id = sqlc.arg(task_id)
  AND deposition.base_twin_version_id = sqlc.arg(base_twin_version_id)
  AND deposition.replaces_proposal_id IS NOT DISTINCT FROM sqlc.narg(replaces_proposal_id)
  AND deposition.evidence_digest = sqlc.arg(evidence_digest)
LIMIT 1;

-- name: GetTwinDepositionByProposal :one
SELECT *
FROM twin_deposition
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id);

-- name: GetTwinDeposition :one
SELECT *
FROM twin_deposition
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: ListTwinDepositionsForTask :many
SELECT *
FROM twin_deposition
WHERE workspace_id = sqlc.arg(workspace_id)
  AND task_id = sqlc.arg(task_id)
ORDER BY created_at DESC, id DESC;

-- name: UpdateTwinDepositionState :one
UPDATE twin_deposition
SET state = sqlc.arg(state), updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND (state = 'pending' OR state = sqlc.arg(state))
RETURNING *;

-- name: RejectReviewedTwinDepositions :exec
UPDATE twin_deposition deposition
SET state = 'rejected', updated_at = now()
WHERE deposition.workspace_id = sqlc.arg(workspace_id)
  AND deposition.state = 'pending'
  AND EXISTS (
      SELECT 1
      FROM twin_proposal_review review
      WHERE review.workspace_id = deposition.workspace_id
        AND review.proposal_id = deposition.proposal_id
        AND review.decision = 'rejected'
  );

-- name: DeleteWorkspaceTwinExecutionData :exec
WITH deleted_feedback AS (
    DELETE FROM twin_run_feedback WHERE workspace_id = sqlc.arg(workspace_id)
), deleted_depositions AS (
    DELETE FROM twin_deposition WHERE workspace_id = sqlc.arg(workspace_id)
), deleted_attributions AS (
    DELETE FROM twin_task_attribution WHERE workspace_id = sqlc.arg(workspace_id)
)
DELETE FROM twin_binding WHERE twin_binding.workspace_id = sqlc.arg(workspace_id);
