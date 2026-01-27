package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/service"
)

const (
	ContextKeyTenantID = "tenant_id"
	ContextKeyUserID   = "user_id"
	ContextKeyEmail    = "email"
	ContextKeyRole     = "role"
	ContextKeyClaims   = "claims"
)

// AuthMiddleware returns Gin middleware that validates JWT tokens and injects
// tenant and user context.
func AuthMiddleware(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "missing or invalid authorization header"},
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := authService.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "invalid or expired token"},
			})
			return
		}

		c.Set(ContextKeyTenantID, claims.TenantID)
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyEmail, claims.Email)
		c.Set(ContextKeyRole, string(claims.Role))
		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

// RequireRole returns middleware that checks the user's role against allowed roles.
func RequireRole(roles ...domain.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr, exists := c.Get(ContextKeyRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   gin.H{"code": "FORBIDDEN", "message": "role not found in context"},
			})
			return
		}

		userRole := domain.UserRole(roleStr.(string))
		for _, r := range roles {
			if userRole == r {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   gin.H{"code": "FORBIDDEN", "message": "insufficient permissions"},
		})
	}
}

// GetTenantID extracts the tenant ID from the Gin context.
func GetTenantID(c *gin.Context) (uuid.UUID, error) {
	val, exists := c.Get(ContextKeyTenantID)
	if !exists {
		return uuid.Nil, domain.ErrUnauthorized
	}
	return val.(uuid.UUID), nil
}

// GetUserID extracts the user ID from the Gin context.
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	val, exists := c.Get(ContextKeyUserID)
	if !exists {
		return uuid.Nil, domain.ErrUnauthorized
	}
	return val.(uuid.UUID), nil
}

// GetRole extracts the user role string from the Gin context.
func GetRole(c *gin.Context) string {
	val, exists := c.Get(ContextKeyRole)
	if !exists {
		return ""
	}
	return val.(string)
}
