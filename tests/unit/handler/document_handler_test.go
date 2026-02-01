package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/handler"
	"satvos/internal/service"
	"satvos/mocks"
)

func newDocumentHandler() (*handler.DocumentHandler, *mocks.MockDocumentService) {
	mockSvc := new(mocks.MockDocumentService)
	h := handler.NewDocumentHandler(mockSvc)
	return h, mockSvc
}

// --- Create ---

func TestDocumentHandler_Create_Success(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	fileID := uuid.New()
	collectionID := uuid.New()
	docID := uuid.New()

	expected := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		CollectionID:  collectionID,
		FileID:        fileID,
		DocumentType:  "invoice",
		ParsingStatus: domain.ParsingStatusPending,
		ReviewStatus:  domain.ReviewStatusPending,
	}

	mockSvc.On("CreateAndParse", mock.Anything, mock.MatchedBy(func(input *service.CreateDocumentInput) bool {
		return input.TenantID == tenantID &&
			input.FileID == fileID &&
			input.CollectionID == collectionID &&
			input.DocumentType == "invoice" &&
			input.CreatedBy == userID
	})).Return(expected, nil)

	body, _ := json.Marshal(map[string]string{
		"file_id":       fileID.String(),
		"collection_id": collectionID.String(),
		"document_type": "invoice",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthContext(c, tenantID, userID, "member")

	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	mockSvc.AssertExpectations(t)
}

func TestDocumentHandler_Create_MissingFields(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	// Missing document_type
	body, _ := json.Marshal(map[string]string{
		"file_id":       uuid.New().String(),
		"collection_id": uuid.New().String(),
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthContext(c, tenantID, userID, "member")

	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_Create_NoAuth(t *testing.T) {
	h, _ := newDocumentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents", http.NoBody)

	h.Create(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDocumentHandler_Create_DuplicateDocument(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	mockSvc.On("CreateAndParse", mock.Anything, mock.AnythingOfType("*service.CreateDocumentInput")).
		Return(nil, domain.ErrDocumentAlreadyExists)

	body, _ := json.Marshal(map[string]string{
		"file_id":       uuid.New().String(),
		"collection_id": uuid.New().String(),
		"document_type": "invoice",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthContext(c, tenantID, userID, "member")

	h.Create(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDocumentHandler_Create_FileNotFound(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	mockSvc.On("CreateAndParse", mock.Anything, mock.AnythingOfType("*service.CreateDocumentInput")).
		Return(nil, domain.ErrNotFound)

	body, _ := json.Marshal(map[string]string{
		"file_id":       uuid.New().String(),
		"collection_id": uuid.New().String(),
		"document_type": "invoice",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthContext(c, tenantID, userID, "member")

	h.Create(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- GetByID ---

func TestDocumentHandler_GetByID_Success(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	expected := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	mockSvc.On("GetByID", mock.Anything, tenantID, docID).Return(expected, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/documents/"+docID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.GetByID(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	mockSvc.AssertExpectations(t)
}

func TestDocumentHandler_GetByID_NotFound(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	mockSvc.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/documents/"+docID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.GetByID(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDocumentHandler_GetByID_InvalidID(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/documents/not-a-uuid", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	setAuthContext(c, tenantID, userID, "member")

	h.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- List ---

func TestDocumentHandler_List_ByTenant(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	docs := []domain.Document{
		{ID: uuid.New(), TenantID: tenantID, ParsingStatus: domain.ParsingStatusCompleted},
	}

	mockSvc.On("ListByTenant", mock.Anything, tenantID, 0, 20).Return(docs, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/documents?offset=0&limit=20", http.NoBody)
	setAuthContext(c, tenantID, userID, "member")

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Meta)
	mockSvc.AssertExpectations(t)
}

func TestDocumentHandler_List_ByCollection(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	docs := []domain.Document{
		{ID: uuid.New(), TenantID: tenantID, CollectionID: collectionID},
	}

	mockSvc.On("ListByCollection", mock.Anything, tenantID, collectionID, 0, 20).
		Return(docs, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet,
		"/api/v1/documents?collection_id="+collectionID.String()+"&offset=0&limit=20", http.NoBody)
	setAuthContext(c, tenantID, userID, "member")

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Meta)
	mockSvc.AssertExpectations(t)
}

func TestDocumentHandler_List_InvalidCollectionID(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet,
		"/api/v1/documents?collection_id=not-a-uuid", http.NoBody)
	setAuthContext(c, tenantID, userID, "member")

	h.List(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_List_NoAuth(t *testing.T) {
	h, _ := newDocumentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/documents", http.NoBody)

	h.List(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Retry ---

func TestDocumentHandler_Retry_Success(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	expected := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusPending,
	}

	mockSvc.On("RetryParse", mock.Anything, tenantID, docID).Return(expected, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents/"+docID.String()+"/retry", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.Retry(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestDocumentHandler_Retry_NotFound(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	mockSvc.On("RetryParse", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents/"+docID.String()+"/retry", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.Retry(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDocumentHandler_Retry_InvalidID(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/documents/bad-id/retry", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}
	setAuthContext(c, tenantID, userID, "member")

	h.Retry(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- UpdateReview ---

func TestDocumentHandler_UpdateReview_Approved(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	expected := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ReviewStatus:  domain.ReviewStatusApproved,
		ReviewerNotes: "Verified",
	}

	mockSvc.On("UpdateReview", mock.Anything, mock.MatchedBy(func(input *service.UpdateReviewInput) bool {
		return input.TenantID == tenantID &&
			input.DocumentID == docID &&
			input.ReviewerID == userID &&
			input.Status == domain.ReviewStatusApproved &&
			input.Notes == "Verified"
	})).Return(expected, nil)

	body, _ := json.Marshal(map[string]string{
		"status": "approved",
		"notes":  "Verified",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/documents/"+docID.String()+"/review", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.UpdateReview(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestDocumentHandler_UpdateReview_Rejected(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	expected := &domain.Document{
		ID:           docID,
		ReviewStatus: domain.ReviewStatusRejected,
	}

	mockSvc.On("UpdateReview", mock.Anything, mock.AnythingOfType("*service.UpdateReviewInput")).
		Return(expected, nil)

	body, _ := json.Marshal(map[string]string{
		"status": "rejected",
		"notes":  "Wrong amounts",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/documents/"+docID.String()+"/review", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.UpdateReview(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDocumentHandler_UpdateReview_InvalidStatus(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	body, _ := json.Marshal(map[string]string{
		"status": "maybe",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/documents/"+docID.String()+"/review", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.UpdateReview(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_UpdateReview_MissingStatus(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	body, _ := json.Marshal(map[string]string{
		"notes": "No status provided",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/documents/"+docID.String()+"/review", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.UpdateReview(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_UpdateReview_NotParsed(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	mockSvc.On("UpdateReview", mock.Anything, mock.AnythingOfType("*service.UpdateReviewInput")).
		Return(nil, domain.ErrDocumentNotParsed)

	body, _ := json.Marshal(map[string]string{
		"status": "approved",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/documents/"+docID.String()+"/review", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.UpdateReview(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_UpdateReview_InvalidDocID(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/documents/bad-id/review", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}
	setAuthContext(c, tenantID, userID, "member")

	h.UpdateReview(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_UpdateReview_NoAuth(t *testing.T) {
	h, _ := newDocumentHandler()

	docID := uuid.New()

	body, _ := json.Marshal(map[string]string{"status": "approved"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/documents/"+docID.String()+"/review", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}

	h.UpdateReview(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Delete ---

func TestDocumentHandler_Delete_Success(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	mockSvc.On("Delete", mock.Anything, tenantID, docID).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/documents/"+docID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "admin")

	h.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestDocumentHandler_Delete_NotFound(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	mockSvc.On("Delete", mock.Anything, tenantID, docID).Return(domain.ErrDocumentNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/documents/"+docID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	setAuthContext(c, tenantID, userID, "admin")

	h.Delete(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDocumentHandler_Delete_InvalidID(t *testing.T) {
	h, _ := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/documents/bad-id", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}
	setAuthContext(c, tenantID, userID, "admin")

	h.Delete(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_Delete_NoAuth(t *testing.T) {
	h, _ := newDocumentHandler()

	docID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/documents/"+docID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}

	h.Delete(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
