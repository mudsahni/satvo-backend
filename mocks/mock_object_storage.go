package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"satvos/internal/port"
)

// MockObjectStorage is a mock implementation of port.ObjectStorage.
type MockObjectStorage struct {
	mock.Mock
}

func (m *MockObjectStorage) Upload(ctx context.Context, input port.UploadInput) (*port.UploadOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*port.UploadOutput), args.Error(1)
}

func (m *MockObjectStorage) Download(ctx context.Context, bucket, key string) ([]byte, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockObjectStorage) Delete(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}

func (m *MockObjectStorage) GetPresignedURL(ctx context.Context, bucket, key string, expirySeconds int64) (string, error) {
	args := m.Called(ctx, bucket, key, expirySeconds)
	return args.String(0), args.Error(1)
}
