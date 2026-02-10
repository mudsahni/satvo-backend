package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"satvos/internal/service"
)

// MockPasswordResetService is a mock implementation of service.PasswordResetService.
type MockPasswordResetService struct {
	mock.Mock
}

func (m *MockPasswordResetService) ForgotPassword(ctx context.Context, input service.ForgotPasswordInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockPasswordResetService) ResetPassword(ctx context.Context, input service.ResetPasswordInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}
