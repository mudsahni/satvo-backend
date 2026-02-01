BEGIN;

-- Add validation_results JSONB column to documents table
ALTER TABLE documents ADD COLUMN validation_results JSONB NOT NULL DEFAULT '[]';

-- Drop the document_validation_results table
DROP TABLE IF EXISTS document_validation_results;

COMMIT;
