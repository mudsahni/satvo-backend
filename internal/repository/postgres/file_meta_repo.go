package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"satvos/internal/domain"
	"satvos/internal/port"
)

type fileMetaRepo struct {
	db *sqlx.DB
}

// NewFileMetaRepo creates a new PostgreSQL-backed FileMetaRepository.
func NewFileMetaRepo(db *sqlx.DB) port.FileMetaRepository {
	return &fileMetaRepo{db: db}
}

func (r *fileMetaRepo) Create(ctx context.Context, meta *domain.FileMeta) error {
	now := time.Now().UTC()
	meta.CreatedAt = now
	meta.UpdatedAt = now

	query := `INSERT INTO file_metadata
		(id, tenant_id, uploaded_by, file_name, original_name, file_type, file_size,
		 s3_bucket, s3_key, content_type, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := r.db.ExecContext(ctx, query,
		meta.ID, meta.TenantID, meta.UploadedBy, meta.FileName, meta.OriginalName,
		meta.FileType, meta.FileSize, meta.S3Bucket, meta.S3Key, meta.ContentType,
		meta.Status, meta.CreatedAt, meta.UpdatedAt)
	if err != nil {
		return fmt.Errorf("fileMetaRepo.Create: %w", err)
	}
	return nil
}

func (r *fileMetaRepo) GetByID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.FileMeta, error) {
	var meta domain.FileMeta
	err := r.db.GetContext(ctx, &meta,
		"SELECT * FROM file_metadata WHERE id = $1 AND tenant_id = $2", fileID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("fileMetaRepo.GetByID: %w", err)
	}
	return &meta, nil
}

func (r *fileMetaRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.FileMeta, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM file_metadata WHERE tenant_id = $1 AND status != $2",
		tenantID, domain.FileStatusDeleted)
	if err != nil {
		return nil, 0, fmt.Errorf("fileMetaRepo.ListByTenant count: %w", err)
	}

	var files []domain.FileMeta
	err = r.db.SelectContext(ctx, &files,
		`SELECT * FROM file_metadata
		 WHERE tenant_id = $1 AND status != $2
		 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
		tenantID, domain.FileStatusDeleted, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("fileMetaRepo.ListByTenant: %w", err)
	}
	return files, total, nil
}

func (r *fileMetaRepo) ListByUploader(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.FileMeta, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM file_metadata WHERE tenant_id = $1 AND uploaded_by = $2 AND status != $3",
		tenantID, userID, domain.FileStatusDeleted)
	if err != nil {
		return nil, 0, fmt.Errorf("fileMetaRepo.ListByUploader count: %w", err)
	}

	var files []domain.FileMeta
	err = r.db.SelectContext(ctx, &files,
		`SELECT * FROM file_metadata
		 WHERE tenant_id = $1 AND uploaded_by = $2 AND status != $3
		 ORDER BY created_at DESC LIMIT $4 OFFSET $5`,
		tenantID, userID, domain.FileStatusDeleted, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("fileMetaRepo.ListByUploader: %w", err)
	}
	return files, total, nil
}

func (r *fileMetaRepo) UpdateStatus(ctx context.Context, tenantID, fileID uuid.UUID, status domain.FileStatus) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE file_metadata SET status = $1, updated_at = $2 WHERE id = $3 AND tenant_id = $4",
		status, time.Now().UTC(), fileID, tenantID)
	if err != nil {
		return fmt.Errorf("fileMetaRepo.UpdateStatus: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *fileMetaRepo) Delete(ctx context.Context, tenantID, fileID uuid.UUID) error {
	return r.UpdateStatus(ctx, tenantID, fileID, domain.FileStatusDeleted)
}
