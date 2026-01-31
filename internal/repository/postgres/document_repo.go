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

type documentRepo struct {
	db *sqlx.DB
}

// NewDocumentRepo creates a new PostgreSQL-backed DocumentRepository.
func NewDocumentRepo(db *sqlx.DB) port.DocumentRepository {
	return &documentRepo{db: db}
}

func (r *documentRepo) Create(ctx context.Context, doc *domain.Document) error {
	now := time.Now().UTC()
	doc.CreatedAt = now
	doc.UpdatedAt = now

	query := `INSERT INTO documents (
		id, tenant_id, collection_id, file_id, document_type,
		parser_model, parser_prompt, structured_data, confidence_scores,
		parsing_status, parsing_error, parsed_at,
		review_status, reviewed_by, reviewed_at, reviewer_notes,
		created_by, created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5,
		$6, $7, $8, $9,
		$10, $11, $12,
		$13, $14, $15, $16,
		$17, $18, $19
	)`

	_, err := r.db.ExecContext(ctx, query,
		doc.ID, doc.TenantID, doc.CollectionID, doc.FileID, doc.DocumentType,
		doc.ParserModel, doc.ParserPrompt, doc.StructuredData, doc.ConfidenceScores,
		doc.ParsingStatus, doc.ParsingError, doc.ParsedAt,
		doc.ReviewStatus, doc.ReviewedBy, doc.ReviewedAt, doc.ReviewerNotes,
		doc.CreatedBy, doc.CreatedAt, doc.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "file_id") {
			return domain.ErrDocumentAlreadyExists
		}
		return fmt.Errorf("documentRepo.Create: %w", err)
	}
	return nil
}

func (r *documentRepo) GetByID(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error) {
	var doc domain.Document
	err := r.db.GetContext(ctx, &doc,
		"SELECT * FROM documents WHERE id = $1 AND tenant_id = $2", docID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrDocumentNotFound
		}
		return nil, fmt.Errorf("documentRepo.GetByID: %w", err)
	}
	return &doc, nil
}

func (r *documentRepo) GetByFileID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.Document, error) {
	var doc domain.Document
	err := r.db.GetContext(ctx, &doc,
		"SELECT * FROM documents WHERE file_id = $1 AND tenant_id = $2", fileID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrDocumentNotFound
		}
		return nil, fmt.Errorf("documentRepo.GetByFileID: %w", err)
	}
	return &doc, nil
}

func (r *documentRepo) ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM documents WHERE tenant_id = $1 AND collection_id = $2",
		tenantID, collectionID)
	if err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListByCollection count: %w", err)
	}

	var docs []domain.Document
	err = r.db.SelectContext(ctx, &docs,
		`SELECT * FROM documents WHERE tenant_id = $1 AND collection_id = $2
		 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
		tenantID, collectionID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListByCollection: %w", err)
	}
	return docs, total, nil
}

func (r *documentRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM documents WHERE tenant_id = $1", tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListByTenant count: %w", err)
	}

	var docs []domain.Document
	err = r.db.SelectContext(ctx, &docs,
		`SELECT * FROM documents WHERE tenant_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListByTenant: %w", err)
	}
	return docs, total, nil
}

func (r *documentRepo) UpdateStructuredData(ctx context.Context, doc *domain.Document) error {
	doc.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE documents SET
			structured_data = $1, confidence_scores = $2,
			parsing_status = $3, parsing_error = $4, parsed_at = $5,
			parser_model = $6, parser_prompt = $7, updated_at = $8
		 WHERE id = $9 AND tenant_id = $10`,
		doc.StructuredData, doc.ConfidenceScores,
		doc.ParsingStatus, doc.ParsingError, doc.ParsedAt,
		doc.ParserModel, doc.ParserPrompt, doc.UpdatedAt,
		doc.ID, doc.TenantID)
	if err != nil {
		return fmt.Errorf("documentRepo.UpdateStructuredData: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

func (r *documentRepo) UpdateReviewStatus(ctx context.Context, doc *domain.Document) error {
	doc.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE documents SET
			review_status = $1, reviewed_by = $2, reviewed_at = $3,
			reviewer_notes = $4, updated_at = $5
		 WHERE id = $6 AND tenant_id = $7`,
		doc.ReviewStatus, doc.ReviewedBy, doc.ReviewedAt,
		doc.ReviewerNotes, doc.UpdatedAt,
		doc.ID, doc.TenantID)
	if err != nil {
		return fmt.Errorf("documentRepo.UpdateReviewStatus: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}

func (r *documentRepo) Delete(ctx context.Context, tenantID, docID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM documents WHERE id = $1 AND tenant_id = $2",
		docID, tenantID)
	if err != nil {
		return fmt.Errorf("documentRepo.Delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}
