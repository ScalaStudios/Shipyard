ALTER TABLE environments ADD COLUMN deploy_pipeline_slug TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN run_id UUID REFERENCES pipeline_runs(id) ON DELETE SET NULL;

UPDATE shipyard_meta SET value = '16-deployments', updated_at = now() WHERE key = 'schema_phase';
