package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/service"
)

// MockCollectionService is a mock implementation of service.CollectionService.
type MockCollectionService struct {
	mock.Mock
}

func (m *MockCollectionService) Create(ctx context.Context, input *service.CreateCollectionInput) (*domain.Collection, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Collection), args.Error(1)
}

func (m *MockCollectionService) GetByID(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole) (*domain.Collection, error) {
	args := m.Called(ctx, tenantID, collectionID, userID, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Collection), args.Error(1)
}

func (m *MockCollectionService) List(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole, offset, limit int) ([]domain.Collection, int, error) {
	args := m.Called(ctx, tenantID, userID, role, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Collection), args.Int(1), args.Error(2)
}

func (m *MockCollectionService) Update(ctx context.Context, input *service.UpdateCollectionInput) (*domain.Collection, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Collection), args.Error(1)
}

func (m *MockCollectionService) Delete(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole) error {
	args := m.Called(ctx, tenantID, collectionID, userID, role)
	return args.Error(0)
}

func (m *MockCollectionService) ListFiles(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole, offset, limit int) ([]domain.FileMeta, int, error) {
	args := m.Called(ctx, tenantID, collectionID, userID, role, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.FileMeta), args.Int(1), args.Error(2)
}

func (m *MockCollectionService) BatchUploadFiles(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole, files []service.BatchUploadFileInput) ([]service.BatchUploadResult, error) {
	args := m.Called(ctx, tenantID, collectionID, userID, role, files)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]service.BatchUploadResult), args.Error(1)
}

func (m *MockCollectionService) RemoveFile(ctx context.Context, tenantID, collectionID, fileID, userID uuid.UUID, role domain.UserRole) error {
	args := m.Called(ctx, tenantID, collectionID, fileID, userID, role)
	return args.Error(0)
}

func (m *MockCollectionService) AddFileToCollection(ctx context.Context, tenantID, collectionID, fileID, userID uuid.UUID, role domain.UserRole) error {
	args := m.Called(ctx, tenantID, collectionID, fileID, userID, role)
	return args.Error(0)
}

func (m *MockCollectionService) SetPermission(ctx context.Context, input *service.SetPermissionInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockCollectionService) ListPermissions(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole, offset, limit int) ([]domain.CollectionPermissionEntry, int, error) {
	args := m.Called(ctx, tenantID, collectionID, userID, role, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.CollectionPermissionEntry), args.Int(1), args.Error(2)
}

func (m *MockCollectionService) RemovePermission(ctx context.Context, tenantID, collectionID, targetUserID, userID uuid.UUID, role domain.UserRole) error {
	args := m.Called(ctx, tenantID, collectionID, targetUserID, userID, role)
	return args.Error(0)
}
