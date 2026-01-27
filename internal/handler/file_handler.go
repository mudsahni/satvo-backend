package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"satvos/internal/middleware"
	"satvos/internal/service"
)

// FileHandler handles file upload and management endpoints.
type FileHandler struct {
	fileService service.FileService
}

// NewFileHandler creates a new FileHandler.
func NewFileHandler(fileService service.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

// Upload handles POST /api/v1/files/upload
func (h *FileHandler) Upload(c *gin.Context) {
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

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		RespondError(c, http.StatusBadRequest, "MISSING_FILE", "file field is required")
		return
	}
	defer file.Close()

	input := service.FileUploadInput{
		TenantID:   tenantID,
		UploadedBy: userID,
		File:       file,
		Header:     header,
	}

	meta, err := h.fileService.Upload(c.Request.Context(), input)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondCreated(c, meta)
}

// List handles GET /api/v1/files
func (h *FileHandler) List(c *gin.Context) {
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

	files, total, err := h.fileService.List(c.Request.Context(), tenantID, offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondPaginated(c, files, PagMeta{Total: total, Offset: offset, Limit: limit})
}

// GetByID handles GET /api/v1/files/:id
func (h *FileHandler) GetByID(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid file ID")
		return
	}

	meta, err := h.fileService.GetByID(c.Request.Context(), tenantID, fileID)
	if err != nil {
		HandleError(c, err)
		return
	}

	downloadURL, err := h.fileService.GetDownloadURL(c.Request.Context(), tenantID, fileID)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{
		"file":         meta,
		"download_url": downloadURL,
	})
}

// Delete handles DELETE /api/v1/files/:id
func (h *FileHandler) Delete(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid file ID")
		return
	}

	if err := h.fileService.Delete(c.Request.Context(), tenantID, fileID); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "file deleted"})
}
