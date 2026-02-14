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
