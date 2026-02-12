DROP INDEX IF EXISTS idx_users_provider_lookup;
ALTER TABLE users DROP COLUMN IF EXISTS provider_user_id;
ALTER TABLE users DROP COLUMN IF EXISTS auth_provider;
ALTER TABLE users ALTER COLUMN password_hash DROP DEFAULT;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
