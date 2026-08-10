-- Phase 5 OCI registry metadata + Phase 6 releases/deployments + Phase 9 packages

CREATE TABLE oci_repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT oci_repositories_name_format CHECK (name ~ '^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$')
);

CREATE UNIQUE INDEX oci_repositories_project_name_uidx ON oci_repositories (project_id, name);

CREATE TABLE oci_manifests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES oci_repositories(id) ON DELETE CASCADE,
    digest TEXT NOT NULL,
    media_type TEXT NOT NULL,
    raw JSONB NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT oci_manifests_digest_format CHECK (digest ~ '^sha256:[a-f0-9]{64}$')
);

CREATE UNIQUE INDEX oci_manifests_repo_digest_uidx ON oci_manifests (repository_id, digest);

CREATE TABLE oci_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES oci_repositories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    manifest_id UUID NOT NULL REFERENCES oci_manifests(id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT oci_tags_name_len CHECK (char_length(name) BETWEEN 1 AND 128)
);

CREATE UNIQUE INDEX oci_tags_repo_name_uidx ON oci_tags (repository_id, name);

CREATE TABLE environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT environments_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$')
);

CREATE UNIQUE INDEX environments_project_slug_uidx ON environments (project_id, slug);

CREATE TABLE releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    run_id UUID REFERENCES pipeline_runs(id) ON DELETE SET NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT releases_version_len CHECK (char_length(version) BETWEEN 1 AND 128)
);

CREATE UNIQUE INDEX releases_project_version_uidx ON releases (project_id, version);

CREATE TABLE release_artifacts (
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    artifact_id UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    PRIMARY KEY (release_id, artifact_id)
);

CREATE TABLE deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    CONSTRAINT deployments_status_check CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'canceled'))
);

CREATE INDEX deployments_project_created_idx ON deployments (project_id, created_at DESC);

CREATE TABLE package_repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    format TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT package_repositories_format_check CHECK (format IN ('generic', 'maven', 'npm'))
);

CREATE UNIQUE INDEX package_repositories_project_name_uidx ON package_repositories (project_id, name);

CREATE TABLE package_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES package_repositories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    digest TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT package_versions_digest_format CHECK (digest ~ '^sha256:[a-f0-9]{64}$')
);

CREATE UNIQUE INDEX package_versions_repo_name_version_uidx ON package_versions (repository_id, name, version);

CREATE TABLE cluster_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id TEXT NOT NULL UNIQUE,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_leader BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE cluster_leases (
    name TEXT PRIMARY KEY,
    owner_node_id TEXT NOT NULL,
    fencing_token BIGINT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

UPDATE shipyard_meta
SET value = '9-packages', updated_at = now()
WHERE key = 'schema_phase';
