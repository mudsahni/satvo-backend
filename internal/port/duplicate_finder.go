package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DuplicateMatch holds enough information about a matching document for an actionable warning message.
type DuplicateMatch struct {
	DocumentName string    `db:"name"`
	CreatedAt    time.Time `db:"created_at"`
}

// DuplicateInvoiceFinder checks for other documents with the same seller GSTIN + invoice number.
type DuplicateInvoiceFinder interface {
	FindDuplicates(ctx context.Context, tenantID, excludeDocID uuid.UUID,
		sellerGSTIN, invoiceNumber string) ([]DuplicateMatch, error)
}
