-- name: UpsertSkillEvolutionLoop :one
INSERT INTO skill_evolution_loop (
    workspace_id, skill_id, enabled, mode, cooldown_seconds, minimum_signals,
    max_evidence_refs, max_replay_samples, max_cost_usd_ticks, policy_version,
    next_eligible_at
)
SELECT
    sqlc.arg(workspace_id), skill.id, sqlc.arg(enabled), sqlc.arg(mode),
    sqlc.arg(cooldown_seconds), sqlc.arg(minimum_signals),
    sqlc.arg(max_evidence_refs), sqlc.arg(max_replay_samples),
    sqlc.arg(max_cost_usd_ticks), sqlc.arg(policy_version),
    sqlc.narg(next_eligible_at)::timestamptz
FROM skill
WHERE skill.workspace_id = sqlc.arg(workspace_id)
  AND skill.id = sqlc.arg(skill_id)
ON CONFLICT (workspace_id, skill_id) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    mode = EXCLUDED.mode,
    cooldown_seconds = EXCLUDED.cooldown_seconds,
    minimum_signals = EXCLUDED.minimum_signals,
    max_evidence_refs = EXCLUDED.max_evidence_refs,
    max_replay_samples = EXCLUDED.max_replay_samples,
    max_cost_usd_ticks = EXCLUDED.max_cost_usd_ticks,
    policy_version = EXCLUDED.policy_version,
    next_eligible_at = EXCLUDED.next_eligible_at,
    updated_at = now()
RETURNING skill_evolution_loop.*;

-- name: GetSkillEvolutionLoop :one
SELECT * FROM skill_evolution_loop
WHERE workspace_id = sqlc.arg(workspace_id)
  AND skill_id = sqlc.arg(skill_id);

-- name: ListEligibleSkillEvolutionLoops :many
SELECT * FROM skill_evolution_loop
WHERE enabled
  AND mode = 'propose'
  AND (next_eligible_at IS NULL OR next_eligible_at <= sqlc.arg(eligible_at)::timestamptz)
  AND (sqlc.narg(after_id)::uuid IS NULL OR id > sqlc.narg(after_id)::uuid)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListScheduledSkillEvolutionLoops :many
SELECT * FROM skill_evolution_loop
WHERE enabled
  AND mode IN ('observe', 'propose')
  AND (next_eligible_at IS NULL OR next_eligible_at <= sqlc.arg(eligible_at)::timestamptz)
  AND (sqlc.narg(after_id)::uuid IS NULL OR id > sqlc.narg(after_id)::uuid)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: RecordSkillEvolutionLoopObservation :one
UPDATE skill_evolution_loop
SET last_observed_at = sqlc.arg(observed_at),
    next_eligible_at = sqlc.narg(next_eligible_at)::timestamptz,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
RETURNING *;

-- name: CreateSkillEvolutionRevision :one
INSERT INTO skill_evolution_revision (
    workspace_id, skill_id, kind, ownership_class, source, bundle_hash,
    metadata_digest, name, description, primary_content, byte_count,
    support_file_count, created_by_id
)
SELECT
    sqlc.arg(workspace_id), skill.id, sqlc.arg(kind), sqlc.arg(ownership_class),
    sqlc.arg(source), sqlc.arg(bundle_hash), sqlc.arg(metadata_digest),
    sqlc.arg(name), sqlc.arg(description), sqlc.arg(primary_content),
    sqlc.arg(byte_count), sqlc.arg(support_file_count), skill.created_by
FROM skill
WHERE skill.workspace_id = sqlc.arg(workspace_id)
  AND skill.id = sqlc.arg(skill_id)
  AND skill.created_by IS NOT DISTINCT FROM sqlc.narg(created_by_id)::uuid
ON CONFLICT (workspace_id, skill_id, bundle_hash) DO NOTHING
RETURNING skill_evolution_revision.*;

-- name: GetSkillEvolutionRevision :one
SELECT * FROM skill_evolution_revision
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: GetSkillEvolutionRevisionByHash :one
SELECT * FROM skill_evolution_revision
WHERE workspace_id = sqlc.arg(workspace_id)
  AND skill_id = sqlc.arg(skill_id)
  AND bundle_hash = sqlc.arg(bundle_hash);

-- name: ListSkillEvolutionRevisions :many
SELECT * FROM skill_evolution_revision
WHERE workspace_id = sqlc.arg(workspace_id)
  AND skill_id = sqlc.arg(skill_id)
  AND (
      sqlc.narg(after_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: CreateSkillEvolutionRevisionFile :one
INSERT INTO skill_evolution_revision_file (
    workspace_id, revision_id, path, content, digest, byte_count
)
SELECT
    revision.workspace_id, revision.id, sqlc.arg(path), sqlc.arg(content),
    sqlc.arg(digest), sqlc.arg(byte_count)
FROM skill_evolution_revision revision
WHERE revision.workspace_id = sqlc.arg(workspace_id)
  AND revision.id = sqlc.arg(revision_id)
ON CONFLICT (workspace_id, revision_id, path) DO NOTHING
RETURNING skill_evolution_revision_file.*;

-- name: ListSkillEvolutionRevisionFiles :many
SELECT * FROM skill_evolution_revision_file
WHERE workspace_id = sqlc.arg(workspace_id)
  AND revision_id = sqlc.arg(revision_id)
ORDER BY path;

-- name: CreateSkillEvolutionProposal :one
INSERT INTO skill_evolution_proposal (
    workspace_id, skill_id, loop_id, base_revision_id, base_hash,
    generation_idempotency_key, requested_by_id
)
SELECT
    loop.workspace_id, loop.skill_id, loop.id, base.id, base.bundle_hash,
    sqlc.arg(generation_idempotency_key), sqlc.narg(requested_by_id)::uuid
FROM skill_evolution_loop loop
JOIN skill_evolution_revision base
  ON base.workspace_id = loop.workspace_id
 AND base.skill_id = loop.skill_id
 AND base.id = sqlc.arg(base_revision_id)
 AND base.bundle_hash = sqlc.arg(base_hash)
WHERE loop.workspace_id = sqlc.arg(workspace_id)
  AND loop.skill_id = sqlc.arg(skill_id)
  AND loop.id = sqlc.arg(loop_id)
  AND (
      sqlc.narg(requested_by_id)::uuid IS NULL
      OR EXISTS (
          SELECT 1 FROM member requester
          WHERE requester.workspace_id = loop.workspace_id
            AND requester.user_id = sqlc.narg(requested_by_id)::uuid
      )
  )
ON CONFLICT DO NOTHING
RETURNING skill_evolution_proposal.*;

-- name: GetSkillEvolutionProposal :one
SELECT * FROM skill_evolution_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: GetSkillEvolutionProposalByGenerationKey :one
SELECT * FROM skill_evolution_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND skill_id = sqlc.arg(skill_id)
  AND generation_idempotency_key = sqlc.arg(generation_idempotency_key);

-- name: ListSkillEvolutionProposals :many
SELECT id, workspace_id, skill_id, loop_id, state, base_revision_id,
       candidate_revision_id, base_hash, candidate_hash, rationale_digest,
       failure_reason, stale_reason, generation_idempotency_key,
       requested_by_id, started_at, completed_at, created_at, updated_at
FROM skill_evolution_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND skill_id = sqlc.arg(skill_id)
  AND (
      sqlc.narg(after_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: TransitionSkillEvolutionProposal :one
UPDATE skill_evolution_proposal proposal
SET state = sqlc.arg(next_state),
    candidate_revision_id = COALESCE(sqlc.narg(candidate_revision_id)::uuid, proposal.candidate_revision_id),
    candidate_hash = COALESCE(sqlc.narg(candidate_hash)::text, proposal.candidate_hash),
    rationale_digest = COALESCE(sqlc.narg(rationale_digest)::text, proposal.rationale_digest),
    observed_pattern = COALESCE(sqlc.narg(observed_pattern)::text, proposal.observed_pattern),
    expected_benefit = COALESCE(sqlc.narg(expected_benefit)::text, proposal.expected_benefit),
    regression_risk = COALESCE(sqlc.narg(regression_risk)::text, proposal.regression_risk),
    failure_reason = sqlc.narg(failure_reason)::text,
    stale_reason = sqlc.narg(stale_reason)::text,
    started_at = CASE WHEN sqlc.arg(next_state)::text = 'running' THEN COALESCE(proposal.started_at, now()) ELSE proposal.started_at END,
    completed_at = CASE WHEN sqlc.arg(next_state)::text IN ('failed', 'stale', 'rejected', 'published', 'publication_unknown') THEN now() ELSE proposal.completed_at END,
    updated_at = now()
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.id = sqlc.arg(id)
  AND proposal.state = sqlc.arg(expected_state)
  AND (
      sqlc.narg(candidate_revision_id)::uuid IS NULL
      OR EXISTS (
          SELECT 1 FROM skill_evolution_revision revision
          WHERE revision.workspace_id = proposal.workspace_id
            AND revision.skill_id = proposal.skill_id
            AND revision.id = sqlc.narg(candidate_revision_id)::uuid
            AND revision.bundle_hash = sqlc.narg(candidate_hash)::text
      )
  )
RETURNING proposal.*;

-- name: CreateSkillEvolutionEvidence :one
INSERT INTO skill_evolution_evidence (
    workspace_id, proposal_id, kind, source_id, source_revision_id,
    target_skill_id, source_state, digest, eligibility, observed_at
)
SELECT
    proposal.workspace_id, proposal.id, sqlc.arg(kind), sqlc.arg(source_id),
    sqlc.arg(source_revision_id), sqlc.narg(target_skill_id)::uuid,
    sqlc.arg(source_state), sqlc.arg(digest), sqlc.arg(eligibility), sqlc.arg(observed_at)
FROM skill_evolution_proposal proposal
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.id = sqlc.arg(proposal_id)
  AND (
      sqlc.narg(target_skill_id)::uuid IS NULL
      OR EXISTS (
          SELECT 1 FROM skill
          WHERE skill.workspace_id = proposal.workspace_id
            AND skill.id = proposal.skill_id
            AND skill.id = sqlc.narg(target_skill_id)::uuid
      )
  )
ON CONFLICT (workspace_id, proposal_id, kind, source_id, source_revision_id) DO NOTHING
RETURNING skill_evolution_evidence.*;

-- name: ListSkillEvolutionEvidence :many
SELECT * FROM skill_evolution_evidence
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id)
ORDER BY observed_at, id;

-- name: GetSkillEvolutionEvidenceByIdentity :one
SELECT * FROM skill_evolution_evidence
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id)
  AND kind = sqlc.arg(kind)
  AND source_id = sqlc.arg(source_id)
  AND source_revision_id = sqlc.arg(source_revision_id);

-- name: CreateSkillEvolutionEvaluation :one
INSERT INTO skill_evolution_evaluation (
    workspace_id, proposal_id, kind, result, adapter, adapter_version,
    policy_version, result_digest, safe_metrics, cost_usd_ticks, duration_ms,
    idempotency_key
)
SELECT
    proposal.workspace_id, proposal.id, sqlc.arg(kind), sqlc.arg(result),
    sqlc.arg(adapter), sqlc.arg(adapter_version), sqlc.arg(policy_version),
    sqlc.arg(result_digest), sqlc.arg(safe_metrics), sqlc.narg(cost_usd_ticks)::bigint,
    sqlc.arg(duration_ms), sqlc.arg(idempotency_key)
FROM skill_evolution_proposal proposal
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.id = sqlc.arg(proposal_id)
ON CONFLICT (workspace_id, proposal_id, idempotency_key) DO NOTHING
RETURNING skill_evolution_evaluation.*;

-- name: ListSkillEvolutionEvaluations :many
SELECT * FROM skill_evolution_evaluation
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id)
ORDER BY created_at, id;

-- name: GetSkillEvolutionEvaluationByIdempotencyKey :one
SELECT * FROM skill_evolution_evaluation
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: CreateSkillEvolutionReview :one
INSERT INTO skill_evolution_review (
    workspace_id, proposal_id, candidate_revision_id, decision, actor_id,
    reason, idempotency_key
)
SELECT
    proposal.workspace_id, proposal.id, proposal.candidate_revision_id,
    sqlc.arg(decision), sqlc.arg(actor_id), sqlc.narg(reason)::text,
    sqlc.arg(idempotency_key)
FROM skill_evolution_proposal proposal
JOIN skill_evolution_revision candidate
  ON candidate.workspace_id = proposal.workspace_id
 AND candidate.skill_id = proposal.skill_id
 AND candidate.id = proposal.candidate_revision_id
JOIN member actor
  ON actor.workspace_id = proposal.workspace_id
 AND actor.user_id = sqlc.arg(actor_id)
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.id = sqlc.arg(proposal_id)
  AND proposal.candidate_revision_id = sqlc.arg(candidate_revision_id)
ON CONFLICT (workspace_id, proposal_id, idempotency_key) DO NOTHING
RETURNING skill_evolution_review.*;

-- name: ListSkillEvolutionReviews :many
SELECT * FROM skill_evolution_review
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id)
ORDER BY created_at, id;

-- name: GetSkillEvolutionReviewByIdempotencyKey :one
SELECT * FROM skill_evolution_review
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: CreateSkillEvolutionRelease :one
INSERT INTO skill_evolution_release (
    workspace_id, skill_id, proposal_id, source_release_id, revision_id,
    kind, expected_base_hash, actor_id, idempotency_key
)
SELECT
    revision.workspace_id, revision.skill_id, sqlc.narg(proposal_id)::uuid,
    sqlc.narg(source_release_id)::uuid, revision.id, sqlc.arg(kind),
    sqlc.arg(expected_base_hash), sqlc.arg(actor_id), sqlc.arg(idempotency_key)
FROM skill_evolution_revision revision
JOIN member actor
  ON actor.workspace_id = revision.workspace_id
 AND actor.user_id = sqlc.arg(actor_id)
WHERE revision.workspace_id = sqlc.arg(workspace_id)
  AND revision.skill_id = sqlc.arg(skill_id)
  AND revision.id = sqlc.arg(revision_id)
  AND (
      (sqlc.arg(kind)::text = 'publish' AND EXISTS (
          SELECT 1 FROM skill_evolution_proposal proposal
          WHERE proposal.workspace_id = revision.workspace_id
            AND proposal.skill_id = revision.skill_id
            AND proposal.id = sqlc.narg(proposal_id)::uuid
            AND proposal.candidate_revision_id = revision.id
            AND proposal.base_hash = sqlc.arg(expected_base_hash)
            AND proposal.state = 'publishing'
            AND EXISTS (
                SELECT 1 FROM skill_evolution_review review
                WHERE review.workspace_id = proposal.workspace_id
                  AND review.proposal_id = proposal.id
                  AND review.candidate_revision_id = revision.id
                  AND review.decision = 'publish'
            )
      ))
      OR
      (sqlc.arg(kind)::text = 'rollback' AND EXISTS (
          SELECT 1 FROM skill_evolution_release release
          WHERE release.workspace_id = revision.workspace_id
            AND release.skill_id = revision.skill_id
            AND release.id = sqlc.narg(source_release_id)::uuid
            AND release.outcome = 'succeeded'
            AND release.pre_hash IS NOT NULL
            AND release.post_hash IS NOT NULL
            AND revision.bundle_hash = release.pre_hash
            AND sqlc.arg(expected_base_hash) = release.post_hash
      ))
  )
ON CONFLICT (workspace_id, idempotency_key) DO NOTHING
RETURNING skill_evolution_release.*;

-- name: GetSkillEvolutionRelease :one
SELECT * FROM skill_evolution_release
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: GetSkillEvolutionReleaseByIdempotencyKey :one
SELECT * FROM skill_evolution_release
WHERE workspace_id = sqlc.arg(workspace_id)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: TransitionSkillEvolutionRelease :one
UPDATE skill_evolution_release release
SET outcome = sqlc.arg(next_outcome),
    pre_hash = sqlc.narg(pre_hash)::text,
    post_hash = sqlc.narg(post_hash)::text,
    error_code = sqlc.narg(error_code)::text,
    completed_at = CASE WHEN sqlc.arg(next_outcome)::text <> 'pending' THEN now() ELSE NULL END
WHERE release.workspace_id = sqlc.arg(workspace_id)
  AND release.id = sqlc.arg(id)
  AND release.outcome = sqlc.arg(expected_outcome)
  AND (
      sqlc.arg(next_outcome)::text <> 'succeeded'
      OR (
          sqlc.narg(pre_hash)::text = release.expected_base_hash
          AND sqlc.narg(error_code)::text IS NULL
          AND EXISTS (
              SELECT 1 FROM skill_evolution_revision revision
              WHERE revision.workspace_id = release.workspace_id
                AND revision.skill_id = release.skill_id
                AND revision.id = release.revision_id
                AND revision.bundle_hash = sqlc.narg(post_hash)::text
          )
      )
  )
RETURNING release.*;

-- name: ListSkillEvolutionReleases :many
SELECT * FROM skill_evolution_release
WHERE workspace_id = sqlc.arg(workspace_id)
  AND skill_id = sqlc.arg(skill_id)
  AND (
      sqlc.narg(after_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: RecordSkillEvolutionTaskAttribution :one
INSERT INTO skill_evolution_task_attribution (
    workspace_id, task_id, runtime_id, skill_id, revision_id, manifest_version,
    source, bundle_hash, manifest_digest, eligibility, reason,
    dispatch_snapshot_id, task_dispatched_at
)
SELECT
    sqlc.arg(workspace_id), task.id, sqlc.arg(runtime_id), skill.id, revision.id,
    sqlc.arg(manifest_version), sqlc.arg(source), sqlc.arg(bundle_hash),
    sqlc.arg(manifest_digest), sqlc.arg(eligibility), sqlc.arg(reason),
    snapshot.id, snapshot.task_dispatched_at
FROM agent_task_queue task
JOIN agent
  ON agent.id = task.agent_id
 AND agent.workspace_id = sqlc.arg(workspace_id)
JOIN agent_runtime runtime
  ON runtime.id = sqlc.arg(runtime_id)
 AND runtime.workspace_id = sqlc.arg(workspace_id)
JOIN skill
  ON skill.id = sqlc.arg(skill_id)
 AND skill.workspace_id = sqlc.arg(workspace_id)
JOIN skill_evolution_revision revision
  ON revision.id = sqlc.arg(revision_id)
 AND revision.workspace_id = sqlc.arg(workspace_id)
 AND revision.skill_id = skill.id
 AND revision.source = sqlc.arg(source)
 AND revision.bundle_hash = sqlc.arg(bundle_hash)
JOIN skill_evolution_task_dispatch_snapshot snapshot
  ON snapshot.id = sqlc.arg(dispatch_snapshot_id)
 AND snapshot.workspace_id = sqlc.arg(workspace_id)
 AND snapshot.task_id = task.id
 AND snapshot.agent_id = task.agent_id
 AND snapshot.runtime_id = sqlc.arg(runtime_id)
 AND snapshot.task_dispatched_at = sqlc.arg(task_dispatched_at)
 AND snapshot.identities @> jsonb_build_array(jsonb_build_object(
       'source', sqlc.arg(source)::text,
       'skill_id', sqlc.arg(skill_id)::uuid::text
     ))
WHERE task.id = sqlc.arg(task_id)
  AND task.runtime_id = sqlc.arg(runtime_id)
  AND task.dispatched_at = sqlc.arg(task_dispatched_at)
  AND task.completed_at IS NOT NULL
  AND task.status IN ('completed', 'failed', 'cancelled')
ON CONFLICT (workspace_id, task_id, skill_id) DO NOTHING
RETURNING skill_evolution_task_attribution.*;

-- name: GetSkillEvolutionTaskAttribution :one
SELECT * FROM skill_evolution_task_attribution
WHERE workspace_id = sqlc.arg(workspace_id)
  AND task_id = sqlc.arg(task_id)
  AND skill_id = sqlc.arg(skill_id);

-- name: ListSkillEvolutionTaskAttributions :many
SELECT * FROM skill_evolution_task_attribution
WHERE workspace_id = sqlc.arg(workspace_id)
  AND task_id = sqlc.arg(task_id)
ORDER BY created_at, id;

-- name: ListExactSkillEvolutionTaskIDs :many
SELECT task_id
FROM skill_evolution_task_attribution
WHERE workspace_id = sqlc.arg(workspace_id)
  AND skill_id = sqlc.arg(skill_id)
  AND eligibility = 'eligible'
GROUP BY task_id
ORDER BY max(created_at) DESC, task_id DESC
LIMIT sqlc.arg(page_size);

-- name: HasExactSkillEvolutionTask :one
SELECT EXISTS (
    SELECT 1
    FROM skill_evolution_task_attribution
    WHERE workspace_id = sqlc.arg(workspace_id)
      AND task_id = sqlc.arg(task_id)
      AND skill_id = sqlc.arg(skill_id)
      AND eligibility = 'eligible'
);

-- name: RecordSkillEvolutionTaskDispatchSnapshot :one
WITH workspace_guard AS MATERIALIZED (
    SELECT workspace.id
    FROM workspace
    WHERE workspace.id = sqlc.arg(workspace_id)
    FOR KEY SHARE
)
INSERT INTO skill_evolution_task_dispatch_snapshot (
    workspace_id, task_id, agent_id, runtime_id, task_dispatched_at,
    contract_version, identities, identity_count, identities_digest
)
SELECT
    workspace_guard.id, task.id, agent.id, task.runtime_id, task.dispatched_at,
    sqlc.arg(contract_version), sqlc.arg(identities)::jsonb,
    sqlc.arg(identity_count), sqlc.arg(identities_digest)
FROM workspace_guard
JOIN agent_task_queue task
  ON task.id = sqlc.arg(task_id)
 AND task.runtime_id = sqlc.arg(runtime_id)
JOIN agent
  ON agent.id = sqlc.arg(agent_id)
 AND agent.id = task.agent_id
 AND agent.workspace_id = workspace_guard.id
WHERE task.dispatched_at = sqlc.arg(task_dispatched_at)
  AND task.status IN ('dispatched', 'waiting_local_directory', 'running')
ON CONFLICT (workspace_id, task_id, runtime_id, task_dispatched_at) DO NOTHING
RETURNING skill_evolution_task_dispatch_snapshot.*;

-- name: GetSkillEvolutionTaskDispatchSnapshot :one
SELECT *
FROM skill_evolution_task_dispatch_snapshot
WHERE workspace_id = sqlc.arg(workspace_id)
  AND task_id = sqlc.arg(task_id)
  AND agent_id = sqlc.arg(agent_id)
  AND runtime_id = sqlc.arg(runtime_id)
  AND task_dispatched_at = sqlc.arg(task_dispatched_at);

-- name: DeleteWorkspaceSkillEvolutionData :exec
WITH deleted_scheduler_history AS (
    DELETE FROM sys_cron_executions
    WHERE job_name = 'skill_evolution'
      AND scope_kind = 'workspace'
      AND scope_id = sqlc.arg(workspace_id)::uuid::text
), deleted_task_attributions AS (
    DELETE FROM skill_evolution_task_attribution WHERE skill_evolution_task_attribution.workspace_id = sqlc.arg(workspace_id)
), deleted_task_dispatch_snapshots AS (
    DELETE FROM skill_evolution_task_dispatch_snapshot WHERE skill_evolution_task_dispatch_snapshot.workspace_id = sqlc.arg(workspace_id)
), deleted_task_run_reviews AS (
    DELETE FROM task_run_review WHERE task_run_review.workspace_id = sqlc.arg(workspace_id)
), deleted_releases AS (
    DELETE FROM skill_evolution_release WHERE skill_evolution_release.workspace_id = sqlc.arg(workspace_id)
), deleted_reviews AS (
    DELETE FROM skill_evolution_review WHERE skill_evolution_review.workspace_id = sqlc.arg(workspace_id)
), deleted_evaluations AS (
    DELETE FROM skill_evolution_evaluation WHERE skill_evolution_evaluation.workspace_id = sqlc.arg(workspace_id)
), deleted_evidence AS (
    DELETE FROM skill_evolution_evidence WHERE skill_evolution_evidence.workspace_id = sqlc.arg(workspace_id)
), deleted_proposals AS (
    DELETE FROM skill_evolution_proposal WHERE skill_evolution_proposal.workspace_id = sqlc.arg(workspace_id)
), deleted_revision_files AS (
    DELETE FROM skill_evolution_revision_file WHERE skill_evolution_revision_file.workspace_id = sqlc.arg(workspace_id)
), deleted_revisions AS (
    DELETE FROM skill_evolution_revision WHERE skill_evolution_revision.workspace_id = sqlc.arg(workspace_id)
)
DELETE FROM skill_evolution_loop WHERE skill_evolution_loop.workspace_id = sqlc.arg(workspace_id);
