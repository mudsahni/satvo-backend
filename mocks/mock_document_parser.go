package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"satvos/internal/port"
)

// MockDocumentParser is a mock implementation of port.DocumentParser.
type MockDocumentParser struct {
	mock.Mock
}

func (m *MockDocumentParser) Parse(ctx context.Context, input port.ParseInput) (*port.ParseOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*port.ParseOutput), args.Error(1)
}
