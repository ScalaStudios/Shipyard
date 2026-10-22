ALTER TABLE package_versions ADD COLUMN IF NOT EXISTS protected boolean NOT NULL DEFAULT false;
