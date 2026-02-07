package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"satvos/internal/domain"
	"satvos/internal/port"
)

type statsRepo struct {
	db *sqlx.DB
}

// NewStatsRepo creates a new PostgreSQL-backed StatsRepository.
func NewStatsRepo(db *sqlx.DB) port.StatsRepository {
	return &statsRepo{db: db}
}

const tenantDocStatsQuery = `SELECT
	COUNT(*) AS total_documents,
	COUNT(CASE WHEN parsing_status = 'completed' THEN 1 END) AS parsing_completed,
	COUNT(CASE WHEN parsing_status = 'failed' THEN 1 END) AS parsing_failed,
	COUNT(CASE WHEN parsing_status = 'processing' THEN 1 END) AS parsing_processing,
	COUNT(CASE WHEN parsing_status = 'pending' THEN 1 END) AS parsing_pending,
	COUNT(CASE WHEN parsing_status = 'queued' THEN 1 END) AS parsing_queued,
	COUNT(CASE WHEN validation_status = 'valid' THEN 1 END) AS validation_valid,
	COUNT(CASE WHEN validation_status = 'warning' THEN 1 END) AS validation_warning,
	COUNT(CASE WHEN validation_status = 'invalid' THEN 1 END) AS validation_invalid,
	COUNT(CASE WHEN reconciliation_status = 'valid' THEN 1 END) AS reconciliation_valid,
	COUNT(CASE WHEN reconciliation_status = 'warning' THEN 1 END) AS reconciliation_warning,
	COUNT(CASE WHEN reconciliation_status = 'invalid' THEN 1 END) AS reconciliation_invalid,
	COUNT(CASE WHEN review_status = 'pending' THEN 1 END) AS review_pending,
	COUNT(CASE WHEN review_status = 'approved' THEN 1 END) AS review_approved,
	COUNT(CASE WHEN review_status = 'rejected' THEN 1 END) AS review_rejected
FROM documents WHERE tenant_id = $1`

const userDocStatsQuery = `SELECT
	COUNT(*) AS total_documents,
	COUNT(CASE WHEN d.parsing_status = 'completed' THEN 1 END) AS parsing_completed,
	COUNT(CASE WHEN d.parsing_status = 'failed' THEN 1 END) AS parsing_failed,
	COUNT(CASE WHEN d.parsing_status = 'processing' THEN 1 END) AS parsing_processing,
	COUNT(CASE WHEN d.parsing_status = 'pending' THEN 1 END) AS parsing_pending,
	COUNT(CASE WHEN d.parsing_status = 'queued' THEN 1 END) AS parsing_queued,
	COUNT(CASE WHEN d.validation_status = 'valid' THEN 1 END) AS validation_valid,
	COUNT(CASE WHEN d.validation_status = 'warning' THEN 1 END) AS validation_warning,
	COUNT(CASE WHEN d.validation_status = 'invalid' THEN 1 END) AS validation_invalid,
	COUNT(CASE WHEN d.reconciliation_status = 'valid' THEN 1 END) AS reconciliation_valid,
	COUNT(CASE WHEN d.reconciliation_status = 'warning' THEN 1 END) AS reconciliation_warning,
	COUNT(CASE WHEN d.reconciliation_status = 'invalid' THEN 1 END) AS reconciliation_invalid,
	COUNT(CASE WHEN d.review_status = 'pending' THEN 1 END) AS review_pending,
	COUNT(CASE WHEN d.review_status = 'approved' THEN 1 END) AS review_approved,
	COUNT(CASE WHEN d.review_status = 'rejected' THEN 1 END) AS review_rejected
FROM documents d
INNER JOIN collection_permissions cp ON cp.collection_id = d.collection_id
WHERE d.tenant_id = $1 AND cp.user_id = $2`

func (r *statsRepo) GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*domain.Stats, error) {
	var stats domain.Stats
	if err := r.db.GetContext(ctx, &stats, tenantDocStatsQuery, tenantID); err != nil {
		return nil, fmt.Errorf("statsRepo.GetTenantStats docs: %w", err)
	}

	var collectionsCount int
	if err := r.db.GetContext(ctx, &collectionsCount,
		"SELECT COUNT(*) FROM collections WHERE tenant_id = $1", tenantID); err != nil {
		return nil, fmt.Errorf("statsRepo.GetTenantStats collections: %w", err)
	}
	stats.TotalCollections = collectionsCount

	return &stats, nil
}

func (r *statsRepo) GetUserStats(ctx context.Context, tenantID, userID uuid.UUID) (*domain.Stats, error) {
	var stats domain.Stats
	if err := r.db.GetContext(ctx, &stats, userDocStatsQuery, tenantID, userID); err != nil {
		return nil, fmt.Errorf("statsRepo.GetUserStats docs: %w", err)
	}

	var collectionsCount int
	if err := r.db.GetContext(ctx, &collectionsCount,
		`SELECT COUNT(DISTINCT cp.collection_id) FROM collection_permissions cp
		 INNER JOIN collections c ON c.id = cp.collection_id
		 WHERE c.tenant_id = $1 AND cp.user_id = $2`, tenantID, userID); err != nil {
		return nil, fmt.Errorf("statsRepo.GetUserStats collections: %w", err)
	}
	stats.TotalCollections = collectionsCount

	return &stats, nil
}
