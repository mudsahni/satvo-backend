BEGIN;

CREATE TABLE hsn_codes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(8)   NOT NULL,
    description     TEXT         NOT NULL DEFAULT '',
    gst_rate        NUMERIC(5,2) NOT NULL,
    condition_desc  TEXT         NOT NULL DEFAULT '',
    effective_from  DATE         NOT NULL DEFAULT '2017-07-01',
    effective_to    DATE,
    parent_code     VARCHAR(8),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hsn_codes_code ON hsn_codes (code);
CREATE INDEX idx_hsn_codes_parent ON hsn_codes (parent_code) WHERE parent_code IS NOT NULL;
CREATE UNIQUE INDEX idx_hsn_codes_unique ON hsn_codes (code, gst_rate, condition_desc, effective_from);

COMMIT;
