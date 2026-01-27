BEGIN;

CREATE TABLE file_metadata (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    uploaded_by   UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    file_name     VARCHAR(500) NOT NULL,
    original_name VARCHAR(500) NOT NULL,
    file_type     VARCHAR(10)  NOT NULL,
    file_size     BIGINT NOT NULL,
    s3_bucket     VARCHAR(255) NOT NULL,
    s3_key        VARCHAR(1000) NOT NULL,
    content_type  VARCHAR(100) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_file_metadata_tenant_id ON file_metadata (tenant_id);
CREATE INDEX idx_file_metadata_uploaded_by ON file_metadata (tenant_id, uploaded_by);
CREATE INDEX idx_file_metadata_status ON file_metadata (tenant_id, status);

COMMIT;
