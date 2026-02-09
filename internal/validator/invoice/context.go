package invoice

import (
	"context"

	"github.com/google/uuid"
)

type contextKey int

const (
	tenantIDKey  contextKey = iota
	documentIDKey
)

// WithValidationContext enriches the context with tenantID and docID for validators that need DB access.
func WithValidationContext(ctx context.Context, tenantID, docID uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, tenantIDKey, tenantID)
	ctx = context.WithValue(ctx, documentIDKey, docID)
	return ctx
}

// TenantIDFromContext extracts the tenant ID set by WithValidationContext.
func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(tenantIDKey).(uuid.UUID)
	return v, ok
}

// DocumentIDFromContext extracts the document ID set by WithValidationContext.
func DocumentIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(documentIDKey).(uuid.UUID)
	return v, ok
}
