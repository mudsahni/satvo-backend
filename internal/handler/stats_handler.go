package handler

import (
	"github.com/gin-gonic/gin"

	"satvos/internal/service"
)

// StatsHandler handles stats endpoints.
type StatsHandler struct {
	statsService service.StatsService
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(statsService service.StatsService) *StatsHandler {
	return &StatsHandler{statsService: statsService}
}

// GetStats handles GET /api/v1/stats
// @Summary Get tenant statistics
// @Description Get aggregate counts for documents, collections, and their statuses. Admin/manager/member see tenant-wide stats, viewers see only stats from collections they have permission on.
// @Tags stats
// @Produce json
// @Success 200 {object} Response{data=domain.Stats} "Aggregate statistics"
// @Failure 401 {object} ErrorResponseBody "Unauthorized"
// @Security BearerAuth
// @Router /stats [get]
func (h *StatsHandler) GetStats(c *gin.Context) {
	tenantID, userID, role, ok := extractAuthContext(c)
	if !ok {
		return
	}

	stats, err := h.statsService.GetStats(c.Request.Context(), tenantID, userID, role)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, stats)
}
