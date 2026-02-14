CREATE TABLE document_summaries (
    document_id           UUID PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    tenant_id             UUID NOT NULL,
    collection_id         UUID NOT NULL,

    -- Invoice identity
    invoice_number        VARCHAR(100),
    invoice_date          DATE,
    due_date              DATE,
    invoice_type          VARCHAR(50),
    currency              VARCHAR(10),
    place_of_supply       VARCHAR(100),
    reverse_charge        BOOLEAN DEFAULT FALSE,
    has_irn               BOOLEAN DEFAULT FALSE,

    -- Parties
    seller_name           VARCHAR(500),
    seller_gstin          VARCHAR(15),
    seller_state          VARCHAR(100),
    seller_state_code     VARCHAR(10),
    buyer_name            VARCHAR(500),
    buyer_gstin           VARCHAR(15),
    buyer_state           VARCHAR(100),
    buyer_state_code      VARCHAR(10),

    -- Financials
    subtotal              NUMERIC(15,2) DEFAULT 0,
    total_discount        NUMERIC(15,2) DEFAULT 0,
    taxable_amount        NUMERIC(15,2) DEFAULT 0,
    cgst                  NUMERIC(15,2) DEFAULT 0,
    sgst                  NUMERIC(15,2) DEFAULT 0,
    igst                  NUMERIC(15,2) DEFAULT 0,
    cess                  NUMERIC(15,2) DEFAULT 0,
    total_amount          NUMERIC(15,2) DEFAULT 0,

    -- Line item stats
    line_item_count       INTEGER DEFAULT 0,
    distinct_hsn_codes    TEXT[],

    -- Document status snapshot
    parsing_status        VARCHAR(20),
    review_status         VARCHAR(20),
    validation_status     VARCHAR(20),
    reconciliation_status VARCHAR(20),

    -- Timestamps
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_doc_summaries_tenant ON document_summaries(tenant_id);
CREATE INDEX idx_doc_summaries_seller ON document_summaries(tenant_id, seller_gstin);
CREATE INDEX idx_doc_summaries_buyer ON document_summaries(tenant_id, buyer_gstin);
CREATE INDEX idx_doc_summaries_date ON document_summaries(tenant_id, invoice_date);
CREATE INDEX idx_doc_summaries_collection ON document_summaries(tenant_id, collection_id);
CREATE INDEX idx_doc_summaries_seller_buyer ON document_summaries(tenant_id, seller_gstin, buyer_gstin);
