CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_evolution_revision_file_workspace_path_uidx ON skill_evolution_revision_file (workspace_id, revision_id, path);
