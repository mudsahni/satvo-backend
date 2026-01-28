package router

import (
	"github.com/gin-gonic/gin"

	"satvos/internal/domain"
	"satvos/internal/handler"
	"satvos/internal/middleware"
	"satvos/internal/service"
)

// Setup configures the Gin engine with all routes and middleware.
func Setup(
	authSvc service.AuthService,
	authH *handler.AuthHandler,
	fileH *handler.FileHandler,
	tenantH *handler.TenantHandler,
	userH *handler.UserHandler,
	healthH *handler.HealthHandler,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(middleware.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())

	// Health checks
	r.GET("/healthz", healthH.Liveness)
	r.GET("/readyz", healthH.Readiness)

	v1 := r.Group("/api/v1")

	// Public auth routes
	auth := v1.Group("/auth")
	auth.POST("/login", authH.Login)
	auth.POST("/refresh", authH.RefreshToken)

	// Protected routes - require valid JWT
	protected := v1.Group("")
	protected.Use(middleware.AuthMiddleware(authSvc))

	// File routes
	files := protected.Group("/files")
	files.POST("/upload", fileH.Upload)
	files.GET("", fileH.List)
	files.GET("/:id", fileH.GetByID)
	files.DELETE("/:id", middleware.RequireRole(domain.RoleAdmin), fileH.Delete)

	// User management (tenant-scoped)
	users := protected.Group("/users")
	users.POST("", middleware.RequireRole(domain.RoleAdmin), userH.Create)
	users.GET("", middleware.RequireRole(domain.RoleAdmin), userH.List)
	users.GET("/:id", userH.GetByID)
	users.PUT("/:id", userH.Update)
	users.DELETE("/:id", middleware.RequireRole(domain.RoleAdmin), userH.Delete)

	// Admin routes - tenant management
	admin := v1.Group("/admin")
	admin.Use(middleware.AuthMiddleware(authSvc))
	admin.Use(middleware.RequireRole(domain.RoleAdmin))
	admin.POST("/tenants", tenantH.Create)
	admin.GET("/tenants", tenantH.List)
	admin.GET("/tenants/:id", tenantH.GetByID)
	admin.PUT("/tenants/:id", tenantH.Update)
	admin.DELETE("/tenants/:id", tenantH.Delete)

	return r
}
