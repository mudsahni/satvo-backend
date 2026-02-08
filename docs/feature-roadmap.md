# SATVOS Feature Roadmap

Planned features ranked from simplest to most challenging, with detailed context for implementation.

---

## Feature 1: Duplicate Invoice Detection

**Complexity: Low** | Estimated effort: 1-2 days | No external dependencies

### The Problem

The same invoice gets booked twice — same vendor sends it via email and WhatsApp, the accountant uploads both, or a re-upload happens after a failed parse. Double-booking means overstated expenses and duplicate ITC claims, which triggers GST scrutiny.

### What It Does

When a document finishes parsing, automatically check if another document in the same tenant has the same (seller_gstin + invoice_number) or (seller_gstin + invoice_number + total). Flag potential duplicates as a validation warning.

### Implementation

**Approach: New validation rule + repository query**

No new tables needed. The data already exists in `documents.structured_data` JSONB.

#### 1. Repository Layer

Add to `DocumentRepository`:

```go
// FindPotentialDuplicates returns documents with matching seller GSTIN + invoice number.
// Excludes the document itself. Searches structured_data JSONB.
FindPotentialDuplicates(ctx context.Context, tenantID, excludeDocID uuid.UUID,
    sellerGSTIN, invoiceNumber string) ([]domain.Document, error)
```

SQL uses JSONB operators:

```sql
SELECT * FROM documents
WHERE tenant_id = $1
  AND id != $2
  AND parsing_status = 'completed'
  AND structured_data->>'seller'->>'gstin' = $3
  AND structured_data->>'invoice'->>'invoice_number' = $4
ORDER BY created_at DESC
LIMIT 5
```

Consider adding a GIN index on the JSONB fields if query performance is a concern:

```sql
CREATE INDEX idx_documents_seller_gstin ON documents
  ((structured_data->'seller'->>'gstin')) WHERE parsing_status = 'completed';
CREATE INDEX idx_documents_invoice_number ON documents
  ((structured_data->'invoice'->>'invoice_number')) WHERE parsing_status = 'completed';
```

#### 2. Validator

New file: `internal/validator/invoice/duplicate.go`

Like HSN validators, this needs external data (the repository), so use the closure-capture pattern:

```go
func DuplicateValidator(finder DuplicateFinder) *BuiltinValidator
```

Where `DuplicateFinder` is a small interface (or function type) that wraps the repo query. The validator:
- Extracts seller GSTIN + invoice number from the invoice
- Calls the finder
- If matches found: returns a warning with "Potential duplicate of document <name> (uploaded <date>)"
- If no matches: passes

Severity: **warning** (not error — could be a legitimate correction/revision).
Not reconciliation-critical.

#### 3. Wiring

In `cmd/server/main.go`, after HSN validator registration, create and register the duplicate validator with the document repo injected.

#### 4. Files Changed

| File | Action |
|------|--------|
| `internal/port/document_repository.go` | Add `FindPotentialDuplicates` method |
| `internal/repository/postgres/document_repo.go` | Implement the query |
| `internal/validator/invoice/duplicate.go` | New validator |
| `cmd/server/main.go` | Wire and register |
| `tests/unit/validator/invoice/duplicate_test.go` | Tests |

---

## Feature 2: GSTIN Live Verification

**Complexity: Low-Medium** | Estimated effort: 2-3 days | External dependency: GST public API

### The Problem

An accountant processes an invoice from a vendor whose GSTIN has been cancelled. They claim ITC. Months later, the GST department denies the credit and issues a demand notice with interest and penalty. This is one of the most common and expensive compliance failures.

### What It Does

After parsing, verify seller and buyer GSTINs against the government's public GSTIN search API. Check:
- Is the GSTIN active, cancelled, or suspended?
- Does the legal/trade name match the invoice?
- Is the entity a composition dealer? (ITC not claimable from composition dealers)
- When was the GSTIN registered? (invoices before registration date are invalid)

### Implementation

#### 1. External API

The GST portal provides a public (no auth needed) API for GSTIN search:

```
GET https://services.gst.gov.in/services/api/search/taxpayerDetails?gstin=29ABCDE1234F1Z5
```

Response includes:
- `sts`: Status — "Active", "Cancelled", "Suspended", "Inactive"
- `lgnm`: Legal name
- `tradeNam`: Trade name
- `dty`: Dealer type — "Regular", "Composition", "SEZ", etc.
- `rgdt`: Registration date (dd/mm/yyyy)
- `cxdt`: Cancellation date (if cancelled)
- `ctb`: Constitution of business
- `pradr`: Principal address (with state code)

**Rate limits**: The public API has undocumented rate limits. Cache aggressively — GSTIN status rarely changes.

#### 2. Port Interface

```go
// internal/port/gstin_verifier.go
type GSTINStatus struct {
    GSTIN            string
    Status           string    // "Active", "Cancelled", "Suspended"
    LegalName        string
    TradeName        string
    DealerType       string    // "Regular", "Composition", etc.
    RegisteredDate   time.Time
    CancelledDate    *time.Time
    StateCode        string
}

type GSTINVerifier interface {
    Verify(ctx context.Context, gstin string) (*GSTINStatus, error)
}
```

#### 3. Implementation with Cache

```go
// internal/gstin/verifier.go
type cachedVerifier struct {
    cache map[string]*cachedEntry  // GSTIN → cached result
    mu    sync.RWMutex
    ttl   time.Duration           // e.g., 24 hours
}
```

In-memory cache with TTL. GSTINs don't change status frequently, so 24h cache is reasonable. Could upgrade to Redis later.

For the API call itself, use standard `net/http` with a timeout. Parse the JSON response into `GSTINStatus`.

#### 4. Validators (3 new rules)

| Key | Check | Severity |
|-----|-------|----------|
| `logic.seller.gstin_active` | Seller GSTIN is Active | **error** |
| `logic.buyer.gstin_active` | Buyer GSTIN is Active | warning |
| `logic.seller.gstin_composition` | Seller is not a Composition dealer | warning |

The seller GSTIN being cancelled is an **error** (not warning) because ITC is definitively not claimable. Buyer GSTIN inactive is a warning because it doesn't affect ITC.

Composition dealer check is critical: ITC cannot be claimed on invoices from composition dealers under Section 16(2) of CGST Act.

Could also add name-matching (fuzzy match legal/trade name against invoice seller name) as a warning.

#### 5. Wiring

Closure-capture pattern again. Create the verifier at startup, inject into validators.

#### 6. Files Changed

| File | Action |
|------|--------|
| `internal/port/gstin_verifier.go` | New interface |
| `internal/gstin/verifier.go` | New implementation with cache |
| `internal/validator/invoice/gstin_live.go` | 3 new validators |
| `cmd/server/main.go` | Wire verifier, register validators |
| `tests/unit/validator/invoice/gstin_live_test.go` | Tests (mock the verifier) |

#### 7. Considerations

- **Graceful degradation**: If the GST API is down or rate-limited, skip the check (pass with a "could not verify" message). Never block invoice processing because an external API is unavailable.
- **Privacy**: GSTIN is public information (searchable on GST portal by anyone). No privacy concern.
- **Cost**: Free. The public API has no authentication or billing.
- **Future**: Could add a `POST /admin/gstin/cache/clear` endpoint for manual cache invalidation.

---

## Feature 3: Duplicate Detection Extended — Cross-Period Anomaly Alerts

**Complexity: Medium** | Estimated effort: 3-4 days | No external dependencies

### The Problem

An accountant processing March invoices doesn't notice that total CGST input has doubled compared to February, or that a vendor who normally bills Rs 50K/month suddenly has a Rs 5L invoice. These anomalies often indicate errors (duplicate bookings, wrong amounts) or fraud.

### What It Does

Provides collection-level and tenant-level analytics:
- **Period-over-period comparison**: Total taxable amount, CGST, SGST, IGST this month vs last month
- **Vendor concentration**: Top 10 vendors by invoice value
- **Outlier detection**: Invoices where the total is >2 standard deviations from that vendor's average
- **Summary metrics**: Average invoice value, total ITC claimable, document count by status

### Implementation

#### 1. New Analytics Endpoints

```
GET /api/v1/analytics/summary?period=2025-01&collection_id=...
GET /api/v1/analytics/vendors?period=2025-01&collection_id=...
GET /api/v1/analytics/comparison?period1=2025-01&period2=2024-12&collection_id=...
```

#### 2. Repository Layer

New `AnalyticsRepository` interface with SQL queries that aggregate over `structured_data` JSONB:

```go
type PeriodSummary struct {
    Period          string  // "2025-01"
    DocumentCount   int
    TotalTaxable    float64
    TotalCGST       float64
    TotalSGST       float64
    TotalIGST       float64
    TotalInvoiceVal float64
    AvgInvoiceVal   float64
}

type VendorSummary struct {
    SellerGSTIN   string
    SellerName    string
    InvoiceCount  int
    TotalValue    float64
    AvgValue      float64
}
```

These queries use PostgreSQL JSONB extraction + aggregation:

```sql
SELECT
    COUNT(*) as document_count,
    SUM((structured_data->'totals'->>'taxable_amount')::numeric) as total_taxable,
    SUM((structured_data->'totals'->>'cgst')::numeric) as total_cgst
FROM documents
WHERE tenant_id = $1
  AND parsing_status = 'completed'
  AND (structured_data->'invoice'->>'invoice_date') BETWEEN $2 AND $3
```

#### 3. Service + Handler

Standard service/handler layers. The analytics service is read-only, no complex business logic.

#### 4. Files Changed

| File | Action |
|------|--------|
| `internal/domain/models.go` | Add analytics DTOs |
| `internal/port/analytics_repository.go` | New interface |
| `internal/repository/postgres/analytics_repo.go` | SQL implementations |
| `internal/service/analytics_service.go` | New service |
| `internal/handler/analytics_handler.go` | New handler |
| `internal/router/router.go` | Add routes |
| `cmd/server/main.go` | Wire |

#### 5. Considerations

- JSONB aggregation can be slow on large datasets. Add functional indexes on commonly queried fields if needed.
- Date filtering depends on the parsed `invoice_date` string format being consistent (our parser outputs `DD/MM/YYYY` or `YYYY-MM-DD` — may need normalization).
- Consider materialized views for frequently queried aggregations if performance becomes an issue.

---

## Feature 4: Accounting Software Export (Tally XML)

**Complexity: Medium** | Estimated effort: 4-5 days | No external dependencies

### The Problem

After SATVOS extracts and validates invoice data, the accountant still has to manually type every field into Tally (used by ~80% of Indian accountants), Zoho Books, or another accounting package. This is the most time-consuming remaining step — the extraction saves reading time, but the data still needs to reach the accounting software.

### What It Does

Export validated documents as:
1. **Tally XML** — Tally's native import format (highest priority, most users)
2. **Zoho Books CSV** — Zoho's bulk import format
3. **Generic JSON** — for custom integrations

### Implementation: Tally XML Export

#### 1. Tally XML Format

Tally uses a proprietary XML format for imports. A purchase voucher looks like:

```xml
<ENVELOPE>
 <HEADER>
  <TALLYREQUEST>Import Data</TALLYREQUEST>
 </HEADER>
 <BODY>
  <IMPORTDATA>
   <REQUESTDESC><REPORTNAME>Vouchers</REPORTNAME></REQUESTDESC>
   <REQUESTDATA>
    <TALLYMESSAGE>
     <VOUCHER VCHTYPE="Purchase" ACTION="Create">
      <DATE>20250115</DATE>
      <NARRATION>Invoice #INV-001 from Vendor Name</NARRATION>
      <VOUCHERNUMBER>INV-001</VOUCHERNUMBER>
      <PARTYLEDGERNAME>Vendor Name</PARTYLEDGERNAME>
      <ALLLEDGERENTRIES.LIST>
       <LEDGERNAME>Purchase Account</LEDGERNAME>
       <AMOUNT>-10000.00</AMOUNT>
      </ALLLEDGERENTRIES.LIST>
      <ALLLEDGERENTRIES.LIST>
       <LEDGERNAME>CGST Input</LEDGERNAME>
       <AMOUNT>-900.00</AMOUNT>
      </ALLLEDGERENTRIES.LIST>
      <ALLLEDGERENTRIES.LIST>
       <LEDGERNAME>SGST Input</LEDGERNAME>
       <AMOUNT>-900.00</AMOUNT>
      </ALLLEDGERENTRIES.LIST>
      <ALLLEDGERENTRIES.LIST>
       <LEDGERNAME>Vendor Name</LEDGERNAME>
       <AMOUNT>11800.00</AMOUNT>
      </ALLLEDGERENTRIES.LIST>
     </VOUCHER>
    </TALLYMESSAGE>
   </REQUESTDATA>
  </IMPORTDATA>
 </BODY>
</ENVELOPE>
```

Key mapping decisions:
- **Ledger names** must match existing ledgers in the Tally company file. We'll need configurable ledger name mappings (e.g., "CGST Input 9%" → user's actual ledger name).
- **Date format**: Tally uses `YYYYMMDD`.
- **Amounts**: Debit is negative, credit is positive in Tally XML.

#### 2. Configuration: Ledger Mapping

New table (tenant-scoped):

```sql
CREATE TABLE export_ledger_mappings (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    export_type VARCHAR(20) NOT NULL DEFAULT 'tally',  -- 'tally', 'zoho', etc.
    field_key   VARCHAR(50) NOT NULL,  -- 'purchase', 'cgst_input', 'sgst_input', 'igst_input', etc.
    ledger_name VARCHAR(255) NOT NULL, -- User's actual ledger name in Tally
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, export_type, field_key)
);
```

Default mappings seeded on first export attempt. Admin can customize via settings endpoint.

#### 3. Endpoints

```
GET  /api/v1/collections/:id/export/tally     — Export collection as Tally XML
GET  /api/v1/documents/:id/export/tally        — Export single document as Tally XML
GET  /api/v1/settings/export-mappings          — List ledger mappings
PUT  /api/v1/settings/export-mappings          — Update ledger mappings
```

#### 4. Export Logic

New package `internal/tallyexport/`:

```go
type TallyExporter struct {
    mappings map[string]string  // field_key → ledger_name
}

func (e *TallyExporter) ExportVoucher(doc *domain.Document) ([]byte, error)
func (e *TallyExporter) ExportCollection(docs []domain.Document) ([]byte, error)
```

Logic per document:
1. Parse `structured_data` into `GSTInvoice`
2. Create voucher XML with date, narration, invoice number
3. Debit entries: purchase amount (taxable), CGST input, SGST input, IGST input
4. Credit entry: vendor party ledger (total amount)
5. Only export documents with `parsing_status = completed`

#### 5. Files Changed

| File | Action |
|------|--------|
| `db/migrations/000014_export_ledger_mappings.up.sql` | New table |
| `internal/domain/models.go` | ExportLedgerMapping struct |
| `internal/port/export_mapping_repository.go` | New interface |
| `internal/repository/postgres/export_mapping_repo.go` | Implementation |
| `internal/tallyexport/writer.go` | Tally XML generation |
| `internal/service/export_service.go` | Orchestration |
| `internal/handler/export_handler.go` | Endpoints |
| `internal/router/router.go` | Routes |
| `cmd/server/main.go` | Wire |

#### 6. Considerations

- Tally XML is poorly documented. Validate against Tally Prime's actual import behavior.
- Ledger names are case-sensitive in Tally. Warn users about exact matching.
- Consider a "dry run" mode that shows the XML preview without downloading.
- Zoho Books CSV can be a second export format using the same mapping table with `export_type = 'zoho'`.

---

## Feature 5: GSTR-2A/2B Auto-Reconciliation

**Complexity: High** | Estimated effort: 8-12 days | External dependency: GSTR-2A/2B data files

### The Problem

This is the accountant's #1 monthly pain point. Every month they must:
1. Download GSTR-2A (auto-populated from supplier filings) from the GST portal
2. Match every purchase invoice against the 2A data
3. Find discrepancies: invoices in books but not in 2A, invoices in 2A but not in books, amount mismatches
4. Decide which ITC to claim and which to defer
5. File GSTR-3B with the correct ITC figures

For a business with 500 purchase invoices/month, this takes 2-3 days of manual work. Errors lead to ITC reversals, interest, and penalties.

### What It Does

1. **Import GSTR-2A/2B data** (JSON or Excel from GST portal)
2. **Auto-match** each 2A/2B invoice against uploaded documents using the 22 reconciliation-critical fields
3. **Categorize** every invoice into: Matched, Mismatched (with specific field differences), Missing from 2A/2B, Missing from Books
4. **Generate reconciliation report** with actionable summaries
5. **Track ITC eligibility** based on match status

### Implementation

#### 1. Data Model

New table for imported government data:

```sql
CREATE TABLE gstr_returns (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    return_type     VARCHAR(10) NOT NULL,   -- '2A', '2B'
    return_period   VARCHAR(7) NOT NULL,     -- '2025-01' (YYYY-MM)
    uploaded_by     UUID NOT NULL REFERENCES users(id),
    raw_data        JSONB,                   -- Full JSON as uploaded
    processed       BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, return_type, return_period)
);

CREATE TABLE gstr_invoices (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    return_id       UUID NOT NULL REFERENCES gstr_returns(id) ON DELETE CASCADE,
    return_type     VARCHAR(10) NOT NULL,
    return_period   VARCHAR(7) NOT NULL,
    supplier_gstin  VARCHAR(15) NOT NULL,
    supplier_name   VARCHAR(255),
    invoice_number  VARCHAR(50) NOT NULL,
    invoice_date    DATE,
    invoice_value   NUMERIC(15,2),
    taxable_value   NUMERIC(15,2),
    cgst            NUMERIC(15,2) DEFAULT 0,
    sgst            NUMERIC(15,2) DEFAULT 0,
    igst            NUMERIC(15,2) DEFAULT 0,
    cess            NUMERIC(15,2) DEFAULT 0,
    place_of_supply VARCHAR(2),
    reverse_charge  BOOLEAN DEFAULT false,
    invoice_type    VARCHAR(20),            -- 'B2B', 'CDNR', etc.
    irn             VARCHAR(64),
    raw_entry       JSONB,                  -- Original JSON entry
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gstr_inv_match ON gstr_invoices (tenant_id, supplier_gstin, invoice_number);

CREATE TABLE reconciliation_results (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    return_id       UUID NOT NULL REFERENCES gstr_returns(id) ON DELETE CASCADE,
    document_id     UUID REFERENCES documents(id) ON DELETE SET NULL,
    gstr_invoice_id UUID REFERENCES gstr_invoices(id) ON DELETE SET NULL,
    match_status    VARCHAR(20) NOT NULL,   -- 'matched', 'mismatched', 'missing_from_books', 'missing_from_2a'
    mismatches      JSONB,                  -- Array of {field, book_value, gstr_value, tolerance}
    itc_eligible    BOOLEAN,
    notes           TEXT DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### 2. GSTR-2A/2B JSON Format

The GST portal exports GSTR-2A as JSON with this structure (simplified):

```json
{
  "gstin": "29BUYER0000B1Z5",
  "fp": "012025",
  "b2b": [
    {
      "ctin": "29SELLER000S1Z5",
      "inv": [
        {
          "inum": "INV-001",
          "idt": "15-01-2025",
          "val": 11800,
          "pos": "29",
          "rchrg": "N",
          "itms": [
            {
              "num": 1,
              "itm_det": {
                "txval": 10000,
                "camt": 900,
                "samt": 900,
                "rt": 18
              }
            }
          ]
        }
      ]
    }
  ]
}
```

GSTR-2B has a similar but slightly different structure. Both need parsers.

#### 3. Matching Algorithm

For each GSTR invoice, find the matching document:

```
Step 1: Exact match on (supplier_gstin, invoice_number)
Step 2: If multiple matches, prefer matching invoice_date
Step 3: Compare reconciliation-critical fields with tolerances:
        - Amounts: ±1 rupee (rounding tolerance)
        - Dates: exact match required
        - GSTIN: exact match required
        - Place of supply: exact match required
Step 4: Classify result
```

Match statuses:
- **Matched**: All critical fields match within tolerance
- **Mismatched**: Invoice found but amounts or other fields differ (with specific differences listed)
- **Missing from 2A/2B**: In our books but supplier hasn't filed (ITC at risk)
- **Missing from Books**: In 2A/2B but we haven't uploaded (potential missed ITC claim)

#### 4. Endpoints

```
POST /api/v1/reconciliation/upload          — Upload GSTR-2A/2B JSON/Excel
GET  /api/v1/reconciliation/periods         — List uploaded return periods
GET  /api/v1/reconciliation/:period/summary — Reconciliation summary
GET  /api/v1/reconciliation/:period/results — Detailed results (paginated, filterable by status)
GET  /api/v1/reconciliation/:period/export  — Export reconciliation report as CSV/Excel
```

#### 5. Reconciliation Report Output

```json
{
  "period": "2025-01",
  "return_type": "2B",
  "summary": {
    "total_in_books": 487,
    "total_in_2b": 501,
    "matched": 412,
    "mismatched": 45,
    "missing_from_2b": 30,
    "missing_from_books": 14,
    "total_itc_claimable": 1850000.00,
    "total_itc_at_risk": 245000.00,
    "total_itc_missed": 67000.00
  },
  "mismatches_by_field": {
    "invoice_value": 18,
    "taxable_amount": 12,
    "cgst": 8,
    "invoice_date": 7
  }
}
```

#### 6. Files Changed

| File | Action |
|------|--------|
| `db/migrations/000014_reconciliation.up.sql` | 3 new tables |
| `internal/domain/models.go` | GSTRReturn, GSTRInvoice, ReconciliationResult structs |
| `internal/domain/enums.go` | MatchStatus enum |
| `internal/port/reconciliation_repository.go` | New interfaces |
| `internal/repository/postgres/reconciliation_repo.go` | SQL implementations |
| `internal/reconciliation/parser_2a.go` | GSTR-2A JSON parser |
| `internal/reconciliation/parser_2b.go` | GSTR-2B JSON parser |
| `internal/reconciliation/matcher.go` | Matching algorithm |
| `internal/service/reconciliation_service.go` | Orchestration |
| `internal/handler/reconciliation_handler.go` | Endpoints |
| `internal/router/router.go` | Routes |
| `cmd/server/main.go` | Wire |

#### 7. Considerations

- GSTR-2A is real-time (updates as suppliers file). GSTR-2B is static (generated on 14th of the following month). Most accountants reconcile against 2B.
- Invoice number normalization is critical: suppliers may file "INV-001" while the invoice says "INV/001" or "INV 001". Need fuzzy matching on invoice numbers (strip special chars, compare alphanumeric only).
- Date format differences: GST portal uses DD-MM-YYYY, our parser may output DD/MM/YYYY or YYYY-MM-DD. Normalize before matching.
- This feature alone would justify the tool for most accounting firms. It's the highest-value feature by far but also the most complex.

---

## Feature 6: ITC Eligibility Engine

**Complexity: High** | Estimated effort: 5-7 days | No external dependencies (pairs well with Feature 5)

### The Problem

Not all GST paid is claimable as Input Tax Credit. The rules are complex, change frequently, and most accountants rely on memory or manual checks. Common mistakes:
- Claiming ITC on blocked items (motor vehicles, food, personal use)
- Claiming ITC from composition dealers
- Missing the time limit for ITC claims
- Not reversing ITC on invoices missing from GSTR-2B

### What It Does

For each invoice, assess ITC eligibility based on:
1. **Section 17(5) blocked credits**: Certain HSN codes (motor vehicles, food/beverages, health/fitness, travel, memberships) have blocked ITC regardless of business purpose
2. **Composition dealer check**: ITC not allowed if seller is a composition dealer (ties into Feature 2)
3. **Time limit check**: ITC must be claimed by the earlier of (a) 30th Nov of the following year or (b) the date of filing annual return. Flag invoices approaching the deadline.
4. **Reverse charge applicability**: If reverse charge applies, the buyer must self-assess and pay GST. Flag invoices where reverse charge is indicated.

### Implementation

#### 1. ITC Rules Data

New table for ITC rules (government-defined, not tenant-scoped):

```sql
CREATE TABLE itc_rules (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    hsn_code        VARCHAR(8),          -- NULL = applies to all
    rule_type       VARCHAR(30) NOT NULL, -- 'blocked_17_5', 'time_limit', 'reverse_charge'
    description     TEXT NOT NULL,
    section_ref     VARCHAR(50),          -- e.g., '17(5)(a)', '16(4)'
    effective_from  DATE NOT NULL DEFAULT '2017-07-01',
    effective_to    DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Seed with known blocked categories:
- HSN 8703 (motor vehicles, except for certain business uses)
- HSN 9963 (food & beverages)
- HSN 9972 (health & fitness)
- HSN 9964 (travel, except for business)
- etc.

#### 2. Validators (4 new rules)

| Key | Check | Severity |
|-----|-------|----------|
| `logic.invoice.itc_blocked_17_5` | Line items match blocked HSN codes | warning |
| `logic.invoice.itc_time_limit` | Invoice date within ITC claim window | warning |
| `logic.invoice.reverse_charge_flag` | Reverse charge indicated — buyer must self-assess | warning |
| `logic.seller.composition_dealer` | Seller is composition dealer — no ITC (needs Feature 2) | error |

#### 3. Time Limit Logic

```go
func isITCWithinTimeLimit(invoiceDate time.Time) bool {
    // ITC must be claimed by 30th November of the year following the financial year
    // Financial year: April to March
    fy := invoiceDate.Year()
    if invoiceDate.Month() < time.April {
        fy-- // Jan-Mar belongs to previous FY
    }
    deadline := time.Date(fy+2, time.November, 30, 0, 0, 0, 0, time.UTC)
    return time.Now().Before(deadline)
}
```

#### 4. Files Changed

| File | Action |
|------|--------|
| `db/migrations/000015_itc_rules.up.sql` | New table + seed data |
| `internal/port/itc_repository.go` | New interface |
| `internal/repository/postgres/itc_repo.go` | Implementation |
| `internal/validator/invoice/itc.go` | 4 new validators |
| `cmd/server/main.go` | Wire |
| `tests/unit/validator/invoice/itc_test.go` | Tests |

---

## Implementation Priority — Recommended Order

```
                     Value to Accountant
                     ▲
                     │
    Feature 5        │  ★★★★★  GSTR-2A/2B Reconciliation
    (8-12 days)      │
                     │
    Feature 4        │  ★★★★   Tally XML Export
    (4-5 days)       │
                     │
    Feature 2        │  ★★★★   GSTIN Live Verification
    (2-3 days)       │
                     │
    Feature 6        │  ★★★    ITC Eligibility Engine
    (5-7 days)       │
                     │
    Feature 3        │  ★★★    Analytics / Anomaly Detection
    (3-4 days)       │
                     │
    Feature 1        │  ★★     Duplicate Invoice Detection
    (1-2 days)       │
                     └────────────────────────────────────► Implementation Effort
                       1-2d    2-3d    3-4d    5-7d   8-12d
```

### Recommended sequence:

| Order | Feature | Days | Why This Order |
|-------|---------|------|----------------|
| 1 | Duplicate Detection | 1-2 | Quick win. Builds on existing validator pattern. Immediately useful. |
| 2 | GSTIN Live Verification | 2-3 | High value, moderate effort. Catches a problem that costs real money (denied ITC). Establishes the external-API-with-cache pattern. |
| 3 | Analytics / Anomaly Detection | 3-4 | Read-only feature, lower risk. Gives accountants visibility they've never had. Foundation for the reconciliation UI. |
| 4 | Tally XML Export | 4-5 | Closes the loop from "upload PDF" to "import into accounting software". Biggest time-saver after parsing. |
| 5 | GSTR-2A/2B Reconciliation | 8-12 | The crown jewel. Most complex but highest value. By this point, we have a solid data foundation. |
| 6 | ITC Eligibility Engine | 5-7 | Pairs naturally with reconciliation (both inform ITC decisions). Build after reconciliation is stable. |

### The First Three (Features 1-3) can be done in ~1 week and give you a substantially more capable product to show to accountants.

### Feature 5 (Reconciliation) is the one that makes accountants *need* the product rather than just *like* it. But it's also the one where getting it wrong (incorrect matching, missed invoices) would damage trust. Building features 1-3 first establishes patterns and hardens the codebase before tackling reconciliation.
