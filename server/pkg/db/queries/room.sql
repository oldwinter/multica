-- name: CreateRoom :one
INSERT INTO room (
    workspace_id, title, instructions, created_by_user_id,
    facilitator_agent_id, facilitator_squad_id, daily_turn_limit,
    schedule_interval_minutes, next_wake_at
) VALUES (
    @workspace_id, @title, @instructions, @created_by_user_id,
    @facilitator_agent_id, sqlc.narg(facilitator_squad_id),
    sqlc.narg(daily_turn_limit), sqlc.narg(schedule_interval_minutes),
    sqlc.narg(next_wake_at)
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

-- name: UpdateRoom :one
UPDATE room
SET title = @title,
    instructions = @instructions,
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

-- name: GetRoomEntry :one
SELECT * FROM room_entry
WHERE id = @id AND workspace_id = @workspace_id AND room_id = @room_id;

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
    status, refusal_reason, planned_at, completed_at
)
SELECT @workspace_id, @room_id, allocated.last_cycle_sequence, @source,
       @wake_key, sqlc.narg(triggering_entry_id), @status,
       sqlc.narg(refusal_reason), sqlc.narg(planned_at),
       CASE WHEN @status = 'refused' THEN now() ELSE NULL END
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
    workspace_id, room_id, cycle_id, agent_id, squad_id, status, refusal_reason
) VALUES (
    @workspace_id, @room_id, @cycle_id, @agent_id,
    sqlc.narg(squad_id), @status, sqlc.narg(refusal_reason)
)
ON CONFLICT (cycle_id, agent_id) DO NOTHING
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
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, status, priority, context,
    force_fresh_session, squad_id, originator_user_id, accountable_user_id,
    runtime_mcp_overlay, runtime_connected_apps,
    originator_source, trigger_evidence_kind, trigger_evidence_ref_id,
    room_turn_id, session_id, work_dir
) VALUES (
    @agent_id, @runtime_id, NULL, 'queued', @priority, @context::jsonb,
    @force_fresh_session, sqlc.narg(squad_id), sqlc.narg(originator_user_id),
    sqlc.narg(accountable_user_id), sqlc.narg(runtime_mcp_overlay)::jsonb,
    sqlc.narg(runtime_connected_apps)::jsonb, @originator_source, @trigger_evidence_kind,
    @trigger_evidence_ref_id, @room_turn_id, sqlc.narg(session_id),
    sqlc.narg(work_dir)
)
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
    created_by_user_id
) VALUES (
    @id, @workspace_id, @room_id, sqlc.narg(cycle_id), sqlc.narg(turn_id),
    sqlc.narg(entry_id), @kind, @idempotency_key, sqlc.narg(target_id),
    @title, @body, sqlc.narg(rationale), @source_digest, @created_by_user_id
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
WITH deleted_artifacts AS (
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
