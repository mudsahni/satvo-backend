package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"satvos/internal/service"
)

// TenantHandler handles tenant management endpoints.
type TenantHandler struct {
	tenantService service.TenantService
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler(tenantService service.TenantService) *TenantHandler {
	return &TenantHandler{tenantService: tenantService}
}

// Create handles POST /api/v1/admin/tenants
func (h *TenantHandler) Create(c *gin.Context) {
	var input service.CreateTenantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	tenant, err := h.tenantService.Create(c.Request.Context(), input)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondCreated(c, tenant)
}

// List handles GET /api/v1/admin/tenants
func (h *TenantHandler) List(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	tenants, total, err := h.tenantService.List(c.Request.Context(), offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondPaginated(c, tenants, PagMeta{Total: total, Offset: offset, Limit: limit})
}

// GetByID handles GET /api/v1/admin/tenants/:id
func (h *TenantHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid tenant ID")
		return
	}

	tenant, err := h.tenantService.GetByID(c.Request.Context(), id)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, tenant)
}

// Update handles PUT /api/v1/admin/tenants/:id
func (h *TenantHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid tenant ID")
		return
	}

	var input service.UpdateTenantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	tenant, err := h.tenantService.Update(c.Request.Context(), id, input)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, tenant)
}

// Delete handles DELETE /api/v1/admin/tenants/:id
func (h *TenantHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid tenant ID")
		return
	}

	if err := h.tenantService.Delete(c.Request.Context(), id); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "tenant deleted"})
}
