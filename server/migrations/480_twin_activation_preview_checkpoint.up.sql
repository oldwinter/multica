CREATE TABLE twin_activation_preview_checkpoint (
    workspace_id UUID NOT NULL,
    twin_version_id UUID NOT NULL,
    policy_state TEXT NOT NULL CHECK (policy_state IN ('preview', 'enabled')),
    compiled_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
