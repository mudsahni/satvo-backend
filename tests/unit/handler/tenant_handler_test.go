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

func newTenantHandler() (*handler.TenantHandler, *mocks.MockTenantService) {
	mockSvc := new(mocks.MockTenantService)
	h := handler.NewTenantHandler(mockSvc)
	return h, mockSvc
}

// --- Create ---

func TestTenantHandler_Create_Success(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenantID := uuid.New()
	expected := &domain.Tenant{
		ID:       tenantID,
		Name:     "Acme Corp",
		Slug:     "acme",
		IsActive: true,
	}

	mockSvc.On("Create", mock.Anything, mock.MatchedBy(func(input service.CreateTenantInput) bool {
		return input.Name == "Acme Corp" && input.Slug == "acme"
	})).Return(expected, nil)

	body, _ := json.Marshal(map[string]string{
		"name": "Acme Corp",
		"slug": "acme",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/admin/tenants", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	mockSvc.AssertExpectations(t)
}

func TestTenantHandler_Create_MissingFields(t *testing.T) {
	h, _ := newTenantHandler()

	body, _ := json.Marshal(map[string]string{
		"name": "Acme Corp",
		// missing slug
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/admin/tenants", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantHandler_Create_DuplicateSlug(t *testing.T) {
	h, mockSvc := newTenantHandler()

	mockSvc.On("Create", mock.Anything, mock.AnythingOfType("service.CreateTenantInput")).
		Return(nil, domain.ErrDuplicateTenantSlug)

	body, _ := json.Marshal(map[string]string{
		"name": "Acme Corp",
		"slug": "acme",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/admin/tenants", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- List ---

func TestTenantHandler_List_Success(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenants := []domain.Tenant{
		{ID: uuid.New(), Name: "Acme", Slug: "acme", IsActive: true},
		{ID: uuid.New(), Name: "Beta", Slug: "beta", IsActive: true},
	}

	mockSvc.On("List", mock.Anything, 0, 20).Return(tenants, 2, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/admin/tenants?offset=0&limit=20", http.NoBody)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Meta)
	assert.Equal(t, 2, resp.Meta.Total)
	mockSvc.AssertExpectations(t)
}

// --- GetByID ---

func TestTenantHandler_GetByID_Success(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenantID := uuid.New()
	expected := &domain.Tenant{
		ID:       tenantID,
		Name:     "Acme",
		Slug:     "acme",
		IsActive: true,
	}

	mockSvc.On("GetByID", mock.Anything, tenantID).Return(expected, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/admin/tenants/"+tenantID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: tenantID.String()}}

	h.GetByID(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestTenantHandler_GetByID_NotFound(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenantID := uuid.New()
	mockSvc.On("GetByID", mock.Anything, tenantID).Return(nil, domain.ErrNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/admin/tenants/"+tenantID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: tenantID.String()}}

	h.GetByID(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTenantHandler_GetByID_InvalidID(t *testing.T) {
	h, _ := newTenantHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/admin/tenants/not-a-uuid", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	h.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Update ---

func TestTenantHandler_Update_Success(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenantID := uuid.New()
	updated := &domain.Tenant{
		ID:       tenantID,
		Name:     "Acme Industries",
		Slug:     "acme",
		IsActive: true,
	}

	mockSvc.On("Update", mock.Anything, tenantID, mock.AnythingOfType("service.UpdateTenantInput")).
		Return(updated, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":      "Acme Industries",
		"is_active": true,
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/admin/tenants/"+tenantID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: tenantID.String()}}

	h.Update(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestTenantHandler_Update_InvalidID(t *testing.T) {
	h, _ := newTenantHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/admin/tenants/bad-id", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}

	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantHandler_Update_NotFound(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenantID := uuid.New()
	mockSvc.On("Update", mock.Anything, tenantID, mock.AnythingOfType("service.UpdateTenantInput")).
		Return(nil, domain.ErrNotFound)

	body, _ := json.Marshal(map[string]string{"name": "New Name"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/admin/tenants/"+tenantID.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: tenantID.String()}}

	h.Update(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Delete ---

func TestTenantHandler_Delete_Success(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenantID := uuid.New()
	mockSvc.On("Delete", mock.Anything, tenantID).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/admin/tenants/"+tenantID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: tenantID.String()}}

	h.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestTenantHandler_Delete_NotFound(t *testing.T) {
	h, mockSvc := newTenantHandler()

	tenantID := uuid.New()
	mockSvc.On("Delete", mock.Anything, tenantID).Return(domain.ErrNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/admin/tenants/"+tenantID.String(), http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: tenantID.String()}}

	h.Delete(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTenantHandler_Delete_InvalidID(t *testing.T) {
	h, _ := newTenantHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/admin/tenants/bad-id", http.NoBody)
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}

	h.Delete(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
