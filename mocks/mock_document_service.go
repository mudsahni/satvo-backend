package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/service"
	"satvos/internal/validator"
)

// MockDocumentService is a mock implementation of service.DocumentService.
type MockDocumentService struct {
	mock.Mock
}

func (m *MockDocumentService) CreateAndParse(ctx context.Context, input *service.CreateDocumentInput) (*domain.Document, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Document), args.Error(1)
}

func (m *MockDocumentService) GetByID(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error) {
	args := m.Called(ctx, tenantID, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Document), args.Error(1)
}

func (m *MockDocumentService) GetByFileID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.Document, error) {
	args := m.Called(ctx, tenantID, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Document), args.Error(1)
}

func (m *MockDocumentService) ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	args := m.Called(ctx, tenantID, collectionID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Document), args.Int(1), args.Error(2)
}

func (m *MockDocumentService) ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	args := m.Called(ctx, tenantID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Document), args.Int(1), args.Error(2)
}

func (m *MockDocumentService) UpdateReview(ctx context.Context, input *service.UpdateReviewInput) (*domain.Document, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Document), args.Error(1)
}

func (m *MockDocumentService) RetryParse(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error) {
	args := m.Called(ctx, tenantID, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Document), args.Error(1)
}

func (m *MockDocumentService) ValidateDocument(ctx context.Context, tenantID, docID uuid.UUID) error {
	args := m.Called(ctx, tenantID, docID)
	return args.Error(0)
}

func (m *MockDocumentService) GetValidation(ctx context.Context, tenantID, docID uuid.UUID) (*validator.ValidationResponse, error) {
	args := m.Called(ctx, tenantID, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*validator.ValidationResponse), args.Error(1)
}

func (m *MockDocumentService) Delete(ctx context.Context, tenantID, docID uuid.UUID) error {
	args := m.Called(ctx, tenantID, docID)
	return args.Error(0)
}
