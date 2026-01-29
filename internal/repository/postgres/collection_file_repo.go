package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"satvos/internal/domain"
	"satvos/internal/port"
)

type collectionFileRepo struct {
	db *sqlx.DB
}

// NewCollectionFileRepo creates a new PostgreSQL-backed CollectionFileRepository.
func NewCollectionFileRepo(db *sqlx.DB) port.CollectionFileRepository {
	return &collectionFileRepo{db: db}
}

func (r *collectionFileRepo) Add(ctx context.Context, cf *domain.CollectionFile) error {
	cf.AddedAt = time.Now().UTC()

	query := `INSERT INTO collection_files (collection_id, file_id, tenant_id, added_by, added_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(ctx, query,
		cf.CollectionID, cf.FileID, cf.TenantID, cf.AddedBy, cf.AddedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return domain.ErrDuplicateCollectionFile
		}
		return fmt.Errorf("collectionFileRepo.Add: %w", err)
	}
	return nil
}

func (r *collectionFileRepo) Remove(ctx context.Context, collectionID, fileID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM collection_files WHERE collection_id = $1 AND file_id = $2",
		collectionID, fileID)
	if err != nil {
		return fmt.Errorf("collectionFileRepo.Remove: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *collectionFileRepo) ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, offset, limit int) ([]domain.FileMeta, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM collection_files cf
		 INNER JOIN file_metadata fm ON fm.id = cf.file_id
		 WHERE cf.collection_id = $1 AND cf.tenant_id = $2 AND fm.status != $3`,
		collectionID, tenantID, domain.FileStatusDeleted)
	if err != nil {
		return nil, 0, fmt.Errorf("collectionFileRepo.ListByCollection count: %w", err)
	}

	var files []domain.FileMeta
	err = r.db.SelectContext(ctx, &files,
		`SELECT fm.* FROM file_metadata fm
		 INNER JOIN collection_files cf ON cf.file_id = fm.id
		 WHERE cf.collection_id = $1 AND cf.tenant_id = $2 AND fm.status != $3
		 ORDER BY cf.added_at DESC LIMIT $4 OFFSET $5`,
		collectionID, tenantID, domain.FileStatusDeleted, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("collectionFileRepo.ListByCollection: %w", err)
	}
	return files, total, nil
}
