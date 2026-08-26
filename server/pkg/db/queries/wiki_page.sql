-- name: ListWikiPagesByWorkspaceScope :many
SELECT id, workspace_id, scope, project_id, owner_user_id, path, title,
       created_by, created_at, updated_at, current_revision_number,
       current_revision_id, content_digest, last_source_kind, last_actor_type, last_actor_id
FROM wiki_page
WHERE workspace_id = $1
  AND scope = 'workspace'
ORDER BY path ASC;

-- name: ListWikiPagesByProject :many
SELECT id, workspace_id, scope, project_id, owner_user_id, path, title,
       created_by, created_at, updated_at, current_revision_number,
       current_revision_id, content_digest, last_source_kind, last_actor_type, last_actor_id
FROM wiki_page
WHERE workspace_id = $1
  AND scope = 'project'
  AND project_id = $2
ORDER BY path ASC;

-- name: ListWikiPagesByOwner :many
-- Personal wiki is cross-workspace: keyed only by owner_user_id.
SELECT id, workspace_id, scope, project_id, owner_user_id, path, title,
       created_by, created_at, updated_at, current_revision_number,
       current_revision_id, content_digest, last_source_kind, last_actor_type, last_actor_id
FROM wiki_page
WHERE scope = 'user'
  AND owner_user_id = $1
ORDER BY path ASC;

-- name: SearchWikiPagesInWorkspace :many
WITH needle AS (
    SELECT websearch_to_tsquery('simple', sqlc.arg(search_query)) AS query
)
SELECT page.* FROM wiki_page page CROSS JOIN needle
WHERE page.workspace_id = sqlc.arg(workspace_id)
  AND scope IN ('workspace', 'project')
  AND (
      to_tsvector('simple', title || ' ' || path || ' ' || content) @@ needle.query
      OR LOWER(title || ' ' || path || ' ' || content) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%'
  )
ORDER BY
  ts_rank_cd(to_tsvector('simple', title || ' ' || path || ' ' || content), needle.query) DESC,
  CASE WHEN LOWER(title) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 0
       WHEN LOWER(path) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 1
       ELSE 2 END,
  updated_at DESC, path ASC
LIMIT sqlc.arg(result_limit);

-- name: SearchWikiPagesInProject :many
WITH needle AS (
    SELECT websearch_to_tsquery('simple', sqlc.arg(search_query)) AS query
)
SELECT page.* FROM wiki_page page CROSS JOIN needle
WHERE page.workspace_id = sqlc.arg(workspace_id)
  AND scope = 'project'
  AND project_id = sqlc.arg(project_id)
  AND (
      to_tsvector('simple', title || ' ' || path || ' ' || content) @@ needle.query
      OR LOWER(title || ' ' || path || ' ' || content) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%'
  )
ORDER BY
  ts_rank_cd(to_tsvector('simple', title || ' ' || path || ' ' || content), needle.query) DESC,
  CASE WHEN LOWER(title) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 0
       WHEN LOWER(path) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 1
       ELSE 2 END,
  updated_at DESC, path ASC
LIMIT sqlc.arg(result_limit);

-- name: SearchWikiPagesByOwner :many
WITH needle AS (
    SELECT websearch_to_tsquery('simple', sqlc.arg(search_query)) AS query
)
SELECT page.* FROM wiki_page page CROSS JOIN needle
WHERE scope = 'user'
  AND owner_user_id = sqlc.arg(owner_user_id)
  AND (
      to_tsvector('simple', title || ' ' || path || ' ' || content) @@ needle.query
      OR LOWER(title || ' ' || path || ' ' || content) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%'
  )
ORDER BY
  ts_rank_cd(to_tsvector('simple', title || ' ' || path || ' ' || content), needle.query) DESC,
  CASE WHEN LOWER(title) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 0
       WHEN LOWER(path) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 1
       ELSE 2 END,
  updated_at DESC, path ASC
LIMIT sqlc.arg(result_limit);

-- name: SearchWikiPagesAll :many
WITH needle AS (
    SELECT websearch_to_tsquery('simple', sqlc.arg(search_query)) AS query
)
SELECT page.* FROM wiki_page page CROSS JOIN needle
WHERE (
        (workspace_id = sqlc.arg(workspace_id) AND scope IN ('workspace', 'project'))
        OR (workspace_id IS NULL AND scope = 'user' AND owner_user_id = sqlc.arg(owner_user_id))
      )
  AND (
      to_tsvector('simple', title || ' ' || path || ' ' || content) @@ needle.query
      OR LOWER(title || ' ' || path || ' ' || content) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%'
  )
ORDER BY
  ts_rank_cd(to_tsvector('simple', title || ' ' || path || ' ' || content), needle.query) DESC,
  CASE WHEN LOWER(title) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 0
       WHEN LOWER(path) LIKE '%' || LOWER(sqlc.arg(search_query)) || '%' THEN 1
       ELSE 2 END,
  scope ASC, updated_at DESC, path ASC
LIMIT sqlc.arg(result_limit);

-- name: GetWikiPage :one
SELECT * FROM wiki_page
WHERE id = $1;

-- name: GetWikiPageForActor :one
-- Authorization belongs in the predicate so an unauthorized row's content is
-- never returned to the API process. workspace_id is nullable for personal
-- library calls that do not have an active workspace.
SELECT * FROM wiki_page page
WHERE page.id = sqlc.arg(page_id)
  AND (
      (page.scope IN ('workspace', 'project')
       AND page.workspace_id = sqlc.narg(workspace_id)::uuid)
      OR
      (page.scope = 'user'
       AND page.workspace_id IS NULL
       AND page.owner_user_id = sqlc.narg(owner_user_id)::uuid)
  );

-- name: GetWikiPageInWorkspace :one
SELECT * FROM wiki_page
WHERE id = $1 AND workspace_id = $2;

-- name: GetRoomArtifactInWorkspaceForWikiEvidence :one
-- Evidence validation needs a tenant-bound existence check. Returning only the
-- identifier prevents proposal validation from loading unrelated room content.
SELECT id FROM room_artifact
WHERE id = sqlc.arg(id)
  AND workspace_id = sqlc.arg(workspace_id);

-- name: CreateWikiPage :one
WITH created AS (
    INSERT INTO wiki_page (
        workspace_id, scope, project_id, owner_user_id, path, title, content,
        content_digest, created_by, last_actor_id
    ) VALUES (
        sqlc.narg(workspace_id), sqlc.arg(scope), sqlc.narg(project_id), sqlc.narg(owner_user_id),
        sqlc.arg(path), sqlc.arg(title), sqlc.arg(content),
        'sha256:' || encode(sha256(convert_to(sqlc.arg(content), 'UTF8')), 'hex'),
        sqlc.narg(created_by), sqlc.narg(created_by)
    )
    RETURNING *
), revision AS (
    INSERT INTO wiki_page_revision (
        id, workspace_id, owner_user_id, page_id, revision_number, path, title,
        content, content_digest, actor_type, actor_id, source_kind
    )
    SELECT current_revision_id, workspace_id, owner_user_id, id, current_revision_number, path, title,
           content, content_digest, 'member', created_by, 'human'
    FROM created
    RETURNING page_id
)
SELECT created.* FROM created JOIN revision ON revision.page_id = created.id;

-- name: CreateWikiPageWithProvenance :one
WITH created AS (
    INSERT INTO wiki_page (
        workspace_id, scope, project_id, owner_user_id, path, title, content,
        content_digest, created_by, last_source_kind, last_actor_type, last_actor_id
    ) VALUES (
        sqlc.narg(workspace_id), sqlc.arg(scope), sqlc.narg(project_id), sqlc.narg(owner_user_id),
        sqlc.arg(path), sqlc.arg(title), sqlc.arg(content),
        'sha256:' || encode(sha256(convert_to(sqlc.arg(content), 'UTF8')), 'hex'),
        sqlc.narg(actor_id), sqlc.arg(source_kind), sqlc.arg(actor_type), sqlc.narg(actor_id)
    )
    RETURNING *
), revision AS (
    INSERT INTO wiki_page_revision (
        id, workspace_id, owner_user_id, page_id, revision_number, path, title,
        content, content_digest, actor_type, actor_id, source_kind, source_ref_id
    )
    SELECT current_revision_id, workspace_id, owner_user_id, id, current_revision_number, path, title,
           content, content_digest, sqlc.arg(actor_type), sqlc.narg(actor_id),
           sqlc.arg(source_kind), sqlc.narg(source_ref_id)
    FROM created
    RETURNING page_id
)
SELECT created.* FROM created JOIN revision ON revision.page_id = created.id;

-- name: UpdateWikiPage :one
WITH updated AS (
    UPDATE wiki_page page SET
        path = COALESCE(sqlc.narg(new_path)::text, page.path),
        title = COALESCE(sqlc.narg(new_title)::text, page.title),
        content = COALESCE(sqlc.narg(new_content)::text, page.content),
        content_digest = 'sha256:' || encode(sha256(convert_to(COALESCE(sqlc.narg(new_content)::text, page.content), 'UTF8')), 'hex'),
        current_revision_number = page.current_revision_number + 1,
        current_revision_id = gen_random_uuid(),
        last_source_kind = 'human',
        last_actor_type = 'member',
        last_actor_id = sqlc.arg(actor_id),
        updated_at = now()
    WHERE page.id = sqlc.arg(page_id)
      AND page.current_revision_number = sqlc.arg(expected_revision_number)
    RETURNING page.*
), revision AS (
    INSERT INTO wiki_page_revision (
        id, workspace_id, owner_user_id, page_id, revision_number, path, title,
        content, content_digest, actor_type, actor_id, source_kind
    )
    SELECT current_revision_id, workspace_id, owner_user_id, id, current_revision_number, path, title,
           content, content_digest, 'member', sqlc.arg(actor_id), 'human'
    FROM updated
    RETURNING page_id
)
SELECT updated.* FROM updated JOIN revision ON revision.page_id = updated.id;

-- name: ListWikiPageRevisions :many
SELECT * FROM wiki_page_revision
WHERE page_id = $1
ORDER BY revision_number DESC;

-- name: GetWikiPageRevision :one
SELECT * FROM wiki_page_revision
WHERE page_id = sqlc.arg(page_id)
  AND id = sqlc.arg(id);

-- name: GetWikiPageRevisionForActor :one
-- Stable evidence lookup is independent of the live page row. Authorization is
-- evaluated on the immutable revision's own tenant/owner columns before its
-- content is returned to the API process.
SELECT * FROM wiki_page_revision revision
WHERE revision.id = sqlc.arg(revision_id)
  AND (
      revision.workspace_id = sqlc.narg(workspace_id)::uuid
      OR
      (revision.workspace_id IS NULL
       AND revision.owner_user_id = sqlc.narg(owner_user_id)::uuid)
  );

-- name: RestoreWikiPageRevision :one
WITH restored AS (
    SELECT revision.*
    FROM wiki_page_revision revision
    WHERE revision.page_id = sqlc.arg(page_id)
      AND revision.id = sqlc.arg(revision_id)
), updated AS (
    UPDATE wiki_page page SET
        path = restored.path,
        title = restored.title,
        content = restored.content,
        content_digest = restored.content_digest,
        current_revision_number = page.current_revision_number + 1,
        current_revision_id = gen_random_uuid(),
        last_source_kind = 'restore',
        last_actor_type = 'member',
        last_actor_id = sqlc.arg(actor_id),
        updated_at = now()
    FROM restored
    WHERE page.id = restored.page_id
      AND page.current_revision_number = sqlc.arg(expected_revision_number)
    RETURNING page.*, restored.id AS restored_revision_id
), revision AS (
    INSERT INTO wiki_page_revision (
        id, workspace_id, owner_user_id, page_id, revision_number, path, title,
        content, content_digest, actor_type, actor_id, source_kind, source_ref_id
    )
    SELECT current_revision_id, workspace_id, owner_user_id, id, current_revision_number, path, title,
           content, content_digest, 'member', sqlc.arg(actor_id), 'restore', restored_revision_id
    FROM updated
    RETURNING page_id
)
SELECT updated.id, updated.workspace_id, updated.scope, updated.project_id,
       updated.owner_user_id, updated.path, updated.title, updated.content,
       updated.created_by, updated.created_at, updated.updated_at,
       updated.current_revision_number, updated.current_revision_id, updated.content_digest,
       updated.last_source_kind, updated.last_actor_type, updated.last_actor_id
FROM updated JOIN revision ON revision.page_id = updated.id;

-- name: CreateWikiPageEditProposal :one
WITH existing AS (
    SELECT proposal.*
    FROM wiki_page_edit_proposal proposal
    WHERE proposal.workspace_id = sqlc.arg(workspace_id)
      AND proposal.agent_id = sqlc.arg(agent_id)
      AND proposal.idempotency_key = sqlc.arg(idempotency_key)
), inserted AS (
    INSERT INTO wiki_page_edit_proposal (
        workspace_id, page_id, base_revision_number, proposed_path,
        proposed_title, proposed_content, content_digest, rationale,
        evidence_refs, agent_id, idempotency_key
    )
    SELECT page.workspace_id, page.id, sqlc.arg(base_revision_number),
           sqlc.arg(proposed_path), sqlc.arg(proposed_title), sqlc.arg(proposed_content),
           'sha256:' || encode(sha256(convert_to(sqlc.arg(proposed_content), 'UTF8')), 'hex'),
           sqlc.arg(rationale), sqlc.arg(evidence_refs)::jsonb,
           sqlc.arg(agent_id), sqlc.arg(idempotency_key)
    FROM wiki_page page
    WHERE page.id = sqlc.arg(page_id)
      AND page.workspace_id = sqlc.arg(workspace_id)
      AND page.scope IN ('workspace', 'project')
      AND page.current_revision_number = sqlc.arg(base_revision_number)
      AND NOT EXISTS (SELECT 1 FROM existing)
    ON CONFLICT (workspace_id, agent_id, idempotency_key) DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT * FROM existing
LIMIT 1;

-- name: GetWikiPageEditProposal :one
SELECT * FROM wiki_page_edit_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND page_id = sqlc.arg(page_id)
  AND id = sqlc.arg(id);

-- name: GetWikiPageEditProposalByIdempotencyKey :one
SELECT * FROM wiki_page_edit_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_id = sqlc.arg(agent_id)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: ListWikiPageEditProposals :many
SELECT * FROM wiki_page_edit_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND page_id = sqlc.arg(page_id)
ORDER BY created_at DESC;

-- name: AcceptWikiPageEditProposal :one
WITH candidate AS (
    SELECT proposal.*
    FROM wiki_page_edit_proposal proposal
    JOIN wiki_page page
      ON page.workspace_id = proposal.workspace_id
     AND page.id = proposal.page_id
    WHERE proposal.workspace_id = sqlc.arg(workspace_id)
      AND proposal.page_id = sqlc.arg(page_id)
      AND proposal.id = sqlc.arg(proposal_id)
      AND proposal.status = 'pending'
      AND proposal.base_revision_number = sqlc.arg(expected_revision_number)
      AND page.current_revision_number = sqlc.arg(expected_revision_number)
    FOR UPDATE OF proposal, page
), updated AS (
    UPDATE wiki_page page SET
        path = COALESCE(sqlc.narg(override_path)::text, candidate.proposed_path),
        title = COALESCE(sqlc.narg(override_title)::text, candidate.proposed_title),
        content = COALESCE(sqlc.narg(override_content)::text, candidate.proposed_content),
        content_digest = 'sha256:' || encode(sha256(convert_to(COALESCE(sqlc.narg(override_content)::text, candidate.proposed_content), 'UTF8')), 'hex'),
        current_revision_number = page.current_revision_number + 1,
        current_revision_id = gen_random_uuid(),
        last_source_kind = 'agent_proposal',
        last_actor_type = 'member',
        last_actor_id = sqlc.arg(reviewer_id),
        updated_at = now()
    FROM candidate
    WHERE page.id = candidate.page_id
    RETURNING page.*, candidate.id AS proposal_id
), revision AS (
    INSERT INTO wiki_page_revision (
        id, workspace_id, owner_user_id, page_id, revision_number, path, title,
        content, content_digest, actor_type, actor_id, source_kind, source_ref_id
    )
    SELECT current_revision_id, workspace_id, owner_user_id, id, current_revision_number, path, title,
           content, content_digest, 'member', sqlc.arg(reviewer_id), 'agent_proposal', proposal_id
    FROM updated
    RETURNING id, page_id
), reviewed AS (
    UPDATE wiki_page_edit_proposal proposal SET
        status = 'accepted',
        reviewed_by_id = sqlc.arg(reviewer_id),
        review_reason = sqlc.narg(review_reason),
        reviewed_at = now(),
        accepted_revision_id = revision.id
    FROM revision
    WHERE proposal.id = sqlc.arg(proposal_id)
    RETURNING proposal.id
)
SELECT updated.id, updated.workspace_id, updated.scope, updated.project_id,
       updated.owner_user_id, updated.path, updated.title, updated.content,
       updated.created_by, updated.created_at, updated.updated_at,
       updated.current_revision_number, updated.current_revision_id, updated.content_digest,
       updated.last_source_kind, updated.last_actor_type, updated.last_actor_id
FROM updated JOIN reviewed ON reviewed.id = updated.proposal_id;

-- name: RejectWikiPageEditProposal :one
UPDATE wiki_page_edit_proposal SET
    status = 'rejected',
    reviewed_by_id = sqlc.arg(reviewer_id),
    review_reason = sqlc.narg(review_reason),
    reviewed_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND page_id = sqlc.arg(page_id)
  AND id = sqlc.arg(id)
  AND status = 'pending'
RETURNING *;

-- name: DeleteWikiPage :exec
WITH deleted_proposals AS (
    DELETE FROM wiki_page_edit_proposal WHERE page_id = sqlc.arg(deleted_page_id)
)
DELETE FROM wiki_page page WHERE page.id = sqlc.arg(deleted_page_id)::uuid;
