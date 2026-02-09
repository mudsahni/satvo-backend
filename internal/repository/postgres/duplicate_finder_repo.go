package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"satvos/internal/port"
)

type duplicateFinderRepo struct {
	db *sqlx.DB
}

// NewDuplicateFinderRepo creates a new PostgreSQL-backed DuplicateInvoiceFinder.
func NewDuplicateFinderRepo(db *sqlx.DB) port.DuplicateInvoiceFinder {
	return &duplicateFinderRepo{db: db}
}

func (r *duplicateFinderRepo) FindDuplicates(
	ctx context.Context,
	tenantID, excludeDocID uuid.UUID,
	sellerGSTIN, invoiceNumber string,
) ([]port.DuplicateMatch, error) {
	var matches []port.DuplicateMatch
	err := r.db.SelectContext(ctx, &matches, `
		SELECT name, created_at
		FROM documents
		WHERE tenant_id = $1
		  AND id != $2
		  AND parsing_status = 'completed'
		  AND structured_data @> jsonb_build_object(
		      'seller', jsonb_build_object('gstin', $3),
		      'invoice', jsonb_build_object('invoice_number', $4)
		  )
		ORDER BY created_at DESC
		LIMIT 5`,
		tenantID, excludeDocID, sellerGSTIN, invoiceNumber,
	)
	if err != nil {
		return nil, err
	}
	return matches, nil
}
