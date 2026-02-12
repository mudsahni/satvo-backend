package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"satvos/internal/service"
)

// MockSocialAuthService is a mock implementation of service.SocialAuthService.
type MockSocialAuthService struct {
	mock.Mock
}

func (m *MockSocialAuthService) SocialLogin(ctx context.Context, input service.SocialLoginInput) (*service.SocialLoginOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.SocialLoginOutput), args.Error(1)
}
