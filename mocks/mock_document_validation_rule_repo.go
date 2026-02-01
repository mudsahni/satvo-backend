package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
)

// MockDocumentValidationRuleRepo is a mock implementation of port.DocumentValidationRuleRepository.
type MockDocumentValidationRuleRepo struct {
	mock.Mock
}

func (m *MockDocumentValidationRuleRepo) Create(ctx context.Context, rule *domain.DocumentValidationRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockDocumentValidationRuleRepo) GetByID(ctx context.Context, tenantID, ruleID uuid.UUID) (*domain.DocumentValidationRule, error) {
	args := m.Called(ctx, tenantID, ruleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentValidationRule), args.Error(1)
}

func (m *MockDocumentValidationRuleRepo) ListByDocumentType(ctx context.Context, tenantID uuid.UUID, docType string, collectionID *uuid.UUID) ([]domain.DocumentValidationRule, error) {
	args := m.Called(ctx, tenantID, docType, collectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.DocumentValidationRule), args.Error(1)
}

func (m *MockDocumentValidationRuleRepo) ListBuiltinKeys(ctx context.Context, tenantID uuid.UUID, docType string) ([]string, error) {
	args := m.Called(ctx, tenantID, docType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockDocumentValidationRuleRepo) Update(ctx context.Context, rule *domain.DocumentValidationRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockDocumentValidationRuleRepo) Delete(ctx context.Context, tenantID, ruleID uuid.UUID) error {
	args := m.Called(ctx, tenantID, ruleID)
	return args.Error(0)
}
