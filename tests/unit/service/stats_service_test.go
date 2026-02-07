package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/service"
	"satvos/mocks"
)

func TestStatsService_GetStats_AdminCallsTenantStats(t *testing.T) {
	mockRepo := new(mocks.MockStatsRepo)
	svc := service.NewStatsService(mockRepo)

	tenantID := uuid.New()
	userID := uuid.New()

	expected := &domain.Stats{TotalDocuments: 100, TotalCollections: 5}
	mockRepo.On("GetTenantStats", mock.Anything, tenantID).Return(expected, nil)

	result, err := svc.GetStats(context.Background(), tenantID, userID, domain.RoleAdmin)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	mockRepo.AssertExpectations(t)
}

func TestStatsService_GetStats_ManagerCallsTenantStats(t *testing.T) {
	mockRepo := new(mocks.MockStatsRepo)
	svc := service.NewStatsService(mockRepo)

	tenantID := uuid.New()
	userID := uuid.New()

	expected := &domain.Stats{TotalDocuments: 50}
	mockRepo.On("GetTenantStats", mock.Anything, tenantID).Return(expected, nil)

	result, err := svc.GetStats(context.Background(), tenantID, userID, domain.RoleManager)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	mockRepo.AssertExpectations(t)
}

func TestStatsService_GetStats_MemberCallsTenantStats(t *testing.T) {
	mockRepo := new(mocks.MockStatsRepo)
	svc := service.NewStatsService(mockRepo)

	tenantID := uuid.New()
	userID := uuid.New()

	expected := &domain.Stats{TotalDocuments: 50}
	mockRepo.On("GetTenantStats", mock.Anything, tenantID).Return(expected, nil)

	result, err := svc.GetStats(context.Background(), tenantID, userID, domain.RoleMember)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	mockRepo.AssertExpectations(t)
}

func TestStatsService_GetStats_ViewerCallsUserStats(t *testing.T) {
	mockRepo := new(mocks.MockStatsRepo)
	svc := service.NewStatsService(mockRepo)

	tenantID := uuid.New()
	userID := uuid.New()

	expected := &domain.Stats{TotalDocuments: 10, TotalCollections: 2}
	mockRepo.On("GetUserStats", mock.Anything, tenantID, userID).Return(expected, nil)

	result, err := svc.GetStats(context.Background(), tenantID, userID, domain.RoleViewer)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	mockRepo.AssertExpectations(t)
}

func TestStatsService_GetStats_RepoError(t *testing.T) {
	mockRepo := new(mocks.MockStatsRepo)
	svc := service.NewStatsService(mockRepo)

	tenantID := uuid.New()
	userID := uuid.New()

	mockRepo.On("GetTenantStats", mock.Anything, tenantID).Return(nil, errors.New("db error"))

	result, err := svc.GetStats(context.Background(), tenantID, userID, domain.RoleAdmin)
	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestStatsService_GetStats_ViewerRepoError(t *testing.T) {
	mockRepo := new(mocks.MockStatsRepo)
	svc := service.NewStatsService(mockRepo)

	tenantID := uuid.New()
	userID := uuid.New()

	mockRepo.On("GetUserStats", mock.Anything, tenantID, userID).Return(nil, errors.New("db error"))

	result, err := svc.GetStats(context.Background(), tenantID, userID, domain.RoleViewer)
	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}
