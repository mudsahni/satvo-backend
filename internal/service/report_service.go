package service

import (
	"context"

	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/port"
)

// ReportService provides financial reporting over parsed documents.
type ReportService interface {
	SellerSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.SellerSummaryRow, int, error)
	BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.BuyerSummaryRow, int, error)
	PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters *domain.ReportFilters) ([]domain.PartyLedgerRow, int, error)
	FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.FinancialSummaryRow, error)
	TaxSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.TaxSummaryRow, error)
	HSNSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.HSNSummaryRow, int, error)
	CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.CollectionOverviewRow, error)
}

type reportService struct {
	reportRepo port.ReportRepository
}

func NewReportService(reportRepo port.ReportRepository) ReportService {
	return &reportService{reportRepo: reportRepo}
}

func (s *reportService) SellerSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.SellerSummaryRow, int, error) {
	return s.reportRepo.SellerSummary(ctx, tenantID, filters)
}

func (s *reportService) BuyerSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.BuyerSummaryRow, int, error) {
	return s.reportRepo.BuyerSummary(ctx, tenantID, filters)
}

func (s *reportService) PartyLedger(ctx context.Context, tenantID uuid.UUID, gstin string, filters *domain.ReportFilters) ([]domain.PartyLedgerRow, int, error) {
	return s.reportRepo.PartyLedger(ctx, tenantID, gstin, filters)
}

func (s *reportService) FinancialSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.FinancialSummaryRow, error) {
	return s.reportRepo.FinancialSummary(ctx, tenantID, filters)
}

func (s *reportService) TaxSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.TaxSummaryRow, error) {
	return s.reportRepo.TaxSummary(ctx, tenantID, filters)
}

func (s *reportService) HSNSummary(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.HSNSummaryRow, int, error) {
	return s.reportRepo.HSNSummary(ctx, tenantID, filters)
}

func (s *reportService) CollectionsOverview(ctx context.Context, tenantID uuid.UUID, filters *domain.ReportFilters) ([]domain.CollectionOverviewRow, error) {
	return s.reportRepo.CollectionsOverview(ctx, tenantID, filters)
}
