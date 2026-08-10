-- Secrets foundation

CREATE TABLE secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID REFERENCES environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    ciphertext BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT secrets_name_len CHECK (char_length(name) BETWEEN 1 AND 128),
    CONSTRAINT secrets_scope_check CHECK (
        organization_id IS NOT NULL OR project_id IS NOT NULL OR environment_id IS NOT NULL
    )
);

CREATE UNIQUE INDEX secrets_org_name_uidx ON secrets (organization_id, name) WHERE organization_id IS NOT NULL AND project_id IS NULL AND environment_id IS NULL;
CREATE UNIQUE INDEX secrets_project_name_uidx ON secrets (project_id, name) WHERE project_id IS NOT NULL AND environment_id IS NULL;
CREATE UNIQUE INDEX secrets_env_name_uidx ON secrets (environment_id, name) WHERE environment_id IS NOT NULL;
