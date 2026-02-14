# Reports Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 7 financial reporting API endpoints backed by a materialized `document_summaries` table that denormalizes parsed invoice data for fast aggregation queries.

**Architecture:** A new `document_summaries` table stores typed, indexed columns extracted from `documents.structured_data` JSONB. Upserted non-blockingly after parse/edit, status-updated after validation/review, cascade-deleted with the document. A `ReportRepository` provides 7 aggregation queries. A thin `ReportService` adds role-based scoping. A `ReportHandler` exposes 7 GET endpoints under `/api/v1/reports/`. A one-time backfill CLI populates summaries for existing documents.

**Tech Stack:** Go 1.24, PostgreSQL 16 (sqlx), Gin, testify/mock

---

### Task 1: Database Migration

**Files:**
- Create: `db/migrations/000020_add_document_summaries.up.sql`
- Create: `db/migrations/000020_add_document_summaries.down.sql`

**Step 1: Write the up migration**

```sql
-- db/migrations/000020_add_document_summaries.up.sql
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
```

**Step 2: Write the down migration**

```sql
-- db/migrations/000020_add_document_summaries.down.sql
DROP TABLE IF EXISTS document_summaries;
```

**Step 3: Verify migration files exist**

Run: `ls db/migrations/000020*`
Expected: both files listed

**Step 4: Commit**

```bash
git add db/migrations/000020_add_document_summaries.up.sql db/migrations/000020_add_document_summaries.down.sql
git commit -m "feat(reports): add document_summaries migration"
```

---

### Task 2: Domain Models

**Files:**
- Modify: `internal/domain/models.go` — add `DocumentSummary` struct and all report row types

**Step 1: Add domain types**

Add the following after the existing `Stats` struct in `internal/domain/models.go`:

```go
// DocumentSummary is a denormalized view of a parsed document for reporting.
type DocumentSummary struct {
	DocumentID         uuid.UUID            `db:"document_id" json:"document_id"`
	TenantID           uuid.UUID            `db:"tenant_id" json:"tenant_id"`
	CollectionID       uuid.UUID            `db:"collection_id" json:"collection_id"`
	InvoiceNumber      string               `db:"invoice_number" json:"invoice_number"`
	InvoiceDate        *time.Time           `db:"invoice_date" json:"invoice_date"`
	DueDate            *time.Time           `db:"due_date" json:"due_date"`
	InvoiceType        string               `db:"invoice_type" json:"invoice_type"`
	Currency           string               `db:"currency" json:"currency"`
	PlaceOfSupply      string               `db:"place_of_supply" json:"place_of_supply"`
	ReverseCharge      bool                 `db:"reverse_charge" json:"reverse_charge"`
	HasIRN             bool                 `db:"has_irn" json:"has_irn"`
	SellerName         string               `db:"seller_name" json:"seller_name"`
	SellerGSTIN        string               `db:"seller_gstin" json:"seller_gstin"`
	SellerState        string               `db:"seller_state" json:"seller_state"`
	SellerStateCode    string               `db:"seller_state_code" json:"seller_state_code"`
	BuyerName          string               `db:"buyer_name" json:"buyer_name"`
	BuyerGSTIN         string               `db:"buyer_gstin" json:"buyer_gstin"`
	BuyerState         string               `db:"buyer_state" json:"buyer_state"`
	BuyerStateCode     string               `db:"buyer_state_code" json:"buyer_state_code"`
	Subtotal           float64              `db:"subtotal" json:"subtotal"`
	TotalDiscount      float64              `db:"total_discount" json:"total_discount"`
	TaxableAmount      float64              `db:"taxable_amount" json:"taxable_amount"`
	CGST               float64              `db:"cgst" json:"cgst"`
	SGST               float64              `db:"sgst" json:"sgst"`
	IGST               float64              `db:"igst" json:"igst"`
	Cess               float64              `db:"cess" json:"cess"`
	TotalAmount        float64              `db:"total_amount" json:"total_amount"`
	LineItemCount      int                  `db:"line_item_count" json:"line_item_count"`
	DistinctHSNCodes   pq.StringArray       `db:"distinct_hsn_codes" json:"distinct_hsn_codes"`
	ParsingStatus      ParsingStatus        `db:"parsing_status" json:"parsing_status"`
	ReviewStatus       ReviewStatus         `db:"review_status" json:"review_status"`
	ValidationStatus   ValidationStatus     `db:"validation_status" json:"validation_status"`
	ReconciliationStatus ReconciliationStatus `db:"reconciliation_status" json:"reconciliation_status"`
	CreatedAt          time.Time            `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time            `db:"updated_at" json:"updated_at"`
}

// ReportFilters holds common filter parameters for report queries.
type ReportFilters struct {
	From         *time.Time
	To           *time.Time
	CollectionID *uuid.UUID
	SellerGSTIN  string
	BuyerGSTIN   string
	Granularity  string // daily, weekly, monthly, quarterly, yearly
	UserID       uuid.UUID
	UserRole     UserRole
	Offset       int
	Limit        int
}

// SellerSummaryRow is one row in the seller summary report.
type SellerSummaryRow struct {
	SellerGSTIN         string  `db:"seller_gstin" json:"seller_gstin"`
	SellerName          string  `db:"seller_name" json:"seller_name"`
	SellerState         string  `db:"seller_state" json:"seller_state"`
	InvoiceCount        int     `db:"invoice_count" json:"invoice_count"`
	TotalAmount         float64 `db:"total_amount" json:"total_amount"`
	TotalTax            float64 `db:"total_tax" json:"total_tax"`
	CGST                float64 `db:"cgst" json:"cgst"`
	SGST                float64 `db:"sgst" json:"sgst"`
	IGST                float64 `db:"igst" json:"igst"`
	AverageInvoiceValue float64 `db:"average_invoice_value" json:"average_invoice_value"`
	FirstInvoiceDate    *time.Time `db:"first_invoice_date" json:"first_invoice_date"`
	LastInvoiceDate     *time.Time `db:"last_invoice_date" json:"last_invoice_date"`
}

// BuyerSummaryRow is one row in the buyer summary report.
type BuyerSummaryRow struct {
	BuyerGSTIN          string  `db:"buyer_gstin" json:"buyer_gstin"`
	BuyerName           string  `db:"buyer_name" json:"buyer_name"`
	BuyerState          string  `db:"buyer_state" json:"buyer_state"`
	InvoiceCount        int     `db:"invoice_count" json:"invoice_count"`
	TotalAmount         float64 `db:"total_amount" json:"total_amount"`
	TotalTax            float64 `db:"total_tax" json:"total_tax"`
	CGST                float64 `db:"cgst" json:"cgst"`
	SGST                float64 `db:"sgst" json:"sgst"`
	IGST                float64 `db:"igst" json:"igst"`
	AverageInvoiceValue float64 `db:"average_invoice_value" json:"average_invoice_value"`
	FirstInvoiceDate    *time.Time `db:"first_invoice_date" json:"first_invoice_date"`
	LastInvoiceDate     *time.Time `db:"last_invoice_date" json:"last_invoice_date"`
}

// PartyLedgerRow is one row in the party ledger report.
type PartyLedgerRow struct {
	DocumentID          uuid.UUID            `db:"document_id" json:"document_id"`
	InvoiceNumber       string               `db:"invoice_number" json:"invoice_number"`
	InvoiceDate         *time.Time           `db:"invoice_date" json:"invoice_date"`
	InvoiceType         string               `db:"invoice_type" json:"invoice_type"`
	CounterpartyName    string               `db:"counterparty_name" json:"counterparty_name"`
	CounterpartyGSTIN   string               `db:"counterparty_gstin" json:"counterparty_gstin"`
	Role                string               `db:"role" json:"role"` // "seller" or "buyer"
	Subtotal            float64              `db:"subtotal" json:"subtotal"`
	TaxableAmount       float64              `db:"taxable_amount" json:"taxable_amount"`
	CGST                float64              `db:"cgst" json:"cgst"`
	SGST                float64              `db:"sgst" json:"sgst"`
	IGST                float64              `db:"igst" json:"igst"`
	TotalAmount         float64              `db:"total_amount" json:"total_amount"`
	ValidationStatus    ValidationStatus     `db:"validation_status" json:"validation_status"`
	ReviewStatus        ReviewStatus         `db:"review_status" json:"review_status"`
}

// FinancialSummaryRow is one row in the financial summary report (one per time period).
type FinancialSummaryRow struct {
	Period       string     `json:"period"`
	PeriodStart  time.Time  `json:"period_start"`
	PeriodEnd    time.Time  `json:"period_end"`
	InvoiceCount int        `db:"invoice_count" json:"invoice_count"`
	Subtotal     float64    `db:"subtotal" json:"subtotal"`
	TaxableAmount float64   `db:"taxable_amount" json:"taxable_amount"`
	CGST         float64    `db:"cgst" json:"cgst"`
	SGST         float64    `db:"sgst" json:"sgst"`
	IGST         float64    `db:"igst" json:"igst"`
	Cess         float64    `db:"cess" json:"cess"`
	TotalAmount  float64    `db:"total_amount" json:"total_amount"`
}

// TaxSummaryRow is one row in the tax summary report (one per time period).
type TaxSummaryRow struct {
	Period              string  `json:"period"`
	PeriodStart         time.Time `json:"period_start"`
	PeriodEnd           time.Time `json:"period_end"`
	IntrastateCount     int     `db:"intrastate_count" json:"intrastate_count"`
	IntrastateTaxable   float64 `db:"intrastate_taxable" json:"intrastate_taxable"`
	CGST                float64 `db:"cgst" json:"cgst"`
	SGST                float64 `db:"sgst" json:"sgst"`
	InterstateCount     int     `db:"interstate_count" json:"interstate_count"`
	InterstateTaxable   float64 `db:"interstate_taxable" json:"interstate_taxable"`
	IGST                float64 `db:"igst" json:"igst"`
	Cess                float64 `db:"cess" json:"cess"`
	TotalTax            float64 `db:"total_tax" json:"total_tax"`
}

// HSNSummaryRow is one row in the HSN summary report.
type HSNSummaryRow struct {
	HSNCode       string  `json:"hsn_code"`
	Description   string  `json:"description"`
	InvoiceCount  int     `json:"invoice_count"`
	LineItemCount int     `json:"line_item_count"`
	TotalQuantity float64 `json:"total_quantity"`
	TaxableAmount float64 `json:"taxable_amount"`
	CGST          float64 `json:"cgst"`
	SGST          float64 `json:"sgst"`
	IGST          float64 `json:"igst"`
	TotalTax      float64 `json:"total_tax"`
}

// CollectionOverviewRow is one row in the collections overview report.
type CollectionOverviewRow struct {
	CollectionID         uuid.UUID `db:"collection_id" json:"collection_id"`
	CollectionName       string    `db:"collection_name" json:"collection_name"`
	DocumentCount        int       `db:"document_count" json:"document_count"`
	TotalAmount          float64   `db:"total_amount" json:"total_amount"`
	ValidationValidPct   float64   `db:"validation_valid_pct" json:"validation_valid_pct"`
	ValidationWarningPct float64   `db:"validation_warning_pct" json:"validation_warning_pct"`
	ValidationInvalidPct float64   `db:"validation_invalid_pct" json:"validation_invalid_pct"`
	ReviewApprovedPct    float64   `db:"review_approved_pct" json:"review_approved_pct"`
	ReviewPendingPct     float64   `db:"review_pending_pct" json:"review_pending_pct"`
}

// SummaryStatusUpdate holds status fields to update on document_summaries.
type SummaryStatusUpdate struct {
	ParsingStatus        ParsingStatus
	ReviewStatus         ReviewStatus
	ValidationStatus     ValidationStatus
	ReconciliationStatus ReconciliationStatus
}
```

Add `"github.com/lib/pq"` to the imports of `models.go` for `pq.StringArray`.

**Step 2: Verify it compiles**

Run: `go build ./internal/domain/...`
Expected: success

**Step 3: Commit**

```bash
git add internal/domain/models.go
git commit -m "feat(reports): add domain models for document summaries and report rows"
```

---

### Task 3: Port Interfaces

**Files:**
- Create: `internal/port/document_summary_repository.go`
- Create: `internal/port/report_repository.go`

**Step 1: Create DocumentSummaryRepository interface**

```go
// internal/port/document_summary_repository.go
package port

import (
	"context"

	"github.com/google/uuid"

	"satvos/internal/domain"
)

// DocumentSummaryRepository manages the materialized document_summaries table.
type DocumentSummaryRepository interface {
	Upsert(ctx context.Context, summary *domain.DocumentSummary) error
	UpdateStatuses(ctx context.Context, documentID uuid.UUID, statuses domain.SummaryStatusUpdate) error
}
```

**Step 2: Create ReportRepository interface**

```go
// internal/port/report_repository.go
package port

import (
	"context"

	"github.com/google/uuid"

	"satvos/internal/domain"
)

// ReportRepository provides aggregation queries for reports.
type ReportRepository interface {
	SellerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.SellerSummaryRow, int, error)
	BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.BuyerSummaryRow, int, error)
	PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters domain.ReportFilters) ([]domain.PartyLedgerRow, int, error)
	FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.FinancialSummaryRow, error)
	TaxSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.TaxSummaryRow, error)
	HSNSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.HSNSummaryRow, int, error)
	CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.CollectionOverviewRow, error)
}
```

**Step 3: Verify it compiles**

Run: `go build ./internal/port/...`
Expected: success

**Step 4: Commit**

```bash
git add internal/port/document_summary_repository.go internal/port/report_repository.go
git commit -m "feat(reports): add port interfaces for summary and report repositories"
```

---

### Task 4: Mocks

**Files:**
- Create: `mocks/mock_document_summary_repo.go`
- Create: `mocks/mock_report_repo.go`
- Create: `mocks/mock_report_service.go`

**Step 1: Create mock for DocumentSummaryRepository**

```go
// mocks/mock_document_summary_repo.go
package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

type MockDocumentSummaryRepo struct {
	mock.Mock
}

func (m *MockDocumentSummaryRepo) Upsert(ctx context.Context, summary *domain.DocumentSummary) error {
	args := m.Called(ctx, summary)
	return args.Error(0)
}

func (m *MockDocumentSummaryRepo) UpdateStatuses(ctx context.Context, documentID uuid.UUID, statuses domain.SummaryStatusUpdate) error {
	args := m.Called(ctx, documentID, statuses)
	return args.Error(0)
}
```

**Step 2: Create mock for ReportRepository**

```go
// mocks/mock_report_repo.go
package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

type MockReportRepo struct {
	mock.Mock
}

func (m *MockReportRepo) SellerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.SellerSummaryRow, int, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.SellerSummaryRow), args.Int(1), args.Error(2)
}

func (m *MockReportRepo) BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.BuyerSummaryRow, int, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.BuyerSummaryRow), args.Int(1), args.Error(2)
}

func (m *MockReportRepo) PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters domain.ReportFilters) ([]domain.PartyLedgerRow, int, error) {
	args := m.Called(ctx, tenantID, gstin, filters)
	return args.Get(0).([]domain.PartyLedgerRow), args.Int(1), args.Error(2)
}

func (m *MockReportRepo) FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.FinancialSummaryRow, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.FinancialSummaryRow), args.Error(1)
}

func (m *MockReportRepo) TaxSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.TaxSummaryRow, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.TaxSummaryRow), args.Error(1)
}

func (m *MockReportRepo) HSNSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.HSNSummaryRow, int, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.HSNSummaryRow), args.Int(1), args.Error(2)
}

func (m *MockReportRepo) CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.CollectionOverviewRow, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.CollectionOverviewRow), args.Error(1)
}
```

**Step 3: Create mock for ReportService**

```go
// mocks/mock_report_service.go
package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

type MockReportService struct {
	mock.Mock
}

func (m *MockReportService) SellerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.SellerSummaryRow, int, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.SellerSummaryRow), args.Int(1), args.Error(2)
}

func (m *MockReportService) BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.BuyerSummaryRow, int, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.BuyerSummaryRow), args.Int(1), args.Error(2)
}

func (m *MockReportService) PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters domain.ReportFilters) ([]domain.PartyLedgerRow, int, error) {
	args := m.Called(ctx, tenantID, gstin, filters)
	return args.Get(0).([]domain.PartyLedgerRow), args.Int(1), args.Error(2)
}

func (m *MockReportService) FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.FinancialSummaryRow, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.FinancialSummaryRow), args.Error(1)
}

func (m *MockReportService) TaxSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.TaxSummaryRow, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.TaxSummaryRow), args.Error(1)
}

func (m *MockReportService) HSNSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.HSNSummaryRow, int, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.HSNSummaryRow), args.Int(1), args.Error(2)
}

func (m *MockReportService) CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.CollectionOverviewRow, error) {
	args := m.Called(ctx, tenantID, filters)
	return args.Get(0).([]domain.CollectionOverviewRow), args.Error(1)
}
```

**Step 4: Verify it compiles**

Run: `go build ./mocks/...`
Expected: success

**Step 5: Commit**

```bash
git add mocks/mock_document_summary_repo.go mocks/mock_report_repo.go mocks/mock_report_service.go
git commit -m "feat(reports): add mocks for summary repo, report repo, report service"
```

---

### Task 5: Document Summary Repository (Postgres)

**Files:**
- Create: `internal/repository/postgres/document_summary_repo.go`

**Step 1: Implement the repository**

```go
// internal/repository/postgres/document_summary_repo.go
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"satvos/internal/domain"
)

type documentSummaryRepo struct {
	db *sqlx.DB
}

func NewDocumentSummaryRepo(db *sqlx.DB) *documentSummaryRepo {
	return &documentSummaryRepo{db: db}
}

func (r *documentSummaryRepo) Upsert(ctx context.Context, summary *domain.DocumentSummary) error {
	query := `
		INSERT INTO document_summaries (
			document_id, tenant_id, collection_id,
			invoice_number, invoice_date, due_date, invoice_type, currency,
			place_of_supply, reverse_charge, has_irn,
			seller_name, seller_gstin, seller_state, seller_state_code,
			buyer_name, buyer_gstin, buyer_state, buyer_state_code,
			subtotal, total_discount, taxable_amount, cgst, sgst, igst, cess, total_amount,
			line_item_count, distinct_hsn_codes,
			parsing_status, review_status, validation_status, reconciliation_status,
			created_at, updated_at
		) VALUES (
			:document_id, :tenant_id, :collection_id,
			:invoice_number, :invoice_date, :due_date, :invoice_type, :currency,
			:place_of_supply, :reverse_charge, :has_irn,
			:seller_name, :seller_gstin, :seller_state, :seller_state_code,
			:buyer_name, :buyer_gstin, :buyer_state, :buyer_state_code,
			:subtotal, :total_discount, :taxable_amount, :cgst, :sgst, :igst, :cess, :total_amount,
			:line_item_count, :distinct_hsn_codes,
			:parsing_status, :review_status, :validation_status, :reconciliation_status,
			NOW(), NOW()
		)
		ON CONFLICT (document_id) DO UPDATE SET
			invoice_number = EXCLUDED.invoice_number,
			invoice_date = EXCLUDED.invoice_date,
			due_date = EXCLUDED.due_date,
			invoice_type = EXCLUDED.invoice_type,
			currency = EXCLUDED.currency,
			place_of_supply = EXCLUDED.place_of_supply,
			reverse_charge = EXCLUDED.reverse_charge,
			has_irn = EXCLUDED.has_irn,
			seller_name = EXCLUDED.seller_name,
			seller_gstin = EXCLUDED.seller_gstin,
			seller_state = EXCLUDED.seller_state,
			seller_state_code = EXCLUDED.seller_state_code,
			buyer_name = EXCLUDED.buyer_name,
			buyer_gstin = EXCLUDED.buyer_gstin,
			buyer_state = EXCLUDED.buyer_state,
			buyer_state_code = EXCLUDED.buyer_state_code,
			subtotal = EXCLUDED.subtotal,
			total_discount = EXCLUDED.total_discount,
			taxable_amount = EXCLUDED.taxable_amount,
			cgst = EXCLUDED.cgst,
			sgst = EXCLUDED.sgst,
			igst = EXCLUDED.igst,
			cess = EXCLUDED.cess,
			total_amount = EXCLUDED.total_amount,
			line_item_count = EXCLUDED.line_item_count,
			distinct_hsn_codes = EXCLUDED.distinct_hsn_codes,
			parsing_status = EXCLUDED.parsing_status,
			review_status = EXCLUDED.review_status,
			validation_status = EXCLUDED.validation_status,
			reconciliation_status = EXCLUDED.reconciliation_status,
			updated_at = NOW()`

	_, err := r.db.NamedExecContext(ctx, query, summary)
	if err != nil {
		return fmt.Errorf("upserting document summary: %w", err)
	}
	return nil
}

func (r *documentSummaryRepo) UpdateStatuses(ctx context.Context, documentID uuid.UUID, statuses domain.SummaryStatusUpdate) error {
	query := `
		UPDATE document_summaries SET
			parsing_status = $2,
			review_status = $3,
			validation_status = $4,
			reconciliation_status = $5,
			updated_at = NOW()
		WHERE document_id = $1`

	_, err := r.db.ExecContext(ctx, query, documentID,
		statuses.ParsingStatus, statuses.ReviewStatus,
		statuses.ValidationStatus, statuses.ReconciliationStatus)
	if err != nil {
		return fmt.Errorf("updating document summary statuses: %w", err)
	}
	return nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/repository/postgres/...`
Expected: success

**Step 3: Commit**

```bash
git add internal/repository/postgres/document_summary_repo.go
git commit -m "feat(reports): implement document summary postgres repository"
```

---

### Task 6: Summary Extraction Helper + Service Integration

This task adds the helper function that builds a `DocumentSummary` from a `Document` + parsed `GSTInvoice`, then hooks it into the document service at the right points.

**Files:**
- Modify: `internal/service/document_service.go` — add `summaryRepo` field, extraction helper, hook calls

**Step 1: Add `summaryRepo` to the service struct and constructors**

In `internal/service/document_service.go`:

Add the field to the struct (after `validator`):
```go
summaryRepo port.DocumentSummaryRepository
```

Add the parameter to `NewDocumentService` and `NewDocumentServiceWithMerge`. Wire it into the struct initialization. The parameter should be the last one (before the closing paren) to minimize churn on existing callers:
```go
func NewDocumentService(
	docRepo port.DocumentRepository,
	fileRepo port.FileMetaRepository,
	userRepo port.UserRepository,
	permRepo port.CollectionPermissionRepository,
	tagRepo port.DocumentTagRepository,
	docParser port.DocumentParser,
	storage port.ObjectStorage,
	validationEngine *validator.Engine,
	auditRepo port.DocumentAuditRepository,
	summaryRepo port.DocumentSummaryRepository,
) DocumentService {
```

Do the same for `NewDocumentServiceWithMerge`.

**Step 2: Add the summary extraction helper method**

Add this method to `document_service.go`:

```go
// upsertSummary builds a DocumentSummary from a Document and upserts it.
// Non-blocking: errors are logged but never returned.
func (s *documentService) upsertSummary(ctx context.Context, doc *domain.Document) {
	if s.summaryRepo == nil {
		return
	}

	var inv invoice.GSTInvoice
	if err := json.Unmarshal(doc.StructuredData, &inv); err != nil {
		log.Printf("documentService.upsertSummary: failed to unmarshal structured_data for %s: %v", doc.ID, err)
		return
	}

	summary := &domain.DocumentSummary{
		DocumentID:    doc.ID,
		TenantID:      doc.TenantID,
		CollectionID:  doc.CollectionID,
		InvoiceNumber: inv.Invoice.InvoiceNumber,
		InvoiceType:   inv.Invoice.InvoiceType,
		Currency:      inv.Invoice.Currency,
		PlaceOfSupply: inv.Invoice.PlaceOfSupply,
		ReverseCharge: inv.Invoice.ReverseCharge,
		HasIRN:        inv.Invoice.IRN != "",
		SellerName:    inv.Seller.Name,
		SellerGSTIN:   inv.Seller.GSTIN,
		SellerState:   inv.Seller.State,
		SellerStateCode: inv.Seller.StateCode,
		BuyerName:     inv.Buyer.Name,
		BuyerGSTIN:    inv.Buyer.GSTIN,
		BuyerState:    inv.Buyer.State,
		BuyerStateCode: inv.Buyer.StateCode,
		Subtotal:      inv.Totals.Subtotal,
		TotalDiscount: inv.Totals.TotalDiscount,
		TaxableAmount: inv.Totals.TaxableAmount,
		CGST:          inv.Totals.CGST,
		SGST:          inv.Totals.SGST,
		IGST:          inv.Totals.IGST,
		Cess:          inv.Totals.Cess,
		TotalAmount:   inv.Totals.Total,
		LineItemCount: len(inv.LineItems),
		ParsingStatus:        doc.ParsingStatus,
		ReviewStatus:         doc.ReviewStatus,
		ValidationStatus:     doc.ValidationStatus,
		ReconciliationStatus: doc.ReconciliationStatus,
	}

	// Parse invoice date
	summary.InvoiceDate = parseInvoiceDate(inv.Invoice.InvoiceDate)
	summary.DueDate = parseInvoiceDate(inv.Invoice.DueDate)

	// Collect distinct HSN codes
	hsnSet := make(map[string]struct{})
	for i := range inv.LineItems {
		if inv.LineItems[i].HSNSACCode != "" {
			hsnSet[inv.LineItems[i].HSNSACCode] = struct{}{}
		}
	}
	hsns := make([]string, 0, len(hsnSet))
	for code := range hsnSet {
		hsns = append(hsns, code)
	}
	summary.DistinctHSNCodes = hsns

	if err := s.summaryRepo.Upsert(ctx, summary); err != nil {
		log.Printf("documentService.upsertSummary: failed for %s: %v", doc.ID, err)
	}
}

// updateSummaryStatuses updates only the status columns on the summary row.
func (s *documentService) updateSummaryStatuses(ctx context.Context, doc *domain.Document) {
	if s.summaryRepo == nil {
		return
	}
	if err := s.summaryRepo.UpdateStatuses(ctx, doc.ID, domain.SummaryStatusUpdate{
		ParsingStatus:        doc.ParsingStatus,
		ReviewStatus:         doc.ReviewStatus,
		ValidationStatus:     doc.ValidationStatus,
		ReconciliationStatus: doc.ReconciliationStatus,
	}); err != nil {
		log.Printf("documentService.updateSummaryStatuses: failed for %s: %v", doc.ID, err)
	}
}

// parseInvoiceDate attempts multiple date formats from LLM output.
func parseInvoiceDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	formats := []string{"2006-01-02", "02/01/2006", "02-01-2006", "01/02/2006", "January 2, 2006", "Jan 2, 2006", "2 January 2006"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return &t
		}
	}
	return nil
}
```

Add `"satvos/internal/validator/invoice"` to imports if not already present.

**Step 3: Hook upsertSummary into ParseDocument (after successful parse)**

In `ParseDocument`, after the auto-tags extraction (around line 399-400), add:
```go
	// Upsert document summary for reporting
	s.upsertSummary(ctx, doc)
```

**Step 4: Hook upsertSummary into EditStructuredData (after validation)**

In `EditStructuredData`, after the re-fetch (where `updated` is returned, around line 655), add:
```go
	// Upsert document summary for reporting
	s.upsertSummary(ctx, updated)
```

**Step 5: Hook updateSummaryStatuses into UpdateReview (after save)**

In `UpdateReview`, after `docRepo.UpdateReviewStatus` succeeds (around line 508), add:
```go
	s.updateSummaryStatuses(ctx, doc)
```

**Step 6: Hook updateSummaryStatuses after validation in ParseDocument**

In `ParseDocument`, after `s.validator.ValidateDocument` succeeds (around line 407), re-fetch the doc and update statuses:
```go
	if s.validator != nil {
		if err := s.validator.ValidateDocument(ctx, doc.TenantID, doc.ID); err != nil {
			log.Printf("documentService.ParseDocument: validation failed for %s: %v", doc.ID, err)
		} else {
			s.auditValidationCompleted(ctx, doc.TenantID, doc.ID, nil, "parse")
			// Update summary statuses after validation
			if validatedDoc, err := s.docRepo.GetByID(ctx, doc.TenantID, doc.ID); err == nil {
				s.updateSummaryStatuses(ctx, validatedDoc)
			}
		}
	}
```

Note: This replaces the existing block — the `auditValidationCompleted` call moves inside the else block.

**Step 7: Hook updateSummaryStatuses after validation in EditStructuredData**

In `EditStructuredData`, the validation already runs and the doc is re-fetched. Add after the validation audit (around line 658):
```go
	s.updateSummaryStatuses(ctx, updated)
```

**Step 8: Update callers of NewDocumentService in main.go and tests**

In `cmd/server/main.go`, add `summaryRepo` to `NewDocumentService` and `NewDocumentServiceWithMerge` calls. Create the repo:
```go
summaryRepo := postgres.NewDocumentSummaryRepo(db)
```

Pass it as the last argument to the document service constructor.

In test files (`tests/unit/service/document_service_test.go`), update `setupDocumentService()` to include a `MockDocumentSummaryRepo`. Pass it to the constructor. Add `.Maybe()` on a default mock expectation to prevent unexpected call panics.

**Step 9: Verify it compiles and tests pass**

Run: `go build ./...`
Run: `go test ./... -count=1`
Expected: all pass

**Step 10: Commit**

```bash
git add internal/service/document_service.go cmd/server/main.go tests/unit/service/document_service_test.go
git commit -m "feat(reports): integrate document summary upsert into parse/edit/review flows"
```

---

### Task 7: Report Repository (Postgres)

**Files:**
- Create: `internal/repository/postgres/report_repo.go`

This is the largest task. It contains 7 aggregation query methods. Each method builds a dynamic query with optional filters and role-based scoping.

**Step 1: Implement the report repository**

Create `internal/repository/postgres/report_repo.go` with:

1. **Constructor** — `NewReportRepo(db *sqlx.DB)`

2. **Helper: `buildWhereClause`** — shared across all methods. Takes `tenantID`, `filters domain.ReportFilters` and returns a WHERE clause string + args slice. Handles:
   - `tenant_id = $1` (always)
   - `invoice_date >= $N` (if `From` set)
   - `invoice_date <= $N` (if `To` set)
   - `collection_id = $N` (if `CollectionID` set)
   - `seller_gstin = $N` (if `SellerGSTIN` set)
   - `buyer_gstin = $N` (if `BuyerGSTIN` set)
   - Viewer/free role scoping: `AND collection_id IN (SELECT collection_id FROM collection_permissions WHERE user_id = $N)`

3. **Helper: `dateTruncExpr`** — returns the PostgreSQL `date_trunc` expression for the given granularity:
   - `daily` → `date_trunc('day', invoice_date)`
   - `weekly` → `date_trunc('week', invoice_date)`
   - `monthly` → `date_trunc('month', invoice_date)`
   - `quarterly` → `date_trunc('quarter', invoice_date)`
   - `yearly` → `date_trunc('year', invoice_date)`

4. **Helper: `formatPeriod`** — formats a `time.Time` into a period label:
   - `daily` → `2025-10-15`
   - `weekly` → `2025-W42`
   - `monthly` → `2025-10`
   - `quarterly` → `2025-Q4`
   - `yearly` → `2025`

5. **SellerSummary** — `GROUP BY seller_gstin` with `COUNT(*)`, `SUM(total_amount)`, `SUM(cgst+sgst+igst)`, `AVG(total_amount)`, `MIN(invoice_date)`, `MAX(invoice_date)`. Uses `MAX(seller_name)` for display name. Ordered by `total_amount DESC`. Paginated with `OFFSET/LIMIT` and a count query.

6. **BuyerSummary** — Same pattern as SellerSummary but grouped by `buyer_gstin`.

7. **PartyLedger** — Selects rows where `seller_gstin = $gstin OR buyer_gstin = $gstin`. Adds a computed `role` column (`CASE WHEN seller_gstin = $gstin THEN 'seller' ELSE 'buyer'`), and counterparty name/gstin. Ordered by `invoice_date ASC`. Paginated.

8. **FinancialSummary** — Groups by `date_trunc(granularity, invoice_date)`. Returns period, count, subtotal, taxable_amount, cgst, sgst, igst, cess, total_amount. Ordered by period. No pagination (time series).

9. **TaxSummary** — Groups by `date_trunc(granularity, invoice_date)`. Splits intrastate (igst=0) vs interstate (igst>0) using conditional aggregation: `COUNT(CASE WHEN igst > 0 ...)`, `SUM(CASE WHEN igst > 0 THEN taxable_amount ...)`. Ordered by period.

10. **HSNSummary** — This one queries `documents.structured_data` JSONB directly (not the summary table):
    ```sql
    SELECT
        item->>'hsn_sac_code' AS hsn_code,
        MAX(item->>'description') AS description,
        COUNT(DISTINCT d.id) AS invoice_count,
        COUNT(*) AS line_item_count,
        COALESCE(SUM((item->>'quantity')::numeric), 0) AS total_quantity,
        COALESCE(SUM((item->>'taxable_amount')::numeric), 0) AS taxable_amount,
        COALESCE(SUM((item->>'cgst_amount')::numeric), 0) AS cgst,
        COALESCE(SUM((item->>'sgst_amount')::numeric), 0) AS sgst,
        COALESCE(SUM((item->>'igst_amount')::numeric), 0) AS igst,
        COALESCE(SUM((item->>'cgst_amount')::numeric + (item->>'sgst_amount')::numeric + (item->>'igst_amount')::numeric), 0) AS total_tax
    FROM documents d,
        jsonb_array_elements(d.structured_data->'line_items') AS item
    WHERE d.tenant_id = $1
        AND d.parsing_status = 'completed'
        AND item->>'hsn_sac_code' IS NOT NULL
        AND item->>'hsn_sac_code' != ''
    GROUP BY item->>'hsn_sac_code'
    ORDER BY taxable_amount DESC
    ```
    Uses the same date/collection/role filters but applied to documents table joined with document_summaries for the invoice_date filter.

11. **CollectionsOverview** — Joins `document_summaries` with `collections`. Groups by `collection_id, collection_name`. Computes percentage columns using conditional COUNT. No pagination.

**Step 2: Verify it compiles**

Run: `go build ./internal/repository/postgres/...`
Expected: success

**Step 3: Commit**

```bash
git add internal/repository/postgres/report_repo.go
git commit -m "feat(reports): implement report postgres repository with 7 aggregation queries"
```

---

### Task 8: Report Service

**Files:**
- Create: `internal/service/report_service.go`

**Step 1: Implement the service**

```go
// internal/service/report_service.go
package service

import (
	"context"

	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/port"
)

// ReportService provides financial reporting over parsed documents.
type ReportService interface {
	SellerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.SellerSummaryRow, int, error)
	BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.BuyerSummaryRow, int, error)
	PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters domain.ReportFilters) ([]domain.PartyLedgerRow, int, error)
	FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.FinancialSummaryRow, error)
	TaxSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.TaxSummaryRow, error)
	HSNSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.HSNSummaryRow, int, error)
	CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.CollectionOverviewRow, error)
}

type reportService struct {
	reportRepo port.ReportRepository
}

func NewReportService(reportRepo port.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) SellerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.SellerSummaryRow, int, error) {
	return s.reportRepo.SellerSummary(ctx, tenantID, filters)
}

func (s *reportService) BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.BuyerSummaryRow, int, error) {
	return s.reportRepo.BuyerSummary(ctx, tenantID, filters)
}

func (s *reportService) PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters domain.ReportFilters) ([]domain.PartyLedgerRow, int, error) {
	return s.reportRepo.PartyLedger(ctx, tenantID, gstin, filters)
}

func (s *reportService) FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.FinancialSummaryRow, error) {
	return s.reportRepo.FinancialSummary(ctx, tenantID, filters)
}

func (s *reportService) TaxSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.TaxSummaryRow, error) {
	return s.reportRepo.TaxSummary(ctx, tenantID, filters)
}

func (s *reportService) HSNSummary(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.HSNSummaryRow, int, error) {
	return s.reportRepo.HSNSummary(ctx, tenantID, filters)
}

func (s *reportService) CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters domain.ReportFilters) ([]domain.CollectionOverviewRow, error) {
	return s.reportRepo.CollectionsOverview(ctx, tenantID, filters)
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/service/...`
Expected: success

**Step 3: Commit**

```bash
git add internal/service/report_service.go
git commit -m "feat(reports): implement report service"
```

---

### Task 9: Report Handler

**Files:**
- Create: `internal/handler/report_handler.go`

**Step 1: Implement the handler**

Create `internal/handler/report_handler.go` with:

1. **Struct**: `ReportHandler` with `reportService service.ReportService`
2. **Constructor**: `NewReportHandler(reportService service.ReportService)`
3. **Helper: `parseReportFilters`** — extracts `from`, `to`, `collection_id`, `seller_gstin`, `buyer_gstin`, `granularity`, `offset`, `limit` from query params. Parses dates with `time.Parse("2006-01-02", ...)`. Defaults granularity to `"monthly"`. Validates granularity is one of: daily, weekly, monthly, quarterly, yearly. Injects `UserID` and `UserRole` from auth context.
4. **7 handler methods** — each calls `extractAuthContext`, `parseReportFilters`, delegates to service, returns via `RespondOK` or `RespondPaginated`.
5. **Swagger annotations** on each method following the project convention (see `internal/handler/stats_handler.go` for the pattern).

Handler methods:
- `Sellers(c *gin.Context)` — calls `SellerSummary`, responds with `RespondPaginated`
- `Buyers(c *gin.Context)` — calls `BuyerSummary`, responds with `RespondPaginated`
- `PartyLedger(c *gin.Context)` — reads `gstin` query param (required, returns 400 if missing), calls `PartyLedger`, responds with `RespondPaginated`
- `FinancialSummary(c *gin.Context)` — calls `FinancialSummary`, responds with `RespondOK`
- `TaxSummary(c *gin.Context)` — calls `TaxSummary`, responds with `RespondOK`
- `HSNSummary(c *gin.Context)` — calls `HSNSummary`, responds with `RespondPaginated`
- `CollectionsOverview(c *gin.Context)` — calls `CollectionsOverview`, responds with `RespondOK`

**Step 2: Verify it compiles**

Run: `go build ./internal/handler/...`
Expected: success

**Step 3: Commit**

```bash
git add internal/handler/report_handler.go
git commit -m "feat(reports): implement report handler with 7 endpoints"
```

---

### Task 10: Router + Main Wiring

**Files:**
- Modify: `internal/router/router.go` — add `reportH` parameter and routes
- Modify: `cmd/server/main.go` — create repos, service, handler, pass to router

**Step 1: Update router**

Add `reportH *handler.ReportHandler` parameter to `Setup()`. Add routes after the stats route (line 110):

```go
	// Report routes
	reports := protected.Group("/reports")
	reports.GET("/sellers", reportH.Sellers)
	reports.GET("/buyers", reportH.Buyers)
	reports.GET("/party-ledger", reportH.PartyLedger)
	reports.GET("/financial-summary", reportH.FinancialSummary)
	reports.GET("/tax-summary", reportH.TaxSummary)
	reports.GET("/hsn-summary", reportH.HSNSummary)
	reports.GET("/collections-overview", reportH.CollectionsOverview)
```

**Step 2: Update main.go**

After `statsRepo` creation, add:
```go
	summaryRepo := postgres.NewDocumentSummaryRepo(db)
	reportRepo := postgres.NewReportRepo(db)
```

After `statsSvc`, add:
```go
	reportSvc := service.NewReportService(reportRepo)
```

After `statsH`, add:
```go
	reportH := handler.NewReportHandler(reportSvc)
```

Pass `reportH` to `router.Setup()`.

**Step 3: Update all callers of router.Setup**

Check if any test files call `router.Setup` and update them too.

**Step 4: Verify it compiles**

Run: `go build ./...`
Expected: success

**Step 5: Commit**

```bash
git add internal/router/router.go cmd/server/main.go
git commit -m "feat(reports): wire report handler into router and main"
```

---

### Task 11: Backfill CLI

**Files:**
- Create: `cmd/backfill/main.go`
- Modify: `Makefile` — add `backfill-summaries` target

**Step 1: Implement the backfill command**

Create `cmd/backfill/main.go`. Pattern follows `cmd/seedhsn/main.go`:

1. Load config via `config.Load()`
2. Connect to database
3. Query all documents with `parsing_status = 'completed'` and non-null `structured_data`
4. For each document, unmarshal structured_data into `invoice.GSTInvoice`, build `DocumentSummary`, call `summaryRepo.Upsert`
5. Log progress every 100 documents
6. Report total count at end

The backfill should process in batches of 100 to avoid loading all documents into memory at once.

**Step 2: Add Makefile target**

Add to `Makefile`:
```makefile
backfill-summaries:
	go run ./cmd/backfill
```

**Step 3: Verify it compiles**

Run: `go build ./cmd/backfill/...`
Expected: success

**Step 4: Commit**

```bash
git add cmd/backfill/main.go Makefile
git commit -m "feat(reports): add backfill CLI for existing documents"
```

---

### Task 12: Service Tests — Summary Upsert Integration

**Files:**
- Modify: `tests/unit/service/document_service_test.go`

**Step 1: Write tests**

Add the following tests:

1. **TestDocumentService_ParseDocument_UpsertsSummary** — Verify that after a successful parse, `summaryRepo.Upsert` is called with a properly built `DocumentSummary`. Mock the summary repo, set expectation on `Upsert` with `mock.MatchedBy` to verify key fields (tenant_id, document_id, seller_name, total_amount, etc.).

2. **TestDocumentService_EditStructuredData_UpsertsSummary** — Verify that after editing structured data, `summaryRepo.Upsert` is called with the updated values.

3. **TestDocumentService_UpdateReview_UpdatesSummaryStatuses** — Verify that after a review, `summaryRepo.UpdateStatuses` is called with the new review_status.

4. **TestDocumentService_SummaryUpsertFailure_DoesNotFailParse** — Verify that if `summaryRepo.Upsert` returns an error, the parse still succeeds (non-blocking).

5. **TestDocumentService_NilSummaryRepo_NoPanic** — Verify that if summaryRepo is nil, no panic occurs during parse/edit/review.

**Step 2: Run tests**

Run: `go test ./tests/unit/service/ -run TestDocumentService_.*Summary -v -count=1`
Expected: all pass

**Step 3: Commit**

```bash
git add tests/unit/service/document_service_test.go
git commit -m "test(reports): add service tests for summary upsert integration"
```

---

### Task 13: Handler Tests

**Files:**
- Create: `tests/unit/handler/report_handler_test.go`

**Step 1: Write handler tests**

Test each of the 7 endpoints:

1. **TestReportHandler_Sellers_Success** — Mock service returns sellers, verify 200 + JSON body
2. **TestReportHandler_Buyers_Success** — Same for buyers
3. **TestReportHandler_PartyLedger_Success** — With `?gstin=29ABC...`, verify 200
4. **TestReportHandler_PartyLedger_MissingGSTIN** — No gstin param, verify 400
5. **TestReportHandler_FinancialSummary_Success** — With `?from=2025-04-01&to=2026-03-31&granularity=monthly`
6. **TestReportHandler_TaxSummary_Success** — Similar
7. **TestReportHandler_HSNSummary_Success** — Verify paginated response
8. **TestReportHandler_CollectionsOverview_Success** — Verify response
9. **TestReportHandler_InvalidDateFilter** — `?from=not-a-date`, verify 400
10. **TestReportHandler_InvalidGranularity** — `?granularity=hourly`, verify 400
11. **TestReportHandler_NoAuth** — No auth context, verify 401

Use the same test patterns as `tests/unit/handler/document_handler_test.go`: `gin.CreateTestContext`, `httptest.NewRecorder`, `setAuthContext`.

**Step 2: Run tests**

Run: `go test ./tests/unit/handler/ -run TestReportHandler -v -count=1`
Expected: all pass

**Step 3: Commit**

```bash
git add tests/unit/handler/report_handler_test.go
git commit -m "test(reports): add handler tests for all 7 report endpoints"
```

---

### Task 14: Swagger + CLAUDE.md + Final Verification

**Files:**
- Modify: `CLAUDE.md` — update architecture docs
- Regenerate: `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go`

**Step 1: Regenerate swagger docs**

Run: `swag init -g cmd/server/main.go -o docs`
Expected: success, no errors

**Step 2: Verify new endpoints appear in swagger**

Run: `grep -c "reports" docs/swagger.yaml`
Expected: multiple matches for `/reports/sellers`, `/reports/buyers`, etc.

**Step 3: Update CLAUDE.md**

- Update migration count (19 → 20, add `→ document-summaries`)
- Add `report_handler.go` description to handler section
- Add `report_service.go` description to service section
- Add `document_summary_repository.go` and `report_repository.go` to port section
- Add `document_summary_repo.go` and `report_repo.go` to repository section
- Add Document Lifecycle step for summary upsert
- Add Key Conventions entry for reports
- Add Gotchas: summary upsert non-blocking, HSN report uses JSONB not summary table
- Add `NewDocumentService` takes new `summaryRepo` param
- Update router.Setup param count

**Step 4: Full verification**

Run: `go build ./...`
Run: `go test ./... -race -count=1`
Run: `golangci-lint run ./...`
Expected: all green

**Step 5: Commit**

```bash
git add CLAUDE.md docs/
git commit -m "docs(reports): update CLAUDE.md and regenerate swagger docs"
```

---

### Task Summary

| Task | Component | Files |
|------|-----------|-------|
| 1 | Database migration | 2 new SQL files |
| 2 | Domain models | Modify `models.go` |
| 3 | Port interfaces | 2 new files |
| 4 | Mocks | 3 new files |
| 5 | Summary repository | 1 new file |
| 6 | Service integration (upsert hooks) | Modify `document_service.go`, `main.go`, tests |
| 7 | Report repository (7 queries) | 1 new file |
| 8 | Report service | 1 new file |
| 9 | Report handler | 1 new file |
| 10 | Router + main wiring | Modify `router.go`, `main.go` |
| 11 | Backfill CLI | 1 new file, modify `Makefile` |
| 12 | Service tests | Modify service test file |
| 13 | Handler tests | 1 new test file |
| 14 | Swagger + docs + final verification | Modify `CLAUDE.md`, regenerate docs |
