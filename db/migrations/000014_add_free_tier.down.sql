BEGIN;

ALTER TABLE users DROP COLUMN IF EXISTS monthly_document_limit;
ALTER TABLE users DROP COLUMN IF EXISTS documents_used_this_period;
ALTER TABLE users DROP COLUMN IF EXISTS current_period_start;

COMMIT;
