package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantGuard returns middleware that ensures tenant context is present.
// It relies on AuthMiddleware having already set the tenant_id.
func TenantGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get(ContextKeyTenantID)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   gin.H{"code": "UNAUTHORIZED", "message": "tenant context required"},
			})
			return
		}
		c.Next()
	}
}
