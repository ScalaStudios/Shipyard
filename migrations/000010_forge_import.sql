CREATE TABLE forge_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'pat',
    name TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    access_token TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT forge_credentials_provider_check CHECK (provider IN (
        'github', 'gitea', 'forgejo', 'gitlab', 'gogs', 'onedev',
        'gitbucket', 'codebase', 'pagure', 'codeberg', 'generic'
    )),
    CONSTRAINT forge_credentials_kind_check CHECK (kind IN (
        'pat', 'oauth_user', 'github_app_install', 'forgejo_oauth'
    )),
    CONSTRAINT forge_credentials_name_format CHECK (name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$')
);

CREATE UNIQUE INDEX forge_credentials_org_name_uidx ON forge_credentials (organization_id, name);
CREATE INDEX forge_credentials_org_idx ON forge_credentials (organization_id);

CREATE TABLE forge_import_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    credential_id UUID NOT NULL REFERENCES forge_credentials(id) ON DELETE CASCADE,
    remote_org TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    total_count INT NOT NULL DEFAULT 0,
    completed_count INT NOT NULL DEFAULT 0,
    created_count INT NOT NULL DEFAULT 0,
    skipped_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT forge_import_jobs_status_check CHECK (status IN (
        'queued', 'running', 'completed', 'failed'
    ))
);

CREATE INDEX forge_import_jobs_org_created_idx ON forge_import_jobs (organization_id, created_at DESC);

CREATE TABLE forge_import_job_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES forge_import_jobs(id) ON DELETE CASCADE,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    connection_id UUID REFERENCES scm_connections(id) ON DELETE SET NULL,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT forge_import_job_items_status_check CHECK (status IN (
        'queued', 'created', 'skipped_exists', 'failed'
    ))
);

CREATE INDEX forge_import_job_items_job_idx ON forge_import_job_items (job_id);

UPDATE shipyard_meta
SET value = '10-forge-import', updated_at = now()
WHERE key = 'schema_phase';
