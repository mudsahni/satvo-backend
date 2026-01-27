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

type userRepo struct {
	db *sqlx.DB
}

// NewUserRepo creates a new PostgreSQL-backed UserRepository.
func NewUserRepo(db *sqlx.DB) port.UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {
	user.ID = uuid.New()
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	query := `INSERT INTO users (id, tenant_id, email, password_hash, full_name, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.TenantID, user.Email, user.PasswordHash, user.FullName,
		user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return domain.ErrDuplicateEmail
		}
		return fmt.Errorf("userRepo.Create: %w", err)
	}
	return nil
}

func (r *userRepo) GetByID(ctx context.Context, tenantID, userID uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user,
		"SELECT * FROM users WHERE id = $1 AND tenant_id = $2", userID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("userRepo.GetByID: %w", err)
	}
	return &user, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user,
		"SELECT * FROM users WHERE tenant_id = $1 AND email = $2", tenantID, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("userRepo.GetByEmail: %w", err)
	}
	return &user, nil
}

func (r *userRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.User, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM users WHERE tenant_id = $1", tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("userRepo.ListByTenant count: %w", err)
	}

	var users []domain.User
	err = r.db.SelectContext(ctx, &users,
		"SELECT * FROM users WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("userRepo.ListByTenant: %w", err)
	}
	return users, total, nil
}

func (r *userRepo) Update(ctx context.Context, user *domain.User) error {
	user.UpdatedAt = time.Now().UTC()
	query := `UPDATE users SET email = $1, full_name = $2, role = $3, is_active = $4, updated_at = $5
		WHERE id = $6 AND tenant_id = $7`
	result, err := r.db.ExecContext(ctx, query,
		user.Email, user.FullName, user.Role, user.IsActive, user.UpdatedAt, user.ID, user.TenantID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return domain.ErrDuplicateEmail
		}
		return fmt.Errorf("userRepo.Update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *userRepo) Delete(ctx context.Context, tenantID, userID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM users WHERE id = $1 AND tenant_id = $2", userID, tenantID)
	if err != nil {
		return fmt.Errorf("userRepo.Delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
