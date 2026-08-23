-- name: LockTwinLifecycle :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(workspace_id)::uuid::text, 1))
WHERE EXISTS (
    SELECT 1 FROM workspace WHERE id = sqlc.arg(workspace_id)
);

-- name: CreateTwinProposal :one
WITH inserted AS (
    INSERT INTO twin_proposal (
        workspace_id, kind, source_wiki_revision_id, base_twin_version_id,
        content, content_digest, requested_by_id
    )
    SELECT sqlc.arg(workspace_id), sqlc.arg(kind), source.id,
           sqlc.narg(base_twin_version_id), sqlc.arg(content),
           sqlc.arg(content_digest), sqlc.narg(requested_by_id)
    FROM lm_wiki_revision source
    LEFT JOIN twin_version base
      ON base.workspace_id = sqlc.arg(workspace_id)
     AND base.id = sqlc.narg(base_twin_version_id)
    WHERE source.workspace_id = sqlc.arg(workspace_id)
      AND source.id = sqlc.arg(source_wiki_revision_id)
      AND (sqlc.narg(base_twin_version_id)::uuid IS NULL OR base.id IS NOT NULL)
    ON CONFLICT (
        workspace_id, kind, source_wiki_revision_id,
        (COALESCE(base_twin_version_id, '00000000-0000-0000-0000-000000000000'::uuid))
    ) WHERE kind IN ('initial', 'evolution') DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT proposal.*
FROM twin_proposal proposal
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.kind = sqlc.arg(kind)
  AND proposal.source_wiki_revision_id = sqlc.arg(source_wiki_revision_id)
  AND proposal.base_twin_version_id IS NOT DISTINCT FROM sqlc.narg(base_twin_version_id)
LIMIT 1;

-- name: CreateTwinProposalV2 :one
WITH inserted AS (
    INSERT INTO twin_proposal (
        workspace_id, kind, source_wiki_revision_id, base_twin_version_id,
        schema_version, content, content_digest, requested_by_id
    )
    SELECT sqlc.arg(workspace_id), sqlc.arg(kind), source.id,
           sqlc.narg(base_twin_version_id), 2, sqlc.arg(content),
           sqlc.arg(content_digest), sqlc.narg(requested_by_id)
    FROM lm_wiki_revision source
    LEFT JOIN twin_version base
      ON base.workspace_id = sqlc.arg(workspace_id)
     AND base.id = sqlc.narg(base_twin_version_id)
    WHERE source.workspace_id = sqlc.arg(workspace_id)
      AND source.id = sqlc.arg(source_wiki_revision_id)
      AND (sqlc.narg(base_twin_version_id)::uuid IS NULL OR base.id IS NOT NULL)
    ON CONFLICT (
        workspace_id, kind, source_wiki_revision_id,
        (COALESCE(base_twin_version_id, '00000000-0000-0000-0000-000000000000'::uuid))
    ) WHERE kind IN ('initial', 'evolution') DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT proposal.*
FROM twin_proposal proposal
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.kind = sqlc.arg(kind)
  AND proposal.source_wiki_revision_id = sqlc.arg(source_wiki_revision_id)
  AND proposal.base_twin_version_id IS NOT DISTINCT FROM sqlc.narg(base_twin_version_id)
LIMIT 1;

-- name: GetTwinProposalByNaturalKey :one
SELECT * FROM twin_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND kind = sqlc.arg(kind)
  AND source_wiki_revision_id = sqlc.arg(source_wiki_revision_id)
  AND base_twin_version_id IS NOT DISTINCT FROM sqlc.narg(base_twin_version_id);

-- name: CreateTwinDepositionProposalV2 :one
INSERT INTO twin_proposal (
    workspace_id, kind, source_wiki_revision_id, base_twin_version_id,
    schema_version, content, content_digest, requested_by_id
)
SELECT sqlc.arg(workspace_id), 'deposition', source.id, base.id,
       2, sqlc.arg(content), sqlc.arg(content_digest), sqlc.arg(requested_by_id)
FROM twin_version base
JOIN lm_wiki_revision source
  ON source.workspace_id = base.workspace_id
 AND source.id = base.source_wiki_revision_id
WHERE base.workspace_id = sqlc.arg(workspace_id)
  AND base.id = sqlc.arg(base_twin_version_id)
  AND source.id = sqlc.arg(source_wiki_revision_id)
RETURNING *;

-- name: CreateTwinProposalCorrectionV2 :one
WITH inserted AS (
    INSERT INTO twin_proposal (
        workspace_id, kind, source_wiki_revision_id, base_twin_version_id,
        schema_version, content, content_digest, requested_by_id,
        replaces_proposal_id
    )
    SELECT target.workspace_id, 'correction', target.source_wiki_revision_id,
           target.base_twin_version_id, 2, sqlc.arg(content),
           sqlc.arg(content_digest), sqlc.arg(requested_by_id), target.id
    FROM twin_proposal target
    LEFT JOIN twin_proposal_review review
      ON review.workspace_id = target.workspace_id
     AND review.proposal_id = target.id
    WHERE target.workspace_id = sqlc.arg(workspace_id)
      AND target.id = sqlc.arg(replaces_proposal_id)
      AND target.kind IN ('initial', 'evolution', 'correction')
      AND review.id IS NULL
    ON CONFLICT (workspace_id, replaces_proposal_id)
      WHERE replaces_proposal_id IS NOT NULL DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT proposal.*
FROM twin_proposal proposal
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.replaces_proposal_id = sqlc.arg(replaces_proposal_id)
LIMIT 1;

-- name: GetTwinProposal :one
SELECT * FROM twin_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: GetTwinProposalReplacement :one
SELECT * FROM twin_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
  AND replaces_proposal_id = sqlc.arg(proposal_id);

-- name: GetPendingTwinProposal :one
SELECT proposal.*
FROM twin_proposal proposal
LEFT JOIN twin_proposal_review review
  ON review.workspace_id = proposal.workspace_id
 AND review.proposal_id = proposal.id
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND review.id IS NULL
ORDER BY proposal.created_at DESC
LIMIT 1;

-- name: ListTwinProposals :many
SELECT * FROM twin_proposal
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY created_at DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: GetTwinProposalSourceWikiRevision :one
SELECT revision.*
FROM lm_wiki_revision revision
JOIN twin_proposal proposal
  ON proposal.workspace_id = revision.workspace_id
 AND proposal.source_wiki_revision_id = revision.id
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.id = sqlc.arg(proposal_id);

-- name: GetTwinProposalBaseVersion :one
SELECT version.*
FROM twin_version version
JOIN twin_proposal proposal
  ON proposal.workspace_id = version.workspace_id
 AND proposal.base_twin_version_id = version.id
WHERE proposal.workspace_id = sqlc.arg(workspace_id)
  AND proposal.id = sqlc.arg(proposal_id);

-- name: CreateTwinProposalReview :one
WITH inserted AS (
    INSERT INTO twin_proposal_review (
        workspace_id, proposal_id, decision, reviewer_id, reason
    )
    SELECT sqlc.arg(workspace_id), proposal.id, sqlc.arg(decision),
           sqlc.arg(reviewer_id), sqlc.narg(reason)
    FROM twin_proposal proposal
    WHERE proposal.workspace_id = sqlc.arg(workspace_id)
      AND proposal.id = sqlc.arg(proposal_id)
    ON CONFLICT (workspace_id, proposal_id) DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT review.*
FROM twin_proposal_review review
WHERE review.workspace_id = sqlc.arg(workspace_id)
  AND review.proposal_id = sqlc.arg(proposal_id)
LIMIT 1;

-- name: GetTwinProposalReview :one
SELECT * FROM twin_proposal_review
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id);

-- name: ListTwinProposalReviews :many
SELECT * FROM twin_proposal_review
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY created_at DESC;

-- name: RejectOtherPendingTwinProposals :exec
WITH input AS (
    SELECT sqlc.arg(workspace_id)::uuid AS workspace_id,
           sqlc.arg(accepted_proposal_id)::uuid AS accepted_proposal_id,
           sqlc.arg(reviewer_id)::uuid AS reviewer_id,
           sqlc.arg(reason)::text AS reason
)
INSERT INTO twin_proposal_review (
    workspace_id, proposal_id, decision, reviewer_id, reason
)
SELECT proposal.workspace_id, proposal.id, 'rejected', input.reviewer_id, input.reason
FROM twin_proposal proposal
CROSS JOIN input
WHERE proposal.workspace_id = input.workspace_id
  AND proposal.id <> input.accepted_proposal_id
  AND NOT EXISTS (
      SELECT 1
      FROM twin_proposal_review review
      WHERE review.workspace_id = proposal.workspace_id
        AND review.proposal_id = proposal.id
  )
ON CONFLICT (workspace_id, proposal_id) DO NOTHING;

-- name: CreateTwinVersion :one
WITH next_version AS (
    SELECT COALESCE(MAX(version_number), 0) + 1 AS version_number
    FROM twin_version
    WHERE twin_version.workspace_id = sqlc.arg(workspace_id)
), inserted AS (
    INSERT INTO twin_version (
        workspace_id, version_number, proposal_id, source_wiki_revision_id,
        prior_version_id, schema_version, content, content_digest,
        signed_off_by_id
    )
    SELECT sqlc.arg(workspace_id), next_version.version_number, proposal.id,
           proposal.source_wiki_revision_id, proposal.base_twin_version_id,
           proposal.schema_version, proposal.content, proposal.content_digest,
           sqlc.arg(signed_off_by_id)
    FROM twin_proposal proposal
    JOIN twin_proposal_review review
      ON review.workspace_id = proposal.workspace_id
     AND review.proposal_id = proposal.id
     AND review.decision = 'accepted'
    CROSS JOIN next_version
    WHERE proposal.workspace_id = sqlc.arg(workspace_id)
      AND proposal.id = sqlc.arg(proposal_id)
    ON CONFLICT (workspace_id, proposal_id) DO NOTHING
    RETURNING *
)
SELECT * FROM inserted
UNION ALL
SELECT version.*
FROM twin_version version
WHERE version.workspace_id = sqlc.arg(workspace_id)
  AND version.proposal_id = sqlc.arg(proposal_id)
LIMIT 1;

-- name: GetTwinVersionByProposal :one
SELECT * FROM twin_version
WHERE workspace_id = sqlc.arg(workspace_id)
  AND proposal_id = sqlc.arg(proposal_id);

-- name: GetTwinVersion :one
SELECT * FROM twin_version
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id);

-- name: GetCurrentTwinVersion :one
SELECT * FROM twin_version
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY version_number DESC
LIMIT 1;

-- name: ListTwinVersions :many
SELECT * FROM twin_version
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY version_number DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
