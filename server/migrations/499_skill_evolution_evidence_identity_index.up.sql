CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_evidence_identity_uidx ON skill_evolution_evidence (workspace_id, proposal_id, kind, source_id, source_revision_id);
