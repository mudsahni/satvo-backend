package port

import (
	"context"

	"github.com/google/uuid"

	"satvos/internal/domain"
)

// DocumentAuditRepository defines the contract for document audit log persistence.
type DocumentAuditRepository interface {
	Create(ctx context.Context, entry *domain.DocumentAuditEntry) error
	ListByDocument(ctx context.Context, tenantID, documentID uuid.UUID, offset, limit int) ([]domain.DocumentAuditEntry, int, error)
}
