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

type documentValidationResultRepo struct {
	db *sqlx.DB
}

// NewDocumentValidationResultRepo creates a new PostgreSQL-backed DocumentValidationResultRepository.
func NewDocumentValidationResultRepo(db *sqlx.DB) port.DocumentValidationResultRepository {
	return &documentValidationResultRepo{db: db}
}

func (r *documentValidationResultRepo) CreateBatch(ctx context.Context, results []domain.DocumentValidationResult) error {
	if len(results) == 0 {
		return nil
	}

	now := time.Now().UTC()
	valueStrings := make([]string, 0, len(results))
	valueArgs := make([]interface{}, 0, len(results)*9)

	for i, res := range results {
		res.ValidatedAt = now
		base := i * 9
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9))
		valueArgs = append(valueArgs,
			res.ID, res.DocumentID, res.RuleID, res.TenantID,
			res.Passed, res.FieldPath, res.ExpectedValue, res.ActualValue, res.Message)
	}

	query := fmt.Sprintf(
		`INSERT INTO document_validation_results (
			id, document_id, rule_id, tenant_id,
			passed, field_path, expected_value, actual_value, message
		) VALUES %s`,
		strings.Join(valueStrings, ", "))

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("documentValidationResultRepo.CreateBatch: %w", err)
	}
	return nil
}

func (r *documentValidationResultRepo) ListByDocument(ctx context.Context, documentID uuid.UUID) ([]domain.DocumentValidationResult, error) {
	var results []domain.DocumentValidationResult
	err := r.db.SelectContext(ctx, &results,
		"SELECT * FROM document_validation_results WHERE document_id = $1 ORDER BY validated_at",
		documentID)
	if err != nil {
		return nil, fmt.Errorf("documentValidationResultRepo.ListByDocument: %w", err)
	}
	return results, nil
}

func (r *documentValidationResultRepo) DeleteByDocument(ctx context.Context, documentID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM document_validation_results WHERE document_id = $1", documentID)
	if err != nil {
		return fmt.Errorf("documentValidationResultRepo.DeleteByDocument: %w", err)
	}
	return nil
}
