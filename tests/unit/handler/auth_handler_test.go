package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/handler"
	"satvos/internal/service"
	"satvos/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthHandler_Login_Success(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)
	h := handler.NewAuthHandler(mockAuth)

	tokenPair := &service.TokenPair{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	mockAuth.On("Login", mock.Anything, service.LoginInput{
		TenantSlug: "test-tenant",
		Email:      "user@test.com",
		Password:   "password123",
	}).Return(tokenPair, nil)

	body, _ := json.Marshal(map[string]string{
		"tenant_slug": "test-tenant",
		"email":       "user@test.com",
		"password":    "password123",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	mockAuth.AssertExpectations(t)
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)
	h := handler.NewAuthHandler(mockAuth)

	mockAuth.On("Login", mock.Anything, mock.AnythingOfType("service.LoginInput")).
		Return(nil, domain.ErrInvalidCredentials)

	body, _ := json.Marshal(map[string]string{
		"tenant_slug": "test-tenant",
		"email":       "user@test.com",
		"password":    "wrongpassword",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_Login_ValidationError(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)
	h := handler.NewAuthHandler(mockAuth)

	// Missing required fields
	body, _ := json.Marshal(map[string]string{
		"email": "not-an-email",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)
	h := handler.NewAuthHandler(mockAuth)

	tokenPair := &service.TokenPair{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	mockAuth.On("RefreshToken", mock.Anything, "valid-refresh-token").Return(tokenPair, nil)

	body, _ := json.Marshal(map[string]string{
		"refresh_token": "valid-refresh-token",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RefreshToken(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAuth.AssertExpectations(t)
}
