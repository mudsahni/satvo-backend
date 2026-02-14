ALTER TABLE document_summaries ALTER COLUMN seller_gstin TYPE VARCHAR(20);
ALTER TABLE document_summaries ALTER COLUMN buyer_gstin TYPE VARCHAR(20);
ALTER TABLE document_summaries ALTER COLUMN seller_state_code TYPE VARCHAR(10);
ALTER TABLE document_summaries ALTER COLUMN buyer_state_code TYPE VARCHAR(10);
ALTER TABLE document_summaries ALTER COLUMN currency TYPE VARCHAR(10);
ALTER TABLE document_summaries ALTER COLUMN invoice_type TYPE VARCHAR(50);
