-- Phase 0 foundation schema.
-- Identity and pipeline tables arrive in later phases.

CREATE TABLE IF NOT EXISTS shipyard_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO shipyard_meta (key, value)
VALUES ('schema_phase', '0-foundation')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();
