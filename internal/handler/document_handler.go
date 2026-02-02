package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/middleware"
	"satvos/internal/service"
)

// DocumentHandler handles document parsing endpoints.
type DocumentHandler struct {
	documentService service.DocumentService
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(documentService service.DocumentService) *DocumentHandler {
	return &DocumentHandler{documentService: documentService}
}

// Create handles POST /api/v1/documents
func (h *DocumentHandler) Create(c *gin.Context) {
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
	role := domain.UserRole(middleware.GetRole(c))

	var req struct {
		FileID       uuid.UUID       `json:"file_id" binding:"required"`
		CollectionID uuid.UUID       `json:"collection_id" binding:"required"`
		DocumentType string          `json:"document_type" binding:"required"`
		ParseMode    domain.ParseMode `json:"parse_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "file_id, collection_id, and document_type are required")
		return
	}

	// Default to single parse mode
	if req.ParseMode == "" {
		req.ParseMode = domain.ParseModeSingle
	}
	if !domain.ValidParseModes[req.ParseMode] {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "parse_mode must be 'single' or 'dual'")
		return
	}

	doc, err := h.documentService.CreateAndParse(c.Request.Context(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: req.CollectionID,
		FileID:       req.FileID,
		DocumentType: req.DocumentType,
		ParseMode:    req.ParseMode,
		CreatedBy:    userID,
		Role:         role,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondCreated(c, doc)
}

// GetByID handles GET /api/v1/documents/:id
func (h *DocumentHandler) GetByID(c *gin.Context) {
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
	role := domain.UserRole(middleware.GetRole(c))

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid document ID")
		return
	}

	doc, err := h.documentService.GetByID(c.Request.Context(), tenantID, docID, userID, role)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, doc)
}

// List handles GET /api/v1/documents
func (h *DocumentHandler) List(c *gin.Context) {
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
	role := domain.UserRole(middleware.GetRole(c))

	offset, limit := parsePagination(c)

	collectionIDStr := c.Query("collection_id")
	if collectionIDStr != "" {
		collectionID, err := uuid.Parse(collectionIDStr)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid collection_id")
			return
		}
		docs, total, err := h.documentService.ListByCollection(c.Request.Context(), tenantID, collectionID, userID, role, offset, limit)
		if err != nil {
			HandleError(c, err)
			return
		}
		RespondPaginated(c, docs, PagMeta{Total: total, Offset: offset, Limit: limit})
		return
	}

	docs, total, err := h.documentService.ListByTenant(c.Request.Context(), tenantID, userID, role, offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondPaginated(c, docs, PagMeta{Total: total, Offset: offset, Limit: limit})
}

// Retry handles POST /api/v1/documents/:id/retry
func (h *DocumentHandler) Retry(c *gin.Context) {
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
	role := domain.UserRole(middleware.GetRole(c))

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid document ID")
		return
	}

	doc, err := h.documentService.RetryParse(c.Request.Context(), tenantID, docID, userID, role)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, doc)
}

// UpdateReview handles PUT /api/v1/documents/:id/review
func (h *DocumentHandler) UpdateReview(c *gin.Context) {
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
	role := domain.UserRole(middleware.GetRole(c))

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid document ID")
		return
	}

	var req struct {
		Status domain.ReviewStatus `json:"status" binding:"required"`
		Notes  string              `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "status is required (approved or rejected)")
		return
	}

	if req.Status != domain.ReviewStatusApproved && req.Status != domain.ReviewStatusRejected {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "status must be 'approved' or 'rejected'")
		return
	}

	doc, err := h.documentService.UpdateReview(c.Request.Context(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: userID,
		Role:       role,
		Status:     req.Status,
		Notes:      req.Notes,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, doc)
}

// Validate handles POST /api/v1/documents/:id/validate
func (h *DocumentHandler) Validate(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid document ID")
		return
	}

	if err := h.documentService.ValidateDocument(c.Request.Context(), tenantID, docID); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "validation completed"})
}

// GetValidation handles GET /api/v1/documents/:id/validation
func (h *DocumentHandler) GetValidation(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing tenant context")
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid document ID")
		return
	}

	result, err := h.documentService.GetValidation(c.Request.Context(), tenantID, docID)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, result)
}

// Delete handles DELETE /api/v1/documents/:id
func (h *DocumentHandler) Delete(c *gin.Context) {
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
	role := domain.UserRole(middleware.GetRole(c))

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid document ID")
		return
	}

	if err := h.documentService.Delete(c.Request.Context(), tenantID, docID, userID, role); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "document deleted"})
}
