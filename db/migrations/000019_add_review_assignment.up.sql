ALTER TABLE documents ADD COLUMN assigned_to UUID;
ALTER TABLE documents ADD COLUMN assigned_at TIMESTAMPTZ;
ALTER TABLE documents ADD COLUMN assigned_by UUID;

CREATE INDEX idx_documents_assigned_to ON documents (tenant_id, assigned_to) WHERE assigned_to IS NOT NULL;
