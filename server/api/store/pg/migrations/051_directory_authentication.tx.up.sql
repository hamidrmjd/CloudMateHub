ALTER TYPE user_origin ADD VALUE IF NOT EXISTS 'ad';
--bun:split
ALTER TYPE user_origin ADD VALUE IF NOT EXISTS 'ldap';
--bun:split
ALTER TYPE user_auth_method ADD VALUE IF NOT EXISTS 'ldap';
