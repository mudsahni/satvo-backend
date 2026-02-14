package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/parser"
	"satvos/internal/port"
	"satvos/internal/validator"
	"satvos/internal/validator/invoice"
)

const defaultMaxParseAttempts = 5

// CreateDocumentInput is the DTO for creating a document and triggering parsing.
type CreateDocumentInput struct {
	TenantID     uuid.UUID
	CollectionID uuid.UUID
	FileID       uuid.UUID
	DocumentType string
	ParseMode    domain.ParseMode
	Name         string
	Tags         map[string]string
	CreatedBy    uuid.UUID
	Role         domain.UserRole
}

// EditStructuredDataInput is the DTO for manually editing a document's structured data.
type EditStructuredDataInput struct {
	TenantID       uuid.UUID
	DocumentID     uuid.UUID
	UserID         uuid.UUID
	Role           domain.UserRole
	StructuredData json.RawMessage
}

// AssignDocumentInput is the DTO for assigning a document to a reviewer.
type AssignDocumentInput struct {
	TenantID   uuid.UUID
	DocumentID uuid.UUID
	CallerID   uuid.UUID
	CallerRole domain.UserRole
	AssigneeID *uuid.UUID // nil = unassign
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
	ListByCollection(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	ListByTenant(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	AssignDocument(ctx context.Context, input *AssignDocumentInput) (*domain.Document, error)
	ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	UpdateReview(ctx context.Context, input *UpdateReviewInput) (*domain.Document, error)
	EditStructuredData(ctx context.Context, input *EditStructuredDataInput) (*domain.Document, error)
	RetryParse(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) (*domain.Document, error)
	ValidateDocument(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) error
	GetValidation(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) (*validator.ValidationResponse, error)
	Delete(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) error
	ListTags(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) ([]domain.DocumentTag, error)
	AddTags(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole, tags map[string]string) ([]domain.DocumentTag, error)
	DeleteTag(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole, tagID uuid.UUID) error
	SearchByTag(ctx context.Context, tenantID uuid.UUID, key, value string, offset, limit int) ([]domain.Document, int, error)
	ParseDocument(ctx context.Context, doc *domain.Document, maxAttempts int)
}

type documentService struct {
	docRepo     port.DocumentRepository
	fileRepo    port.FileMetaRepository
	userRepo    port.UserRepository
	permRepo    port.CollectionPermissionRepository
	tagRepo     port.DocumentTagRepository
	auditRepo   port.DocumentAuditRepository
	parser      port.DocumentParser
	mergeParser port.DocumentParser // optional merge parser for dual mode
	storage     port.ObjectStorage
	validator   *validator.Engine
}

// NewDocumentService creates a new DocumentService implementation.
func NewDocumentService(
	docRepo port.DocumentRepository,
	fileRepo port.FileMetaRepository,
	userRepo port.UserRepository,
	permRepo port.CollectionPermissionRepository,
	tagRepo port.DocumentTagRepository,
	docParser port.DocumentParser,
	storage port.ObjectStorage,
	validationEngine *validator.Engine,
	auditRepo port.DocumentAuditRepository,
) DocumentService {
	return &documentService{
		docRepo:   docRepo,
		fileRepo:  fileRepo,
		userRepo:  userRepo,
		permRepo:  permRepo,
		tagRepo:   tagRepo,
		auditRepo: auditRepo,
		parser:    docParser,
		storage:   storage,
		validator: validationEngine,
	}
}

// NewDocumentServiceWithMerge creates a DocumentService with dual-parse support.
func NewDocumentServiceWithMerge(
	docRepo port.DocumentRepository,
	fileRepo port.FileMetaRepository,
	userRepo port.UserRepository,
	permRepo port.CollectionPermissionRepository,
	tagRepo port.DocumentTagRepository,
	docParser port.DocumentParser,
	mergeDocParser port.DocumentParser,
	storage port.ObjectStorage,
	validationEngine *validator.Engine,
	auditRepo port.DocumentAuditRepository,
) DocumentService {
	return &documentService{
		docRepo:     docRepo,
		fileRepo:    fileRepo,
		userRepo:    userRepo,
		permRepo:    permRepo,
		tagRepo:     tagRepo,
		auditRepo:   auditRepo,
		parser:      docParser,
		mergeParser: mergeDocParser,
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

// audit records a document mutation in the audit log. Failures are logged but never block business logic.
func (s *documentService) audit(ctx context.Context, tenantID, docID uuid.UUID, userID *uuid.UUID, action domain.AuditAction, changes json.RawMessage) {
	if s.auditRepo == nil {
		return
	}
	if changes == nil {
		changes = json.RawMessage("{}")
	}
	entry := &domain.DocumentAuditEntry{
		ID:         uuid.New(),
		TenantID:   tenantID,
		DocumentID: docID,
		UserID:     userID,
		Action:     string(action),
		Changes:    changes,
	}
	if err := s.auditRepo.Create(ctx, entry); err != nil {
		log.Printf("documentService.audit: failed to write audit entry for %s/%s: %v", action, docID, err)
	}
}

func (s *documentService) auditValidationCompleted(ctx context.Context, tenantID, docID uuid.UUID, userID *uuid.UUID, trigger string) {
	if s.auditRepo == nil {
		return
	}
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		log.Printf("documentService.auditValidationCompleted: failed to fetch doc %s: %v", docID, err)
		return
	}
	changes, _ := json.Marshal(map[string]string{
		"validation_status":      string(doc.ValidationStatus),
		"reconciliation_status":  string(doc.ReconciliationStatus),
		"trigger":                trigger,
	})
	s.audit(ctx, tenantID, docID, userID, domain.AuditDocumentValidationCompleted, changes)
}

func (s *documentService) CreateAndParse(ctx context.Context, input *CreateDocumentInput) (*domain.Document, error) {
	// Check editor+ permission on the collection
	if err := s.requireCollectionPerm(ctx, input.CollectionID, input.CreatedBy, input.Role, domain.CollectionPermEditor); err != nil {
		return nil, err
	}

	// Check and increment quota (no-op for unlimited users)
	if err := s.userRepo.CheckAndIncrementQuota(ctx, input.TenantID, input.CreatedBy); err != nil {
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

	name := input.Name
	if name == "" {
		name = file.OriginalName
	}

	doc := &domain.Document{
		ID:                   uuid.New(),
		TenantID:             input.TenantID,
		CollectionID:         input.CollectionID,
		FileID:               input.FileID,
		Name:                 name,
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

	changesJSON, _ := json.Marshal(map[string]interface{}{
		"collection_id": input.CollectionID, "file_id": input.FileID,
		"document_type": input.DocumentType, "parse_mode": string(parseMode),
	})
	s.audit(ctx, doc.TenantID, doc.ID, &input.CreatedBy, domain.AuditDocumentCreated, changesJSON)

	// Save user-provided tags
	if len(input.Tags) > 0 && s.tagRepo != nil {
		tags := make([]domain.DocumentTag, 0, len(input.Tags))
		for k, v := range input.Tags {
			tags = append(tags, domain.DocumentTag{
				ID:         uuid.New(),
				DocumentID: doc.ID,
				TenantID:   doc.TenantID,
				Key:        k,
				Value:      v,
				Source:     "user",
			})
		}
		if tagErr := s.tagRepo.CreateBatch(ctx, tags); tagErr != nil {
			log.Printf("documentService.CreateAndParse: failed to save user tags for %s: %v", doc.ID, tagErr)
		}
	}

	// Copy before launching goroutine so the caller's value is independent of background work
	result := *doc

	// Launch background parsing
	go s.parseInBackground(doc.ID, doc.TenantID)

	return &result, nil
}

func (s *documentService) selectParser(mode domain.ParseMode) port.DocumentParser {
	if mode == domain.ParseModeDual && s.mergeParser != nil {
		return s.mergeParser
	}
	return s.parser
}

func (s *documentService) parseInBackground(docID, tenantID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("documentService.parseInBackground: starting parsing for document %s", docID)

	// Set status to processing
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		log.Printf("documentService.parseInBackground: failed to get document %s: %v", docID, err)
		return
	}
	doc.ParseAttempts++
	doc.ParsingStatus = domain.ParsingStatusProcessing
	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		log.Printf("documentService.parseInBackground: failed to set processing status for %s: %v", docID, err)
		return
	}

	s.ParseDocument(ctx, doc, defaultMaxParseAttempts)
}

// ParseDocument performs the core parse logic: file lookup, S3 download, LLM parse,
// error handling (with rate-limit queueing), result saving, auto-tags, and validation.
// It is called by both parseInBackground and the queue worker.
// The doc must already be in processing status with ParseAttempts incremented.
func (s *documentService) ParseDocument(ctx context.Context, doc *domain.Document, maxAttempts int) {
	// Look up file for S3 coordinates
	file, err := s.fileRepo.GetByID(ctx, doc.TenantID, doc.FileID)
	if err != nil {
		s.failParsing(ctx, doc, fmt.Sprintf("downloading file: %v", err))
		return
	}

	// Download file bytes from S3
	fileBytes, err := s.storage.Download(ctx, file.S3Bucket, file.S3Key)
	if err != nil {
		s.failParsing(ctx, doc, fmt.Sprintf("downloading file: %v", err))
		return
	}

	// Select parser based on document's parse mode
	activeParser := s.selectParser(doc.ParseMode)

	// Call parser
	output, err := activeParser.Parse(ctx, port.ParseInput{
		FileBytes:    fileBytes,
		ContentType:  file.ContentType,
		DocumentType: doc.DocumentType,
	})
	if err != nil {
		s.handleParseError(ctx, doc, err, maxAttempts)
		return
	}

	// Update with results
	now := time.Now().UTC()
	doc.StructuredData = output.StructuredData
	doc.ConfidenceScores = output.ConfidenceScores
	doc.ParserModel = output.ModelUsed
	doc.SecondaryParserModel = output.SecondaryModel
	doc.ParserPrompt = output.PromptUsed
	doc.ParsingStatus = domain.ParsingStatusCompleted
	doc.ParsingError = ""
	doc.ParsedAt = &now
	doc.RetryAfter = nil

	// Save field provenance if present
	if len(output.FieldProvenance) > 0 {
		if provenanceJSON, jsonErr := json.Marshal(output.FieldProvenance); jsonErr == nil {
			doc.FieldProvenance = provenanceJSON
		}
	}

	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		log.Printf("documentService.ParseDocument: failed to save results for %s: %v", doc.ID, err)
		return
	}

	parseChanges, _ := json.Marshal(map[string]interface{}{
		"parser_model": doc.ParserModel, "parse_mode": string(doc.ParseMode), "attempt": doc.ParseAttempts,
	})
	s.audit(ctx, doc.TenantID, doc.ID, nil, domain.AuditDocumentParseCompleted, parseChanges)

	log.Printf("documentService.ParseDocument: document %s parsed successfully", doc.ID)

	// Extract auto-tags from parsed data
	if s.tagRepo != nil {
		s.extractAndSaveAutoTags(ctx, doc.ID, doc.TenantID, doc.StructuredData)
	}

	// Run validation after successful parsing
	if s.validator != nil {
		if err := s.validator.ValidateDocument(ctx, doc.TenantID, doc.ID); err != nil {
			log.Printf("documentService.ParseDocument: validation failed for %s: %v", doc.ID, err)
		} else {
			s.auditValidationCompleted(ctx, doc.TenantID, doc.ID, nil, "parse")
		}
	}
}

// handleParseError checks if the error is a rate limit and queues the document for retry
// if under the max attempts threshold. Otherwise, marks parsing as permanently failed.
func (s *documentService) handleParseError(ctx context.Context, doc *domain.Document, parseErr error, maxAttempts int) {
	var rlErr *parser.RateLimitError
	if errors.As(parseErr, &rlErr) && doc.ParseAttempts < maxAttempts {
		retryAt := time.Now().Add(rlErr.RetryAfter)
		doc.ParsingStatus = domain.ParsingStatusQueued
		doc.ParsingError = fmt.Sprintf("rate limited by %s, queued for retry", rlErr.Provider)
		doc.RetryAfter = &retryAt
		if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
			log.Printf("documentService.handleParseError: failed to queue document %s: %v", doc.ID, err)
		} else {
			queueChanges, _ := json.Marshal(map[string]interface{}{
				"retry_after": retryAt.Format(time.RFC3339), "attempt": doc.ParseAttempts,
			})
			s.audit(ctx, doc.TenantID, doc.ID, nil, domain.AuditDocumentParseQueued, queueChanges)
			log.Printf("documentService.handleParseError: document %s queued for retry after %s", doc.ID, retryAt.Format(time.RFC3339))
		}
		return
	}
	s.failParsing(ctx, doc, fmt.Sprintf("parsing document: %v", parseErr))
}

func (s *documentService) failParsing(ctx context.Context, doc *domain.Document, errMsg string) {
	log.Printf("documentService.failParsing: document %s failed: %s", doc.ID, errMsg)
	doc.ParsingStatus = domain.ParsingStatusFailed
	doc.ParsingError = errMsg
	doc.RetryAfter = nil
	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		log.Printf("documentService.failParsing: failed to update status for %s: %v", doc.ID, err)
	}
	failChanges, _ := json.Marshal(map[string]interface{}{"error": errMsg, "attempt": doc.ParseAttempts})
	s.audit(ctx, doc.TenantID, doc.ID, nil, domain.AuditDocumentParseFailed, failChanges)
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

func (s *documentService) ListByCollection(ctx context.Context, tenantID, collectionID, userID uuid.UUID, role domain.UserRole, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	if err := s.requireCollectionPerm(ctx, collectionID, userID, role, domain.CollectionPermViewer); err != nil {
		return nil, 0, err
	}
	return s.docRepo.ListByCollection(ctx, tenantID, collectionID, assignedTo, offset, limit)
}

func (s *documentService) ListByTenant(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	// Admin, manager, and member see all documents
	if role == domain.RoleAdmin || role == domain.RoleManager || role == domain.RoleMember {
		return s.docRepo.ListByTenant(ctx, tenantID, assignedTo, offset, limit)
	}
	// Viewer sees only documents in collections they have access to
	return s.docRepo.ListByUserCollections(ctx, tenantID, userID, assignedTo, offset, limit)
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

	reviewChanges, _ := json.Marshal(map[string]interface{}{"status": string(input.Status), "notes": input.Notes})
	s.audit(ctx, input.TenantID, input.DocumentID, &input.ReviewerID, domain.AuditDocumentReview, reviewChanges)

	return doc, nil
}

func (s *documentService) AssignDocument(ctx context.Context, input *AssignDocumentInput) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, input.TenantID, input.DocumentID)
	if err != nil {
		return nil, err
	}

	// Check editor+ permission on the collection (caller)
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, input.CallerID, input.CallerRole, domain.CollectionPermEditor); err != nil {
		return nil, err
	}

	if doc.ParsingStatus != domain.ParsingStatusCompleted {
		return nil, domain.ErrDocumentNotParsed
	}

	previousAssignee := doc.AssignedTo

	if input.AssigneeID != nil {
		// Verify assignee exists in tenant
		assignee, err := s.userRepo.GetByID(ctx, input.TenantID, *input.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("assignee not found: %w", err)
		}

		// Verify assignee has editor+ effective permission on collection
		if err := s.requireCollectionPerm(ctx, doc.CollectionID, *input.AssigneeID, assignee.Role, domain.CollectionPermEditor); err != nil {
			return nil, domain.ErrAssigneeCannotReview
		}

		now := time.Now().UTC()
		doc.AssignedTo = input.AssigneeID
		doc.AssignedAt = &now
		doc.AssignedBy = &input.CallerID
	} else {
		// Unassign
		doc.AssignedTo = nil
		doc.AssignedAt = nil
		doc.AssignedBy = nil
	}

	if err := s.docRepo.UpdateAssignment(ctx, doc); err != nil {
		return nil, fmt.Errorf("updating assignment: %w", err)
	}

	// Audit
	var changes json.RawMessage
	if input.AssigneeID != nil {
		changes, _ = json.Marshal(map[string]interface{}{
			"assigned_to": input.AssigneeID.String(), "assigned_by": input.CallerID.String(),
		})
	} else {
		prev := ""
		if previousAssignee != nil {
			prev = previousAssignee.String()
		}
		changes, _ = json.Marshal(map[string]interface{}{
			"assigned_to": nil, "assigned_by": input.CallerID.String(), "previous_assignee": prev,
		})
	}
	s.audit(ctx, input.TenantID, input.DocumentID, &input.CallerID, domain.AuditDocumentAssigned, changes)

	return doc, nil
}

func (s *documentService) ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	return s.docRepo.ListReviewQueue(ctx, tenantID, userID, offset, limit)
}

func (s *documentService) EditStructuredData(ctx context.Context, input *EditStructuredDataInput) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, input.TenantID, input.DocumentID)
	if err != nil {
		return nil, err
	}

	// Check editor+ permission on the collection
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, input.UserID, input.Role, domain.CollectionPermEditor); err != nil {
		return nil, err
	}

	if doc.ParsingStatus != domain.ParsingStatusCompleted {
		return nil, domain.ErrDocumentNotParsed
	}

	// Validate that the structured data can be unmarshalled into GSTInvoice
	var inv invoice.GSTInvoice
	if err := json.Unmarshal(input.StructuredData, &inv); err != nil {
		return nil, domain.ErrInvalidStructuredData
	}

	// Build all-1.0 confidence scores (human-verified data)
	confidenceScores := buildFullConfidenceScores(&inv)
	confidenceJSON, err := json.Marshal(confidenceScores)
	if err != nil {
		return nil, fmt.Errorf("marshaling confidence scores: %w", err)
	}

	// Update document fields
	doc.StructuredData = input.StructuredData
	doc.ConfidenceScores = confidenceJSON
	doc.FieldProvenance = json.RawMessage(`{"source":"manual_edit"}`)

	// Reset validation and reconciliation status
	doc.ValidationStatus = domain.ValidationStatusPending
	doc.ValidationResults = json.RawMessage("[]")
	doc.ReconciliationStatus = domain.ReconciliationStatusPending

	// Persist structured data changes
	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		return nil, fmt.Errorf("updating structured data: %w", err)
	}

	s.audit(ctx, input.TenantID, input.DocumentID, &input.UserID, domain.AuditDocumentEditStructured, json.RawMessage(`{"provenance":"manual_edit"}`))

	// Reset review status
	doc.ReviewStatus = domain.ReviewStatusPending
	doc.ReviewedBy = nil
	doc.ReviewedAt = nil
	doc.ReviewerNotes = ""

	if err := s.docRepo.UpdateReviewStatus(ctx, doc); err != nil {
		return nil, fmt.Errorf("resetting review status: %w", err)
	}

	// Re-extract auto-tags from the new structured data
	if s.tagRepo != nil {
		s.extractAndSaveAutoTags(ctx, doc.ID, doc.TenantID, doc.StructuredData)
	}

	// Run validation synchronously
	if s.validator != nil {
		if err := s.validator.ValidateDocument(ctx, input.TenantID, input.DocumentID); err != nil {
			log.Printf("documentService.EditStructuredData: validation failed for %s: %v", input.DocumentID, err)
		}
	}

	// Re-fetch to get updated validation results
	updated, err := s.docRepo.GetByID(ctx, input.TenantID, input.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching document after edit: %w", err)
	}

	if s.validator != nil {
		s.auditValidationCompleted(ctx, input.TenantID, input.DocumentID, &input.UserID, "edit")
	}

	return updated, nil
}

// buildFullConfidenceScores creates confidence scores with all fields set to 1.0.
func buildFullConfidenceScores(inv *invoice.GSTInvoice) invoice.ConfidenceScores {
	scores := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{
			InvoiceNumber:         1.0,
			InvoiceDate:           1.0,
			DueDate:               1.0,
			InvoiceType:           1.0,
			Currency:              1.0,
			PlaceOfSupply:         1.0,
			ReverseCharge:         1.0,
			IRN:                   1.0,
			AcknowledgementNumber: 1.0,
			AcknowledgementDate:   1.0,
		},
		Seller: invoice.PartyConfidence{
			Name:      1.0,
			Address:   1.0,
			GSTIN:     1.0,
			PAN:       1.0,
			State:     1.0,
			StateCode: 1.0,
		},
		Buyer: invoice.PartyConfidence{
			Name:      1.0,
			Address:   1.0,
			GSTIN:     1.0,
			PAN:       1.0,
			State:     1.0,
			StateCode: 1.0,
		},
		Totals: invoice.TotalsConfidence{
			Subtotal:      1.0,
			TotalDiscount: 1.0,
			TaxableAmount: 1.0,
			CGST:          1.0,
			SGST:          1.0,
			IGST:          1.0,
			Cess:          1.0,
			RoundOff:      1.0,
			Total:         1.0,
		},
		Payment: invoice.PaymentConfidence{
			BankName:      1.0,
			AccountNumber: 1.0,
			IFSCCode:      1.0,
			PaymentTerms:  1.0,
		},
	}

	lineItems := make([]invoice.LineItemConfidence, len(inv.LineItems))
	for i := range lineItems {
		lineItems[i] = invoice.LineItemConfidence{
			Description:   1.0,
			HSNSACCode:    1.0,
			Quantity:      1.0,
			Unit:          1.0,
			UnitPrice:     1.0,
			Discount:      1.0,
			TaxableAmount: 1.0,
			CGSTRate:      1.0,
			CGSTAmount:    1.0,
			SGSTRate:      1.0,
			SGSTAmount:    1.0,
			IGSTRate:      1.0,
			IGSTAmount:    1.0,
			Total:         1.0,
		}
	}
	scores.LineItems = lineItems

	return scores
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

	// Verify the file still exists
	if _, err := s.fileRepo.GetByID(ctx, tenantID, doc.FileID); err != nil {
		return nil, fmt.Errorf("looking up file for retry: %w", err)
	}

	// Delete auto-tags before re-parsing
	if s.tagRepo != nil {
		if tagErr := s.tagRepo.DeleteByDocumentAndSource(ctx, docID, "auto"); tagErr != nil {
			log.Printf("documentService.RetryParse: failed to delete auto-tags for %s: %v", docID, tagErr)
		}
	}

	// Reset to pending and clear assignment
	doc.ParsingStatus = domain.ParsingStatusPending
	doc.ParsingError = ""
	doc.StructuredData = json.RawMessage("{}")
	doc.ConfidenceScores = json.RawMessage("{}")
	doc.AssignedTo = nil
	doc.AssignedAt = nil
	doc.AssignedBy = nil
	if err := s.docRepo.UpdateStructuredData(ctx, doc); err != nil {
		return nil, fmt.Errorf("resetting document for retry: %w", err)
	}

	s.audit(ctx, tenantID, docID, &userID, domain.AuditDocumentRetry, nil)

	log.Printf("documentService.RetryParse: retrying parsing for document %s", docID)

	// Copy before launching goroutine so the caller's value is independent of background work
	result := *doc

	go s.parseInBackground(doc.ID, doc.TenantID)

	return &result, nil
}

func (s *documentService) ValidateDocument(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) error {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return err
	}
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermEditor); err != nil {
		return err
	}
	if doc.ParsingStatus != domain.ParsingStatusCompleted {
		return domain.ErrDocumentNotParsed
	}
	if s.validator == nil {
		return fmt.Errorf("validation engine not configured")
	}
	s.audit(ctx, tenantID, docID, &userID, domain.AuditDocumentValidate, nil)
	if err := s.validator.ValidateDocument(ctx, tenantID, docID); err != nil {
		return err
	}
	s.auditValidationCompleted(ctx, tenantID, docID, &userID, "manual")
	return nil
}

func (s *documentService) GetValidation(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) (*validator.ValidationResponse, error) {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermViewer); err != nil {
		return nil, err
	}
	if s.validator == nil {
		return nil, fmt.Errorf("validation engine not configured")
	}
	return s.validator.GetValidation(ctx, tenantID, docID)
}

func (s *documentService) Delete(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) error {
	s.audit(ctx, tenantID, docID, &userID, domain.AuditDocumentDeleted, nil)
	return s.docRepo.Delete(ctx, tenantID, docID)
}

func (s *documentService) ListTags(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole) ([]domain.DocumentTag, error) {
	// Verify document exists and user has access
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermViewer); err != nil {
		return nil, err
	}
	return s.tagRepo.ListByDocument(ctx, docID)
}

func (s *documentService) AddTags(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole, tagsMap map[string]string) ([]domain.DocumentTag, error) {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return nil, err
	}
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermEditor); err != nil {
		return nil, err
	}

	tags := make([]domain.DocumentTag, 0, len(tagsMap))
	for k, v := range tagsMap {
		tags = append(tags, domain.DocumentTag{
			ID:         uuid.New(),
			DocumentID: docID,
			TenantID:   tenantID,
			Key:        k,
			Value:      v,
			Source:     "user",
		})
	}

	if err := s.tagRepo.CreateBatch(ctx, tags); err != nil {
		return nil, fmt.Errorf("adding tags: %w", err)
	}

	tagChanges, _ := json.Marshal(tagsMap)
	s.audit(ctx, tenantID, docID, &userID, domain.AuditDocumentTagsAdded, tagChanges)

	return tags, nil
}

func (s *documentService) DeleteTag(ctx context.Context, tenantID, docID, userID uuid.UUID, role domain.UserRole, tagID uuid.UUID) error {
	doc, err := s.docRepo.GetByID(ctx, tenantID, docID)
	if err != nil {
		return err
	}
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, userID, role, domain.CollectionPermEditor); err != nil {
		return err
	}
	if err := s.tagRepo.DeleteByID(ctx, docID, tagID); err != nil {
		return err
	}
	deleteTagChanges, _ := json.Marshal(map[string]interface{}{"tag_id": tagID})
	s.audit(ctx, tenantID, docID, &userID, domain.AuditDocumentTagDeleted, deleteTagChanges)
	return nil
}

func (s *documentService) SearchByTag(ctx context.Context, tenantID uuid.UUID, key, value string, offset, limit int) ([]domain.Document, int, error) {
	return s.tagRepo.SearchByTag(ctx, tenantID, key, value, offset, limit)
}

func (s *documentService) extractAndSaveAutoTags(ctx context.Context, docID, tenantID uuid.UUID, structuredData json.RawMessage) {
	var inv invoice.GSTInvoice
	if err := json.Unmarshal(structuredData, &inv); err != nil {
		log.Printf("documentService.extractAndSaveAutoTags: failed to unmarshal structured data for %s: %v", docID, err)
		return
	}

	tagMap := map[string]string{}
	if inv.Invoice.InvoiceNumber != "" {
		tagMap["invoice_number"] = inv.Invoice.InvoiceNumber
	}
	if inv.Invoice.InvoiceDate != "" {
		tagMap["invoice_date"] = inv.Invoice.InvoiceDate
	}
	if inv.Seller.Name != "" {
		tagMap["seller_name"] = inv.Seller.Name
	}
	if inv.Seller.GSTIN != "" {
		tagMap["seller_gstin"] = inv.Seller.GSTIN
	}
	if inv.Buyer.Name != "" {
		tagMap["buyer_name"] = inv.Buyer.Name
	}
	if inv.Buyer.GSTIN != "" {
		tagMap["buyer_gstin"] = inv.Buyer.GSTIN
	}
	if inv.Invoice.InvoiceType != "" {
		tagMap["invoice_type"] = inv.Invoice.InvoiceType
	}
	if inv.Invoice.PlaceOfSupply != "" {
		tagMap["place_of_supply"] = inv.Invoice.PlaceOfSupply
	}
	if inv.Invoice.IRN != "" {
		tagMap["irn"] = inv.Invoice.IRN
	}
	if inv.Totals.Total != 0 {
		tagMap["total_amount"] = fmt.Sprintf("%.2f", inv.Totals.Total)
	}

	if len(tagMap) == 0 {
		return
	}

	// Delete existing auto-tags and save new ones
	if err := s.tagRepo.DeleteByDocumentAndSource(ctx, docID, "auto"); err != nil {
		log.Printf("documentService.extractAndSaveAutoTags: failed to delete old auto-tags for %s: %v", docID, err)
	}

	tags := make([]domain.DocumentTag, 0, len(tagMap))
	for k, v := range tagMap {
		tags = append(tags, domain.DocumentTag{
			ID:         uuid.New(),
			DocumentID: docID,
			TenantID:   tenantID,
			Key:        k,
			Value:      v,
			Source:     "auto",
		})
	}

	if err := s.tagRepo.CreateBatch(ctx, tags); err != nil {
		log.Printf("documentService.extractAndSaveAutoTags: failed to save auto-tags for %s: %v", docID, err)
	}
}
