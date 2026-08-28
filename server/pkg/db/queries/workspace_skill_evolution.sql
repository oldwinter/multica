-- Workspace Skill access owned by the SkillEvolution publisher.

-- name: GetWorkspaceSkillForEvolution :one
SELECT *
FROM skill
WHERE id = @skill_id
  AND workspace_id = @workspace_id;

-- name: LockWorkspaceSkillForEvolution :one
SELECT *
FROM skill
WHERE id = @skill_id
  AND workspace_id = @workspace_id
FOR UPDATE;

-- name: ListWorkspaceSkillFilesForEvolution :many
SELECT skill_file.*
FROM skill_file
JOIN skill ON skill.id = skill_file.skill_id
WHERE skill_file.skill_id = @skill_id
  AND skill.workspace_id = @workspace_id
ORDER BY skill_file.path ASC;

-- name: LockWorkspaceSkillFilesForEvolution :many
SELECT skill_file.*
FROM skill_file
JOIN skill ON skill.id = skill_file.skill_id
WHERE skill_file.skill_id = @skill_id
  AND skill.workspace_id = @workspace_id
ORDER BY skill_file.path ASC
FOR UPDATE OF skill_file;

-- name: UpdateWorkspaceSkillBundleForEvolution :one
UPDATE skill
SET name = @name,
    description = @description,
    content = @content
WHERE id = @skill_id
  AND workspace_id = @workspace_id
RETURNING *;

-- name: DeleteWorkspaceSkillFilesForEvolution :exec
DELETE FROM skill_file
USING skill
WHERE skill_file.skill_id = skill.id
  AND skill.id = @skill_id
  AND skill.workspace_id = @workspace_id;

-- name: CreateWorkspaceSkillFileForEvolution :one
INSERT INTO skill_file (skill_id, path, content)
SELECT skill.id, @path, @content
FROM skill
WHERE skill.id = @skill_id
  AND skill.workspace_id = @workspace_id
RETURNING *;
