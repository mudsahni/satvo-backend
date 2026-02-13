package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockDocumentAuditRepo is a mock implementation of port.DocumentAuditRepository.
type MockDocumentAuditRepo struct {
	mock.Mock
}

func (m *MockDocumentAuditRepo) Create(ctx context.Context, entry *domain.DocumentAuditEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockDocumentAuditRepo) ListByDocument(ctx context.Context, tenantID, documentID uuid.UUID, offset, limit int) ([]domain.DocumentAuditEntry, int, error) {
	args := m.Called(ctx, tenantID, documentID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.DocumentAuditEntry), args.Int(1), args.Error(2)
}
