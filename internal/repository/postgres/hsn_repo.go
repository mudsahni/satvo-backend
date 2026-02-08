package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"

	"satvos/internal/port"
)

type hsnRepo struct {
	db *sqlx.DB
}

// NewHSNRepo creates a new PostgreSQL-backed HSNRepository.
func NewHSNRepo(db *sqlx.DB) port.HSNRepository {
	return &hsnRepo{db: db}
}

func (r *hsnRepo) LoadAll(ctx context.Context) ([]port.HSNEntry, error) {
	var entries []port.HSNEntry
	err := r.db.SelectContext(ctx, &entries,
		`SELECT code, description, gst_rate, condition_desc
		 FROM hsn_codes
		 WHERE effective_to IS NULL OR effective_to >= CURRENT_DATE
		 ORDER BY code, gst_rate`)
	if err != nil {
		return nil, err
	}
	return entries, nil
}
