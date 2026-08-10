-- Discord notification integrations (Sentry-style channel alerts)

CREATE TABLE discord_integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'webhook',
    webhook_url TEXT NOT NULL DEFAULT '',
    bot_token TEXT NOT NULL DEFAULT '',
    channel_id TEXT NOT NULL DEFAULT '',
    notify_on TEXT[] NOT NULL DEFAULT ARRAY['run.started','run.succeeded','run.failed','run.canceled'],
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT discord_integrations_mode_check CHECK (mode IN ('webhook', 'bot')),
    CONSTRAINT discord_integrations_name_format CHECK (name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$')
);

CREATE UNIQUE INDEX discord_integrations_org_name_uidx
    ON discord_integrations (
        organization_id,
        (COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid)),
        name
    );
CREATE INDEX discord_integrations_org_idx ON discord_integrations (organization_id);
CREATE INDEX discord_integrations_project_idx ON discord_integrations (project_id);

UPDATE shipyard_meta
SET value = '9-discord', updated_at = now()
WHERE key = 'schema_phase';
