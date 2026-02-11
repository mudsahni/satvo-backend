package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/service"
)

// MockRegistrationService is a mock implementation of service.RegistrationService.
type MockRegistrationService struct {
	mock.Mock
}

func (m *MockRegistrationService) Register(ctx context.Context, input service.RegisterInput) (*service.RegisterOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.RegisterOutput), args.Error(1)
}

func (m *MockRegistrationService) VerifyEmail(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRegistrationService) ResendVerification(ctx context.Context, tenantID, userID uuid.UUID) error {
	args := m.Called(ctx, tenantID, userID)
	return args.Error(0)
}
