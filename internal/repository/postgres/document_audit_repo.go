package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"satvos/internal/domain"
	"satvos/internal/port"
)

type documentAuditRepo struct {
	db *sqlx.DB
}

// NewDocumentAuditRepo creates a new PostgreSQL-backed DocumentAuditRepository.
func NewDocumentAuditRepo(db *sqlx.DB) port.DocumentAuditRepository {
	return &documentAuditRepo{db: db}
}

func (r *documentAuditRepo) Create(ctx context.Context, entry *domain.DocumentAuditEntry) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO document_audit_log (id, tenant_id, document_id, user_id, action, changes)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.ID, entry.TenantID, entry.DocumentID, entry.UserID, entry.Action, entry.Changes)
	if err != nil {
		return fmt.Errorf("documentAuditRepo.Create: %w", err)
	}
	return nil
}

func (r *documentAuditRepo) ListByDocument(ctx context.Context, tenantID, documentID uuid.UUID, offset, limit int) ([]domain.DocumentAuditEntry, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM document_audit_log WHERE tenant_id = $1 AND document_id = $2`,
		tenantID, documentID)
	if err != nil {
		return nil, 0, fmt.Errorf("documentAuditRepo.ListByDocument count: %w", err)
	}

	var entries []domain.DocumentAuditEntry
	err = r.db.SelectContext(ctx, &entries,
		`SELECT * FROM document_audit_log
		 WHERE tenant_id = $1 AND document_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		tenantID, documentID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("documentAuditRepo.ListByDocument: %w", err)
	}
	return entries, total, nil
}
