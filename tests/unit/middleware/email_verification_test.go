package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/middleware"
	"satvos/mocks"
)

func TestRequireEmailVerified_FreeUser_NotVerified(t *testing.T) {
	userRepo := new(mocks.MockUserRepo)
	tenantID := uuid.New()
	userID := uuid.New()

	userRepo.On("GetByID", mock.Anything, tenantID, userID).Return(&domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Role:          domain.RoleFree,
		EmailVerified: false,
	}, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyTenantID, tenantID)
		c.Set(middleware.ContextKeyUserID, userID)
		c.Set(middleware.ContextKeyRole, string(domain.RoleFree))
		c.Next()
	})
	r.Use(middleware.RequireEmailVerified(userRepo))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "EMAIL_NOT_VERIFIED", errObj["code"])
	userRepo.AssertExpectations(t)
}

func TestRequireEmailVerified_FreeUser_Verified(t *testing.T) {
	userRepo := new(mocks.MockUserRepo)
	tenantID := uuid.New()
	userID := uuid.New()

	userRepo.On("GetByID", mock.Anything, tenantID, userID).Return(&domain.User{
		ID:            userID,
		TenantID:      tenantID,
		Role:          domain.RoleFree,
		EmailVerified: true,
	}, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyTenantID, tenantID)
		c.Set(middleware.ContextKeyUserID, userID)
		c.Set(middleware.ContextKeyRole, string(domain.RoleFree))
		c.Next()
	})
	r.Use(middleware.RequireEmailVerified(userRepo))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	userRepo.AssertExpectations(t)
}

func TestRequireEmailVerified_AdminUser_SkipsCheck(t *testing.T) {
	userRepo := new(mocks.MockUserRepo)
	tenantID := uuid.New()
	userID := uuid.New()

	// userRepo.GetByID should NOT be called for admin
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyTenantID, tenantID)
		c.Set(middleware.ContextKeyUserID, userID)
		c.Set(middleware.ContextKeyRole, string(domain.RoleAdmin))
		c.Next()
	})
	r.Use(middleware.RequireEmailVerified(userRepo))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// GetByID should not have been called
	userRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything, mock.Anything)
}

func TestRequireEmailVerified_MemberUser_SkipsCheck(t *testing.T) {
	userRepo := new(mocks.MockUserRepo)
	tenantID := uuid.New()
	userID := uuid.New()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyTenantID, tenantID)
		c.Set(middleware.ContextKeyUserID, userID)
		c.Set(middleware.ContextKeyRole, string(domain.RoleMember))
		c.Next()
	})
	r.Use(middleware.RequireEmailVerified(userRepo))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	userRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything, mock.Anything)
}

func TestRequireEmailVerified_FreeUser_UserNotFound(t *testing.T) {
	userRepo := new(mocks.MockUserRepo)
	tenantID := uuid.New()
	userID := uuid.New()

	userRepo.On("GetByID", mock.Anything, tenantID, userID).Return(nil, domain.ErrNotFound)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyTenantID, tenantID)
		c.Set(middleware.ContextKeyUserID, userID)
		c.Set(middleware.ContextKeyRole, string(domain.RoleFree))
		c.Next()
	})
	r.Use(middleware.RequireEmailVerified(userRepo))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	userRepo.AssertExpectations(t)
}
