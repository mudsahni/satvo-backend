ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash SET DEFAULT '';
ALTER TABLE users ADD COLUMN auth_provider VARCHAR(50) NOT NULL DEFAULT 'email';
ALTER TABLE users ADD COLUMN provider_user_id VARCHAR(255);
CREATE INDEX idx_users_provider_lookup ON users (tenant_id, auth_provider, provider_user_id)
    WHERE provider_user_id IS NOT NULL;
