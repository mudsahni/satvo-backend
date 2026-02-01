BEGIN;

ALTER TABLE document_validation_rules
    ADD COLUMN is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN builtin_rule_key VARCHAR(100);

CREATE UNIQUE INDEX idx_dvr_tenant_builtin_key
    ON document_validation_rules (tenant_id, builtin_rule_key)
    WHERE builtin_rule_key IS NOT NULL;

ALTER TABLE documents
    ADD COLUMN validation_status VARCHAR(20) NOT NULL DEFAULT 'pending';

CREATE INDEX idx_documents_validation_status ON documents (tenant_id, validation_status);

COMMIT;
