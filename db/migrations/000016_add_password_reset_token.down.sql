BEGIN;
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_token_id;
COMMIT;
