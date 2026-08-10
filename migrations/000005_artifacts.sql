-- Phase 4 artifacts

CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    run_id UUID REFERENCES pipeline_runs(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    digest TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT artifacts_digest_format CHECK (digest ~ '^sha256:[a-f0-9]{64}$')
);

CREATE INDEX artifacts_project_created_idx ON artifacts (project_id, created_at DESC);
CREATE INDEX artifacts_digest_idx ON artifacts (digest);
CREATE UNIQUE INDEX artifacts_project_name_digest_uidx ON artifacts (project_id, name, digest);

UPDATE shipyard_meta
SET value = '4-artifacts', updated_at = now()
WHERE key = 'schema_phase';
