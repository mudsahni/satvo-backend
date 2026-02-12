package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"satvos/internal/port"
)

// MockSocialTokenVerifier is a mock implementation of port.SocialTokenVerifier.
type MockSocialTokenVerifier struct {
	mock.Mock
}

func (m *MockSocialTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (*port.SocialAuthClaims, error) {
	args := m.Called(ctx, idToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*port.SocialAuthClaims), args.Error(1)
}

func (m *MockSocialTokenVerifier) Provider() string {
	args := m.Called()
	return args.String(0)
}
