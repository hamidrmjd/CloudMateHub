ALTER TABLE access_policies
    ADD COLUMN expires_at timestamptz,
    ADD CONSTRAINT access_policies_expiration_requires_allow
        CHECK (expires_at IS NULL OR action = 'allow');
