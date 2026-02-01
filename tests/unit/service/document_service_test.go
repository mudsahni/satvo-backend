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

func setupDocumentService() (
	service.DocumentService,
	*mocks.MockDocumentRepo,
	*mocks.MockFileMetaRepo,
	*mocks.MockDocumentParser,
	*mocks.MockObjectStorage,
) {
	docRepo := new(mocks.MockDocumentRepo)
	fileRepo := new(mocks.MockFileMetaRepo)
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	svc := service.NewDocumentService(docRepo, fileRepo, parser, storage, nil)
	return svc, docRepo, fileRepo, parser, storage
}

// --- CreateAndParse ---

func TestDocumentService_CreateAndParse_Success(t *testing.T) {
	svc, docRepo, fileRepo, parser, storage := setupDocumentService()

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

	// Wait briefly for goroutine to start (not for completion)
	time.Sleep(50 * time.Millisecond)

	fileRepo.AssertExpectations(t)
}

func TestDocumentService_CreateAndParse_FileNotFound(t *testing.T) {
	svc, _, fileRepo, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(nil, domain.ErrNotFound)

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "looking up file")
}

func TestDocumentService_CreateAndParse_DuplicateDocument(t *testing.T) {
	svc, docRepo, fileRepo, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:       fileID,
		TenantID: tenantID,
		S3Bucket: "test-bucket",
		S3Key:    "test-key",
	}

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).
		Return(domain.ErrDocumentAlreadyExists)

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating document")
}

func TestDocumentService_CreateAndParse_CreateRepoError(t *testing.T) {
	svc, docRepo, fileRepo, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:       fileID,
		TenantID: tenantID,
		S3Bucket: "test-bucket",
		S3Key:    "test-key",
	}

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(fileMeta, nil)
	docRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Document")).
		Return(errors.New("db connection error"))

	result, err := svc.CreateAndParse(context.Background(), &service.CreateDocumentInput{
		TenantID:     tenantID,
		CollectionID: uuid.New(),
		FileID:       fileID,
		DocumentType: "invoice",
		CreatedBy:    uuid.New(),
	})

	assert.Nil(t, result)
	assert.Error(t, err)
}

// --- GetByID ---

func TestDocumentService_GetByID_Success(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	expected := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), tenantID, docID)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestDocumentService_GetByID_NotFound(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.GetByID(context.Background(), tenantID, docID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

// --- GetByFileID ---

func TestDocumentService_GetByFileID_Success(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	expected := &domain.Document{
		ID:       uuid.New(),
		TenantID: tenantID,
		FileID:   fileID,
	}

	docRepo.On("GetByFileID", mock.Anything, tenantID, fileID).Return(expected, nil)

	result, err := svc.GetByFileID(context.Background(), tenantID, fileID)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestDocumentService_GetByFileID_NotFound(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	fileID := uuid.New()

	docRepo.On("GetByFileID", mock.Anything, tenantID, fileID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.GetByFileID(context.Background(), tenantID, fileID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

// --- ListByCollection ---

func TestDocumentService_ListByCollection_Success(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	collectionID := uuid.New()

	expected := []domain.Document{
		{ID: uuid.New(), TenantID: tenantID, CollectionID: collectionID},
		{ID: uuid.New(), TenantID: tenantID, CollectionID: collectionID},
	}

	docRepo.On("ListByCollection", mock.Anything, tenantID, collectionID, 0, 20).
		Return(expected, 2, nil)

	docs, total, err := svc.ListByCollection(context.Background(), tenantID, collectionID, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, docs, 2)
	assert.Equal(t, 2, total)
}

func TestDocumentService_ListByCollection_Empty(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	collectionID := uuid.New()

	docRepo.On("ListByCollection", mock.Anything, tenantID, collectionID, 0, 20).
		Return([]domain.Document{}, 0, nil)

	docs, total, err := svc.ListByCollection(context.Background(), tenantID, collectionID, 0, 20)

	assert.NoError(t, err)
	assert.Empty(t, docs)
	assert.Equal(t, 0, total)
}

// --- ListByTenant ---

func TestDocumentService_ListByTenant_Success(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()

	expected := []domain.Document{
		{ID: uuid.New(), TenantID: tenantID},
	}

	docRepo.On("ListByTenant", mock.Anything, tenantID, 0, 20).
		Return(expected, 1, nil)

	docs, total, err := svc.ListByTenant(context.Background(), tenantID, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, docs, 1)
	assert.Equal(t, 1, total)
}

// --- UpdateReview ---

func TestDocumentService_UpdateReview_Approved(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	reviewerID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
		ReviewStatus:  domain.ReviewStatusPending,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: reviewerID,
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
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	reviewerID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
		ReviewStatus:  domain.ReviewStatusPending,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: reviewerID,
		Status:     domain.ReviewStatusRejected,
		Notes:      "Incorrect amounts",
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.ReviewStatusRejected, result.ReviewStatus)
	assert.Equal(t, "Incorrect amounts", result.ReviewerNotes)
}

func TestDocumentService_UpdateReview_NotParsedYet(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusProcessing,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_UpdateReview_PendingStatus(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusPending,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_UpdateReview_FailedStatus(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusFailed,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_UpdateReview_DocNotFound(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

func TestDocumentService_UpdateReview_RepoError(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	existing := &domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	docRepo.On("UpdateReviewStatus", mock.Anything, mock.AnythingOfType("*domain.Document")).
		Return(errors.New("db error"))

	result, err := svc.UpdateReview(context.Background(), &service.UpdateReviewInput{
		TenantID:   tenantID,
		DocumentID: docID,
		ReviewerID: uuid.New(),
		Status:     domain.ReviewStatusApproved,
	})

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating review status")
}

// --- RetryParse ---

func TestDocumentService_RetryParse_Success(t *testing.T) {
	svc, docRepo, fileRepo, parser, storage := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	fileID := uuid.New()

	existing := &domain.Document{
		ID:               docID,
		TenantID:         tenantID,
		FileID:           fileID,
		DocumentType:     "invoice",
		ParsingStatus:    domain.ParsingStatusFailed,
		ParsingError:     "previous error",
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

	result, err := svc.RetryParse(context.Background(), tenantID, docID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.ParsingStatusPending, result.ParsingStatus)
	assert.Empty(t, result.ParsingError)

	// Wait briefly for goroutine to start
	time.Sleep(50 * time.Millisecond)
}

func TestDocumentService_RetryParse_DocNotFound(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(nil, domain.ErrDocumentNotFound)

	result, err := svc.RetryParse(context.Background(), tenantID, docID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

func TestDocumentService_RetryParse_FileNotFound(t *testing.T) {
	svc, docRepo, fileRepo, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	fileID := uuid.New()

	existing := &domain.Document{
		ID:       docID,
		TenantID: tenantID,
		FileID:   fileID,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(existing, nil)
	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(nil, domain.ErrNotFound)

	result, err := svc.RetryParse(context.Background(), tenantID, docID)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "looking up file for retry")
}

// --- Delete ---

func TestDocumentService_Delete_Success(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	docRepo.On("Delete", mock.Anything, tenantID, docID).Return(nil)

	err := svc.Delete(context.Background(), tenantID, docID)

	assert.NoError(t, err)
	docRepo.AssertExpectations(t)
}

func TestDocumentService_Delete_NotFound(t *testing.T) {
	svc, docRepo, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()

	docRepo.On("Delete", mock.Anything, tenantID, docID).Return(domain.ErrDocumentNotFound)

	err := svc.Delete(context.Background(), tenantID, docID)

	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

// --- Background parsing integration test ---

func TestDocumentService_BackgroundParsing_Success(t *testing.T) {
	docRepo := new(mocks.MockDocumentRepo)
	fileRepo := new(mocks.MockFileMetaRepo)
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	svc := service.NewDocumentService(docRepo, fileRepo, parser, storage, nil)

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
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	svc := service.NewDocumentService(docRepo, fileRepo, parser, storage, nil)

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:          fileID,
		TenantID:    tenantID,
		S3Bucket:    "test-bucket",
		S3Key:       "test-key",
		ContentType: "application/pdf",
	}

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
	parser := new(mocks.MockDocumentParser)
	storage := new(mocks.MockObjectStorage)
	svc := service.NewDocumentService(docRepo, fileRepo, parser, storage, nil)

	tenantID := uuid.New()
	fileID := uuid.New()

	fileMeta := &domain.FileMeta{
		ID:          fileID,
		TenantID:    tenantID,
		S3Bucket:    "test-bucket",
		S3Key:       "test-key",
		ContentType: "application/pdf",
	}

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
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Wait for background goroutine to fail
	time.Sleep(200 * time.Millisecond)

	parser.AssertExpectations(t)
}
