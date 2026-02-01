BEGIN;

-- Recreate the document_validation_results table
CREATE TABLE document_validation_results (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id    UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    rule_id        UUID NOT NULL REFERENCES document_validation_rules(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    passed         BOOLEAN NOT NULL,
    field_path     VARCHAR(500) NOT NULL DEFAULT '',
    expected_value TEXT NOT NULL DEFAULT '',
    actual_value   TEXT NOT NULL DEFAULT '',
    message        TEXT NOT NULL DEFAULT '',
    validated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_document_validation_results_document_id ON document_validation_results (document_id);
CREATE INDEX idx_document_validation_results_tenant_rule ON document_validation_results (tenant_id, rule_id);

-- Drop the validation_results column from documents
ALTER TABLE documents DROP COLUMN IF EXISTS validation_results;

COMMIT;
