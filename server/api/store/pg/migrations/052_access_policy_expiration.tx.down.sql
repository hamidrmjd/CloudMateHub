ALTER TABLE access_policies
    DROP CONSTRAINT access_policies_expiration_requires_allow,
    DROP COLUMN expires_at;
