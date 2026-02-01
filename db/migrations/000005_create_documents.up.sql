BEGIN;

CREATE TABLE documents (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    collection_id     UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    file_id           UUID NOT NULL REFERENCES file_metadata(id) ON DELETE CASCADE,
    document_type     VARCHAR(50) NOT NULL,
    parser_model      VARCHAR(100) NOT NULL DEFAULT '',
    parser_prompt     TEXT NOT NULL DEFAULT '',
    structured_data   JSONB NOT NULL DEFAULT '{}',
    confidence_scores JSONB NOT NULL DEFAULT '{}',
    parsing_status    VARCHAR(20) NOT NULL DEFAULT 'pending',
    parsing_error     TEXT NOT NULL DEFAULT '',
    parsed_at         TIMESTAMPTZ,
    review_status     VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewed_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at       TIMESTAMPTZ,
    reviewer_notes    TEXT NOT NULL DEFAULT '',
    created_by        UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_tenant_id ON documents (tenant_id);
CREATE INDEX idx_documents_collection_id ON documents (collection_id);
CREATE UNIQUE INDEX idx_documents_file_id ON documents (file_id);
CREATE INDEX idx_documents_tenant_parsing_status ON documents (tenant_id, parsing_status);
CREATE INDEX idx_documents_tenant_review_status ON documents (tenant_id, review_status);
CREATE INDEX idx_documents_tenant_document_type ON documents (tenant_id, document_type);
CREATE INDEX idx_documents_structured_data ON documents USING GIN (structured_data);

CREATE TABLE document_tags (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key         VARCHAR(100) NOT NULL,
    value       VARCHAR(500) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_document_tags_tenant_key_value ON document_tags (tenant_id, key, value);
CREATE INDEX idx_document_tags_document_id ON document_tags (document_id);
CREATE INDEX idx_document_tags_tenant_key ON document_tags (tenant_id, key);

CREATE TABLE document_validation_rules (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    collection_id   UUID REFERENCES collections(id) ON DELETE CASCADE,
    document_type   VARCHAR(50) NOT NULL,
    rule_name       VARCHAR(255) NOT NULL,
    rule_type       VARCHAR(50) NOT NULL,
    rule_config     JSONB NOT NULL DEFAULT '{}',
    severity        VARCHAR(20) NOT NULL DEFAULT 'error',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_document_validation_rules_tenant_type_active ON document_validation_rules (tenant_id, document_type, is_active);
CREATE INDEX idx_document_validation_rules_collection_id ON document_validation_rules (collection_id);

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

COMMIT;
