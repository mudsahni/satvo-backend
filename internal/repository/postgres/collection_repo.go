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

type collectionRepo struct {
	db *sqlx.DB
}

// NewCollectionRepo creates a new PostgreSQL-backed CollectionRepository.
func NewCollectionRepo(db *sqlx.DB) port.CollectionRepository {
	return &collectionRepo{db: db}
}

func (r *collectionRepo) Create(ctx context.Context, c *domain.Collection) error {
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	query := `INSERT INTO collections (id, tenant_id, name, description, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.TenantID, c.Name, c.Description, c.CreatedBy, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("collectionRepo.Create: %w", err)
	}
	return nil
}

func (r *collectionRepo) GetByID(ctx context.Context, tenantID, collectionID uuid.UUID) (*domain.Collection, error) {
	var c domain.Collection
	err := r.db.GetContext(ctx, &c,
		"SELECT * FROM collections WHERE id = $1 AND tenant_id = $2", collectionID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCollectionNotFound
		}
		return nil, fmt.Errorf("collectionRepo.GetByID: %w", err)
	}
	return &c, nil
}

func (r *collectionRepo) ListByUser(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Collection, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM collections c
		 INNER JOIN collection_permissions cp ON cp.collection_id = c.id
		 WHERE c.tenant_id = $1 AND cp.user_id = $2`,
		tenantID, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("collectionRepo.ListByUser count: %w", err)
	}

	var collections []domain.Collection
	err = r.db.SelectContext(ctx, &collections,
		`SELECT c.* FROM collections c
		 INNER JOIN collection_permissions cp ON cp.collection_id = c.id
		 WHERE c.tenant_id = $1 AND cp.user_id = $2
		 ORDER BY c.created_at DESC LIMIT $3 OFFSET $4`,
		tenantID, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("collectionRepo.ListByUser: %w", err)
	}
	return collections, total, nil
}

func (r *collectionRepo) Update(ctx context.Context, c *domain.Collection) error {
	c.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE collections SET name = $1, description = $2, updated_at = $3
		 WHERE id = $4 AND tenant_id = $5`,
		c.Name, c.Description, c.UpdatedAt, c.ID, c.TenantID)
	if err != nil {
		return fmt.Errorf("collectionRepo.Update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrCollectionNotFound
	}
	return nil
}

func (r *collectionRepo) Delete(ctx context.Context, tenantID, collectionID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM collections WHERE id = $1 AND tenant_id = $2",
		collectionID, tenantID)
	if err != nil {
		return fmt.Errorf("collectionRepo.Delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrCollectionNotFound
	}
	return nil
}
