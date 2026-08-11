-- name: ListWikiPagesByWorkspaceScope :many
SELECT id, workspace_id, scope, project_id, owner_user_id, path, title,
       created_by, created_at, updated_at
FROM wiki_page
WHERE workspace_id = $1
  AND scope = 'workspace'
ORDER BY path ASC;

-- name: ListWikiPagesByProject :many
SELECT id, workspace_id, scope, project_id, owner_user_id, path, title,
       created_by, created_at, updated_at
FROM wiki_page
WHERE workspace_id = $1
  AND scope = 'project'
  AND project_id = $2
ORDER BY path ASC;

-- name: ListWikiPagesByOwner :many
-- Personal wiki is cross-workspace: keyed only by owner_user_id.
SELECT id, workspace_id, scope, project_id, owner_user_id, path, title,
       created_by, created_at, updated_at
FROM wiki_page
WHERE scope = 'user'
  AND owner_user_id = $1
ORDER BY path ASC;

-- name: GetWikiPage :one
SELECT * FROM wiki_page
WHERE id = $1;

-- name: GetWikiPageInWorkspace :one
SELECT * FROM wiki_page
WHERE id = $1 AND workspace_id = $2;

-- name: CreateWikiPage :one
INSERT INTO wiki_page (
    workspace_id, scope, project_id, owner_user_id, path, title, content, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: UpdateWikiPage :one
UPDATE wiki_page SET
    path = COALESCE(sqlc.narg('path'), path),
    title = COALESCE(sqlc.narg('title'), title),
    content = COALESCE(sqlc.narg('content'), content),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteWikiPage :exec
DELETE FROM wiki_page
WHERE id = $1;
