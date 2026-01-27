package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockTenantRepo is a mock implementation of port.TenantRepository.
type MockTenantRepo struct {
	mock.Mock
}

func (m *MockTenantRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockTenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}

func (m *MockTenantRepo) GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}

func (m *MockTenantRepo) List(ctx context.Context, offset, limit int) ([]domain.Tenant, int, error) {
	args := m.Called(ctx, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Tenant), args.Int(1), args.Error(2)
}

func (m *MockTenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockTenantRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
