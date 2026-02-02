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
	ParseMode    domain.ParseMode
	CreatedBy    uuid.UUID
	Role         domain.UserRole
}

// UpdateReviewInput is the DTO for updating a document's review status.
type UpdateReviewInput struct {
	TenantID   uuid.UUID
	DocumentID uuid.UUID
	ReviewerID uuid.UUID
	Role       domain.UserRole
	Status     domain.ReviewStatus
	Notes      string
}

// DocumentService defines the document management contract.
type DocumentService interface {
	CreateAndParse(ctx context.Context, input *CreateDocumentInput) (*domain.Document, error)
	GetByID(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) (*domain.Document, error)
	GetByFileID(ctx context.Context, tenantID, fileID, userID uuid.UUID, role domain.UserRole) (*domain.Document, error)
	ListByCollection(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole, offset, limit int) ([]domain.Document, int, error)
	ListByTenant(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole, offset, limit int) ([]domain.Document, int, error)
	UpdateReview(ctx context.Context, input *UpdateReviewInput) (*domain.Document, error)
	RetryParse(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) (*domain.Document, error)
	ValidateDocument(ctx context.Context, tenantID, docID uuid.UUID) error
	GetValidation(ctx context.Context, tenantID, docID uuid.UUID) (*validator.ValidationResponse, error)
	Delete(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) error
}

type documentService struct {
	docRepo     port.DocumentRepository
	fileRepo    port.FileMetaRepository
	permRepo    port.CollectionPermissionRepository
	parser      port.DocumentParser
	mergeParser port.DocumentParser // optional merge parser for dual mode
	storage     port.ObjectStorage
	validator   *validator.Engine
}

// NewDocumentService creates a new DocumentService implementation.
func NewDocumentService(
	docRepo port.DocumentRepository,
	fileRepo port.FileMetaRepository,
	permRepo port.CollectionPermissionRepository,
	parser port.DocumentParser,
	storage port.ObjectStorage,
	validationEngine *validator.Engine,
) DocumentService {
	return &documentService{
		docRepo:   docRepo,
		fileRepo:  fileRepo,
		permRepo:  permRepo,
		parser:    parser,
		storage:   storage,
		validator: validationEngine,
	}
}

// NewDocumentServiceWithMerge creates a DocumentService with dual-parse support.
func NewDocumentServiceWithMerge(
	docRepo port.DocumentRepository,
	fileRepo port.FileMetaRepository,
	permRepo port.CollectionPermissionRepository,
	parser port.DocumentParser,
	mergeParser port.DocumentParser,
	storage port.ObjectStorage,
	validationEngine *validator.Engine,
) DocumentService {
	return &documentService{
		docRepo:     docRepo,
		fileRepo:    fileRepo,
		permRepo:    permRepo,
		parser:      parser,
		mergeParser: mergeParser,
		storage:     storage,
		validator:   validationEngine,
	}
}

// effectiveCollectionPerm computes the effective collection permission for a user.
func (s *documentService) effectiveCollectionPerm(ctx context.Context, collectionID, userID uuid.UUID, role domain.UserRole) domain.CollectionPermission {
	implicit := domain.ImplicitCollectionPerm(role)

	explicit := domain.CollectionPermission("")
	perm, err := s.permRepo.GetByCollectionAndUser(ctx, collectionID, userID)
	if err == nil {
		explicit = perm.Permission
	}

	if role == domain.RoleViewer {
		if domain.CollectionPermLevel(explicit) > domain.CollectionPermLevel(domain.CollectionPermViewer) {
			explicit = domain.CollectionPermViewer
		}
	}

	if domain.CollectionPermLevel(implicit) >= domain.CollectionPermLevel(explicit) {
		return implicit
	}
	return explicit
}

// requireCollectionPerm checks that the user has the minimum permission on a collection.
func (s *documentService) requireCollectionPerm(ctx context.Context, collectionID, userID uuid.UUID, role domain.UserRole, minLevel domain.CollectionPermission) error {
	eff := s.effectiveCollectionPerm(ctx, collectionID, userID, role)
	if domain.CollectionPermLevel(eff) < domain.CollectionPermLevel(minLevel) {
		return domain.ErrCollectionPermDenied
	}
	return nil
}

func (s *documentService) CreateAndParse(ctx context.Context, input *CreateDocumentInput) (*domain.Document, error) {
	// Check editor+ permission on the collection
	if err := s.requireCollectionPerm(ctx, input.CollectionID, input.CreatedBy, input.Role, domain.CollectionPermEditor); err != nil {
		return nil, err
	}

	// Verify the file exists
	file, err := s.fileRepo.GetByID(ctx, input.TenantID, input.FileID)
	if err != nil {
		return nil, fmt.Errorf("looking up file: %w", err)
	}

	parseMode := input.ParseMode
	if parseMode == "" {
		parseMode = domain.ParseModeSingle
	}
	// Fall back to single if dual requested but no merge parser configured
	if parseMode == domain.ParseModeDual && s.mergeParser == nil {
		log.Printf("documentService.CreateAndParse: dual parse requested but no merge parser configured, falling back to single")
		parseMode = domain.ParseModeSingle
	}

	doc := &domain.Document{
		ID:                   uuid.New(),
		TenantID:             input.TenantID,
		CollectionID:         input.CollectionID,
		FileID:               input.FileID,
		DocumentType:         input.DocumentType,
		ParsingStatus:        domain.ParsingStatusPending,
		ReviewStatus:         domain.ReviewStatusPending,
		ValidationStatus:     domain.ValidationStatusPending,
		ReconciliationStatus: domain.ReconciliationStatusPending,
		ValidationResults:    json.RawMessage("[]"),
		StructuredData:       json.RawMessage("{}"),
		ConfidenceScores:     json.RawMessage("{}"),
		ParseMode:            parseMode,
		FieldProvenance:      json.RawMessage("{}"),
		CreatedBy:            input.CreatedBy,
	}

	log.Printf("documentService.CreateAndParse: creating document %s for file %s (tenant %s)",
		doc.ID, input.FileID, input.TenantID)

	if err := s.docRepo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("creating document: %w", err)
	}

	// Copy before launching goroutine so the caller's value is independent of background work
	result := *doc

	// Launch background parsing
	go s.parseInBackground(doc.ID, doc.TenantID, file.S3Bucket, file.S3Key, file.ContentType, doc.DocumentType)

	return &result, nil
}

func (s *documentService) selectParser(mode domain.ParseMode) port.DocumentParser {
	if mode == domain.ParseModeDual && s.mergeParser != nil {
		return s.mergeParser
	}
	return s.parser
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

	// Select parser based on document's parse mode
	activeParser := s.selectParser(doc.ParseMode)

	// Call parser
	output, err := activeParser.Parse(ctx, port.ParseInput{
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

	// Save field provenance if present
	if len(output.FieldProvenance) > 0 {
		if provenanceJSON, jsonErr := json.Marshal(output.FieldProvenance); jsonErr == nil {
			doc.FieldProvenance = provenanceJSON
		}
	}

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

func (s *documentService) GetByID(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermViewer); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *documentService) GetByFileID(ctx context.Context, tenantID, fileID, userID uuid.UUID, role domain.UserRole) (*domain.Document, error) {
	doc, err := s.docRepo.GetByFileID(ctx, tenantID, fileID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermViewer); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *documentService) ListByCollection(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole, offset, limit int) ([]domain.Document, int, error) {
	if err := s.requireCollectionPerm(ctx, collectionID, userID, role, domain.CollectionPermViewer); err != nil {
		return nil, 0, err
	}
	return s.docRepo.ListByCollection(ctx, tenantID, collectionID, offset, limit)
}

func (s *documentService) ListByTenant(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole, offset, limit int) ([]domain.Document, int, error) {
	// Admin, manager, and member see all documents
	if role == domain.RoleAdmin || role == domain.RoleManager || role == domain.RoleMember {
		return s.docRepo.ListByTenant(ctx, tenantID, offset, limit)
	}
	// Viewer sees only documents in collections they have access to
	return s.docRepo.ListByUserCollections(ctx, tenantID, userID, offset, limit)
}

func (s *documentService) UpdateReview(ctx context.Context, input *UpdateReviewInput) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, input.TenantID, input.DocumentID)
	if err != nil {
		return nil, err
	}

	// Check editor+ permission on the collection
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, input.ReviewerID, input.Role, domain.CollectionPermEditor); err != nil {
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

func (s *documentService) RetryParse(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return nil, err
	}

	// Check editor+ permission on the collection
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermEditor); err != nil {
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

	// Copy before launching goroutine so the caller's value is independent of background work
	result := *doc

	go s.parseInBackground(doc.ID, doc.TenantID, file.S3Bucket, file.S3Key, file.ContentType, doc.DocumentType)

	return &result, nil
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

func (s *documentService) Delete(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) error {
	return s.docRepo.Delete(ctx, tenantID, docID)
}
