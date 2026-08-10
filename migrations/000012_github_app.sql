ALTER TABLE forge_credentials ADD COLUMN IF NOT EXISTS installation_id TEXT NOT NULL DEFAULT '';

CREATE INDEX forge_credentials_installation_idx ON forge_credentials (installation_id) WHERE installation_id <> '';
