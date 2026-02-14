-- Widen all short VARCHAR columns that receive LLM-parsed data.
-- This is a denormalized reporting table; tight constraints add no value
-- when the source is unpredictable AI output.
ALTER TABLE document_summaries ALTER COLUMN seller_gstin TYPE VARCHAR(50);
ALTER TABLE document_summaries ALTER COLUMN buyer_gstin TYPE VARCHAR(50);
ALTER TABLE document_summaries ALTER COLUMN seller_state_code TYPE VARCHAR(50);
ALTER TABLE document_summaries ALTER COLUMN buyer_state_code TYPE VARCHAR(50);
ALTER TABLE document_summaries ALTER COLUMN currency TYPE VARCHAR(20);
ALTER TABLE document_summaries ALTER COLUMN invoice_type TYPE VARCHAR(100);
