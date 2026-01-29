package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockCollectionFileRepo is a mock implementation of port.CollectionFileRepository.
type MockCollectionFileRepo struct {
	mock.Mock
}

func (m *MockCollectionFileRepo) Add(ctx context.Context, cf *domain.CollectionFile) error {
	args := m.Called(ctx, cf)
	return args.Error(0)
}

func (m *MockCollectionFileRepo) Remove(ctx context.Context, collectionID, fileID uuid.UUID) error {
	args := m.Called(ctx, collectionID, fileID)
	return args.Error(0)
}

func (m *MockCollectionFileRepo) ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, offset, limit int) ([]domain.FileMeta, int, error) {
	args := m.Called(ctx, tenantID, collectionID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.FileMeta), args.Int(1), args.Error(2)
}
