package handler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/handler"
	"satvos/mocks"
)

func newCollectionHandler() (*handler.CollectionHandler, *mocks.MockCollectionService) {
	mockSvc := new(mocks.MockCollectionService)
	h := handler.NewCollectionHandler(mockSvc)
	return h, mockSvc
}

// --- Create ---

func TestCollectionHandler_Create_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	expected := &domain.Collection{
		ID:            collectionID,
		TenantID:      tenantID,
		Name:          "Test Collection",
		DocumentCount: 0,
	}

	mockSvc.On("Create", mock.Anything, mock.AnythingOfType("*service.CreateCollectionInput")).
		Return(expected, nil)

	body, _ := json.Marshal(map[string]string{
		"name":        "Test Collection",
		"description": "A test",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/collections", bytes.NewReader(body))
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

func TestCollectionHandler_Create_MissingName(t *testing.T) {
	h, _ := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	body, _ := json.Marshal(map[string]string{
		"description": "No name provided",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/collections", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthContext(c, tenantID, userID, "member")

	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectionHandler_Create_NoAuth(t *testing.T) {
	h, _ := newCollectionHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/collections", http.NoBody)

	h.Create(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- List ---

func TestCollectionHandler_List_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	collections := []domain.Collection{
		{ID: uuid.New(), TenantID: tenantID, Name: "Collection 1", DocumentCount: 5},
	}

	mockSvc.On("List", mock.Anything, tenantID, userID, domain.UserRole("member"), 0, 20).
		Return(collections, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/collections?offset=0&limit=20", http.NoBody)
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

// --- GetByID ---

func TestCollectionHandler_GetByID_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	expected := &domain.Collection{ID: collectionID, TenantID: tenantID, Name: "Test", DocumentCount: 3}
	files := []domain.FileMeta{
		{ID: uuid.New(), TenantID: tenantID, Status: domain.FileStatusUploaded},
	}

	mockSvc.On("GetByID", mock.Anything, tenantID, collectionID, userID, domain.UserRole("member")).Return(expected, nil)
	mockSvc.On("ListFiles", mock.Anything, tenantID, collectionID, userID, domain.UserRole("member"), 0, 20).
		Return(files, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/collections/"+collectionID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.GetByID(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestCollectionHandler_GetByID_InvalidID(t *testing.T) {
	h, _ := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/collections/not-a-uuid", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	setAuthContext(c, tenantID, userID, "member")

	h.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectionHandler_GetByID_NotFound(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	mockSvc.On("GetByID", mock.Anything, tenantID, collectionID, userID, domain.UserRole("member")).
		Return(nil, domain.ErrCollectionNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/collections/"+collectionID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.GetByID(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCollectionHandler_GetByID_PermDenied(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	mockSvc.On("GetByID", mock.Anything, tenantID, collectionID, userID, domain.UserRole("member")).
		Return(nil, domain.ErrCollectionPermDenied)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/collections/"+collectionID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.GetByID(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- Update ---

func TestCollectionHandler_Update_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	updated := &domain.Collection{ID: collectionID, TenantID: tenantID, Name: "Updated", DocumentCount: 7}

	mockSvc.On("Update", mock.Anything, mock.AnythingOfType("*service.UpdateCollectionInput")).
		Return(updated, nil)

	body, _ := json.Marshal(map[string]string{
		"name":        "Updated",
		"description": "Updated desc",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/collections/"+collectionID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.Update(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestCollectionHandler_Update_MissingName(t *testing.T) {
	h, _ := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	body, _ := json.Marshal(map[string]string{
		"description": "No name",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/collections/"+collectionID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Delete ---

func TestCollectionHandler_Delete_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	mockSvc.On("Delete", mock.Anything, tenantID, collectionID, userID, domain.UserRole("admin")).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/collections/"+collectionID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "admin")

	h.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestCollectionHandler_Delete_PermDenied(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	mockSvc.On("Delete", mock.Anything, tenantID, collectionID, userID, domain.UserRole("member")).
		Return(domain.ErrCollectionPermDenied)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/collections/"+collectionID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "member")

	h.Delete(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- RemoveFile ---

func TestCollectionHandler_RemoveFile_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()
	fileID := uuid.New()

	mockSvc.On("RemoveFile", mock.Anything, tenantID, collectionID, fileID, userID, domain.UserRole("member")).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/collections/"+collectionID.String()+"/files/"+fileID.String(), http.NoBody)
	c.Params = gin.Params{
		{Key: "id", Value: collectionID.String()},
		{Key: "fileId", Value: fileID.String()},
	}
	setAuthContext(c, tenantID, userID, "member")

	h.RemoveFile(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestCollectionHandler_RemoveFile_InvalidFileID(t *testing.T) {
	h, _ := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/collections/"+collectionID.String()+"/files/bad-id", http.NoBody)
	c.Params = gin.Params{
		{Key: "id", Value: collectionID.String()},
		{Key: "fileId", Value: "bad-id"},
	}
	setAuthContext(c, tenantID, userID, "member")

	h.RemoveFile(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- SetPermission ---

func TestCollectionHandler_SetPermission_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()
	targetUserID := uuid.New()

	mockSvc.On("SetPermission", mock.Anything, mock.AnythingOfType("*service.SetPermissionInput")).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"user_id":    targetUserID.String(),
		"permission": "editor",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/collections/"+collectionID.String()+"/permissions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "admin")

	h.SetPermission(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestCollectionHandler_SetPermission_MissingFields(t *testing.T) {
	h, _ := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	body, _ := json.Marshal(map[string]string{
		"permission": "editor",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/collections/"+collectionID.String()+"/permissions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "admin")

	h.SetPermission(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- ListPermissions ---

func TestCollectionHandler_ListPermissions_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	perms := []domain.CollectionPermissionEntry{
		{ID: uuid.New(), CollectionID: collectionID, UserID: userID, Permission: domain.CollectionPermOwner},
	}

	mockSvc.On("ListPermissions", mock.Anything, tenantID, collectionID, userID, domain.UserRole("admin"), 0, 20).
		Return(perms, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/collections/"+collectionID.String()+"/permissions?offset=0&limit=20", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: collectionID.String()}}
	setAuthContext(c, tenantID, userID, "admin")

	h.ListPermissions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Meta)
	mockSvc.AssertExpectations(t)
}

// --- RemovePermission ---

func TestCollectionHandler_RemovePermission_Success(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()
	targetUserID := uuid.New()

	mockSvc.On("RemovePermission", mock.Anything, tenantID, collectionID, targetUserID, userID, domain.UserRole("admin")).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete,
		"/api/v1/collections/"+collectionID.String()+"/permissions/"+targetUserID.String(), http.NoBody)
	c.Params = gin.Params{
		{Key: "id", Value: collectionID.String()},
		{Key: "userId", Value: targetUserID.String()},
	}
	setAuthContext(c, tenantID, userID, "admin")

	h.RemovePermission(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestCollectionHandler_RemovePermission_SelfRemoval(t *testing.T) {
	h, mockSvc := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	mockSvc.On("RemovePermission", mock.Anything, tenantID, collectionID, userID, userID, domain.UserRole("admin")).
		Return(domain.ErrSelfPermissionRemoval)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete,
		"/api/v1/collections/"+collectionID.String()+"/permissions/"+userID.String(), http.NoBody)
	c.Params = gin.Params{
		{Key: "id", Value: collectionID.String()},
		{Key: "userId", Value: userID.String()},
	}
	setAuthContext(c, tenantID, userID, "admin")

	h.RemovePermission(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollectionHandler_RemovePermission_InvalidUserID(t *testing.T) {
	h, _ := newCollectionHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete,
		"/api/v1/collections/"+collectionID.String()+"/permissions/bad-id", http.NoBody)
	c.Params = gin.Params{
		{Key: "id", Value: collectionID.String()},
		{Key: "userId", Value: "bad-id"},
	}
	setAuthContext(c, tenantID, userID, "admin")

	h.RemovePermission(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- File upload with collection_id ---

func TestFileHandler_Upload_WithCollectionID_Success(t *testing.T) {
	mockFileSvc := new(mocks.MockFileService)
	mockCollSvc := new(mocks.MockCollectionService)
	h := handler.NewFileHandler(mockFileSvc, mockCollSvc)

	tenantID := uuid.New()
	userID := uuid.New()
	fileID := uuid.New()
	collectionID := uuid.New()

	expectedMeta := &domain.FileMeta{
		ID:           fileID,
		TenantID:     tenantID,
		UploadedBy:   userID,
		FileName:     fileID.String() + ".pdf",
		OriginalName: "test.pdf",
		FileType:     domain.FileTypePDF,
		Status:       domain.FileStatusUploaded,
	}

	mockFileSvc.On("Upload", mock.Anything, mock.AnythingOfType("service.FileUploadInput")).
		Return(expectedMeta, nil)
	mockCollSvc.On("AddFileToCollection", mock.Anything, tenantID, collectionID, fileID, userID, domain.UserRole("member")).
		Return(nil)

	// Build multipart body with file + collection_id
	body := &bytes.Buffer{}
	writer := multipartWriter(body)
	part, _ := writer.CreateFormFile("file", "test.pdf")
	_, _ = part.Write([]byte("%PDF-1.4 test content"))
	_ = writer.WriteField("collection_id", collectionID.String())
	_ = writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/files/upload", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	setAuthContext(c, tenantID, userID, "member")

	h.Upload(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockFileSvc.AssertExpectations(t)
	mockCollSvc.AssertExpectations(t)
}

func TestFileHandler_Upload_WithCollectionID_CollectionFails(t *testing.T) {
	mockFileSvc := new(mocks.MockFileService)
	mockCollSvc := new(mocks.MockCollectionService)
	h := handler.NewFileHandler(mockFileSvc, mockCollSvc)

	tenantID := uuid.New()
	userID := uuid.New()
	fileID := uuid.New()
	collectionID := uuid.New()

	expectedMeta := &domain.FileMeta{
		ID:           fileID,
		TenantID:     tenantID,
		UploadedBy:   userID,
		FileName:     fileID.String() + ".pdf",
		OriginalName: "test.pdf",
		FileType:     domain.FileTypePDF,
		Status:       domain.FileStatusUploaded,
	}

	mockFileSvc.On("Upload", mock.Anything, mock.AnythingOfType("service.FileUploadInput")).
		Return(expectedMeta, nil)
	mockCollSvc.On("AddFileToCollection", mock.Anything, tenantID, collectionID, fileID, userID, domain.UserRole("member")).
		Return(domain.ErrCollectionPermDenied)

	body := &bytes.Buffer{}
	writer := multipartWriter(body)
	part, _ := writer.CreateFormFile("file", "test.pdf")
	_, _ = part.Write([]byte("%PDF-1.4 test content"))
	_ = writer.WriteField("collection_id", collectionID.String())
	_ = writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/files/upload", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	setAuthContext(c, tenantID, userID, "member")

	h.Upload(c)

	// File upload still succeeds with a warning
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	data := resp["data"].(map[string]interface{})
	assert.Contains(t, data["warning"], "failed to add to collection")
}

func multipartWriter(body *bytes.Buffer) *multipart.Writer {
	return multipart.NewWriter(body)
}
