ALTER TABLE oci_manifests ALTER COLUMN raw TYPE BYTEA USING convert_to(raw::text, 'UTF8');
UPDATE oci_manifests SET digest = 'sha256:' || encode(sha256(raw), 'hex');

ALTER TABLE package_versions ADD COLUMN filename TEXT NOT NULL DEFAULT '';
DROP INDEX package_versions_repo_name_version_uidx;
CREATE UNIQUE INDEX package_versions_repo_name_version_file_uidx ON package_versions (repository_id, name, version, filename);

ALTER TABLE package_versions ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX jobs_running_lease_expires_idx ON jobs (lease_expires_at) WHERE status = 'running';

UPDATE shipyard_meta SET value = '14-registry-packages', updated_at = now() WHERE key = 'schema_phase';
