-- Phase 3 runners

CREATE TABLE runners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    labels TEXT[] NOT NULL DEFAULT '{}',
    capabilities TEXT[] NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'offline',
    last_heartbeat_at TIMESTAMPTZ,
    registered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    drained BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT runners_status_check CHECK (status IN ('offline', 'idle', 'busy', 'draining'))
);

CREATE INDEX runners_labels_idx ON runners USING GIN (labels);
CREATE INDEX runners_status_idx ON runners (status);

CREATE TABLE runner_registration_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE jobs
    ADD CONSTRAINT jobs_runner_id_fkey FOREIGN KEY (runner_id) REFERENCES runners(id) ON DELETE SET NULL;

UPDATE shipyard_meta
SET value = '3-runners', updated_at = now()
WHERE key = 'schema_phase';
