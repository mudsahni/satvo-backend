package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	db *sqlx.DB
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *sqlx.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Liveness handles GET /healthz
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readiness handles GET /readyz
func (h *HealthHandler) Readiness(c *gin.Context) {
	if err := h.db.PingContext(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "error": "database not reachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
