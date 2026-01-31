package handler

import (
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/middleware"
	"satvos/internal/service"
)

// CollectionHandler handles collection management endpoints.
type CollectionHandler struct {
	collectionService service.CollectionService
}

// NewCollectionHandler creates a new CollectionHandler.
func NewCollectionHandler(collectionService service.CollectionService) *CollectionHandler {
	return &CollectionHandler{collectionService: collectionService}
}

// Create handles POST /api/v1/collections
func (h *CollectionHandler) Create(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}

	collection, err := h.collectionService.Create(c.Request.Context(), service.CreateCollectionInput{
		TenantID:    tenantID,
		CreatedBy:   userID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondCreated(c, collection)
}

// List handles GET /api/v1/collections
func (h *CollectionHandler) List(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	offset, limit := parsePagination(c)

	collections, total, err := h.collectionService.List(c.Request.Context(), tenantID, userID, offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondPaginated(c, collections, PagMeta{Total: total, Offset: offset, Limit: limit})
}

// GetByID handles GET /api/v1/collections/:id
func (h *CollectionHandler) GetByID(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	collection, err := h.collectionService.GetByID(c.Request.Context(), tenantID, collectionID, userID)
	if err != nil {
		HandleError(c, err)
		return
	}

	// Also fetch files for the collection (first page)
	offset, limit := parsePagination(c)
	files, totalFiles, err := h.collectionService.ListFiles(c.Request.Context(), tenantID, collectionID, userID, offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{
		"collection": collection,
		"files":      files,
		"files_meta": PagMeta{Total: totalFiles, Offset: offset, Limit: limit},
	})
}

// Update handles PUT /api/v1/collections/:id
func (h *CollectionHandler) Update(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}

	collection, err := h.collectionService.Update(c.Request.Context(), &service.UpdateCollectionInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		UserID:       userID,
		Name:         req.Name,
		Description:  req.Description,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, collection)
}

// Delete handles DELETE /api/v1/collections/:id
func (h *CollectionHandler) Delete(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	if err := h.collectionService.Delete(c.Request.Context(), tenantID, collectionID, userID); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "collection deleted"})
}

// BatchUploadFiles handles POST /api/v1/collections/:id/files
func (h *CollectionHandler) BatchUploadFiles(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "multipart form is required")
		return
	}

	fileHeaders := form.File["files"]
	if len(fileHeaders) == 0 {
		RespondError(c, http.StatusBadRequest, "MISSING_FILES", "at least one file is required in 'files' field")
		return
	}

	inputs := make([]service.BatchUploadFileInput, 0, len(fileHeaders))
	openFiles := make([]multipart.File, 0, len(fileHeaders))
	defer func() {
		for _, f := range openFiles {
			f.Close()
		}
	}()
	for _, fh := range fileHeaders {
		f, err := fh.Open()
		if err != nil {
			RespondError(c, http.StatusBadRequest, "FILE_READ_ERROR", "failed to read uploaded file")
			return
		}
		openFiles = append(openFiles, f)
		inputs = append(inputs, service.BatchUploadFileInput{
			File:   f,
			Header: fh,
		})
	}

	results, err := h.collectionService.BatchUploadFiles(c.Request.Context(), tenantID, collectionID, userID, inputs)
	if err != nil {
		HandleError(c, err)
		return
	}

	// Check if all succeeded or some failed
	allSuccess := true
	for _, r := range results {
		if !r.Success {
			allSuccess = false
			break
		}
	}

	if allSuccess {
		RespondCreated(c, results)
	} else {
		// 207 Multi-Status for partial success
		c.JSON(http.StatusMultiStatus, APIResponse{Success: true, Data: results})
	}
}

// RemoveFile handles DELETE /api/v1/collections/:id/files/:fileId
func (h *CollectionHandler) RemoveFile(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	fileID, err := uuid.Parse(c.Param("fileId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid file ID")
		return
	}

	if err := h.collectionService.RemoveFile(c.Request.Context(), tenantID, collectionID, fileID, userID); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "file removed from collection"})
}

// SetPermission handles POST /api/v1/collections/:id/permissions
func (h *CollectionHandler) SetPermission(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	var req struct {
		UserID     uuid.UUID                   `json:"user_id" binding:"required"`
		Permission domain.CollectionPermission `json:"permission" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "user_id and permission are required")
		return
	}

	if err := h.collectionService.SetPermission(c.Request.Context(), &service.SetPermissionInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		GrantedBy:    userID,
		UserID:       req.UserID,
		Permission:   req.Permission,
	}); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "permission set"})
}

// ListPermissions handles GET /api/v1/collections/:id/permissions
func (h *CollectionHandler) ListPermissions(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	offset, limit := parsePagination(c)

	perms, total, err := h.collectionService.ListPermissions(c.Request.Context(), tenantID, collectionID, userID, offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondPaginated(c, perms, PagMeta{Total: total, Offset: offset, Limit: limit})
}

// RemovePermission handles DELETE /api/v1/collections/:id/permissions/:userId
func (h *CollectionHandler) RemovePermission(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}
	userID, err := middleware.GetUserID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context")
		return
	}

	collectionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection ID")
		return
	}

	targetUserID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid user ID")
		return
	}

	if err := h.collectionService.RemovePermission(c.Request.Context(), tenantID, collectionID, targetUserID, userID); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "permission removed"})
}

// parsePagination extracts offset and limit from query params with defaults.
func parsePagination(c *gin.Context) (offset, limit int) {
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}
