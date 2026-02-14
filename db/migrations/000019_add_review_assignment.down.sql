DROP INDEX IF EXISTS idx_documents_assigned_to;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_by;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_at;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_to;
