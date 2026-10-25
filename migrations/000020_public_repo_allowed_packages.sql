ALTER TABLE package_repositories ADD COLUMN IF NOT EXISTS allowed_packages TEXT[] NOT NULL DEFAULT '{}';
