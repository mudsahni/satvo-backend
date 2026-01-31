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

type documentTagRepo struct {
	db *sqlx.DB
}

// NewDocumentTagRepo creates a new PostgreSQL-backed DocumentTagRepository.
func NewDocumentTagRepo(db *sqlx.DB) port.DocumentTagRepository {
	return &documentTagRepo{db: db}
}

func (r *documentTagRepo) CreateBatch(ctx context.Context, tags []domain.DocumentTag) error {
	if len(tags) == 0 {
		return nil
	}

	now := time.Now().UTC()
	valueStrings := make([]string, 0, len(tags))
	valueArgs := make([]interface{}, 0, len(tags)*5)

	for i, tag := range tags {
		tag.CreatedAt = now
		base := i * 5
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5))
		valueArgs = append(valueArgs, tag.ID, tag.DocumentID, tag.TenantID, tag.Key, tag.Value)
	}

	query := fmt.Sprintf(
		`INSERT INTO document_tags (id, document_id, tenant_id, key, value) VALUES %s`,
		strings.Join(valueStrings, ", "))

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("documentTagRepo.CreateBatch: %w", err)
	}
	return nil
}

func (r *documentTagRepo) ListByDocument(ctx context.Context, documentID uuid.UUID) ([]domain.DocumentTag, error) {
	var tags []domain.DocumentTag
	err := r.db.SelectContext(ctx, &tags,
		"SELECT * FROM document_tags WHERE document_id = $1 ORDER BY key, value",
		documentID)
	if err != nil {
		return nil, fmt.Errorf("documentTagRepo.ListByDocument: %w", err)
	}
	return tags, nil
}

func (r *documentTagRepo) SearchByTag(ctx context.Context, tenantID uuid.UUID, key, value string, offset, limit int) ([]domain.Document, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(DISTINCT d.id) FROM documents d
		 INNER JOIN document_tags dt ON dt.document_id = d.id
		 WHERE dt.tenant_id = $1 AND dt.key = $2 AND dt.value = $3`,
		tenantID, key, value)
	if err != nil {
		return nil, 0, fmt.Errorf("documentTagRepo.SearchByTag count: %w", err)
	}

	var docs []domain.Document
	err = r.db.SelectContext(ctx, &docs,
		`SELECT DISTINCT d.* FROM documents d
		 INNER JOIN document_tags dt ON dt.document_id = d.id
		 WHERE dt.tenant_id = $1 AND dt.key = $2 AND dt.value = $3
		 ORDER BY d.created_at DESC LIMIT $4 OFFSET $5`,
		tenantID, key, value, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("documentTagRepo.SearchByTag: %w", err)
	}
	return docs, total, nil
}

func (r *documentTagRepo) DeleteByDocument(ctx context.Context, documentID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM document_tags WHERE document_id = $1", documentID)
	if err != nil {
		return fmt.Errorf("documentTagRepo.DeleteByDocument: %w", err)
	}
	return nil
}
