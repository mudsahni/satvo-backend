ALTER TABLE documents ADD COLUMN secondary_parser_model VARCHAR(100) DEFAULT '';
ALTER TABLE documents ADD COLUMN parse_attempts INTEGER DEFAULT 0;
