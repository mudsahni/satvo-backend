package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"satvos/internal/domain"
	"satvos/internal/middleware"
	"satvos/internal/service"
	"satvos/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)

	tenantID := uuid.New()
	userID := uuid.New()
	claims := &service.Claims{
		RegisteredClaims: jwt.RegisteredClaims{},
		TenantID:         tenantID,
		UserID:           userID,
		Email:            "user@test.com",
		Role:             domain.RoleMember,
	}

	mockAuth.On("ValidateToken", "valid-token").Return(claims, nil)

	r := gin.New()
	r.Use(middleware.AuthMiddleware(mockAuth))
	r.GET("/test", func(c *gin.Context) {
		tid, _ := middleware.GetTenantID(c)
		uid, _ := middleware.GetUserID(c)
		role := middleware.GetRole(c)
		c.JSON(http.StatusOK, gin.H{
			"tenant_id": tid,
			"user_id":   uid,
			"role":      role,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer valid-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, tenantID.String(), resp["tenant_id"])
	assert.Equal(t, userID.String(), resp["user_id"])
	assert.Equal(t, "member", resp["role"])
	mockAuth.AssertExpectations(t)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)

	r := gin.New()
	r.Use(middleware.AuthMiddleware(mockAuth))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_MalformedBearer(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)

	r := gin.New()
	r.Use(middleware.AuthMiddleware(mockAuth))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Basic some-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	mockAuth := new(mocks.MockAuthService)
	mockAuth.On("ValidateToken", "expired-token").Return(nil, domain.ErrUnauthorized)

	r := gin.New()
	r.Use(middleware.AuthMiddleware(mockAuth))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer expired-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockAuth.AssertExpectations(t)
}

func TestRequireRole_Allowed(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyRole, string(domain.RoleAdmin))
		c.Next()
	})
	r.GET("/admin", middleware.RequireRole(domain.RoleAdmin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Forbidden(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyRole, string(domain.RoleMember))
		c.Next()
	})
	r.GET("/admin", middleware.RequireRole(domain.RoleAdmin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRole_NoRole(t *testing.T) {
	r := gin.New()
	r.GET("/admin", middleware.RequireRole(domain.RoleAdmin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
