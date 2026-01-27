package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"satvos/internal/domain"
	"satvos/internal/port"
)

type tenantRepo struct {
	db *sqlx.DB
}

// NewTenantRepo creates a new PostgreSQL-backed TenantRepository.
func NewTenantRepo(db *sqlx.DB) port.TenantRepository {
	return &tenantRepo{db: db}
}

func (r *tenantRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	tenant.ID = uuid.New()
	now := time.Now().UTC()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now

	query := `INSERT INTO tenants (id, name, slug, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.ExecContext(ctx, query,
		tenant.ID, tenant.Name, tenant.Slug, tenant.IsActive, tenant.CreatedAt, tenant.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "slug") {
			return domain.ErrDuplicateTenantSlug
		}
		return fmt.Errorf("tenantRepo.Create: %w", err)
	}
	return nil
}

func (r *tenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.GetContext(ctx, &tenant, "SELECT * FROM tenants WHERE id = $1", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("tenantRepo.GetByID: %w", err)
	}
	return &tenant, nil
}

func (r *tenantRepo) GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.GetContext(ctx, &tenant, "SELECT * FROM tenants WHERE slug = $1", slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("tenantRepo.GetBySlug: %w", err)
	}
	return &tenant, nil
}

func (r *tenantRepo) List(ctx context.Context, offset, limit int) ([]domain.Tenant, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM tenants")
	if err != nil {
		return nil, 0, fmt.Errorf("tenantRepo.List count: %w", err)
	}

	var tenants []domain.Tenant
	err = r.db.SelectContext(ctx, &tenants,
		"SELECT * FROM tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("tenantRepo.List: %w", err)
	}
	return tenants, total, nil
}

func (r *tenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	tenant.UpdatedAt = time.Now().UTC()
	query := `UPDATE tenants SET name = $1, slug = $2, is_active = $3, updated_at = $4 WHERE id = $5`
	result, err := r.db.ExecContext(ctx, query,
		tenant.Name, tenant.Slug, tenant.IsActive, tenant.UpdatedAt, tenant.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "slug") {
			return domain.ErrDuplicateTenantSlug
		}
		return fmt.Errorf("tenantRepo.Update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *tenantRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM tenants WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("tenantRepo.Delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
