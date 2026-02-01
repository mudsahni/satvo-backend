package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/port"
	"satvos/internal/validator"
)

// CreateDocumentInput is the DTO for creating a document and triggering parsing.
type CreateDocumentInput struct {
	TenantID     uuid.UUID
	CollectionID uuid.UUID
	FileID       uuid.UUID
	DocumentType string
	CreatedBy    uuid.UUID
}

// UpdateReviewInput is the DTO for updating a document's review status.
type UpdateReviewInput struct {
	TenantID   uuid.UUID
	DocumentID uuid.UUID
	ReviewerID uuid.UUID
	Status     domain.ReviewStatus
	Notes      string
}

// DocumentService defines the document management contract.
type DocumentService interface {
	CreateAndParse(ctx context.Context, input CreateDocumentInput) (*domain.Document, error)
	GetByID(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error)
	GetByFileID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.Document, error)
	ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	UpdateReview(ctx context.Context, input UpdateReviewInput) (*domain.Document, error)
	RetryParse(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error)
	ValidateDocument(ctx context.Context, tenantID, docID uuid.UUID) error
	GetValidation(ctx context.Context, tenantID, docID uuid.UUID) (*validator.ValidationResponse, error)
	Delete(ctx context.Context, tenantID, docID uuid.UUID) error
}

type documentService struct {
	docRepo   port.DocumentRepository
	fileRepo  port.FileMetaRepository
	parser    port.DocumentParser
	storage   port.ObjectStorage
	validator *validator.Engine
}

// NewDocumentService creates a new DocumentService implementation.
func NewDocumentService(
	docRepo port.DocumentRepository,
	fileRepo port.FileMetaRepository,
	parser port.DocumentParser,
	storage port.ObjectStorage,
	validationEngine *validator.Engine,
) DocumentService {
	return &documentService{
		docRepo:   docRepo,
		fileRepo:  fileRepo,
		parser:    parser,
		storage:   storage,
		validator: validationEngine,
	}
}

func (s *documentService) CreateAndParse(ctx context.Context, input CreateDocumentInput) (*domain.Document, error) {
	// Verify the file exists
	file, err := s.fileRepo.GetByID(ctx, input.TenantID, input.FileID)
	if err != nil {
		return nil, fmt.Errorf("looking up file: %w", err)
	}

	doc := &domain.Document{
		ID:               uuid.New(),
		TenantID:         input.TenantID,
		CollectionID:     input.CollectionID,
		FileID:           input.FileID,
		DocumentType:     input.DocumentType,
		ParsingStatus:    domain.ParsingStatusPending,
		ReviewStatus:     domain.ReviewStatusPending,
		ValidationStatus:  domain.ValidationStatusPending,
		ValidationResults: json.RawMessage("[]"),
		StructuredData:    json.RawMessage("{}"),
		ConfidenceScores:  json.RawMessage("{}"),
		CreatedBy:        input.CreatedBy,
	}

	log.Printf("documentService.CreateAndParse: creating document %s for file %s (tenant %s)",
		doc.ID, input.FileID, input.TenantID)

	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("creating document: %w", err)
	}

	// Launch background parsing
	go s.parseInBackground(doc.ID, doc.TenantID, file.S3Bucket, file.S3Key, file.ContentType, doc.DocumentType)

	return doc, nil
}

func (s *documentService) parseInBackground(docID, tenantID uuid.UUID, bucket, key, contentType, documentType string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("documentService.parseInBackground: starting parsing for document %s", docID)

	// Set status to processing
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		log.Printf("documentService.parseInBackground: failed to get document %s: %v", docID, err)
		return
	}
	doc.ParsingStatus = domain.ParsingStatusProcessing
	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		log.Printf("documentService.parseInBackground: failed to set processing status for %s: %v", docID, err)
		return
	}

	// Download file bytes from S3
	fileBytes, err := s.storage.Download(ctx, bucket, key)
	if err != nil {
		s.failParsing(ctx, doc, fmt.Sprintf("downloading file: %v", err))
		return
	}

	// Call parser
	output, err := s.parser.Parse(ctx, port.ParseInput{
		FileBytes:    fileBytes,
		ContentType:  contentType,
		DocumentType: documentType,
	})
	if err != nil {
		s.failParsing(ctx, doc, fmt.Sprintf("parsing document: %v", err))
		return
	}

	// Update with results
	now := time.Now().UTC()
	doc.StructuredData = output.StructuredData
	doc.ConfidenceScores = output.ConfidenceScores
	doc.ParserModel = output.ModelUsed
	doc.ParserPrompt = output.PromptUsed
	doc.ParsingStatus = domain.ParsingStatusCompleted
	doc.ParsingError = ""
	doc.ParsedAt = &now

	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		log.Printf("documentService.parseInBackground: failed to save results for %s: %v", docID, err)
		return
	}

	log.Printf("documentService.parseInBackground: document %s parsed successfully", docID)

	// Run validation after successful parsing
	if s.validator != nil {
		if err := s.validator.ValidateDocument(ctx, tenantID, docID); err != nil {
			log.Printf("documentService.parseInBackground: validation failed for %s: %v", docID, err)
		}
	}
}

func (s *documentService) failParsing(ctx context.Context, doc *domain.Document, errMsg string) {
	log.Printf("documentService.parseInBackground: document %s failed: %s", doc.ID, errMsg)
	doc.ParsingStatus = domain.ParsingStatusFailed
	doc.ParsingError = errMsg
	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		log.Printf("documentService.failParsing: failed to update status for %s: %v", doc.ID, err)
	}
}

func (s *documentService) GetByID(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error) {
	return s.docRepo.GetByID(ctx, tenantID, docID)
}

func (s *documentService) GetByFileID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.Document, error) {
	return s.docRepo.GetByFileID(ctx, tenantID, fileID)
}

func (s *documentService) ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	return s.docRepo.ListByCollection(ctx, tenantID, collectionID, offset, limit)
}

func (s *documentService) ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	return s.docRepo.ListByTenant(ctx, tenantID, offset, limit)
}

func (s *documentService) UpdateReview(ctx context.Context, input UpdateReviewInput) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, input.TenantID, input.DocumentID)
	if err != nil {
		return nil, err
	}

	if doc.ParsingStatus != domain.ParsingStatusCompleted {
		return nil, domain.ErrDocumentNotParsed
	}

	now := time.Now().UTC()
	doc.ReviewStatus = input.Status
	doc.ReviewedBy = &input.ReviewerID
	doc.ReviewedAt = &now
	doc.ReviewerNotes = input.Notes

	if err := s.docRepo.UpdateReviewStatus(ctx, doc); err != nil {
		return nil, fmt.Errorf("updating review status: %w", err)
	}

	return doc, nil
}

func (s *documentService) RetryParse(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return nil, err
	}

	// Get the file info for S3 coordinates
	file, err := s.fileRepo.GetByID(ctx, tenantID, doc.FileID)
	if err != nil {
		return nil, fmt.Errorf("looking up file for retry: %w", err)
	}

	// Reset to pending
	doc.ParsingStatus = domain.ParsingStatusPending
	doc.ParsingError = ""
	doc.StructuredData = json.RawMessage("{}")
	doc.ConfidenceScores = json.RawMessage("{}")
	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		return nil, fmt.Errorf("resetting document for retry: %w", err)
	}

	log.Printf("documentService.RetryParse: retrying parsing for document %s", docID)

	go s.parseInBackground(doc.ID, doc.TenantID, file.S3Bucket, file.S3Key, file.ContentType, doc.DocumentType)

	return doc, nil
}

func (s *documentService) ValidateDocument(ctx context.Context, tenantID, docID uuid.UUID) error {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return err
	}
	if doc.ParsingStatus != domain.ParsingStatusCompleted {
		return domain.ErrDocumentNotParsed
	}
	if s.validator == nil {
		return fmt.Errorf("validation engine not configured")
	}
	return s.validator.ValidateDocument(ctx, tenantID, docID)
}

func (s *documentService) GetValidation(ctx context.Context, tenantID, docID uuid.UUID) (*validator.ValidationResponse, error) {
	if s.validator == nil {
		return nil, fmt.Errorf("validation engine not configured")
	}
	return s.validator.GetValidation(ctx, tenantID, docID)
}

func (s *documentService) Delete(ctx context.Context, tenantID, docID uuid.UUID) error {
	return s.docRepo.Delete(ctx, tenantID, docID)
}
