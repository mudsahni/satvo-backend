package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockDocumentRepo is a mock implementation of port.DocumentRepository.
type MockDocumentRepo struct {
	mock.Mock
}

func (m *MockDocumentRepo) Create(ctx context.Context, doc *domain.Document) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}

func (m *MockDocumentRepo) GetByID(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error) {
	args := m.Called(ctx, tenantID, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Document), args.Error(1)
}

func (m *MockDocumentRepo) GetByFileID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.Document, error) {
	args := m.Called(ctx, tenantID, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Document), args.Error(1)
}

func (m *MockDocumentRepo) ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	args := m.Called(ctx, tenantID, collectionID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Document), args.Int(1), args.Error(2)
}

func (m *MockDocumentRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	args := m.Called(ctx, tenantID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Document), args.Int(1), args.Error(2)
}

func (m *MockDocumentRepo) UpdateStructuredData(ctx context.Context, doc *domain.Document) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}

func (m *MockDocumentRepo) UpdateReviewStatus(ctx context.Context, doc *domain.Document) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}

func (m *MockDocumentRepo) Delete(ctx context.Context, tenantID, docID uuid.UUID) error {
	args := m.Called(ctx, tenantID, docID)
	return args.Error(0)
}
