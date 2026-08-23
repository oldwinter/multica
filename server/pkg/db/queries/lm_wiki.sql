-- name: LockLMWikiLifecycle :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(workspace_id)::uuid::text, 0))
WHERE EXISTS (SELECT 1 FROM workspace WHERE id = sqlc.arg(workspace_id));

-- name: ListLMWikiSourceIssues :many
SELECT id::text AS id, number, title, COALESCE(description, '') AS description,
       status, priority, COALESCE(project_id::text, '')::text AS project_id,
       COALESCE(start_date::text, '')::text AS start_date,
       COALESCE(due_date::text, '')::text AS due_date, created_at, updated_at
FROM issue
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY id;

-- name: ListLMWikiSourceProjects :many
SELECT id::text AS id, title, COALESCE(description, '') AS description,
       status, priority, COALESCE(start_date::text, '')::text AS start_date,
       COALESCE(due_date::text, '')::text AS due_date, created_at, updated_at
FROM project
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY id;

-- name: ListLMWikiSourceProjectResources :many
SELECT resource.id::text AS id, resource.project_id::text AS project_id,
       resource.resource_type, COALESCE(resource.label, '') AS label,
       resource.position, COALESCE(resource.resource_ref->>'url', '')::text AS git_url,
       COALESCE(resource.resource_ref->>'ref', '')::text AS ref,
       COALESCE(resource.resource_ref->>'default_branch_hint', '')::text AS default_branch_hint
FROM project_resource resource
JOIN project ON project.workspace_id = sqlc.arg(workspace_id)
            AND project.id = resource.project_id
WHERE resource.workspace_id = sqlc.arg(workspace_id)
  AND resource.resource_type = 'github_repo'
ORDER BY resource.id;

-- name: ListLMWikiSourceAutopilotRuns :many
SELECT run.id::text AS id, run.autopilot_id::text AS autopilot_id,
       autopilot.title AS autopilot_title, run.status, run.source,
       COALESCE(issue.id::text, '')::text AS issue_id,
       run.triggered_at, run.completed_at
FROM autopilot_run run
JOIN autopilot ON autopilot.workspace_id = sqlc.arg(workspace_id)
              AND autopilot.id = run.autopilot_id
LEFT JOIN issue ON issue.workspace_id = sqlc.arg(workspace_id)
               AND issue.id = run.issue_id
WHERE run.status = 'completed'
ORDER BY run.id;

-- name: CreateLMWikiRevision :one
WITH next_revision AS (
    SELECT COALESCE(MAX(revision_number), 0) + 1 AS revision_number
    FROM lm_wiki_revision
    WHERE workspace_id = sqlc.arg(workspace_id)
)
INSERT INTO lm_wiki_revision (
	workspace_id, revision_number, schema_version, source_digest, content,
	source_policy_version, source_policy_digest, remote_generation_enabled,
	trigger_kind, requested_by_id
)
SELECT sqlc.arg(workspace_id), next_revision.revision_number, 2,
       sqlc.arg(source_digest), sqlc.arg(content), sqlc.arg(source_policy_version),
       COALESCE(NULLIF(sqlc.arg(source_policy_digest), ''),
           'sha256:0000000000000000000000000000000000000000000000000000000000000000'),
       sqlc.arg(remote_generation_enabled),
       sqlc.arg(trigger_kind), sqlc.narg(requested_by_id)
FROM next_revision
RETURNING *;

-- name: GetLatestLMWikiRevision :one
SELECT * FROM lm_wiki_revision
WHERE workspace_id = $1
ORDER BY revision_number DESC
LIMIT 1;

-- name: ListLMWikiRevisions :many
SELECT * FROM lm_wiki_revision
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY revision_number DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: GetLMWikiRevision :one
SELECT * FROM lm_wiki_revision
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: CreateLMWikiCitations :exec
INSERT INTO lm_wiki_citation (
    workspace_id, revision_id, ordinal, citation_key, source_type, source_id,
    source_updated_at, locator, label, safe_metadata, source_digest
)
SELECT sqlc.arg(workspace_id), sqlc.arg(revision_id), citation.ordinal,
       citation.citation_key, citation.source_type, citation.source_id,
       citation.source_updated_at, citation.locator, citation.label,
       citation.safe_metadata, citation.source_digest
FROM jsonb_to_recordset(sqlc.arg(citations)::jsonb) AS citation(
    ordinal INTEGER,
    citation_key TEXT,
    source_type TEXT,
    source_id UUID,
    source_updated_at TIMESTAMPTZ,
    locator TEXT,
    label TEXT,
    safe_metadata JSONB,
    source_digest TEXT
)
JOIN lm_wiki_revision revision
  ON revision.workspace_id = sqlc.arg(workspace_id)
 AND revision.id = sqlc.arg(revision_id);

-- name: ListLMWikiCitations :many
SELECT * FROM lm_wiki_citation
WHERE workspace_id = sqlc.arg(workspace_id)
  AND revision_id = sqlc.arg(revision_id)
ORDER BY ordinal ASC;

-- name: CreateLMWikiReview :one
INSERT INTO lm_wiki_review (
    workspace_id, revision_id, decision, reviewer_id, reason
)
SELECT sqlc.arg(workspace_id), revision.id, sqlc.arg(decision),
       sqlc.arg(reviewer_id), sqlc.narg(reason)
FROM lm_wiki_revision revision
WHERE revision.workspace_id = sqlc.arg(workspace_id)
  AND revision.id = sqlc.arg(revision_id)
ON CONFLICT (workspace_id, revision_id) DO NOTHING
RETURNING *;

-- name: GetLMWikiReview :one
SELECT * FROM lm_wiki_review
WHERE workspace_id = sqlc.arg(workspace_id)
  AND revision_id = sqlc.arg(revision_id);

-- name: ListLMWikiReviews :many
SELECT * FROM lm_wiki_review
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY created_at DESC;

-- name: GetAcceptedLMWikiRevision :one
SELECT revision.*
FROM lm_wiki_revision revision
JOIN lm_wiki_review review
  ON review.workspace_id = revision.workspace_id
 AND review.revision_id = revision.id
WHERE revision.workspace_id = sqlc.arg(workspace_id)
  AND revision.id = sqlc.arg(revision_id)
  AND review.decision = 'accepted';

-- name: GetLatestAcceptedLMWikiRevision :one
SELECT revision.*
FROM lm_wiki_revision revision
JOIN lm_wiki_review review
  ON review.workspace_id = revision.workspace_id
 AND review.revision_id = revision.id
WHERE revision.workspace_id = sqlc.arg(workspace_id)
  AND review.decision = 'accepted'
ORDER BY revision.revision_number DESC
LIMIT 1;

-- name: ListLMWikiReconcileWorkspaces :many
SELECT id AS workspace_id
FROM workspace
WHERE id > sqlc.arg(workspace_id)
ORDER BY id ASC
LIMIT sqlc.arg(result_limit);

-- name: GetLMWikiSourcePolicy :one
SELECT * FROM lm_wiki_source_policy
WHERE workspace_id = $1;

-- name: UpsertLMWikiSourcePolicy :one
INSERT INTO lm_wiki_source_policy (
    workspace_id, source_classes, remote_generation_enabled, updated_by_id
) VALUES (
    sqlc.arg(workspace_id), sqlc.arg(source_classes)::jsonb,
    sqlc.arg(remote_generation_enabled), sqlc.arg(updated_by_id)
)
ON CONFLICT (workspace_id) DO UPDATE SET
    policy_version = lm_wiki_source_policy.policy_version + 1,
    source_classes = EXCLUDED.source_classes,
    remote_generation_enabled = EXCLUDED.remote_generation_enabled,
    updated_by_id = EXCLUDED.updated_by_id,
    updated_at = now()
RETURNING *;

-- name: GetWikiPageRevisionForLMWikiPolicy :one
SELECT revision.*
FROM wiki_page page
JOIN wiki_page_revision revision
  ON revision.workspace_id = page.workspace_id
 AND revision.page_id = page.id
WHERE page.workspace_id = sqlc.arg(workspace_id)
  AND page.id = sqlc.arg(page_id)
  AND page.scope IN ('workspace', 'project')
  AND revision.revision_number = sqlc.arg(revision_number);

-- name: DeleteLMWikiSourceWikiPages :exec
DELETE FROM lm_wiki_source_wiki_page
WHERE workspace_id = $1;

-- name: CreateLMWikiSourceWikiPages :exec
INSERT INTO lm_wiki_source_wiki_page (
    workspace_id, page_id, revision_id, revision_number, selected_by_id
)
SELECT sqlc.arg(workspace_id), selected.page_id, selected.revision_id,
       selected.revision_number, sqlc.arg(selected_by_id)
FROM jsonb_to_recordset(sqlc.arg(selections)::jsonb) AS selected(
    page_id UUID,
    revision_id UUID,
    revision_number BIGINT
)
ON CONFLICT (workspace_id, page_id) DO UPDATE SET
    revision_id = EXCLUDED.revision_id,
    revision_number = EXCLUDED.revision_number,
    selected_by_id = EXCLUDED.selected_by_id,
    selected_at = now();

-- name: ListLMWikiSourceWikiPages :many
SELECT * FROM lm_wiki_source_wiki_page
WHERE workspace_id = $1
ORDER BY page_id;

-- name: ListLMWikiSourceWikiPageRevisions :many
SELECT revision.id::text AS revision_id, page.id::text AS page_id,
       page.scope, COALESCE(page.project_id::text, '')::text AS project_id,
       revision.revision_number, revision.path, revision.title, revision.content,
       revision.content_digest, revision.created_at
FROM lm_wiki_source_wiki_page selected
JOIN wiki_page page
  ON page.workspace_id = selected.workspace_id
 AND page.id = selected.page_id
 AND page.scope IN ('workspace', 'project')
JOIN wiki_page_revision revision
  ON revision.workspace_id = selected.workspace_id
 AND revision.page_id = selected.page_id
 AND revision.id = selected.revision_id
 AND revision.revision_number = selected.revision_number
WHERE selected.workspace_id = sqlc.arg(workspace_id)
ORDER BY revision.id;
