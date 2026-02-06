DROP INDEX IF EXISTS idx_documents_parse_queue;
ALTER TABLE documents DROP COLUMN IF EXISTS retry_after;
