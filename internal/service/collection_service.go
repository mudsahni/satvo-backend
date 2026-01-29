package service

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"

	"github.com/google/uuid"

	"satvos/internal/domain"
	"satvos/internal/port"
)

// CreateCollectionInput is the DTO for creating a collection.
type CreateCollectionInput struct {
	TenantID    uuid.UUID
	CreatedBy   uuid.UUID
	Name        string
	Description string
}

// UpdateCollectionInput is the DTO for updating a collection.
type UpdateCollectionInput struct {
	TenantID     uuid.UUID
	CollectionID uuid.UUID
	UserID       uuid.UUID
	Name         string
	Description  string
}

// SetPermissionInput is the DTO for setting a collection permission.
type SetPermissionInput struct {
	TenantID     uuid.UUID
	CollectionID uuid.UUID
	GrantedBy    uuid.UUID
	UserID       uuid.UUID
	Permission   domain.CollectionPermission
}

// BatchUploadFileInput represents a single file in a batch upload.
type BatchUploadFileInput struct {
	File   multipart.File
	Header *multipart.FileHeader
}

// BatchUploadResult contains per-file results from a batch upload.
type BatchUploadResult struct {
	FileName string           `json:"file_name"`
	Success  bool             `json:"success"`
	File     *domain.FileMeta `json:"file,omitempty"`
	Error    string           `json:"error,omitempty"`
}

// CollectionService defines the collection management contract.
type CollectionService interface {
	Create(ctx context.Context, input CreateCollectionInput) (*domain.Collection, error)
	GetByID(ctx context.Context, tenantID, collectionID, userID uuid.UUID) (*domain.Collection, error)
	List(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Collection, int, error)
	Update(ctx context.Context, input UpdateCollectionInput) (*domain.Collection, error)
	Delete(ctx context.Context, tenantID, collectionID, userID uuid.UUID) error
	ListFiles(ctx context.Context, tenantID, collectionID, userID uuid.UUID, offset, limit int) ([]domain.FileMeta, int, error)
	BatchUploadFiles(ctx context.Context, tenantID, collectionID, userID uuid.UUID, files []BatchUploadFileInput) ([]BatchUploadResult, error)
	RemoveFile(ctx context.Context, tenantID, collectionID, fileID, userID uuid.UUID) error
	AddFileToCollection(ctx context.Context, tenantID, collectionID, fileID, userID uuid.UUID) error
	SetPermission(ctx context.Context, input SetPermissionInput) error
	ListPermissions(ctx context.Context, tenantID, collectionID, userID uuid.UUID, offset, limit int) ([]domain.CollectionPermissionEntry, int, error)
	RemovePermission(ctx context.Context, tenantID, collectionID, targetUserID, userID uuid.UUID) error
}

type collectionService struct {
	collectionRepo port.CollectionRepository
	permRepo       port.CollectionPermissionRepository
	fileRepo       port.CollectionFileRepository
	fileSvc        FileService
}

// NewCollectionService creates a new CollectionService implementation.
func NewCollectionService(
	collectionRepo port.CollectionRepository,
	permRepo port.CollectionPermissionRepository,
	fileRepo port.CollectionFileRepository,
	fileSvc FileService,
) CollectionService {
	return &collectionService{
		collectionRepo: collectionRepo,
		permRepo:       permRepo,
		fileRepo:       fileRepo,
		fileSvc:        fileSvc,
	}
}

func (s *collectionService) requirePermission(ctx context.Context, collectionID, userID uuid.UUID, minLevel domain.CollectionPermission) error {
	perm, err := s.permRepo.GetByCollectionAndUser(ctx, collectionID, userID)
	if err != nil {
		return domain.ErrCollectionPermDenied
	}
	if domain.CollectionPermLevel(perm.Permission) < domain.CollectionPermLevel(minLevel) {
		return domain.ErrCollectionPermDenied
	}
	return nil
}

func (s *collectionService) Create(ctx context.Context, input CreateCollectionInput) (*domain.Collection, error) {
	collection := &domain.Collection{
		ID:          uuid.New(),
		TenantID:    input.TenantID,
		Name:        input.Name,
		Description: input.Description,
		CreatedBy:   input.CreatedBy,
	}

	log.Printf("collectionService.Create: creating collection %s for tenant %s by user %s",
		collection.ID, input.TenantID, input.CreatedBy)

	if err := s.collectionRepo.Create(ctx, collection); err != nil {
		log.Printf("collectionService.Create: failed to create collection: %v", err)
		return nil, fmt.Errorf("creating collection: %w", err)
	}

	// Auto-assign owner permission to creator
	ownerPerm := &domain.CollectionPermissionEntry{
		CollectionID: collection.ID,
		TenantID:     input.TenantID,
		UserID:       input.CreatedBy,
		Permission:   domain.CollectionPermOwner,
		GrantedBy:    input.CreatedBy,
	}
	if err := s.permRepo.Upsert(ctx, ownerPerm); err != nil {
		log.Printf("collectionService.Create: failed to assign owner permission for collection %s: %v",
			collection.ID, err)
		return nil, fmt.Errorf("assigning owner permission: %w", err)
	}

	log.Printf("collectionService.Create: collection %s created successfully", collection.ID)
	return collection, nil
}

func (s *collectionService) GetByID(ctx context.Context, tenantID, collectionID, userID uuid.UUID) (*domain.Collection, error) {
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermViewer); err != nil {
		return nil, err
	}
	return s.collectionRepo.GetByID(ctx, tenantID, collectionID)
}

func (s *collectionService) List(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Collection, int, error) {
	return s.collectionRepo.ListByUser(ctx, tenantID, userID, offset, limit)
}

func (s *collectionService) Update(ctx context.Context, input UpdateCollectionInput) (*domain.Collection, error) {
	if err := s.requirePermission(ctx, input.CollectionID, input.UserID, domain.CollectionPermOwner); err != nil {
		return nil, err
	}

	collection, err := s.collectionRepo.GetByID(ctx, input.TenantID, input.CollectionID)
	if err != nil {
		return nil, err
	}

	collection.Name = input.Name
	collection.Description = input.Description

	if err := s.collectionRepo.Update(ctx, collection); err != nil {
		return nil, err
	}

	return collection, nil
}

func (s *collectionService) Delete(ctx context.Context, tenantID, collectionID, userID uuid.UUID) error {
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermOwner); err != nil {
		return err
	}
	log.Printf("collectionService.Delete: deleting collection %s by user %s", collectionID, userID)
	return s.collectionRepo.Delete(ctx, tenantID, collectionID)
}

func (s *collectionService) ListFiles(ctx context.Context, tenantID, collectionID, userID uuid.UUID, offset, limit int) ([]domain.FileMeta, int, error) {
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermViewer); err != nil {
		return nil, 0, err
	}
	return s.fileRepo.ListByCollection(ctx, tenantID, collectionID, offset, limit)
}

func (s *collectionService) BatchUploadFiles(ctx context.Context, tenantID, collectionID, userID uuid.UUID, files []BatchUploadFileInput) ([]BatchUploadResult, error) {
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermEditor); err != nil {
		return nil, err
	}

	log.Printf("collectionService.BatchUploadFiles: uploading %d files to collection %s by user %s",
		len(files), collectionID, userID)

	results := make([]BatchUploadResult, 0, len(files))
	for _, f := range files {
		result := BatchUploadResult{FileName: f.Header.Filename}

		meta, err := s.fileSvc.Upload(ctx, FileUploadInput{
			TenantID:   tenantID,
			UploadedBy: userID,
			File:       f.File,
			Header:     f.Header,
		})
		if err != nil {
			log.Printf("collectionService.BatchUploadFiles: failed to upload file %s: %v",
				f.Header.Filename, err)
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		// Associate file with collection
		cf := &domain.CollectionFile{
			CollectionID: collectionID,
			FileID:       meta.ID,
			TenantID:     tenantID,
			AddedBy:      userID,
		}
		if err := s.fileRepo.Add(ctx, cf); err != nil {
			result.Error = fmt.Sprintf("uploaded but failed to add to collection: %s", err.Error())
			result.File = meta
			results = append(results, result)
			continue
		}

		result.Success = true
		result.File = meta
		results = append(results, result)
	}

	return results, nil
}

func (s *collectionService) RemoveFile(ctx context.Context, tenantID, collectionID, fileID, userID uuid.UUID) error {
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermEditor); err != nil {
		return err
	}
	return s.fileRepo.Remove(ctx, collectionID, fileID)
}

func (s *collectionService) AddFileToCollection(ctx context.Context, tenantID, collectionID, fileID, userID uuid.UUID) error {
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermEditor); err != nil {
		return err
	}
	cf := &domain.CollectionFile{
		CollectionID: collectionID,
		FileID:       fileID,
		TenantID:     tenantID,
		AddedBy:      userID,
	}
	return s.fileRepo.Add(ctx, cf)
}

func (s *collectionService) SetPermission(ctx context.Context, input SetPermissionInput) error {
	if !domain.ValidCollectionPermissions[input.Permission] {
		return domain.ErrInvalidPermission
	}
	if err := s.requirePermission(ctx, input.CollectionID, input.GrantedBy, domain.CollectionPermOwner); err != nil {
		return err
	}

	log.Printf("collectionService.SetPermission: setting %s permission for user %s on collection %s by user %s",
		input.Permission, input.UserID, input.CollectionID, input.GrantedBy)

	perm := &domain.CollectionPermissionEntry{
		CollectionID: input.CollectionID,
		TenantID:     input.TenantID,
		UserID:       input.UserID,
		Permission:   input.Permission,
		GrantedBy:    input.GrantedBy,
	}
	return s.permRepo.Upsert(ctx, perm)
}

func (s *collectionService) ListPermissions(ctx context.Context, tenantID, collectionID, userID uuid.UUID, offset, limit int) ([]domain.CollectionPermissionEntry, int, error) {
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermOwner); err != nil {
		return nil, 0, err
	}
	return s.permRepo.ListByCollection(ctx, collectionID, offset, limit)
}

func (s *collectionService) RemovePermission(ctx context.Context, tenantID, collectionID, targetUserID, userID uuid.UUID) error {
	if targetUserID == userID {
		return domain.ErrSelfPermissionRemoval
	}
	if err := s.requirePermission(ctx, collectionID, userID, domain.CollectionPermOwner); err != nil {
		return err
	}
	log.Printf("collectionService.RemovePermission: removing permission for user %s on collection %s by user %s",
		targetUserID, collectionID, userID)
	return s.permRepo.Delete(ctx, collectionID, targetUserID)
}
