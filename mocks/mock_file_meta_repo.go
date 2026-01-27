package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockFileMetaRepo is a mock implementation of port.FileMetaRepository.
type MockFileMetaRepo struct {
	mock.Mock
}

func (m *MockFileMetaRepo) Create(ctx context.Context, meta *domain.FileMeta) error {
	args := m.Called(ctx, meta)
	return args.Error(0)
}

func (m *MockFileMetaRepo) GetByID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.FileMeta, error) {
	args := m.Called(ctx, tenantID, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FileMeta), args.Error(1)
}

func (m *MockFileMetaRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.FileMeta, int, error) {
	args := m.Called(ctx, tenantID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.FileMeta), args.Int(1), args.Error(2)
}

func (m *MockFileMetaRepo) UpdateStatus(ctx context.Context, tenantID, fileID uuid.UUID, status domain.FileStatus) error {
	args := m.Called(ctx, tenantID, fileID, status)
	return args.Error(0)
}

func (m *MockFileMetaRepo) Delete(ctx context.Context, tenantID, fileID uuid.UUID) error {
	args := m.Called(ctx, tenantID, fileID)
	return args.Error(0)
}
