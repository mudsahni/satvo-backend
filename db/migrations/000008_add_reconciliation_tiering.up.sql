-- Add reconciliation_critical flag to validation rules
ALTER TABLE document_validation_rules
    ADD COLUMN reconciliation_critical BOOLEAN NOT NULL DEFAULT FALSE;

-- Add reconciliation_status to documents
ALTER TABLE documents
    ADD COLUMN reconciliation_status VARCHAR(20) NOT NULL DEFAULT 'pending';

-- Data migration: set reconciliation_critical = TRUE for the 21 critical rule keys
UPDATE document_validation_rules
SET reconciliation_critical = TRUE
WHERE builtin_rule_key IN (
    'req.invoice.number',
    'req.invoice.date',
    'req.invoice.place_of_supply',
    'req.seller.name',
    'req.seller.gstin',
    'req.buyer.gstin',
    'fmt.seller.gstin',
    'fmt.buyer.gstin',
    'fmt.seller.state_code',
    'fmt.buyer.state_code',
    'math.totals.taxable_amount',
    'math.totals.cgst',
    'math.totals.sgst',
    'math.totals.igst',
    'math.totals.grand_total',
    'xf.seller.gstin_state',
    'xf.buyer.gstin_state',
    'xf.tax_type.intrastate',
    'xf.tax_type.interstate',
    'logic.line_items.at_least_one',
    'logic.line_item.exclusive_tax'
);
