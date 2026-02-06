ALTER TABLE documents ADD COLUMN retry_after TIMESTAMPTZ;

CREATE INDEX idx_documents_parse_queue
    ON documents (retry_after ASC)
    WHERE parsing_status = 'queued';
