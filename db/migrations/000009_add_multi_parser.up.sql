-- Add parse_mode and field_provenance to documents
ALTER TABLE documents
    ADD COLUMN parse_mode VARCHAR(20) NOT NULL DEFAULT 'single',
    ADD COLUMN field_provenance JSONB NOT NULL DEFAULT '{}';
