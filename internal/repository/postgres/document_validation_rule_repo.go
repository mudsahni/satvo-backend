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

type documentValidationRuleRepo struct {
	db *sqlx.DB
}

// NewDocumentValidationRuleRepo creates a new PostgreSQL-backed DocumentValidationRuleRepository.
func NewDocumentValidationRuleRepo(db *sqlx.DB) port.DocumentValidationRuleRepository {
	return &documentValidationRuleRepo{db: db}
}

func (r *documentValidationRuleRepo) Create(ctx context.Context, rule *domain.DocumentValidationRule) error {
	now := time.Now().UTC()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	query := `INSERT INTO document_validation_rules (
		id, tenant_id, collection_id, document_type, rule_name,
		rule_type, rule_config, severity, is_active, created_by,
		created_at, updated_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.db.ExecContext(ctx, query,
		rule.ID, rule.TenantID, rule.CollectionID, rule.DocumentType, rule.RuleName,
		rule.RuleType, rule.RuleConfig, rule.Severity, rule.IsActive, rule.CreatedBy,
		rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return fmt.Errorf("documentValidationRuleRepo.Create: %w", err)
	}
	return nil
}

func (r *documentValidationRuleRepo) GetByID(ctx context.Context, tenantID, ruleID uuid.UUID) (*domain.DocumentValidationRule, error) {
	var rule domain.DocumentValidationRule
	err := r.db.GetContext(ctx, &rule,
		"SELECT * FROM document_validation_rules WHERE id = $1 AND tenant_id = $2",
		ruleID, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrValidationRuleNotFound
		}
		return nil, fmt.Errorf("documentValidationRuleRepo.GetByID: %w", err)
	}
	return &rule, nil
}

func (r *documentValidationRuleRepo) ListByDocumentType(ctx context.Context, tenantID uuid.UUID, docType string, collectionID *uuid.UUID) ([]domain.DocumentValidationRule, error) {
	var rules []domain.DocumentValidationRule
	var err error

	if collectionID != nil {
		err = r.db.SelectContext(ctx, &rules,
			`SELECT * FROM document_validation_rules
			 WHERE tenant_id = $1 AND document_type = $2 AND is_active = TRUE
			   AND (collection_id IS NULL OR collection_id = $3)
			 ORDER BY rule_name`,
			tenantID, docType, *collectionID)
	} else {
		err = r.db.SelectContext(ctx, &rules,
			`SELECT * FROM document_validation_rules
			 WHERE tenant_id = $1 AND document_type = $2 AND is_active = TRUE
			   AND collection_id IS NULL
			 ORDER BY rule_name`,
			tenantID, docType)
	}
	if err != nil {
		return nil, fmt.Errorf("documentValidationRuleRepo.ListByDocumentType: %w", err)
	}
	return rules, nil
}

func (r *documentValidationRuleRepo) Update(ctx context.Context, rule *domain.DocumentValidationRule) error {
	rule.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE document_validation_rules SET
			rule_name = $1, rule_type = $2, rule_config = $3,
			severity = $4, is_active = $5, updated_at = $6
		 WHERE id = $7 AND tenant_id = $8`,
		rule.RuleName, rule.RuleType, rule.RuleConfig,
		rule.Severity, rule.IsActive, rule.UpdatedAt,
		rule.ID, rule.TenantID)
	if err != nil {
		return fmt.Errorf("documentValidationRuleRepo.Update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrValidationRuleNotFound
	}
	return nil
}

func (r *documentValidationRuleRepo) Delete(ctx context.Context, tenantID, ruleID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM document_validation_rules WHERE id = $1 AND tenant_id = $2",
		ruleID, tenantID)
	if err != nil {
		return fmt.Errorf("documentValidationRuleRepo.Delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrValidationRuleNotFound
	}
	return nil
}
