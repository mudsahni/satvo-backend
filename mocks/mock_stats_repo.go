package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockStatsRepo is a mock implementation of port.StatsRepository.
type MockStatsRepo struct {
	mock.Mock
}

func (m *MockStatsRepo) GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*domain.Stats, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Stats), args.Error(1)
}

func (m *MockStatsRepo) GetUserStats(ctx context.Context, tenantID, userID uuid.UUID) (*domain.Stats, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Stats), args.Error(1)
}
