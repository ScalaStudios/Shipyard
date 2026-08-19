CREATE TABLE user_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX user_identities_provider_subject_uidx ON user_identities (provider, subject);
CREATE INDEX user_identities_user_idx ON user_identities (user_id);

UPDATE shipyard_meta SET value = '15-oidc-identities', updated_at = now() WHERE key = 'schema_phase';
