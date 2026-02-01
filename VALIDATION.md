# Validation Rules Reference

SATVOS includes an automated validation engine with **50 built-in rules** for GST (Goods and Services Tax) invoices. Rules are organized into 5 categories, each with an error or warning severity level.

## Table of Contents

- [Overview](#overview)
- [Validation Statuses](#validation-statuses)
- [Rule Categories](#rule-categories)
  - [Required Fields (12 rules)](#required-fields-12-rules)
  - [Format (13 rules)](#format-13-rules)
  - [Mathematical (12 rules)](#mathematical-12-rules)
  - [Cross-field (8 rules)](#cross-field-8-rules)
  - [Logical (7 rules)](#logical-7-rules)
- [GST Invoice Schema](#gst-invoice-schema)
- [API Endpoints](#api-endpoints)
- [Architecture](#architecture)
- [Extending the Validator](#extending-the-validator)

## Overview

```
  Document Parsed (structured_data saved)
       │
       ▼
  ┌────────────────────────────────────────────┐
  │         Validation Engine                   │
  │                                             │
  │  1. Auto-seed builtin rules (first run)     │
  │  2. Load active rules for tenant + doc type │
  │  3. Run each rule against parsed data       │
  │  4. Compute per-field statuses              │
  │  5. Save results to documents table (JSONB) │
  └────────────────────────────────────────────┘
       │
       ▼
  validation_status: valid | warning | invalid
```

Validation runs automatically after LLM parsing completes. It can also be re-triggered manually via `POST /documents/:id/validate`.

## Validation Statuses

### Document-Level Status

| Status | Meaning |
|--------|---------|
| `pending` | Not yet validated (parsing still in progress) |
| `valid` | All rules passed |
| `warning` | Some warning-severity rules failed, no errors |
| `invalid` | One or more error-severity rules failed |

### Field-Level Status

| Status | Meaning |
|--------|---------|
| `valid` | All rules passed for this field |
| `invalid` | An error-severity rule failed for this field |
| `unsure` | A warning-severity rule failed, or the LLM confidence score is <= 0.5 |

### Severity Levels

| Severity | Meaning | Impact |
|----------|---------|--------|
| `error` | Critical validation failure | Sets document status to `invalid` |
| `warning` | Data quality concern | Sets document status to `warning` (unless errors also exist) |

---

## Rule Categories

### Required Fields (12 rules)

Checks that mandatory fields are present and non-empty.

| Rule Key | Field Path | Severity | Description |
|----------|-----------|----------|-------------|
| `req.invoice.number` | `invoice.invoice_number` | Error | Invoice number must not be empty |
| `req.invoice.date` | `invoice.invoice_date` | Error | Invoice date must not be empty |
| `req.invoice.place_of_supply` | `invoice.place_of_supply` | Error | Place of supply must not be empty |
| `req.invoice.currency` | `invoice.currency` | Warning | Currency should be specified |
| `req.seller.name` | `seller.name` | Error | Seller name must not be empty |
| `req.seller.gstin` | `seller.gstin` | Error | Seller GSTIN must not be empty |
| `req.seller.state_code` | `seller.state_code` | Error | Seller state code must not be empty |
| `req.buyer.name` | `buyer.name` | Error | Buyer name must not be empty |
| `req.buyer.gstin` | `buyer.gstin` | Error | Buyer GSTIN must not be empty |
| `req.buyer.state_code` | `buyer.state_code` | Error | Buyer state code must not be empty |
| `req.line_item.description` | `line_items[i].description` | Warning | Each line item should have a description |
| `req.line_item.hsn_sac` | `line_items[i].hsn_sac_code` | Warning | Each line item should have an HSN/SAC code |

**Source**: `internal/validator/invoice/required.go`

---

### Format (13 rules)

Validates field values against regex patterns, date formats, enumeration lists, and state code ranges.

| Rule Key | Field Path | Severity | Validation |
|----------|-----------|----------|------------|
| `fmt.seller.gstin` | `seller.gstin` | Error | Matches GSTIN pattern: `^\d{2}[A-Z]{5}\d{4}[A-Z][1-9A-Z]Z[0-9A-Z]$` (15 characters) |
| `fmt.buyer.gstin` | `buyer.gstin` | Error | Same GSTIN pattern |
| `fmt.seller.pan` | `seller.pan` | Error | Matches PAN pattern: `^[A-Z]{5}\d{4}[A-Z]$` (10 characters) |
| `fmt.buyer.pan` | `buyer.pan` | Error | Same PAN pattern |
| `fmt.seller.state_code` | `seller.state_code` | Error | 2-digit code between 01 and 38 |
| `fmt.buyer.state_code` | `buyer.state_code` | Error | Same state code range |
| `fmt.invoice.date` | `invoice.invoice_date` | Error | Parseable date (see supported formats below) |
| `fmt.invoice.due_date` | `invoice.due_date` | Warning | Parseable date |
| `fmt.invoice.currency` | `invoice.currency` | Warning | Valid ISO 4217 currency code (INR, USD, EUR, GBP, JPY, AUD, CAD, CHF, CNY, SGD, AED, SAR, HKD, MYR, THB, NZD, SEK, NOK, DKK, ZAR) |
| `fmt.payment.ifsc` | `payment.ifsc_code` | Warning | Matches IFSC pattern: `^[A-Z]{4}0[A-Z0-9]{6}$` |
| `fmt.payment.account_no` | `payment.account_number` | Warning | 9-18 digits: `^\d{9,18}$` |
| `fmt.line_item.hsn_sac` | `line_items[i].hsn_sac_code` | Warning | 4-8 digits: `^\d{4,8}$` |

**Supported date formats**:

| Format | Example |
|--------|---------|
| `YYYY-MM-DD` | `2024-01-15` |
| `DD-MM-YYYY` | `15-01-2024` |
| `DD/MM/YYYY` | `15/01/2024` |
| `MM-DD-YYYY` | `01-15-2024` |
| `MM/DD/YYYY` | `01/15/2024` |
| `YYYY/MM/DD` | `2024/01/15` |
| `DD Mon YYYY` | `15 Jan 2024` |
| `Mon DD, YYYY` | `Jan 15, 2024` |
| `Month DD, YYYY` | `January 15, 2024` |
| ISO 8601 | `2024-01-15T10:30:00Z` |

**Note**: Empty fields are skipped (treated as passing) for all format checks. Format validation only applies when a value is present.

**Source**: `internal/validator/invoice/format.go`

---

### Mathematical (12 rules)

Verifies arithmetic relationships between numeric fields. All comparisons use a tolerance of **1.00** (i.e., `abs(actual - expected) <= 1.00`) to account for rounding differences in LLM extraction.

#### Line Item Math (per item)

| Rule Key | Field Path | Severity | Formula |
|----------|-----------|----------|---------|
| `math.line_item.taxable_amount` | `line_items[i].taxable_amount` | Error | `(quantity * unit_price) - discount` |
| `math.line_item.cgst_amount` | `line_items[i].cgst_amount` | Error | `taxable_amount * cgst_rate / 100` |
| `math.line_item.sgst_amount` | `line_items[i].sgst_amount` | Error | `taxable_amount * sgst_rate / 100` |
| `math.line_item.igst_amount` | `line_items[i].igst_amount` | Error | `taxable_amount * igst_rate / 100` |
| `math.line_item.total` | `line_items[i].total` | Error | `taxable_amount + cgst_amount + sgst_amount + igst_amount` |

#### Totals Math

| Rule Key | Field Path | Severity | Formula |
|----------|-----------|----------|---------|
| `math.totals.subtotal` | `totals.subtotal` | Error | `SUM(line_items[*].taxable_amount)` |
| `math.totals.taxable_amount` | `totals.taxable_amount` | Error | `subtotal - total_discount` |
| `math.totals.cgst` | `totals.cgst` | Error | `SUM(line_items[*].cgst_amount)` |
| `math.totals.sgst` | `totals.sgst` | Error | `SUM(line_items[*].sgst_amount)` |
| `math.totals.igst` | `totals.igst` | Error | `SUM(line_items[*].igst_amount)` |
| `math.totals.grand_total` | `totals.total` | Error | `taxable_amount + cgst + sgst + igst + cess + round_off` |
| `math.totals.round_off` | `totals.round_off` | Warning | `abs(round_off) <= 0.50` |

**Source**: `internal/validator/invoice/math.go`

---

### Cross-field (8 rules)

Validates consistency between related fields across different sections of the invoice.

#### GSTIN Structural Checks

| Rule Key | Severity | Description |
|----------|----------|-------------|
| `xf.seller.gstin_state` | Error | First 2 digits of seller GSTIN must match `seller.state_code` |
| `xf.buyer.gstin_state` | Error | First 2 digits of buyer GSTIN must match `buyer.state_code` |
| `xf.seller.gstin_pan` | Error | Characters 3-12 of seller GSTIN must match `seller.pan` |
| `xf.buyer.gstin_pan` | Error | Characters 3-12 of buyer GSTIN must match `buyer.pan` |
| `xf.parties.different_gstin` | Warning | Seller and buyer should have different GSTINs |

#### Tax Type Consistency (per line item)

| Rule Key | Severity | Description |
|----------|----------|-------------|
| `xf.tax_type.intrastate` | Error | When seller and buyer are in the same state: each line item must use CGST+SGST, IGST must be 0 |
| `xf.tax_type.interstate` | Error | When seller and buyer are in different states: each line item must use IGST, CGST+SGST must be 0 |

#### Date Consistency

| Rule Key | Severity | Description |
|----------|----------|-------------|
| `xf.invoice.due_after_date` | Warning | Due date must be on or after the invoice date |

**Note**: Rules are skipped (treated as passing) when required fields for comparison are missing or unparseable.

**Source**: `internal/validator/invoice/crossfield.go`

---

### Logical (7 rules)

Validates business logic constraints and data sanity.

| Rule Key | Field Path | Severity | Description |
|----------|-----------|----------|-------------|
| `logic.line_item.non_negative` | `line_items[i].*` | Error | quantity, unit_price, taxable_amount, cgst_amount, sgst_amount, igst_amount, and total must all be >= 0 |
| `logic.line_item.valid_tax_rate` | `line_items[i].{cgst,sgst,igst}_rate` | Warning | Tax rates must be one of the standard GST rates: {0, 0.25, 3, 5, 12, 18, 28} |
| `logic.line_item.cgst_eq_sgst` | `line_items[i]` | Error | CGST rate must equal SGST rate (required for intrastate GST) |
| `logic.line_item.exclusive_tax` | `line_items[i]` | Error | A line item cannot have both CGST/SGST and IGST applied simultaneously |
| `logic.line_items.at_least_one` | `line_items` | Error | Invoice must contain at least one line item |
| `logic.invoice.date_not_future` | `invoice.invoice_date` | Warning | Invoice date should not be in the future |
| `logic.totals.non_negative` | `totals.*` | Error | subtotal, taxable_amount, cgst, sgst, igst, and total must all be >= 0 |

**Source**: `internal/validator/invoice/logical.go`

---

## GST Invoice Schema

The validation rules operate on a typed `GSTInvoice` struct that mirrors the LLM parser output. All field paths in the rules above correspond to this schema.

```
GSTInvoice
├── Invoice
│   ├── invoice_number        string
│   ├── invoice_date          string
│   ├── due_date              string
│   ├── invoice_type          string
│   ├── currency              string      (ISO 4217)
│   ├── place_of_supply       string
│   └── reverse_charge        bool
├── Seller
│   ├── name                  string
│   ├── address               string
│   ├── gstin                 string      (15-char GSTIN)
│   ├── pan                   string      (10-char PAN)
│   ├── state                 string
│   └── state_code            string      (2-digit, 01-38)
├── Buyer
│   └── (same fields as Seller)
├── LineItems[]
│   ├── description           string
│   ├── hsn_sac_code          string      (4-8 digit HSN/SAC)
│   ├── quantity              float64
│   ├── unit                  string
│   ├── unit_price            float64
│   ├── discount              float64
│   ├── taxable_amount        float64     = (qty * price) - discount
│   ├── cgst_rate             float64     (standard GST rate)
│   ├── cgst_amount           float64     = taxable * rate / 100
│   ├── sgst_rate             float64     (intrastate only)
│   ├── sgst_amount           float64
│   ├── igst_rate             float64     (interstate only)
│   ├── igst_amount           float64
│   └── total                 float64     = taxable + all taxes
├── Totals
│   ├── subtotal              float64     = SUM(line_items.taxable_amount)
│   ├── total_discount        float64
│   ├── taxable_amount        float64     = subtotal - total_discount
│   ├── cgst                  float64     = SUM(line_items.cgst_amount)
│   ├── sgst                  float64     = SUM(line_items.sgst_amount)
│   ├── igst                  float64     = SUM(line_items.igst_amount)
│   ├── cess                  float64
│   ├── round_off             float64     (abs <= 0.50)
│   ├── total                 float64     = taxable + taxes + cess + round_off
│   └── amount_in_words       string
├── Payment
│   ├── bank_name             string
│   ├── account_number        string      (9-18 digits)
│   ├── ifsc_code             string      (IFSC format)
│   └── payment_terms         string
└── Notes                     string
```

**Type definitions**: `internal/validator/invoice/types.go`

---

## API Endpoints

### Trigger Validation

```
POST /api/v1/documents/:id/validate
Authorization: Bearer <access_token>
```

Runs the validation engine on a parsed document. Returns 400 if the document has not been parsed yet. Validation also runs automatically after parsing completes.

### Get Validation Results

```
GET /api/v1/documents/:id/validation
Authorization: Bearer <access_token>
```

Returns:

```json
{
  "success": true,
  "data": {
    "document_id": "uuid",
    "validation_status": "warning",
    "summary": {
      "total": 50,
      "passed": 47,
      "errors": 0,
      "warnings": 3
    },
    "results": [
      {
        "rule_name": "Required: Invoice Number",
        "rule_type": "required_field",
        "severity": "error",
        "passed": true,
        "field_path": "invoice.invoice_number",
        "expected_value": "non-empty value",
        "actual_value": "INV-001",
        "message": "Required: Invoice Number: invoice.invoice_number is present"
      }
    ],
    "field_statuses": {
      "invoice.invoice_number": { "status": "valid", "messages": [] },
      "seller.gstin": { "status": "unsure", "messages": ["..."] }
    }
  }
}
```

---

## Architecture

```
cmd/server/main.go
  │  Registers all validators → validator.Registry
  │  Creates validator.Engine(registry, ruleRepo, docRepo)
  │
  ▼
internal/validator/
  ├── engine.go           Orchestrator: ValidateDocument(), GetValidation(), EnsureBuiltinRules()
  ├── validator.go        Validator interface
  ├── registry.go         Map-based validator lookup by rule key
  ├── field_status.go     Computes per-field status from results + confidence scores
  └── invoice/
      ├── types.go          GSTInvoice struct (mirrors parser output)
      ├── builtin_rules.go  AllBuiltinValidators() → collects all 50 validators
      ├── required.go       12 required field validators
      ├── format.go         13 format validators (regex, dates, enums)
      ├── math.go           12 mathematical validators (arithmetic checks)
      ├── crossfield.go     8 cross-field validators (consistency checks)
      └── logical.go        7 logical validators (business rules)
```

### Builtin Rule Auto-seeding

On first validation for a given tenant + document type:

1. The engine queries existing `builtin_rule_key` values from `document_validation_rules`
2. For each registered validator not yet in the database, it creates a new rule row
3. A unique index on `(tenant_id, builtin_rule_key)` prevents duplicates
4. Rules are tenant-scoped and can be individually toggled via `is_active`

This means rules are lazily seeded — no manual setup is needed.

### Storage

Validation results are stored as a JSONB array on the `documents` table (`validation_results` column), not in a separate table. Each entry contains the rule ID, pass/fail status, field path, expected/actual values, message, and timestamp.

---

## Extending the Validator

### Adding a new rule to an existing category

1. Open the appropriate file in `internal/validator/invoice/` (e.g., `logical.go`)
2. Add a new entry to the `*Validators()` function (e.g., `LogicalValidators()`)
3. The rule will be automatically included via `AllBuiltinValidators()` and auto-seeded on next validation run

### Adding a new rule category

1. Create a new file in `internal/validator/invoice/` (e.g., `compliance.go`)
2. Define the validator struct implementing the `Validator` interface (methods: `Validate`, `RuleKey`, `RuleName`, `RuleType`, `Severity`)
3. Create a constructor function (e.g., `ComplianceValidators()`)
4. Add a loop in `AllBuiltinValidators()` in `builtin_rules.go` to include the new validators

### Adding a new document type

1. Create a new directory under `internal/validator/` (e.g., `internal/validator/receipt/`)
2. Define the typed struct in `types.go`
3. Implement validators for the new type
4. Register them in `cmd/server/main.go` with the validator registry
