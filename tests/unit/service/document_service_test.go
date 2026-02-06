package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/domain"
	"satvos/internal/port"
	"satvos/internal/service"
	"satvos/mocks"
)

func setupDocumentService() ( //nolint:gocritic // test helper benefits from multiple named returns
	service.DocumentService,
	*mocks.MockDocumentRepo,
	*mocks.MockFileMetaRepo,
	*mocks.MockCollectionPermissionRepo,
	*mocks.MockDocumentParser,
	*mocks.MockObjectStorage,
	*mocks.MockDocumentTagRepo,
) {
	docRepo := new(mocks.MockDocumentRepo)
	fileRepo := new(mocks.MockFileMetaRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	tagRepo := new(mocks.MockDocumentTagRepo)
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	svc := service.NewDocumentService(docRepo, fileRepo, permRepo, tagRepo, parser, storage, nil)
	return svc, docRepo, fileRepo, permRepo, parser, storage, tagRepo
}

// --- CreateAndParse ---

func TestDocumentService_CreateAndParse_Success(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, parser, storage, _ := setupDocumentService()

	tenantID := uuid.New()
	userID := uuid.New()
	fileID := uuid.New()
	collectionID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:          fileID,
		TenantID:    tenantID,
		S3Bucket:    "test-bucket",
		S3Key:       "tenants/test/files/test.pdf",
		ContentType: "application/pdf",
		Status:      domain.FileStatusUploaded,
	}

	// Admin bypasses collection permission check — permRepo returns error but admin has implicit owner
	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)
	// Background goroutine calls - we need to allow these
	docRepo.On("GetByID", mock.Anything, mock.Anything, mock.Anything).Return(&domain.Document{
		ID:               uuid.New(),
		TenantID:         tenantID,
		ParsingStatus:    domain.ParsingStatusPending,
		StructuredData:   json.RawMessage("{}"),
		ConfidenceScores: json.RawMessage("{}"),
	}, nil).Maybe()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil).Maybe()
	storage.On("Download", mock.Anything, "test-bucket", "tenants/test/files/test.pdf").
		Return([]byte("%PDF-1.4 test content"), nil).Maybe()
	parser.On("Parse", mock.Anything, mock.Anything).Return(&port.ParseOutput{
		StructuredData:   json.RawMessage(`{"invoice":{}}`),
		ConfidenceScores: json.RawMessage(`{"invoice":{}}`),
		ModelUsed:        "test-model",
		PromptUsed:       "test prompt",
	}, nil).Maybe()

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    userID,
		Role:         domain.RoleAdmin,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, tenantID, result.TenantID)
	assert.Equal(t, collectionID, result.CollectionID)
	assert.Equal(t, fileID, result.FileID)
	assert.Equal(t, "invoice", result.DocumentType)
	assert.Equal(t, domain.ParsingStatusPending, result.ParsingStatus)
	assert.Equal(t, domain.ReviewStatusPending, result.ReviewStatus)
	assert.Equal(t, userID, result.CreatedBy)
	assert.Equal(t, 0, result.ParseAttempts)

	// Wait briefly for goroutine to start (not for completion)
	time.Sleep(50 * time.Millisecond)

	fileRepo.AssertExpectations(t)
}

func TestDocumentService_CreateAndParse_FileNotFound(t *testing.T) {
	svc, _, fileRepo, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(nil, domain.ErrNotFound)

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
		Role:         domain.RoleAdmin,
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "looking up file")
}

func TestDocumentService_CreateAndParse_DuplicateDocument(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:       fileID,
		TenantID: tenantID,
		S3Bucket: "test-bucket",
		S3Key:    "test-key",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).
		Return(domain.ErrDocumentAlreadyExists)

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
		Role:         domain.RoleAdmin,
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating document")
}

func TestDocumentService_CreateAndParse_CreateRepoError(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:       fileID,
		TenantID: tenantID,
		S3Bucket: "test-bucket",
		S3Key:    "test-key",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).
		Return(errors.New("db connection error"))

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
		Role:         domain.RoleAdmin,
	})

	assert.Nil(t, result)
	assert.Error(t, err)
}

// --- GetByID ---

func TestDocumentService_GetByID_Success(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	expected := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestDocumentService_GetByID_NotFound(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.GetByID(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

// --- GetByFileID ---

func TestDocumentService_GetByFileID_Success(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()
	userID := uuid.New()

	expected := &domain.Document{
		ID:       uuid.New(),
		TenantID: tenantID,
		FileID:   fileID,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByFileID", mock.Anything, tenantID, fileID).Return(expected, nil)

	result, err := svc.GetByFileID(context.Background(), tenantID, fileID, userID, domain.RoleAdmin)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestDocumentService_GetByFileID_NotFound(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByFileID", mock.Anything, tenantID, fileID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.GetByFileID(context.Background(), tenantID, fileID, userID, domain.RoleAdmin)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

// --- ListByCollection ---

func TestDocumentService_ListByCollection_Success(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	collectionID := uuid.New()
	userID := uuid.New()

	expected := []domain.Document{
		{ID: uuid.New(), TenantID: tenantID, CollectionID: collectionID},
		{ID: uuid.New(), TenantID: tenantID, CollectionID: collectionID},
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("ListByCollection", mock.Anything, tenantID, collectionID, 0, 20).
		Return(expected, 2, nil)

	docs, total, err := svc.ListByCollection(context.Background(), tenantID, collectionID, userID, domain.RoleAdmin, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, docs, 2)
	assert.Equal(t, 2, total)
}

func TestDocumentService_ListByCollection_Empty(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	collectionID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("ListByCollection", mock.Anything, tenantID, collectionID, 0, 20).
		Return([]domain.Document{}, 0, nil)

	docs, total, err := svc.ListByCollection(context.Background(), tenantID, collectionID, userID, domain.RoleAdmin, 0, 20)

	assert.NoError(t, err)
	assert.Empty(t, docs)
	assert.Equal(t, 0, total)
}

// --- ListByTenant ---

func TestDocumentService_ListByTenant_Success(t *testing.T) {
	svc, docRepo, _, _, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	userID := uuid.New()

	expected := []domain.Document{
		{ID: uuid.New(), TenantID: tenantID},
	}

	docRepo.On("ListByTenant", mock.Anything, tenantID, 0, 20).
		Return(expected, 1, nil)

	docs, total, err := svc.ListByTenant(context.Background(), tenantID, userID, domain.RoleAdmin, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, docs, 1)
	assert.Equal(t, 1, total)
}

// --- UpdateReview ---

func TestDocumentService_UpdateReview_Approved(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	reviewerID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
		ReviewStatus:  domain.ReviewStatusPending,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: reviewerID,
		Role:       domain.RoleAdmin,
		Status:     domain.ReviewStatusApproved,
		Notes:      "Looks good",
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.ReviewStatusApproved, result.ReviewStatus)
	assert.Equal(t, &reviewerID, result.ReviewedBy)
	assert.NotNil(t, result.ReviewedAt)
	assert.Equal(t, "Looks good", result.ReviewerNotes)
}

func TestDocumentService_UpdateReview_Rejected(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	reviewerID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
		ReviewStatus:  domain.ReviewStatusPending,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: reviewerID,
		Role:       domain.RoleAdmin,
		Status:     domain.ReviewStatusRejected,
		Notes:      "Incorrect amounts",
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.ReviewStatusRejected, result.ReviewStatus)
	assert.Equal(t, "Incorrect amounts", result.ReviewerNotes)
}

func TestDocumentService_UpdateReview_NotParsedYet(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusProcessing,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Role:       domain.RoleAdmin,
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_UpdateReview_PendingStatus(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusPending,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Role:       domain.RoleAdmin,
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_UpdateReview_FailedStatus(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusFailed,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Role:       domain.RoleAdmin,
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_UpdateReview_DocNotFound(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Role:       domain.RoleAdmin,
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

func TestDocumentService_UpdateReview_RepoError(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).
		Return(errors.New("db error"))

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Role:       domain.RoleAdmin,
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating review status")
}

// --- RetryParse ---

func TestDocumentService_RetryParse_Success(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, parser, storage, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	fileID := uuid.New()
	userID := uuid.New()

	existing := &domain.Document{
		ID:               docID,
		TenantID:         tenantID,
		FileID:           fileID,
		DocumentType:     "invoice",
		ParsingStatus:    domain.ParsingStatusFailed,
		ParsingError:     "previous error",
		ParseAttempts:    2,
		StructuredData:   json.RawMessage("{}"),
		ConfidenceScores: json.RawMessage("{}"),
	}

	fileMeta := &domain.FileMeta{
		ID:          fileID,
		TenantID:    tenantID,
		S3Bucket:    "test-bucket",
		S3Key:       "test-key",
		ContentType: "application/pdf",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, docID, "auto").Return(nil)
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil).Maybe()
	storage.On("Download", mock.Anything, "test-bucket", "test-key").
		Return([]byte("%PDF-1.4 test content"), nil).Maybe()
	parser.On("Parse", mock.Anything, mock.Anything).Return(&port.ParseOutput{
		StructuredData:   json.RawMessage(`{"invoice":{}}`),
		ConfidenceScores: json.RawMessage(`{"invoice":{}}`),
		ModelUsed:        "test-model",
		PromptUsed:       "test prompt",
	}, nil).Maybe()

	result, err := svc.RetryParse(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.ParsingStatusPending, result.ParsingStatus)
	assert.Empty(t, result.ParsingError)
	assert.Equal(t, 2, result.ParseAttempts) // Not reset during retry

	// Wait briefly for goroutine to start
	time.Sleep(50 * time.Millisecond)
}

func TestDocumentService_RetryParse_DocNotFound(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.RetryParse(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

func TestDocumentService_RetryParse_FileNotFound(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, _, _, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	fileID := uuid.New()
	userID := uuid.New()

	existing := &domain.Document{
		ID:       docID,
		TenantID: tenantID,
		FileID:   fileID,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, docID, "auto").Return(nil)
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(nil, domain.ErrNotFound)

	result, err := svc.RetryParse(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "looking up file for retry")
}

// --- Delete ---

func TestDocumentService_Delete_Success(t *testing.T) {
	svc, docRepo, _, _, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	docRepo.On("Delete", mock.Anything, tenantID, docID).Return(nil)

	err := svc.Delete(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.NoError(t, err)
	docRepo.AssertExpectations(t)
}

func TestDocumentService_Delete_NotFound(t *testing.T) {
	svc, docRepo, _, _, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	docRepo.On("Delete", mock.Anything, tenantID, docID).Return(domain.ErrDocumentNotFound)

	err := svc.Delete(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

// --- Background parsing integration test ---

func TestDocumentService_BackgroundParsing_Success(t *testing.T) {
	docRepo := new(mocks.MockDocumentRepo)
	fileRepo := new(mocks.MockFileMetaRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	tagRepo := new(mocks.MockDocumentTagRepo)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, mock.Anything, "auto").Return(nil).Maybe()
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	svc := service.NewDocumentService(docRepo, fileRepo, permRepo, tagRepo, parser, storage, nil)

	tenantID := uuid.New()
	fileID := uuid.New()
	collectionID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:          fileID,
		TenantID:    tenantID,
		S3Bucket:    "test-bucket",
		S3Key:       "tenants/test/files/test.pdf",
		ContentType: "application/pdf",
		Status:      domain.FileStatusUploaded,
	}

	parsedData := json.RawMessage(`{"invoice":{"invoice_number":"INV-001"}}`)
	confidenceScores := json.RawMessage(`{"invoice":{"invoice_number":0.95}}`)

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	// Background goroutine expectations
	docRepo.On("GetByID", mock.Anything, tenantID, mock.AnythingOfType("uuid.UUID")).
		Return(&domain.Document{
			ID:               uuid.New(),
			TenantID:         tenantID,
			ParsingStatus:    domain.ParsingStatusPending,
			StructuredData:   json.RawMessage("{}"),
			ConfidenceScores: json.RawMessage("{}"),
		}, nil)

	storage.On("Download", mock.Anything, "test-bucket", "tenants/test/files/test.pdf").
		Return([]byte("%PDF-1.4 test content"), nil)

	parser.On("Parse", mock.Anything, mock.MatchedBy(func(input port.ParseInput) bool {
		return input.ContentType == "application/pdf" && input.DocumentType == "invoice"
	})).Return(&port.ParseOutput{
		StructuredData:   parsedData,
		ConfidenceScores: confidenceScores,
		ModelUsed:        "claude-sonnet-4-20250514",
		PromptUsed:       "test prompt",
	}, nil)

	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
		Role:         domain.RoleAdmin,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Wait for background goroutine to complete
	time.Sleep(200 * time.Millisecond)

	storage.AssertExpectations(t)
	parser.AssertExpectations(t)
}

func TestDocumentService_BackgroundParsing_DownloadFailure(t *testing.T) {
	docRepo := new(mocks.MockDocumentRepo)
	fileRepo := new(mocks.MockFileMetaRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	tagRepo := new(mocks.MockDocumentTagRepo)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, mock.Anything, "auto").Return(nil).Maybe()
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	svc := service.NewDocumentService(docRepo, fileRepo, permRepo, tagRepo, parser, storage, nil)

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:          fileID,
		TenantID:    tenantID,
		S3Bucket:    "test-bucket",
		S3Key:       "test-key",
		ContentType: "application/pdf",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	docRepo.On("GetByID", mock.Anything, tenantID, mock.AnythingOfType("uuid.UUID")).
		Return(&domain.Document{
			ID:               uuid.New(),
			TenantID:         tenantID,
			ParsingStatus:    domain.ParsingStatusPending,
			StructuredData:   json.RawMessage("{}"),
			ConfidenceScores: json.RawMessage("{}"),
		}, nil)

	// Set processing status
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	storage.On("Download", mock.Anything, "test-bucket", "test-key").
		Return(nil, errors.New("s3 connection refused"))

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
		Role:         domain.RoleAdmin,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Wait for background goroutine to fail
	time.Sleep(200 * time.Millisecond)

	storage.AssertExpectations(t)
}

func TestDocumentService_BackgroundParsing_ParserFailure(t *testing.T) {
	docRepo := new(mocks.MockDocumentRepo)
	fileRepo := new(mocks.MockFileMetaRepo)
	permRepo := new(mocks.MockCollectionPermissionRepo)
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	tagRepo := new(mocks.MockDocumentTagRepo)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, mock.Anything, "auto").Return(nil).Maybe()
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	svc := service.NewDocumentService(docRepo, fileRepo, permRepo, tagRepo, parser, storage, nil)

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:          fileID,
		TenantID:    tenantID,
		S3Bucket:    "test-bucket",
		S3Key:       "test-key",
		ContentType: "application/pdf",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	docRepo.On("GetByID", mock.Anything, tenantID, mock.AnythingOfType("uuid.UUID")).
		Return(&domain.Document{
			ID:               uuid.New(),
			TenantID:         tenantID,
			ParsingStatus:    domain.ParsingStatusPending,
			StructuredData:   json.RawMessage("{}"),
			ConfidenceScores: json.RawMessage("{}"),
		}, nil)

	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	storage.On("Download", mock.Anything, "test-bucket", "test-key").
		Return([]byte("%PDF-1.4 content"), nil)

	parser.On("Parse", mock.Anything, mock.Anything).
		Return(nil, errors.New("API rate limit exceeded"))

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
		Role:         domain.RoleAdmin,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Wait for background goroutine to fail
	time.Sleep(200 * time.Millisecond)

	parser.AssertExpectations(t)
}

// --- CreateAndParse with name and tags ---

func TestDocumentService_CreateAndParse_WithNameAndTags(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, parser, storage, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	userID := uuid.New()
	fileID := uuid.New()
	collectionID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:           fileID,
		TenantID:     tenantID,
		OriginalName: "invoice.pdf",
		S3Bucket:     "test-bucket",
		S3Key:        "test-key",
		ContentType:  "application/pdf",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.MatchedBy(func(doc *domain.Document) bool {
		return doc.Name == "My Custom Name"
	})).Return(nil)
	tagRepo.On("CreateBatch", mock.Anything, mock.MatchedBy(func(tags []domain.DocumentTag) bool {
		return len(tags) == 1 && tags[0].Source == "user"
	})).Return(nil)
	// Background goroutine expectations
	docRepo.On("GetByID", mock.Anything, mock.Anything, mock.Anything).Return(&domain.Document{
		ID: uuid.New(), TenantID: tenantID,
		ParsingStatus: domain.ParsingStatusPending, StructuredData: json.RawMessage("{}"),
		ConfidenceScores: json.RawMessage("{}"),
	}, nil).Maybe()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil).Maybe()
	storage.On("Download", mock.Anything, mock.Anything, mock.Anything).Return([]byte("test"), nil).Maybe()
	parser.On("Parse", mock.Anything, mock.Anything).Return(&port.ParseOutput{
		StructuredData: json.RawMessage("{}"), ConfidenceScores: json.RawMessage("{}"),
		ModelUsed: "m", PromptUsed: "p",
	}, nil).Maybe()
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, mock.Anything, "auto").Return(nil).Maybe()

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		FileID:       fileID,
		DocumentType: "invoice",
		Name:         "My Custom Name",
		Tags:         map[string]string{"vendor": "Acme"},
		CreatedBy:    userID,
		Role:         domain.RoleAdmin,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "My Custom Name", result.Name)

	time.Sleep(50 * time.Millisecond)
	tagRepo.AssertExpectations(t)
}

func TestDocumentService_CreateAndParse_DefaultName(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, parser, storage, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:           fileID,
		TenantID:     tenantID,
		OriginalName: "invoice_2025.pdf",
		S3Bucket:     "test-bucket",
		S3Key:        "test-key",
		ContentType:  "application/pdf",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()
	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.MatchedBy(func(doc *domain.Document) bool {
		return doc.Name == "invoice_2025.pdf"
	})).Return(nil)
	// Background goroutine expectations
	docRepo.On("GetByID", mock.Anything, mock.Anything, mock.Anything).Return(&domain.Document{
		ID: uuid.New(), TenantID: tenantID,
		ParsingStatus: domain.ParsingStatusPending, StructuredData: json.RawMessage("{}"),
		ConfidenceScores: json.RawMessage("{}"),
	}, nil).Maybe()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil).Maybe()
	storage.On("Download", mock.Anything, mock.Anything, mock.Anything).Return([]byte("test"), nil).Maybe()
	parser.On("Parse", mock.Anything, mock.Anything).Return(&port.ParseOutput{
		StructuredData: json.RawMessage("{}"), ConfidenceScores: json.RawMessage("{}"),
		ModelUsed: "m", PromptUsed: "p",
	}, nil).Maybe()
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, mock.Anything, "auto").Return(nil).Maybe()
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
		Role:         domain.RoleAdmin,
	})

	assert.NoError(t, err)
	assert.Equal(t, "invoice_2025.pdf", result.Name)

	time.Sleep(50 * time.Millisecond)
}

// --- ListTags ---

func TestDocumentService_ListTags_Success(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	doc := &domain.Document{ID: docID, TenantID: tenantID}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)

	expectedTags := []domain.DocumentTag{
		{ID: uuid.New(), DocumentID: docID, Key: "vendor", Value: "Acme", Source: "user"},
	}
	tagRepo.On("ListByDocument", mock.Anything, docID).Return(expectedTags, nil)

	tags, err := svc.ListTags(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.Equal(t, "vendor", tags[0].Key)
}

// --- AddTags ---

func TestDocumentService_AddTags_Success(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	doc := &domain.Document{ID: docID, TenantID: tenantID}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	tagRepo.On("CreateBatch", mock.Anything, mock.MatchedBy(func(tags []domain.DocumentTag) bool {
		return len(tags) == 2
	})).Return(nil)

	tags, err := svc.AddTags(context.Background(), tenantID, docID, userID, domain.RoleAdmin,
		map[string]string{"vendor": "Acme", "year": "2025"})

	assert.NoError(t, err)
	assert.Len(t, tags, 2)
}

// --- SearchByTag ---

func TestDocumentService_SearchByTag_Success(t *testing.T) {
	svc, _, _, _, _, _, tagRepo := setupDocumentService()

	tenantID := uuid.New()

	expected := []domain.Document{{ID: uuid.New(), TenantID: tenantID}}
	tagRepo.On("SearchByTag", mock.Anything, tenantID, "vendor", "Acme", 0, 20).
		Return(expected, 1, nil)

	docs, total, err := svc.SearchByTag(context.Background(), tenantID, "vendor", "Acme", 0, 20)

	assert.NoError(t, err)
	assert.Len(t, docs, 1)
	assert.Equal(t, 1, total)
}

// --- RetryParse deletes auto-tags ---

func TestDocumentService_RetryParse_DeletesAutoTags(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, parser, storage, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	fileID := uuid.New()
	userID := uuid.New()

	existing := &domain.Document{
		ID: docID, TenantID: tenantID, FileID: fileID,
		DocumentType: "invoice", ParsingStatus: domain.ParsingStatusCompleted,
		StructuredData: json.RawMessage("{}"), ConfidenceScores: json.RawMessage("{}"),
	}
	fileMeta := &domain.FileMeta{
		ID: fileID, TenantID: tenantID,
		S3Bucket: "b", S3Key: "k", ContentType: "application/pdf",
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, docID, "auto").Return(nil).Once()
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil).Maybe()
	storage.On("Download", mock.Anything, "b", "k").Return([]byte("test"), nil).Maybe()
	parser.On("Parse", mock.Anything, mock.Anything).Return(&port.ParseOutput{
		StructuredData: json.RawMessage("{}"), ConfidenceScores: json.RawMessage("{}"),
		ModelUsed: "m", PromptUsed: "p",
	}, nil).Maybe()

	result, err := svc.RetryParse(context.Background(), tenantID, docID, userID, domain.RoleAdmin)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	time.Sleep(50 * time.Millisecond)
	tagRepo.AssertCalled(t, "DeleteByDocumentAndSource", mock.Anything, docID, "auto")
}

// --- EditStructuredData ---

func TestDocumentService_EditStructuredData_Success(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()

	structuredData := json.RawMessage(`{"invoice":{"invoice_number":"INV-001","invoice_date":"2025-01-15"},"seller":{"name":"Acme"},"buyer":{"name":"Buyer"},"line_items":[],"totals":{"total":1000},"payment":{}}`)

	existing := &domain.Document{
		ID:               docID,
		TenantID:         tenantID,
		CollectionID:     collectionID,
		ParsingStatus:    domain.ParsingStatusCompleted,
		ReviewStatus:     domain.ReviewStatusApproved,
		ValidationStatus: domain.ValidationStatusValid,
		StructuredData:   json.RawMessage(`{"invoice":{}}`),
		ConfidenceScores: json.RawMessage(`{}`),
	}

	updated := &domain.Document{
		ID:               docID,
		TenantID:         tenantID,
		CollectionID:     collectionID,
		ParsingStatus:    domain.ParsingStatusCompleted,
		ReviewStatus:     domain.ReviewStatusPending,
		ValidationStatus: domain.ValidationStatusPending,
		StructuredData:   structuredData,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil).Once()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, docID, "auto").Return(nil)
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	// Re-fetch after edit
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(updated, nil).Maybe()

	result, err := svc.EditStructuredData(context.Background(), &service.EditStructuredDataInput{
		TenantID:       tenantID,
		DocumentID:     docID,
		UserID:         userID,
		Role:           domain.RoleAdmin,
		StructuredData: structuredData,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	docRepo.AssertCalled(t, "UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document"))
	docRepo.AssertCalled(t, "UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document"))
}

func TestDocumentService_EditStructuredData_NotFound(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.EditStructuredData(context.Background(), &service.EditStructuredDataInput{
		TenantID:       tenantID,
		DocumentID:     docID,
		UserID:         userID,
		Role:           domain.RoleAdmin,
		StructuredData: json.RawMessage(`{}`),
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

func TestDocumentService_EditStructuredData_NotParsed(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusPending,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.EditStructuredData(context.Background(), &service.EditStructuredDataInput{
		TenantID:       tenantID,
		DocumentID:     docID,
		UserID:         userID,
		Role:           domain.RoleAdmin,
		StructuredData: json.RawMessage(`{}`),
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_EditStructuredData_PermissionDenied(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, userID).
		Return(nil, errors.New("not found"))

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.EditStructuredData(context.Background(), &service.EditStructuredDataInput{
		TenantID:       tenantID,
		DocumentID:     docID,
		UserID:         userID,
		Role:           domain.RoleViewer,
		StructuredData: json.RawMessage(`{}`),
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}

func TestDocumentService_EditStructuredData_InvalidJSON(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.EditStructuredData(context.Background(), &service.EditStructuredDataInput{
		TenantID:       tenantID,
		DocumentID:     docID,
		UserID:         userID,
		Role:           domain.RoleAdmin,
		StructuredData: json.RawMessage(`not valid json`),
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrInvalidStructuredData)
}

func TestDocumentService_EditStructuredData_ResetsReviewStatus(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()
	reviewerID := uuid.New()
	reviewedAt := time.Now().UTC()

	existing := &domain.Document{
		ID:               docID,
		TenantID:         tenantID,
		ParsingStatus:    domain.ParsingStatusCompleted,
		ReviewStatus:     domain.ReviewStatusApproved,
		ReviewedBy:       &reviewerID,
		ReviewedAt:       &reviewedAt,
		ReviewerNotes:    "Previously approved",
		StructuredData:   json.RawMessage(`{}`),
		ConfidenceScores: json.RawMessage(`{}`),
	}

	updated := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
		ReviewStatus:  domain.ReviewStatusPending,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil).Once()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.MatchedBy(func(doc *domain.Document) bool {
		return doc.ReviewStatus == domain.ReviewStatusPending &&
			doc.ReviewedBy == nil &&
			doc.ReviewedAt == nil &&
			doc.ReviewerNotes == ""
	})).Return(nil)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, docID, "auto").Return(nil)
	tagRepo.On("CreateBatch", mock.Anything, mock.Anything).Return(nil).Maybe()
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(updated, nil).Maybe()

	structuredData := json.RawMessage(`{"invoice":{},"seller":{},"buyer":{},"line_items":[],"totals":{},"payment":{}}`)
	result, err := svc.EditStructuredData(context.Background(), &service.EditStructuredDataInput{
		TenantID:       tenantID,
		DocumentID:     docID,
		UserID:         userID,
		Role:           domain.RoleAdmin,
		StructuredData: structuredData,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.ReviewStatusPending, result.ReviewStatus)
}

func TestDocumentService_EditStructuredData_ReextractsAutoTags(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, tagRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()

	existing := &domain.Document{
		ID:               docID,
		TenantID:         tenantID,
		ParsingStatus:    domain.ParsingStatusCompleted,
		StructuredData:   json.RawMessage(`{}`),
		ConfidenceScores: json.RawMessage(`{}`),
	}

	updated := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	permRepo.On("GetByCollectionAndUser", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("not found")).Maybe()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil).Once()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, docID, "auto").Return(nil).Once()
	tagRepo.On("CreateBatch", mock.Anything, mock.MatchedBy(func(tags []domain.DocumentTag) bool {
		// Should have auto-tags extracted from the invoice data
		for i := range tags {
			if tags[i].Source != "auto" {
				return false
			}
		}
		return true
	})).Return(nil).Maybe()
	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(updated, nil).Maybe()

	structuredData := json.RawMessage(`{"invoice":{"invoice_number":"INV-999","invoice_date":"2025-06-01"},"seller":{"name":"Seller Corp","gstin":"29AABCU9603R1ZM"},"buyer":{"name":"Buyer Inc"},"line_items":[],"totals":{"total":5000},"payment":{}}`)

	result, err := svc.EditStructuredData(context.Background(), &service.EditStructuredDataInput{
		TenantID:       tenantID,
		DocumentID:     docID,
		UserID:         userID,
		Role:           domain.RoleAdmin,
		StructuredData: structuredData,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	tagRepo.AssertCalled(t, "DeleteByDocumentAndSource", mock.Anything, docID, "auto")
	tagRepo.AssertCalled(t, "CreateBatch", mock.Anything, mock.Anything)
}
