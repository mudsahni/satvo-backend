package port

import "context"

// HSNEntry represents a single HSN code entry with its GST rate.
type HSNEntry struct {
	Code          string  `db:"code"`
	Description   string  `db:"description"`
	GSTRate       float64 `db:"gst_rate"`
	ConditionDesc string  `db:"condition_desc"`
}

// HSNRepository defines the contract for HSN code data access.
type HSNRepository interface {
	LoadAll(ctx context.Context) ([]HSNEntry, error)
}
