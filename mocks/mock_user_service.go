package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/service"
)

// MockUserService is a mock implementation of service.UserService.
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) Create(ctx context.Context, tenantID uuid.UUID, input service.CreateUserInput) (*domain.User, error) {
	args := m.Called(ctx, tenantID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) GetByID(ctx context.Context, tenantID, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) List(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.User, int, error) {
	args := m.Called(ctx, tenantID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.User), args.Int(1), args.Error(2)
}

func (m *MockUserService) Update(ctx context.Context, tenantID, userID uuid.UUID, input service.UpdateUserInput) (*domain.User, error) {
	args := m.Called(ctx, tenantID, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) Delete(ctx context.Context, tenantID, userID uuid.UUID) error {
	args := m.Called(ctx, tenantID, userID)
	return args.Error(0)
}
