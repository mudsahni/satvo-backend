package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockStatsService is a mock implementation of service.StatsService.
type MockStatsService struct {
	mock.Mock
}

func (m *MockStatsService) GetStats(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole) (*domain.Stats, error) {
	args := m.Called(ctx, tenantID, userID, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Stats), args.Error(1)
}
