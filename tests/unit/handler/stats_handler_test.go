package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/handler"
	"satvos/mocks"
)

func newStatsHandler() (*handler.StatsHandler, *mocks.MockStatsService) {
	mockSvc := new(mocks.MockStatsService)
	h := handler.NewStatsHandler(mockSvc)
	return h, mockSvc
}

func TestStatsHandler_GetStats_AdminSuccess(t *testing.T) {
	h, mockSvc := newStatsHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	expected := &domain.Stats{
		TotalDocuments:    156,
		TotalCollections:  12,
		ParsingCompleted:  140,
		ParsingFailed:     6,
		ParsingProcessing: 2,
		ParsingPending:    3,
		ParsingQueued:     5,
		ValidationValid:   56,
		ValidationWarning: 14,
		ValidationInvalid: 86,
		ReviewPending:     84,
		ReviewApproved:    60,
		ReviewRejected:    12,
	}

	mockSvc.On("GetStats", mock.Anything, tenantID, userID, domain.RoleAdmin).Return(expected, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/stats", http.NoBody)
	setAuthContext(c, tenantID, userID, "admin")

	h.GetStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	mockSvc.AssertExpectations(t)
}

func TestStatsHandler_GetStats_ViewerSuccess(t *testing.T) {
	h, mockSvc := newStatsHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	expected := &domain.Stats{
		TotalDocuments:   10,
		TotalCollections: 2,
		ParsingCompleted: 8,
		ParsingFailed:    2,
	}

	mockSvc.On("GetStats", mock.Anything, tenantID, userID, domain.RoleViewer).Return(expected, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/stats", http.NoBody)
	setAuthContext(c, tenantID, userID, "viewer")

	h.GetStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	mockSvc.AssertExpectations(t)
}

func TestStatsHandler_GetStats_MissingTenantContext(t *testing.T) {
	h, _ := newStatsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/stats", http.NoBody)
	// No auth context set

	h.GetStats(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestStatsHandler_GetStats_ServiceError(t *testing.T) {
	h, mockSvc := newStatsHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	mockSvc.On("GetStats", mock.Anything, tenantID, userID, domain.RoleAdmin).Return(nil, errors.New("db error"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/stats", http.NoBody)
	setAuthContext(c, tenantID, userID, "admin")

	h.GetStats(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockSvc.AssertExpectations(t)
}
