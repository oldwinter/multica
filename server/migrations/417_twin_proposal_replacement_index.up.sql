CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS twin_proposal_workspace_replacement_uidx ON twin_proposal (workspace_id, replaces_proposal_id) WHERE replaces_proposal_id IS NOT NULL;
