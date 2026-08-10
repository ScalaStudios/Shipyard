-- Phase 2 pipeline engine

CREATE TABLE pipeline_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    yaml_source TEXT NOT NULL,
    parsed JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pipeline_definitions_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$')
);

CREATE UNIQUE INDEX pipeline_definitions_project_slug_uidx ON pipeline_definitions (project_id, slug);

CREATE TABLE pipeline_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id UUID NOT NULL REFERENCES pipeline_definitions(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    number BIGINT NOT NULL,
    status TEXT NOT NULL,
    trigger_type TEXT NOT NULL DEFAULT 'manual',
    triggered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    git_ref TEXT NOT NULL DEFAULT '',
    git_sha TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    queued_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT pipeline_runs_status_check CHECK (status IN (
        'pending', 'queued', 'running', 'succeeded', 'failed', 'canceled', 'skipped'
    ))
);

CREATE UNIQUE INDEX pipeline_runs_pipeline_number_uidx ON pipeline_runs (pipeline_id, number);
CREATE INDEX pipeline_runs_project_created_idx ON pipeline_runs (project_id, created_at DESC);
CREATE INDEX pipeline_runs_status_idx ON pipeline_runs (status);

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    needs TEXT[] NOT NULL DEFAULT '{}',
    runner_labels TEXT[] NOT NULL DEFAULT '{}',
    image TEXT NOT NULL DEFAULT '',
    workspace_path TEXT NOT NULL DEFAULT '',
    attempt INT NOT NULL DEFAULT 1,
    runner_id UUID,
    lease_id TEXT,
    lease_expires_at TIMESTAMPTZ,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    queued_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT jobs_status_check CHECK (status IN (
        'pending', 'queued', 'leased', 'running', 'succeeded', 'failed', 'canceled', 'skipped'
    ))
);

CREATE UNIQUE INDEX jobs_run_name_uidx ON jobs (run_id, name);
CREATE INDEX jobs_status_idx ON jobs (status);
CREATE INDEX jobs_lease_expires_idx ON jobs (lease_expires_at) WHERE status = 'leased';

CREATE TABLE job_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    position INT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    uses TEXT NOT NULL DEFAULT '',
    run_script TEXT NOT NULL DEFAULT '',
    with_args JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending',
    exit_code INT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT job_steps_status_check CHECK (status IN (
        'pending', 'running', 'succeeded', 'failed', 'canceled', 'skipped'
    ))
);

CREATE UNIQUE INDEX job_steps_job_position_uidx ON job_steps (job_id, position);

CREATE TABLE job_logs (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    step_id UUID REFERENCES job_steps(id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    stream TEXT NOT NULL DEFAULT 'stdout',
    line TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT job_logs_stream_check CHECK (stream IN ('stdout', 'stderr', 'system'))
);

CREATE UNIQUE INDEX job_logs_job_seq_uidx ON job_logs (job_id, seq);
CREATE INDEX job_logs_job_created_idx ON job_logs (job_id, created_at);

UPDATE shipyard_meta
SET value = '2-pipelines', updated_at = now()
WHERE key = 'schema_phase';
