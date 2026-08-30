-- name: CreateRoom :one
INSERT INTO room (
    workspace_id, title, instructions, created_by_user_id,
    facilitator_agent_id, facilitator_squad_id, daily_turn_limit,
    schedule_interval_minutes, next_wake_at, objective, success_criteria,
    stop_conditions, template_id, max_cost_ticks, capability_version
) VALUES (
    @workspace_id, @title, @instructions, @created_by_user_id,
    @facilitator_agent_id, sqlc.narg(facilitator_squad_id),
    sqlc.narg(daily_turn_limit), sqlc.narg(schedule_interval_minutes),
    sqlc.narg(next_wake_at), @objective, @success_criteria::jsonb,
    @stop_conditions::jsonb, sqlc.narg(template_id), sqlc.narg(max_cost_ticks),
    @capability_version
)
RETURNING *;

-- name: LockRoomWorkspaceForWrite :one
-- Room rows and their children have no foreign keys to workspace. Every Room
-- transaction that can add graph data takes this shared lock before the Room
-- row; workspace deletion takes FOR UPDATE before sweeping the graph.
SELECT id FROM workspace WHERE id = @workspace_id FOR KEY SHARE;

-- name: GetRoom :one
SELECT * FROM room
WHERE id = @id AND workspace_id = @workspace_id;

-- name: GetRoomForUpdate :one
SELECT * FROM room
WHERE id = @id AND workspace_id = @workspace_id
FOR UPDATE;

-- name: ListRooms :many
SELECT * FROM room
WHERE workspace_id = @workspace_id
ORDER BY updated_at DESC, id;

-- name: ListRoomValueSignals :many
WITH cycle_stats AS (
    SELECT room_id,
           count(*)::bigint AS cycle_count,
           GREATEST(count(*) - 1, 0)::bigint AS repeat_run_count,
           count(*) FILTER (WHERE status = 'failed')::bigint AS failed_cycles,
           count(*) FILTER (WHERE status = 'refused')::bigint AS refused_cycles,
           count(DISTINCT date_trunc('week', created_at))::bigint AS active_weeks
    FROM room_cycle
    WHERE workspace_id = @workspace_id
    GROUP BY room_id
), review_stats AS (
    SELECT room_id,
           count(*) FILTER (WHERE review_status = 'accepted')::bigint AS accepted_outcomes,
           COALESCE(
               percentile_cont(0.5) WITHIN GROUP (
                   ORDER BY extract(epoch FROM (reviewed_at - created_at))
               ) FILTER (WHERE reviewed_at IS NOT NULL),
               0
           )::double precision AS median_review_latency_seconds
    FROM room_memory_revision
    WHERE workspace_id = @workspace_id
    GROUP BY room_id
), artifact_stats AS (
    SELECT room_id,
           count(*) FILTER (WHERE target_id IS NOT NULL)::bigint AS promoted_artifacts
    FROM room_artifact
    WHERE workspace_id = @workspace_id
    GROUP BY room_id
)
SELECT r.id AS room_id,
       accepted.id AS last_accepted_revision_id,
       accepted.reviewed_at AS last_accepted_at,
       latest.id AS last_cycle_id,
       COALESCE(latest.status, '') AS last_run_status,
       COALESCE(latest.phase, '') AS last_run_phase,
       latest.refusal_reason AS last_run_reason,
       COALESCE(latest.completed_at, latest.created_at) AS last_run_at,
       COALESCE(latest_cost.cost_ticks, 0)::bigint AS last_run_cost_ticks,
       COALESCE(cycles.repeat_run_count, 0)::bigint AS repeat_run_count,
       COALESCE(reviews.accepted_outcomes, 0)::bigint AS accepted_outcomes,
       COALESCE(cycles.active_weeks, 0)::bigint AS active_weeks,
       CASE WHEN COALESCE(cycles.active_weeks, 0) = 0 THEN 0::double precision
            ELSE COALESCE(reviews.accepted_outcomes, 0)::double precision / cycles.active_weeks
       END::double precision AS accepted_outcomes_per_active_week,
       COALESCE(reviews.median_review_latency_seconds, 0)::double precision AS median_review_latency_seconds,
       CASE WHEN COALESCE(reviews.accepted_outcomes, 0) = 0 THEN 0::double precision
            ELSE COALESCE(artifacts.promoted_artifacts, 0)::double precision / reviews.accepted_outcomes
       END::double precision AS promotion_rate,
       COALESCE(cycles.failed_cycles, 0)::bigint AS failed_cycles,
       COALESCE(cycles.refused_cycles, 0)::bigint AS refused_cycles
FROM room r
LEFT JOIN cycle_stats cycles ON cycles.room_id = r.id
LEFT JOIN review_stats reviews ON reviews.room_id = r.id
LEFT JOIN artifact_stats artifacts ON artifacts.room_id = r.id
LEFT JOIN LATERAL (
    SELECT revision.id, revision.reviewed_at
    FROM room_memory_revision revision
    WHERE revision.workspace_id = r.workspace_id AND revision.room_id = r.id
      AND revision.review_status = 'accepted'
    ORDER BY revision.reviewed_at DESC, revision.id DESC
    LIMIT 1
) accepted ON true
LEFT JOIN LATERAL (
    SELECT cycle.id, cycle.status, cycle.phase, cycle.refusal_reason,
           cycle.completed_at, cycle.created_at
    FROM room_cycle cycle
    WHERE cycle.workspace_id = r.workspace_id AND cycle.room_id = r.id
    ORDER BY cycle.sequence DESC, cycle.id DESC
    LIMIT 1
) latest ON true
LEFT JOIN LATERAL (
    SELECT COALESCE(sum(tu.cost_usd_ticks), 0)::bigint AS cost_ticks
    FROM room_turn turn_row
    LEFT JOIN agent_task_queue task ON task.room_turn_id = turn_row.id
    LEFT JOIN task_usage tu ON tu.task_id = task.id
    WHERE turn_row.workspace_id = r.workspace_id AND turn_row.room_id = r.id
      AND turn_row.cycle_id = latest.id AND turn_row.status <> 'refused'
) latest_cost ON true
WHERE r.workspace_id = @workspace_id
ORDER BY r.updated_at DESC, r.id;

-- name: UpdateRoom :one
UPDATE room
SET title = @title,
    instructions = @instructions,
    objective = @objective,
    success_criteria = @success_criteria::jsonb,
    stop_conditions = @stop_conditions::jsonb,
    template_id = sqlc.narg(template_id),
    max_cost_ticks = sqlc.narg(max_cost_ticks),
    daily_turn_limit = sqlc.narg(daily_turn_limit),
    schedule_interval_minutes = sqlc.narg(schedule_interval_minutes),
    next_wake_at = sqlc.narg(next_wake_at),
    updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
RETURNING *;

-- name: SetRoomStatus :one
UPDATE room
SET status = @status, updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
RETURNING *;

-- name: SetRoomActiveCycle :one
UPDATE room
SET active_cycle_id = sqlc.narg(active_cycle_id), updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
  AND active_cycle_id IS NOT DISTINCT FROM sqlc.narg(expected_active_cycle_id)::uuid
RETURNING *;

-- name: AdvanceRoomSchedule :one
UPDATE room
SET next_wake_at = @next_wake_at, updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
  AND next_wake_at = @expected_next_wake_at
RETURNING *;

-- name: ListDueRooms :many
SELECT * FROM room
WHERE status IN ('active', 'paused')
  AND schedule_interval_minutes IS NOT NULL
  AND next_wake_at IS NOT NULL
  AND next_wake_at <= @due_at
ORDER BY next_wake_at, id
LIMIT @limit_count;

-- name: AddRoomParticipant :one
INSERT INTO room_participant (
    workspace_id, room_id, participant_type, participant_id, role, source_squad_id
) VALUES (
    @workspace_id, @room_id, @participant_type, @participant_id, @role,
    sqlc.narg(source_squad_id)
)
ON CONFLICT (room_id, participant_type, participant_id) WHERE left_at IS NULL
DO UPDATE SET role = EXCLUDED.role, source_squad_id = EXCLUDED.source_squad_id
RETURNING *;

-- name: ListRoomParticipants :many
SELECT * FROM room_participant
WHERE workspace_id = @workspace_id AND room_id = @room_id AND left_at IS NULL
ORDER BY CASE role WHEN 'facilitator' THEN 0 WHEN 'participant' THEN 1 ELSE 2 END,
         joined_at, id;

-- name: RemoveRoomParticipant :one
UPDATE room_participant
SET left_at = now()
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND left_at IS NULL AND role <> 'facilitator'
RETURNING *;

-- name: AddRoomEntry :one
WITH allocated AS (
    UPDATE room
    SET last_entry_ordinal = last_entry_ordinal + 1, updated_at = now()
    WHERE id = @room_id AND workspace_id = @workspace_id
    RETURNING last_entry_ordinal
)
INSERT INTO room_entry (
    workspace_id, room_id, cycle_id, turn_id, ordinal, entry_type,
    author_type, author_id, body, mentions
)
SELECT @workspace_id, @room_id, sqlc.narg(cycle_id), sqlc.narg(turn_id),
       allocated.last_entry_ordinal, @entry_type, @author_type,
       sqlc.narg(author_id), @body, @mentions::jsonb
FROM allocated
RETURNING *;

-- name: ListRoomEntries :many
SELECT * FROM room_entry
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND ordinal > @after_ordinal
ORDER BY ordinal DESC
LIMIT @limit_count;

-- name: ListRoomResultEntriesByCycle :many
SELECT * FROM room_entry
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND cycle_id = @cycle_id AND entry_type = 'result'
ORDER BY ordinal, id;

-- name: ListRoomParticipantResultEntriesByCycle :many
SELECT re.* FROM room_entry re
JOIN room_turn rt ON rt.id = re.turn_id
WHERE re.workspace_id = @workspace_id AND re.room_id = @room_id
  AND re.cycle_id = @cycle_id AND re.entry_type = 'result'
  AND rt.workspace_id = re.workspace_id AND rt.room_id = re.room_id
  AND rt.cycle_id = re.cycle_id AND rt.turn_kind = 'participant'
ORDER BY re.ordinal, re.id;

-- name: GetRoomEntry :one
SELECT * FROM room_entry
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id;

-- name: ListRoomEntriesByIDs :many
SELECT * FROM room_entry
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND id = ANY(@entry_ids::uuid[])
ORDER BY ordinal, id;

-- name: CreateRoomCycle :one
WITH allocated AS (
    UPDATE room
    SET last_cycle_sequence = last_cycle_sequence + 1,
        updated_at = now()
    WHERE id = @room_id AND workspace_id = @workspace_id
    RETURNING last_cycle_sequence
)
INSERT INTO room_cycle (
    workspace_id, room_id, sequence, source, wake_key, triggering_entry_id,
    status, phase, refusal_reason, planned_at, completed_at, expected_max_turns,
    cost_limit_ticks
)
SELECT @workspace_id, @room_id, allocated.last_cycle_sequence, @source,
       @wake_key, sqlc.narg(triggering_entry_id), @status, @phase,
       sqlc.narg(refusal_reason), sqlc.narg(planned_at),
       CASE WHEN @status = 'refused' THEN now() ELSE NULL END,
       @expected_max_turns, sqlc.narg(cost_limit_ticks)
FROM allocated
ON CONFLICT (room_id, wake_key) DO NOTHING
RETURNING *;

-- name: GetRoomCycleByWakeKey :one
SELECT * FROM room_cycle
WHERE workspace_id = @workspace_id AND room_id = @room_id AND wake_key = @wake_key;

-- name: GetRoomCycle :one
SELECT * FROM room_cycle
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id;

-- name: ListRoomCycles :many
SELECT * FROM room_cycle
WHERE workspace_id = @workspace_id AND room_id = @room_id
ORDER BY sequence DESC
LIMIT @limit_count;

-- name: CountRoomTurnsSince :one
SELECT count(*) FROM room_turn
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND status <> 'refused' AND created_at >= @since_at;

-- name: CreateRoomTurn :one
INSERT INTO room_turn (
    workspace_id, room_id, cycle_id, agent_id, squad_id, turn_kind,
    attempt, status, refusal_reason, idempotency_key
) VALUES (
    @workspace_id, @room_id, @cycle_id, @agent_id,
    sqlc.narg(squad_id), @turn_kind, @attempt, @status, sqlc.narg(refusal_reason),
    sqlc.narg(idempotency_key)
)
ON CONFLICT (cycle_id, turn_kind, agent_id, attempt) DO NOTHING
RETURNING *;

-- name: GetRoomTurn :one
SELECT * FROM room_turn
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id;

-- name: GetRoomTurnByTask :one
SELECT rt.* FROM room_turn rt
JOIN agent_task_queue atq ON atq.room_turn_id = rt.id
WHERE atq.id = @task_id;

-- name: GetLatestTaskForRoomTurn :one
SELECT * FROM agent_task_queue
WHERE room_turn_id = @room_turn_id
ORDER BY attempt DESC, created_at DESC, id DESC
LIMIT 1;

-- name: ListUnsyncedTerminalRoomTasks :many
SELECT task.*
FROM agent_task_queue task
JOIN room_turn turn ON turn.id = task.room_turn_id
WHERE task.status IN ('completed', 'failed', 'cancelled')
  AND turn.status IN ('queued', 'dispatched', 'running')
  AND NOT EXISTS (
      SELECT 1
      FROM agent_task_queue newer
      WHERE newer.room_turn_id = task.room_turn_id
        AND (newer.attempt, newer.created_at, newer.id) > (task.attempt, task.created_at, task.id)
  )
ORDER BY task.completed_at, task.id
LIMIT @limit_count;

-- name: ListRoomTurns :many
SELECT * FROM room_turn
WHERE workspace_id = @workspace_id AND room_id = @room_id
ORDER BY created_at DESC, id;

-- name: ListRoomTurnsByCycle :many
SELECT * FROM room_turn
WHERE workspace_id = @workspace_id AND room_id = @room_id AND cycle_id = @cycle_id
ORDER BY created_at, id;

-- name: GetLastCompletedRoomTurnForAgent :one
SELECT * FROM room_turn
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND agent_id = @agent_id AND status = 'completed' AND session_id IS NOT NULL
ORDER BY completed_at DESC, id DESC
LIMIT 1;

-- name: CreateRoomTask :one
-- Fenced against workspace teardown: lock_task_owner_rows (migration 284)
-- locks the agent/runtime owners before the task becomes visible, so a room
-- cycle cannot strand work in a workspace that teardown just removed.
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, status, priority, context,
    force_fresh_session, squad_id, originator_user_id, accountable_user_id,
    runtime_mcp_overlay, runtime_connected_apps,
    originator_source, trigger_evidence_kind, trigger_evidence_ref_id,
    room_turn_id, session_id, work_dir, max_attempts
) SELECT
    @agent_id, @runtime_id, NULL, 'queued', @priority, @context::jsonb,
    @force_fresh_session, sqlc.narg(squad_id), sqlc.narg(originator_user_id),
    sqlc.narg(accountable_user_id), sqlc.narg(runtime_mcp_overlay)::jsonb,
    sqlc.narg(runtime_connected_apps)::jsonb, @originator_source, @trigger_evidence_kind,
    @trigger_evidence_ref_id, @room_turn_id, sqlc.narg(session_id),
    sqlc.narg(work_dir), 1
WHERE lock_task_owner_rows(@agent_id, NULL, @runtime_id)
RETURNING *;

-- name: UpdateRoomBudget :one
UPDATE room
SET daily_turn_limit = sqlc.narg(daily_turn_limit)::integer,
    max_cost_ticks = sqlc.narg(max_cost_ticks)::bigint,
    updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
RETURNING *;

-- name: MarkRoomTurnRunning :one
UPDATE room_turn
SET status = 'running', started_at = COALESCE(started_at, now())
WHERE id = @id AND status IN ('queued', 'dispatched')
RETURNING *;

-- name: CompleteRoomTurn :one
UPDATE room_turn
SET status = @status,
    result = sqlc.narg(result),
    session_id = sqlc.narg(session_id),
    work_dir = sqlc.narg(work_dir),
    started_at = COALESCE(started_at, @started_at),
    completed_at = COALESCE(completed_at, now())
WHERE id = @id AND status IN ('queued', 'dispatched', 'running')
RETURNING *;

-- name: CompleteRoomCycle :one
UPDATE room_cycle
SET status = @status,
    phase = CASE
        WHEN @status::text IN ('completed', 'failed', 'cancelled') THEN @status::text
        ELSE phase
    END,
    started_at = COALESCE(started_at, @started_at),
    completed_at = COALESCE(completed_at, now())
WHERE id = @id AND workspace_id = @workspace_id
  AND status IN ('queued', 'running')
RETURNING *;

-- name: MarkRoomCycleRunning :one
UPDATE room_cycle
SET status = 'running', started_at = COALESCE(started_at, now())
WHERE id = @id AND workspace_id = @workspace_id AND status = 'queued'
RETURNING *;

-- name: ClearRoomActiveCycle :one
UPDATE room
SET active_cycle_id = NULL, updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
  AND active_cycle_id = @completed_cycle_id
RETURNING *;

-- name: UpdateRoomMemory :one
UPDATE room
SET memory = @memory::jsonb,
    memory_version = memory_version + 1,
    active_cycle_id = CASE WHEN active_cycle_id = sqlc.narg(completed_cycle_id)::uuid THEN NULL ELSE active_cycle_id END,
    updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
  AND memory_version = @expected_memory_version
RETURNING *;

-- name: CreateRoomArtifact :one
INSERT INTO room_artifact (
    id, workspace_id, room_id, cycle_id, turn_id, entry_id, kind,
    idempotency_key, target_id, title, body, rationale, source_digest,
    created_by_user_id, memory_revision_id, recommendation_key, citation_entry_ids
) VALUES (
    @id, @workspace_id, @room_id, sqlc.narg(cycle_id), sqlc.narg(turn_id),
    sqlc.narg(entry_id), @kind, @idempotency_key, sqlc.narg(target_id),
    @title, @body, sqlc.narg(rationale), @source_digest, @created_by_user_id,
    sqlc.narg(memory_revision_id), sqlc.narg(recommendation_key), @citation_entry_ids::jsonb
)
ON CONFLICT (room_id, kind, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetRoomArtifactByKey :one
SELECT * FROM room_artifact
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND kind = @kind AND idempotency_key = @idempotency_key;

-- name: ListRoomArtifacts :many
SELECT * FROM room_artifact
WHERE workspace_id = @workspace_id AND room_id = @room_id
ORDER BY created_at DESC, id;

-- name: SetRoomArtifactTarget :one
UPDATE room_artifact
SET target_id = @target_id
WHERE id = @id AND workspace_id = @workspace_id AND target_id IS NULL
RETURNING *;

-- name: DeleteWorkspaceRoomData :exec
WITH deleted_recommendation_reviews AS (
    DELETE FROM room_recommendation_review rrr WHERE rrr.workspace_id = @target_workspace_id
), deleted_memory_revisions AS (
    DELETE FROM room_memory_revision rmr WHERE rmr.workspace_id = @target_workspace_id
), deleted_artifacts AS (
    DELETE FROM room_artifact ra WHERE ra.workspace_id = @target_workspace_id
), deleted_turns AS (
    DELETE FROM room_turn rt WHERE rt.workspace_id = @target_workspace_id
), deleted_cycles AS (
    DELETE FROM room_cycle rc WHERE rc.workspace_id = @target_workspace_id
), deleted_entries AS (
    DELETE FROM room_entry re WHERE re.workspace_id = @target_workspace_id
), deleted_participants AS (
    DELETE FROM room_participant rp WHERE rp.workspace_id = @target_workspace_id
)
DELETE FROM room r WHERE r.workspace_id = @target_workspace_id;

-- name: SetRoomCycleSynthesizing :one
UPDATE room_cycle
SET status = 'running', phase = 'synthesizing', synthesis_turn_id = @synthesis_turn_id,
    synthesis_error = NULL, started_at = COALESCE(started_at, now())
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'gathering' AND status IN ('queued', 'running')
RETURNING *;

-- name: SetRoomCycleSynthesisBlocked :one
UPDATE room_cycle
SET status = 'running', phase = 'awaiting_review', synthesis_error = @synthesis_error::jsonb,
    started_at = COALESCE(started_at, now())
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'gathering' AND status IN ('queued', 'running')
RETURNING *;

-- name: SetRoomCycleAwaitingReview :one
UPDATE room_cycle
SET status = 'running', phase = 'awaiting_review', memory_revision_id = sqlc.narg(memory_revision_id),
    synthesis_error = sqlc.narg(synthesis_error)::jsonb
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'synthesizing' AND status = 'running'
RETURNING *;

-- name: SetRoomCycleSynthesisRetry :one
UPDATE room_cycle
SET phase = 'synthesizing', synthesis_turn_id = @synthesis_turn_id, synthesis_error = NULL
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'awaiting_review' AND synthesis_error IS NOT NULL AND status = 'running'
RETURNING *;

-- name: SetRoomCycleReviewRetryable :one
UPDATE room_cycle
SET synthesis_error = @synthesis_error::jsonb
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'awaiting_review' AND status = 'running'
RETURNING *;

-- name: FailRoomOutcomeCycle :one
UPDATE room_cycle
SET status = 'failed', phase = 'failed', synthesis_error = @synthesis_error::jsonb,
    completed_at = COALESCE(completed_at, now())
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'gathering' AND status = 'running'
RETURNING *;

-- name: SetRoomCyclePendingRevision :one
UPDATE room_cycle
SET memory_revision_id = @memory_revision_id, synthesis_error = NULL
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'awaiting_review' AND status = 'running'
RETURNING *;

-- name: SetRoomCycleReviewed :one
UPDATE room_cycle
SET status = @status, phase = @phase, completed_at = COALESCE(completed_at, now())
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND phase = 'awaiting_review' AND status = 'running'
RETURNING *;

-- name: NextRoomSynthesisAttempt :one
SELECT COALESCE(max(attempt), 0)::int + 1
FROM room_turn
WHERE workspace_id = @workspace_id AND room_id = @room_id AND cycle_id = @cycle_id
  AND turn_kind = 'synthesis' AND agent_id = @agent_id;

-- name: GetRoomSynthesisTurnByKey :one
SELECT * FROM room_turn
WHERE workspace_id = @workspace_id AND room_id = @room_id AND cycle_id = @cycle_id
  AND turn_kind = 'synthesis' AND idempotency_key = @idempotency_key;

-- name: CreateRoomMemoryRevision :one
WITH allocated AS (
    UPDATE room
    SET last_memory_revision_version = last_memory_revision_version + 1,
        updated_at = now()
    WHERE id = @room_id AND workspace_id = @workspace_id
    RETURNING last_memory_revision_version
)
INSERT INTO room_memory_revision (
    workspace_id, room_id, cycle_id, synthesis_turn_id, version,
    schema_version, synthesis, digest, corrected_from_revision_id,
    creator_type, creator_id
)
SELECT @workspace_id, @room_id, @cycle_id, @synthesis_turn_id,
       allocated.last_memory_revision_version, @schema_version,
       @synthesis::jsonb, @digest, sqlc.narg(corrected_from_revision_id),
       @creator_type, @creator_id
FROM allocated
RETURNING *;

-- name: GetRoomMemoryRevision :one
SELECT * FROM room_memory_revision
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id;

-- name: GetRoomMemoryRevisionByReviewKey :one
SELECT * FROM room_memory_revision
WHERE workspace_id = @workspace_id AND room_id = @room_id AND cycle_id = @cycle_id
  AND review_idempotency_key = @review_idempotency_key;

-- name: GetCorrectedRoomMemoryRevision :one
SELECT * FROM room_memory_revision
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND corrected_from_revision_id = @corrected_from_revision_id
ORDER BY version DESC
LIMIT 1;

-- name: ListRoomMemoryRevisions :many
WITH selected_revision_ids AS (
    SELECT recent.id
    FROM (
        SELECT revision.id
        FROM room_memory_revision revision
        WHERE revision.workspace_id = @workspace_id AND revision.room_id = @room_id
        ORDER BY revision.version DESC
        LIMIT 100
    ) recent
    UNION
    SELECT room.accepted_memory_revision_id
    FROM room
    WHERE room.workspace_id = @workspace_id AND room.id = @room_id
      AND room.accepted_memory_revision_id IS NOT NULL
    UNION
    SELECT active_cycle.memory_revision_id
    FROM room
    JOIN room_cycle active_cycle ON active_cycle.id = room.active_cycle_id
    WHERE room.workspace_id = @workspace_id AND room.id = @room_id
      AND active_cycle.memory_revision_id IS NOT NULL
)
SELECT revision.*
FROM room_memory_revision revision
JOIN selected_revision_ids selected ON selected.id = revision.id
WHERE revision.workspace_id = @workspace_id AND revision.room_id = @room_id
ORDER BY revision.version DESC;

-- name: ReviewRoomMemoryRevision :one
UPDATE room_memory_revision
SET review_status = @review_status, reviewed_by_user_id = @reviewed_by_user_id,
    reviewed_at = now(), review_idempotency_key = @review_idempotency_key,
    review_request_digest = @review_request_digest
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id
  AND review_status = 'pending'
RETURNING *;

-- name: AcceptRoomMemoryRevision :one
UPDATE room
SET accepted_memory_revision_id = @revision_id,
    memory = @synthesis::jsonb,
    memory_version = memory_version + 1,
    active_cycle_id = CASE WHEN active_cycle_id = @cycle_id THEN NULL ELSE active_cycle_id END,
    updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
  AND memory_version = @expected_memory_version
RETURNING *;

-- name: ClearRoomCycleAfterReview :one
UPDATE room
SET active_cycle_id = CASE WHEN active_cycle_id = @cycle_id THEN NULL ELSE active_cycle_id END,
    updated_at = now()
WHERE id = @id AND workspace_id = @workspace_id
RETURNING *;

-- name: CancelRoomCycle :one
WITH cancelled_tasks AS (
    UPDATE agent_task_queue
    SET status = 'cancelled', completed_at = COALESCE(completed_at, now()),
        prepare_lease_expires_at = NULL
    WHERE room_turn_id IN (
        SELECT id FROM room_turn
        WHERE workspace_id = @workspace_id AND room_id = @room_id AND cycle_id = @cycle_id
    ) AND status IN ('queued', 'deferred', 'dispatched', 'preparing', 'running', 'waiting_local_directory')
), cancelled_turns AS (
    UPDATE room_turn
    SET status = 'cancelled', completed_at = COALESCE(completed_at, now())
    WHERE workspace_id = @workspace_id AND room_id = @room_id AND cycle_id = @cycle_id
      AND status IN ('queued', 'dispatched', 'running')
), cleared_room AS (
    UPDATE room
    SET active_cycle_id = NULL, updated_at = now()
    WHERE id = @room_id AND workspace_id = @workspace_id AND active_cycle_id = @cycle_id
)
UPDATE room_cycle
SET status = 'cancelled', phase = 'cancelled', completed_at = COALESCE(completed_at, now()),
    cancel_idempotency_key = @idempotency_key
WHERE room_cycle.id = @cycle_id AND room_cycle.workspace_id = @workspace_id
  AND room_cycle.room_id = @room_id AND room_cycle.status IN ('queued', 'running')
RETURNING *;

-- name: GetRoomUsageSummary :one
WITH turn_usage AS (
    SELECT count(*) FILTER (WHERE rt.status <> 'refused')::bigint AS turns_total,
           COALESCE(sum(attempt_usage.cost_ticks), 0)::bigint AS cost_ticks,
           count(*) FILTER (
               WHERE rt.status <> 'refused' AND attempt_usage.has_task
                 AND attempt_usage.has_uncosted_usage
           )::bigint AS uncosted_turns,
           count(*) FILTER (WHERE rt.status IN ('failed', 'cancelled'))::bigint AS failures
    FROM room_turn rt
    LEFT JOIN LATERAL (
        SELECT COALESCE(bool_or(atq.id IS NOT NULL), false) AS has_task,
               COALESCE(sum(tu.cost_usd_ticks), 0)::bigint AS cost_ticks,
               COALESCE(bool_or(
                   atq.id IS NOT NULL
                   AND (tu.task_id IS NULL OR tu.cost_usd_ticks IS NULL)
               ), false) AS has_uncosted_usage
        FROM agent_task_queue atq
        LEFT JOIN task_usage tu ON tu.task_id = atq.id
        WHERE atq.room_turn_id = rt.id
    ) attempt_usage ON true
    WHERE rt.workspace_id = @workspace_id AND rt.room_id = @room_id
), cycle_usage AS (
    SELECT GREATEST(count(*) - 1, 0)::bigint AS repeat_run_count,
           count(*) FILTER (WHERE status = 'failed')::bigint AS failed_cycles,
           count(*) FILTER (WHERE status = 'refused')::bigint AS refused_cycles,
           count(DISTINCT date_trunc('week', created_at))::bigint AS active_weeks
    FROM room_cycle
    WHERE workspace_id = @workspace_id AND room_id = @room_id
), review_usage AS (
    SELECT count(*) FILTER (WHERE review_status = 'accepted')::bigint AS accepted_syntheses,
           COALESCE(
               percentile_cont(0.5) WITHIN GROUP (
                   ORDER BY extract(epoch FROM (reviewed_at - created_at))
               ) FILTER (WHERE reviewed_at IS NOT NULL),
               0
           )::double precision AS median_review_latency_seconds
    FROM room_memory_revision
    WHERE workspace_id = @workspace_id AND room_id = @room_id
), artifact_usage AS (
    SELECT count(*) FILTER (WHERE target_id IS NOT NULL)::bigint AS promoted_artifacts
    FROM room_artifact
    WHERE workspace_id = @workspace_id AND room_id = @room_id
)
SELECT turns.turns_total, turns.cost_ticks, turns.uncosted_turns, turns.failures,
       reviews.accepted_syntheses, artifacts.promoted_artifacts,
       cycles.repeat_run_count, cycles.active_weeks, reviews.median_review_latency_seconds,
       CASE WHEN cycles.active_weeks = 0 THEN 0::double precision
            ELSE reviews.accepted_syntheses::double precision / cycles.active_weeks
       END::double precision AS accepted_outcomes_per_active_week,
       CASE WHEN reviews.accepted_syntheses = 0 THEN 0::double precision
            ELSE artifacts.promoted_artifacts::double precision / reviews.accepted_syntheses
       END::double precision AS promotion_rate,
       cycles.failed_cycles, cycles.refused_cycles,
       CASE WHEN reviews.accepted_syntheses = 0 THEN 0::double precision
            ELSE turns.cost_ticks::double precision / reviews.accepted_syntheses
       END::double precision AS cost_ticks_per_accepted_outcome
FROM turn_usage turns, cycle_usage cycles, review_usage reviews, artifact_usage artifacts;

-- name: GetRoomCycleUsageSummary :one
SELECT
    COALESCE(sum(tu.cost_usd_ticks), 0)::bigint AS cost_ticks,
    count(DISTINCT atq.id) FILTER (
        WHERE atq.id IS NOT NULL AND (tu.task_id IS NULL OR tu.cost_usd_ticks IS NULL)
    )::bigint AS uncosted_turns
FROM room_turn rt
LEFT JOIN agent_task_queue atq ON atq.room_turn_id = rt.id
LEFT JOIN task_usage tu ON tu.task_id = atq.id
WHERE rt.workspace_id = @workspace_id AND rt.room_id = @room_id
  AND rt.cycle_id = @cycle_id AND rt.status <> 'refused';

-- name: CountRoomTurnsByCycle :one
SELECT count(*)::bigint
FROM room_turn
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND cycle_id = @cycle_id AND status <> 'refused';

-- name: DeferUnsupportedRoomTaskAfterClaim :one
-- An old daemon can advertise room-tasks-v1 while lacking room-outcomes-v2.
-- Move only the exact unsupported claim out of the hot queue long enough for
-- the same request to make progress on compatible work. The normal deferred
-- promoter makes it eligible again, including immediately after an upgrade.
UPDATE agent_task_queue
SET status = 'deferred',
    fire_at = now() + make_interval(secs => @retry_after_secs::double precision),
    dispatched_at = NULL,
    prepare_lease_expires_at = NULL,
    delivered_comment_ids = '{}'
WHERE id = @task_id
  AND runtime_id = @runtime_id
  AND room_turn_id IS NOT NULL
  AND status = 'dispatched'
  AND started_at IS NULL
  AND dispatched_at = @dispatched_at
RETURNING *;

-- name: MakeSupportedDeferredRoomTasksDue :execrows
-- A daemon may have upgraded after an older process deferred a Room task it
-- could not understand. Make only tasks supported by the current request due;
-- the existing deferred promoter performs the queued transition and side
-- effects before claim candidate selection.
UPDATE agent_task_queue
SET fire_at = now()
WHERE runtime_id = ANY(@runtime_ids::uuid[])
  AND room_turn_id IS NOT NULL
  AND status = 'deferred'
  AND (
      (@supports_room_tasks_v1::boolean AND context->>'schema_version' = '1')
      OR (@supports_room_outcomes_v2::boolean AND context->>'schema_version' = '2')
  );

-- name: CreateRoomRecommendationReview :one
INSERT INTO room_recommendation_review (
    workspace_id, room_id, memory_revision_id, recommendation_key, status,
    idempotency_key, request_digest, artifact_id, reviewed_by_user_id
) VALUES (
    @workspace_id, @room_id, @memory_revision_id, @recommendation_key, @status,
    @idempotency_key, @request_digest, sqlc.narg(artifact_id), @reviewed_by_user_id
)
ON CONFLICT (room_id, memory_revision_id, recommendation_key) DO NOTHING
RETURNING *;

-- name: GetRoomRecommendationReview :one
SELECT * FROM room_recommendation_review
WHERE workspace_id = @workspace_id AND room_id = @room_id
  AND memory_revision_id = @memory_revision_id AND recommendation_key = @recommendation_key;

-- name: ListRoomRecommendationReviews :many
WITH selected_revision_ids AS (
    SELECT recent.id
    FROM (
        SELECT revision.id
        FROM room_memory_revision revision
        WHERE revision.workspace_id = @workspace_id AND revision.room_id = @room_id
        ORDER BY revision.version DESC
        LIMIT 100
    ) recent
    UNION
    SELECT room.accepted_memory_revision_id
    FROM room
    WHERE room.workspace_id = @workspace_id AND room.id = @room_id
      AND room.accepted_memory_revision_id IS NOT NULL
    UNION
    SELECT active_cycle.memory_revision_id
    FROM room
    JOIN room_cycle active_cycle ON active_cycle.id = room.active_cycle_id
    WHERE room.workspace_id = @workspace_id AND room.id = @room_id
      AND active_cycle.memory_revision_id IS NOT NULL
)
SELECT review.*
FROM room_recommendation_review review
JOIN selected_revision_ids selected ON selected.id = review.memory_revision_id
WHERE review.workspace_id = @workspace_id AND review.room_id = @room_id
ORDER BY review.reviewed_at DESC, review.id
LIMIT 10200;
