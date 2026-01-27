package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"satvos/internal/middleware"
	"satvos/internal/service"
)

// UserHandler handles user management endpoints.
type UserHandler struct {
	userService service.UserService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Create handles POST /api/v1/users
func (h *UserHandler) Create(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	var input service.CreateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	user, err := h.userService.Create(c.Request.Context(), tenantID, input)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondCreated(c, user)
}

// List handles GET /api/v1/users
func (h *UserHandler) List(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	users, total, err := h.userService.List(c.Request.Context(), tenantID, offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondPaginated(c, users, PagMeta{Total: total, Offset: offset, Limit: limit})
}

// GetByID handles GET /api/v1/users/:id
func (h *UserHandler) GetByID(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid user ID")
		return
	}

	// Allow self-access or admin access
	currentUserID, _ := middleware.GetUserID(c)
	currentRole := middleware.GetRole(c)
	if currentUserID != userID && currentRole != "admin" {
		RespondError(c, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), tenantID, userID)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, user)
}

// Update handles PUT /api/v1/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid user ID")
		return
	}

	// Allow self-access or admin access
	currentUserID, _ := middleware.GetUserID(c)
	currentRole := middleware.GetRole(c)
	if currentUserID != userID && currentRole != "admin" {
		RespondError(c, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
		return
	}

	var input service.UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Non-admins cannot change their own role
	if currentRole != "admin" && input.Role != nil {
		RespondError(c, http.StatusForbidden, "FORBIDDEN", "only admins can change user roles")
		return
	}

	user, err := h.userService.Update(c.Request.Context(), tenantID, userID, input)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, user)
}

// Delete handles DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid user ID")
		return
	}

	if err := h.userService.Delete(c.Request.Context(), tenantID, userID); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "user deleted"})
}
