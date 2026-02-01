BEGIN;

DROP INDEX IF EXISTS idx_documents_validation_status;
ALTER TABLE documents DROP COLUMN IF EXISTS validation_status;

DROP INDEX IF EXISTS idx_dvr_tenant_builtin_key;
ALTER TABLE document_validation_rules DROP COLUMN IF EXISTS builtin_rule_key;
ALTER TABLE document_validation_rules DROP COLUMN IF EXISTS is_builtin;

COMMIT;
