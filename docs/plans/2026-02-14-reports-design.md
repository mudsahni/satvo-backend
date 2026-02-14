# Reports Feature Design

## Overview

Add financial reporting endpoints to SATVOS backed by a materialized `document_summaries` table. Reports aggregate parsed invoice data (sellers, buyers, spend, taxes, HSN codes) across documents a user can access. Serves both in-house finance teams and accounting firms managing multiple clients.

## Architecture Decision

**Materialized reporting table.** A `document_summaries` table denormalizes key financial fields from `documents.structured_data` JSONB into properly typed, indexed columns. Upserted on parse/edit, status-updated on validation/review, cascade-deleted with the document. Reports query this table with standard SQL — fast GROUP BY, SUM, proper date indexes.

Alternative considered: querying JSONB directly. Rejected because JSONB extraction can't use standard indexes and degrades at scale. The API contract is the same either way, so this is an implementation detail hidden from the frontend.

## Table Schema

```sql
CREATE TABLE document_summaries (
    document_id       UUID PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    tenant_id         UUID NOT NULL,
    collection_id     UUID NOT NULL,

    -- Invoice identity
    invoice_number    VARCHAR(100),
    invoice_date      DATE,
    due_date          DATE,
    invoice_type      VARCHAR(50),
    currency          VARCHAR(10),
    place_of_supply   VARCHAR(100),
    reverse_charge    BOOLEAN DEFAULT FALSE,
    has_irn           BOOLEAN DEFAULT FALSE,

    -- Parties
    seller_name       VARCHAR(500),
    seller_gstin      VARCHAR(15),
    seller_state      VARCHAR(100),
    seller_state_code VARCHAR(10),
    buyer_name        VARCHAR(500),
    buyer_gstin       VARCHAR(15),
    buyer_state       VARCHAR(100),
    buyer_state_code  VARCHAR(10),

    -- Financials (NUMERIC for exact arithmetic)
    subtotal          NUMERIC(15,2) DEFAULT 0,
    total_discount    NUMERIC(15,2) DEFAULT 0,
    taxable_amount    NUMERIC(15,2) DEFAULT 0,
    cgst              NUMERIC(15,2) DEFAULT 0,
    sgst              NUMERIC(15,2) DEFAULT 0,
    igst              NUMERIC(15,2) DEFAULT 0,
    cess              NUMERIC(15,2) DEFAULT 0,
    total_amount      NUMERIC(15,2) DEFAULT 0,

    -- Line item stats
    line_item_count   INTEGER DEFAULT 0,
    distinct_hsn_codes TEXT[],

    -- Document status snapshot
    parsing_status    VARCHAR(20),
    review_status     VARCHAR(20),
    validation_status VARCHAR(20),
    reconciliation_status VARCHAR(20),

    -- Timestamps
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_doc_summaries_tenant ON document_summaries(tenant_id);
CREATE INDEX idx_doc_summaries_seller ON document_summaries(tenant_id, seller_gstin);
CREATE INDEX idx_doc_summaries_buyer ON document_summaries(tenant_id, buyer_gstin);
CREATE INDEX idx_doc_summaries_date ON document_summaries(tenant_id, invoice_date);
CREATE INDEX idx_doc_summaries_collection ON document_summaries(tenant_id, collection_id);
CREATE INDEX idx_doc_summaries_seller_buyer ON document_summaries(tenant_id, seller_gstin, buyer_gstin);
```

### Key decisions

- `ON DELETE CASCADE` — summary row auto-removed when document deleted, no manual sync.
- `NUMERIC(15,2)` — exact decimal arithmetic, no floating point drift in financial sums.
- `distinct_hsn_codes TEXT[]` — enables `@>` array containment queries.
- Status columns duplicated — avoids JOIN to documents table for most report queries.
- GSTIN-based indexes — GSTIN is canonical identifier; names can vary in spelling.

## Report Endpoints

All endpoints:
- Behind existing auth middleware, tenant-scoped from JWT
- Viewer/free users: scoped to collections they have access to (JOIN `collection_permissions`)
- Admin/manager/member: tenant-wide

### Common Query Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `from` | date (YYYY-MM-DD) | none | Invoice date start (inclusive) |
| `to` | date (YYYY-MM-DD) | none | Invoice date end (inclusive) |
| `collection_id` | UUID | none | Scope to one collection |
| `seller_gstin` | string | none | Filter by seller |
| `buyer_gstin` | string | none | Filter by buyer |
| `granularity` | string | `monthly` | Time grouping: daily, weekly, monthly, quarterly, yearly |
| `offset` | int | 0 | Pagination offset |
| `limit` | int | 20 | Pagination limit |

### 1. Seller Summary — `GET /api/v1/reports/sellers`

Lists all unique sellers with: invoice count, total spend, total tax (CGST+SGST+IGST), average invoice value, date range (first/last invoice).

Filters: `from`, `to`, `collection_id`, `buyer_gstin`, `offset`, `limit`.

Response:
```json
{
  "success": true,
  "data": [
    {
      "seller_gstin": "29ABCDE1234F1Z5",
      "seller_name": "Seller Corp",
      "seller_state": "Karnataka",
      "invoice_count": 42,
      "total_amount": 1250000.00,
      "total_tax": 225000.00,
      "cgst": 67500.00,
      "sgst": 67500.00,
      "igst": 90000.00,
      "average_invoice_value": 29761.90,
      "first_invoice_date": "2025-04-01",
      "last_invoice_date": "2026-01-28"
    }
  ],
  "meta": { "total": 15, "offset": 0, "limit": 20 }
}
```

Use case: "Who are our top 10 vendors by spend this quarter?"

### 2. Buyer Summary — `GET /api/v1/reports/buyers`

Same structure as seller summary, grouped by buyer.

Filters: `from`, `to`, `collection_id`, `seller_gstin`, `offset`, `limit`.

Use case: Accounting firm asking "Which of our client's customers generate the most revenue?"

### 3. Party Ledger — `GET /api/v1/reports/party-ledger?gstin=<GSTIN>`

All invoices for a specific GSTIN (as seller or buyer), chronologically.

Shows: document_id, invoice_number, invoice_date, invoice_type, seller/buyer info, total, tax breakdown, validation/review status.

Filters: `gstin` (required), `from`, `to`, `collection_id`, `offset`, `limit`.

Response:
```json
{
  "success": true,
  "data": [
    {
      "document_id": "uuid",
      "invoice_number": "INV-001",
      "invoice_date": "2025-06-15",
      "invoice_type": "tax",
      "counterparty_name": "Buyer Inc",
      "counterparty_gstin": "27XYZAB5678C1D2",
      "role": "seller",
      "subtotal": 50000.00,
      "taxable_amount": 50000.00,
      "cgst": 4500.00,
      "sgst": 4500.00,
      "igst": 0.00,
      "total_amount": 59000.00,
      "validation_status": "valid",
      "review_status": "approved"
    }
  ],
  "meta": { "total": 42, "offset": 0, "limit": 20 }
}
```

Use case: "Show me every invoice from Vendor X this financial year" — reconciliation against vendor statement.

### 4. Financial Summary — `GET /api/v1/reports/financial-summary`

Aggregated totals grouped by time period.

Filters: `from`, `to`, `granularity`, `collection_id`, `seller_gstin`, `buyer_gstin`.

Response:
```json
{
  "success": true,
  "data": [
    {
      "period": "2025-10",
      "period_start": "2025-10-01",
      "period_end": "2025-10-31",
      "invoice_count": 87,
      "subtotal": 4500000.00,
      "taxable_amount": 4200000.00,
      "cgst": 378000.00,
      "sgst": 378000.00,
      "igst": 126000.00,
      "cess": 0.00,
      "total_amount": 5082000.00
    }
  ]
}
```

Use case: "Monthly tax liability trend for FY 2025-26."

### 5. Tax Summary — `GET /api/v1/reports/tax-summary`

Tax breakdown by type over time periods. Includes interstate vs intrastate split (IGST > 0 = interstate).

Filters: `from`, `to`, `granularity`, `collection_id`, `seller_gstin`, `buyer_gstin`.

Response:
```json
{
  "success": true,
  "data": [
    {
      "period": "2025-Q3",
      "period_start": "2025-10-01",
      "period_end": "2025-12-31",
      "intrastate_count": 65,
      "intrastate_taxable": 3200000.00,
      "cgst": 288000.00,
      "sgst": 288000.00,
      "interstate_count": 22,
      "interstate_taxable": 1000000.00,
      "igst": 180000.00,
      "cess": 0.00,
      "total_tax": 756000.00
    }
  ]
}
```

Use case: GST return filing — "How much IGST did we pay in Q3?"

### 6. HSN Summary — `GET /api/v1/reports/hsn-summary`

Grouped by HSN code: total taxable amount, total tax, invoice count, line item count.

This report queries `documents.structured_data` JSONB with `jsonb_array_elements` rather than the summary table — it's the only report that needs line-item granularity, and a second denormalized table isn't worth the complexity.

Filters: `from`, `to`, `collection_id`, `offset`, `limit`.

Response:
```json
{
  "success": true,
  "data": [
    {
      "hsn_code": "8471",
      "description": "Automatic data processing machines",
      "invoice_count": 15,
      "line_item_count": 23,
      "total_quantity": 460.00,
      "taxable_amount": 2300000.00,
      "cgst": 207000.00,
      "sgst": 207000.00,
      "igst": 0.00,
      "total_tax": 414000.00
    }
  ],
  "meta": { "total": 8, "offset": 0, "limit": 20 }
}
```

Use case: HSN-wise summary required for GSTR-1 filing.

### 7. Collections Overview — `GET /api/v1/reports/collections-overview`

Per-collection: document count, total amount, validation pass rate, review completion rate.

Filters: `from`, `to`.

Response:
```json
{
  "success": true,
  "data": [
    {
      "collection_id": "uuid",
      "collection_name": "Q3 Purchase Invoices",
      "document_count": 87,
      "total_amount": 5082000.00,
      "validation_valid_pct": 78.2,
      "validation_warning_pct": 12.6,
      "validation_invalid_pct": 9.2,
      "review_approved_pct": 65.5,
      "review_pending_pct": 34.5
    }
  ]
}
```

Use case: Accounting firm tracking workload — "Which client batch has the most unreviewed invoices?"

## Data Flow

### Summary upsert triggers

| Event | Action |
|-------|--------|
| Parse completes (CreateAndParse) | Upsert full summary row |
| Edit structured data (EditStructuredData) | Upsert full summary row |
| Retry completes (RetryParse) | Upsert full summary row |
| Validation completes | Update status columns only |
| Review changes | Update status columns only |
| Document deleted | CASCADE deletes summary row |

### Interfaces

```go
// port/document_summary_repository.go
type DocumentSummaryRepository interface {
    Upsert(ctx context.Context, summary *domain.DocumentSummary) error
    UpdateStatuses(ctx context.Context, documentID uuid.UUID, statuses StatusUpdate) error
}

// port/report_repository.go
type ReportRepository interface {
    SellerSummary(ctx context.Context, tenantID uuid.UUID, filters ReportFilters) ([]SellerSummaryRow, int, error)
    BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters ReportFilters) ([]BuyerSummaryRow, int, error)
    PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters ReportFilters) ([]PartyLedgerRow, int, error)
    FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters ReportFilters) ([]FinancialSummaryRow, error)
    TaxSummary(ctx context.Context, tenantID uuid.UUID, filters ReportFilters) ([]TaxSummaryRow, error)
    HSNSummary(ctx context.Context, tenantID uuid.UUID, filters ReportFilters) ([]HSNSummaryRow, int, error)
    CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters ReportFilters) ([]CollectionOverviewRow, error)
}
```

### Service layer

A thin `ReportService` that:
1. Validates filters (date parsing, UUID parsing)
2. Injects role-based scoping into `ReportFilters` (userID + role for viewer/free)
3. Delegates to `ReportRepository`

No business logic beyond filter validation and role scoping.

### Handler & Router

```
GET /api/v1/reports/sellers               → ReportHandler.Sellers
GET /api/v1/reports/buyers                → ReportHandler.Buyers
GET /api/v1/reports/party-ledger          → ReportHandler.PartyLedger
GET /api/v1/reports/financial-summary     → ReportHandler.FinancialSummary
GET /api/v1/reports/tax-summary           → ReportHandler.TaxSummary
GET /api/v1/reports/hsn-summary           → ReportHandler.HSNSummary
GET /api/v1/reports/collections-overview  → ReportHandler.CollectionsOverview
```

All behind existing auth middleware. No special role requirement.

## Error Handling

**Summary upsert failures are non-blocking.** Same pattern as audit trail — errors logged, never propagated. A document with missing/stale summary data won't appear in reports but the parse/edit operation still succeeds.

**Invalid dates from LLM.** The upsert attempts `time.Parse` with common formats (`2006-01-02`, `02/01/2006`, `02-01-2006`). If all fail, `invoice_date` is NULL — excluded from date-filtered reports, still appears in party summaries.

**Only completed documents.** Summary rows only created when parsing succeeds. Failed/pending/queued documents have no summary row.

**Empty results.** All endpoints return empty arrays (not 404). Financial/tax summary return empty period arrays with zero values for the requested date range — consistent x-axis for frontend charts.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Duplicate seller names, different GSTINs | Grouped by GSTIN (canonical), name from most recent invoice |
| Same GSTIN as both seller and buyer | Separate rows in seller vs buyer reports |
| Credit notes (negative totals) | Stored as-is, sums naturally subtract. `invoice_type` filter lets frontend separate |
| Manual edit changes seller/buyer | Upsert overwrites summary row |
| Concurrent parse + report query | PostgreSQL MVCC — report sees committed data only |

## Backfill

Existing documents won't have summary rows. A one-time backfill command:

```bash
make backfill-summaries
```

Implemented as `cmd/backfill/main.go` (same pattern as `cmd/seedhsn/main.go`). Reads all `parsing_status=completed` documents, extracts structured_data, upserts summary rows. Idempotent — safe to run multiple times.

## Testing Strategy

**Repository tests:**
- Upsert from parsed invoice, verify all fields
- Upsert with malformed date → NULL invoice_date, no error
- Second upsert overwrites correctly
- UpdateStatuses changes status columns only

**Report repository tests:**
- Seed 10-20 summaries with known values, verify each report aggregation
- Date range filtering, collection scoping, GSTIN filtering
- Viewer role scoping (only sees own collections)
- Empty result sets

**Service tests:**
- Upsert called after successful parse/edit
- UpdateStatuses called after validation/review
- Summary failure doesn't fail the parse
- Summary not created for failed parses

**Handler tests:**
- Valid filters, pagination, auth context
- Invalid filter values (bad dates, bad UUIDs)
- Missing auth → 401
- Swagger annotations on all 7 endpoints
