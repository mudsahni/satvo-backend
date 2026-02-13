CREATE TABLE document_audit_log (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL,
    document_id UUID NOT NULL,
    user_id     UUID,
    action      VARCHAR(50) NOT NULL,
    changes     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_document ON document_audit_log (document_id, created_at DESC);
CREATE INDEX idx_audit_log_tenant   ON document_audit_log (tenant_id, created_at DESC);
CREATE INDEX idx_audit_log_user     ON document_audit_log (user_id, created_at DESC) WHERE user_id IS NOT NULL;
