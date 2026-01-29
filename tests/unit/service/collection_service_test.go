package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/service"
	"satvos/mocks"
)

func setupCollectionService() (
	service.CollectionService,
	*mocks.MockCollectionRepo,
	*mocks.MockCollectionPermissionRepo,
	*mocks.MockCollectionFileRepo,
	*mocks.MockFileService,
) {
	collRepo := new(mocks.MockCollectionRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	fileRepo := new(mocks.MockCollectionFileRepo)
	fileSvc := new(mocks.MockFileService)
	svc := service.NewCollectionService(collRepo, permRepo, fileRepo, fileSvc)
	return svc, collRepo, permRepo, fileRepo, fileSvc
}

func ownerPerm(collectionID, userID uuid.UUID) *domain.CollectionPermissionEntry {
	return &domain.CollectionPermissionEntry{
		ID:           uuid.New(),
		CollectionID: collectionID,
		UserID:       userID,
		Permission:   domain.CollectionPermOwner,
	}
}

func editorPerm(collectionID, userID uuid.UUID) *domain.CollectionPermissionEntry {
	return &domain.CollectionPermissionEntry{
		ID:           uuid.New(),
		CollectionID: collectionID,
		UserID:       userID,
		Permission:   domain.CollectionPermEditor,
	}
}

func viewerPerm(collectionID, userID uuid.UUID) *domain.CollectionPermissionEntry {
	return &domain.CollectionPermissionEntry{
		ID:           uuid.New(),
		CollectionID: collectionID,
		UserID:       userID,
		Permission:   domain.CollectionPermViewer,
	}
}

// --- Create ---

func TestCollectionService_Create_Success(t *testing.T) {
	svc, collRepo, permRepo, _, _ := setupCollectionService()

	tenantID := uuid.New()
	userID := uuid.New()

	collRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Collection")).Return(nil)
	permRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.CollectionPermissionEntry")).Return(nil)

	result, err := svc.Create(context.Background(), service.CreateCollectionInput{
		TenantID:    tenantID,
		CreatedBy:   userID,
		Name:        "Test Collection",
		Description: "A test collection",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Test Collection", result.Name)
	assert.Equal(t, "A test collection", result.Description)
	assert.Equal(t, tenantID, result.TenantID)
	assert.Equal(t, userID, result.CreatedBy)

	collRepo.AssertExpectations(t)
	permRepo.AssertExpectations(t)
}

func TestCollectionService_Create_RepoError(t *testing.T) {
	svc, collRepo, _, _, _ := setupCollectionService()

	collRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Collection")).
		Return(errors.New("db error"))

	result, err := svc.Create(context.Background(), service.CreateCollectionInput{
		TenantID:  uuid.New(),
		CreatedBy: uuid.New(),
		Name:      "Test",
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating collection")
}

func TestCollectionService_Create_PermissionUpsertError(t *testing.T) {
	svc, collRepo, permRepo, _, _ := setupCollectionService()

	collRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Collection")).Return(nil)
	permRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.CollectionPermissionEntry")).
		Return(errors.New("db error"))

	result, err := svc.Create(context.Background(), service.CreateCollectionInput{
		TenantID:  uuid.New(),
		CreatedBy: uuid.New(),
		Name:      "Test",
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "assigning owner permission")
}

// --- GetByID ---

func TestCollectionService_GetByID_Success(t *testing.T) {
	svc, collRepo, permRepo, _, _ := setupCollectionService()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	expected := &domain.Collection{ID: collectionID, TenantID: tenantID, Name: "Test"}

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(viewerPerm(collectionID, userID), nil)
	collRepo.On("GetByID", mock.Anything, tenantID, collectionID).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), tenantID, collectionID, userID)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestCollectionService_GetByID_NoPermission(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(nil, domain.ErrCollectionPermDenied)

	result, err := svc.GetByID(context.Background(), uuid.New(), collectionID, userID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

// --- List ---

func TestCollectionService_List_Success(t *testing.T) {
	svc, collRepo, _, _, _ := setupCollectionService()

	tenantID := uuid.New()
	userID := uuid.New()

	expected := []domain.Collection{
		{ID: uuid.New(), TenantID: tenantID, Name: "Collection 1"},
		{ID: uuid.New(), TenantID: tenantID, Name: "Collection 2"},
	}

	collRepo.On("ListByUser", mock.Anything, tenantID, userID, 0, 20).Return(expected, 2, nil)

	collections, total, err := svc.List(context.Background(), tenantID, userID, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, collections, 2)
	assert.Equal(t, 2, total)
}

// --- Update ---

func TestCollectionService_Update_Success(t *testing.T) {
	svc, collRepo, permRepo, _, _ := setupCollectionService()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	existing := &domain.Collection{ID: collectionID, TenantID: tenantID, Name: "Old Name"}

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(ownerPerm(collectionID, userID), nil)
	collRepo.On("GetByID", mock.Anything, tenantID, collectionID).Return(existing, nil)
	collRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Collection")).Return(nil)

	result, err := svc.Update(context.Background(), service.UpdateCollectionInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		UserID:       userID,
		Name:         "New Name",
		Description:  "New Desc",
	})

	assert.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)
	assert.Equal(t, "New Desc", result.Description)
}

func TestCollectionService_Update_NotOwner(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(editorPerm(collectionID, userID), nil)

	result, err := svc.Update(context.Background(), service.UpdateCollectionInput{
		TenantID:     uuid.New(),
		CollectionID: collectionID,
		UserID:       userID,
		Name:         "New Name",
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

// --- Delete ---

func TestCollectionService_Delete_Success(t *testing.T) {
	svc, collRepo, permRepo, _, _ := setupCollectionService()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(ownerPerm(collectionID, userID), nil)
	collRepo.On("Delete", mock.Anything, tenantID, collectionID).Return(nil)

	err := svc.Delete(context.Background(), tenantID, collectionID, userID)

	assert.NoError(t, err)
	collRepo.AssertExpectations(t)
}

func TestCollectionService_Delete_NotOwner(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(viewerPerm(collectionID, userID), nil)

	err := svc.Delete(context.Background(), uuid.New(), collectionID, userID)

	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

// --- ListFiles ---

func TestCollectionService_ListFiles_Success(t *testing.T) {
	svc, _, permRepo, fileRepo, _ := setupCollectionService()

	tenantID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	expected := []domain.FileMeta{
		{ID: uuid.New(), TenantID: tenantID, Status: domain.FileStatusUploaded},
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(viewerPerm(collectionID, userID), nil)
	fileRepo.On("ListByCollection", mock.Anything, tenantID, collectionID, 0, 20).
		Return(expected, 1, nil)

	files, total, err := svc.ListFiles(context.Background(), tenantID, collectionID, userID, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, 1, total)
}

func TestCollectionService_ListFiles_NoPermission(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(nil, domain.ErrCollectionPermDenied)

	files, total, err := svc.ListFiles(context.Background(), uuid.New(), collectionID, userID, 0, 20)

	assert.Nil(t, files)
	assert.Equal(t, 0, total)
	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

// --- RemoveFile ---

func TestCollectionService_RemoveFile_Success(t *testing.T) {
	svc, _, permRepo, fileRepo, _ := setupCollectionService()

	collectionID := uuid.New()
	fileID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(editorPerm(collectionID, userID), nil)
	fileRepo.On("Remove", mock.Anything, collectionID, fileID).Return(nil)

	err := svc.RemoveFile(context.Background(), uuid.New(), collectionID, fileID, userID)

	assert.NoError(t, err)
}

func TestCollectionService_RemoveFile_ViewerDenied(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(viewerPerm(collectionID, userID), nil)

	err := svc.RemoveFile(context.Background(), uuid.New(), collectionID, uuid.New(), userID)

	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

// --- AddFileToCollection ---

func TestCollectionService_AddFileToCollection_Success(t *testing.T) {
	svc, _, permRepo, fileRepo, _ := setupCollectionService()

	tenantID := uuid.New()
	collectionID := uuid.New()
	fileID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(editorPerm(collectionID, userID), nil)
	fileRepo.On("Add", mock.Anything, mock.AnythingOfType("*domain.CollectionFile")).Return(nil)

	err := svc.AddFileToCollection(context.Background(), tenantID, collectionID, fileID, userID)

	assert.NoError(t, err)
}

func TestCollectionService_AddFileToCollection_Duplicate(t *testing.T) {
	svc, _, permRepo, fileRepo, _ := setupCollectionService()

	collectionID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(editorPerm(collectionID, userID), nil)
	fileRepo.On("Add", mock.Anything, mock.AnythingOfType("*domain.CollectionFile")).
		Return(domain.ErrDuplicateCollectionFile)

	err := svc.AddFileToCollection(context.Background(), uuid.New(), collectionID, uuid.New(), userID)

	assert.ErrorIs(t, err, domain.ErrDuplicateCollectionFile)
}

// --- SetPermission ---

func TestCollectionService_SetPermission_Success(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	tenantID := uuid.New()
	collectionID := uuid.New()
	ownerID := uuid.New()
	targetID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, ownerID).
		Return(ownerPerm(collectionID, ownerID), nil)
	permRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.CollectionPermissionEntry")).Return(nil)

	err := svc.SetPermission(context.Background(), service.SetPermissionInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		GrantedBy:    ownerID,
		UserID:       targetID,
		Permission:   domain.CollectionPermEditor,
	})

	assert.NoError(t, err)
	permRepo.AssertExpectations(t)
}

func TestCollectionService_SetPermission_InvalidPermission(t *testing.T) {
	svc, _, _, _, _ := setupCollectionService()

	err := svc.SetPermission(context.Background(), service.SetPermissionInput{
		TenantID:     uuid.New(),
		CollectionID: uuid.New(),
		GrantedBy:    uuid.New(),
		UserID:       uuid.New(),
		Permission:   "superadmin",
	})

	assert.ErrorIs(t, err, domain.ErrInvalidPermission)
}

func TestCollectionService_SetPermission_NotOwner(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	editorID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, editorID).
		Return(editorPerm(collectionID, editorID), nil)

	err := svc.SetPermission(context.Background(), service.SetPermissionInput{
		TenantID:     uuid.New(),
		CollectionID: collectionID,
		GrantedBy:    editorID,
		UserID:       uuid.New(),
		Permission:   domain.CollectionPermViewer,
	})

	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

// --- ListPermissions ---

func TestCollectionService_ListPermissions_Success(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	ownerID := uuid.New()

	expected := []domain.CollectionPermissionEntry{
		{ID: uuid.New(), CollectionID: collectionID, UserID: ownerID, Permission: domain.CollectionPermOwner},
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, ownerID).
		Return(ownerPerm(collectionID, ownerID), nil)
	permRepo.On("ListByCollection", mock.Anything, collectionID, 0, 20).
		Return(expected, 1, nil)

	perms, total, err := svc.ListPermissions(context.Background(), uuid.New(), collectionID, ownerID, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, perms, 1)
	assert.Equal(t, 1, total)
}

func TestCollectionService_ListPermissions_NotOwner(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	viewerID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, viewerID).
		Return(viewerPerm(collectionID, viewerID), nil)

	perms, total, err := svc.ListPermissions(context.Background(), uuid.New(), collectionID, viewerID, 0, 20)

	assert.Nil(t, perms)
	assert.Equal(t, 0, total)
	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

// --- RemovePermission ---

func TestCollectionService_RemovePermission_Success(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	ownerID := uuid.New()
	targetID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, ownerID).
		Return(ownerPerm(collectionID, ownerID), nil)
	permRepo.On("Delete", mock.Anything, collectionID, targetID).Return(nil)

	err := svc.RemovePermission(context.Background(), uuid.New(), collectionID, targetID, ownerID)

	assert.NoError(t, err)
	permRepo.AssertExpectations(t)
}

func TestCollectionService_RemovePermission_SelfRemoval(t *testing.T) {
	svc, _, _, _, _ := setupCollectionService()

	userID := uuid.New()

	err := svc.RemovePermission(context.Background(), uuid.New(), uuid.New(), userID, userID)

	assert.ErrorIs(t, err, domain.ErrSelfPermissionRemoval)
}

func TestCollectionService_RemovePermission_NotOwner(t *testing.T) {
	svc, _, permRepo, _, _ := setupCollectionService()

	collectionID := uuid.New()
	editorID := uuid.New()
	targetID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, editorID).
		Return(editorPerm(collectionID, editorID), nil)

	err := svc.RemovePermission(context.Background(), uuid.New(), collectionID, targetID, editorID)

	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}
