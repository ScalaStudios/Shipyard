-- SCM connections, webhook deliveries, in-app notifications, run SCM context

CREATE TABLE scm_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    repo_owner TEXT NOT NULL DEFAULT '',
    repo_name TEXT NOT NULL DEFAULT '',
    access_token TEXT NOT NULL DEFAULT '',
    bot_username TEXT NOT NULL DEFAULT 'shipyard[bot]',
    webhook_secret TEXT NOT NULL DEFAULT '',
    pipeline_slug TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT scm_connections_provider_check CHECK (provider IN (
        'github', 'gitea', 'forgejo', 'gitlab', 'gogs', 'onedev',
        'gitbucket', 'codebase', 'pagure', 'codeberg', 'generic'
    )),
    CONSTRAINT scm_connections_name_format CHECK (name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$')
);

CREATE UNIQUE INDEX scm_connections_project_name_uidx ON scm_connections (project_id, name);
CREATE INDEX scm_connections_project_provider_idx ON scm_connections (project_id, provider);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id UUID REFERENCES scm_connections(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    event_type TEXT NOT NULL DEFAULT '',
    delivery_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'accepted',
    run_id UUID REFERENCES pipeline_runs(id) ON DELETE SET NULL,
    error_message TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN (
        'accepted', 'processed', 'ignored', 'failed'
    ))
);

CREATE INDEX webhook_deliveries_project_created_idx ON webhook_deliveries (project_id, created_at DESC);
CREATE INDEX webhook_deliveries_delivery_id_idx ON webhook_deliveries (delivery_id);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    href TEXT NOT NULL DEFAULT '',
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notifications_user_created_idx ON notifications (user_id, created_at DESC);
CREATE INDEX notifications_user_unread_idx ON notifications (user_id) WHERE read_at IS NULL;

ALTER TABLE pipeline_runs
    ADD COLUMN scm_connection_id UUID REFERENCES scm_connections(id) ON DELETE SET NULL,
    ADD COLUMN scm_pr_number INT NOT NULL DEFAULT 0,
    ADD COLUMN scm_comment_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN scm_target_url TEXT NOT NULL DEFAULT '';

UPDATE shipyard_meta
SET value = '8-scm-integrations', updated_at = now()
WHERE key = 'schema_phase';
