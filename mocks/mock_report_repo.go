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
