package invoice_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"satvos/internal/domain"
	"satvos/internal/port"
	"satvos/internal/validator/invoice"
)

// mockDuplicateFinder is a hand-written mock for port.DuplicateInvoiceFinder.
type mockDuplicateFinder struct {
	matches []port.DuplicateMatch
	err     error
}

func (m *mockDuplicateFinder) FindDuplicates(_ context.Context, _, _ uuid.UUID, _, _ string) ([]port.DuplicateMatch, error) {
	return m.matches, m.err
}

// --- Context helper tests ---

func TestValidationContext_RoundTrip(t *testing.T) {
	tenantID := uuid.New()
	docID := uuid.New()

	ctx := invoice.WithValidationContext(context.Background(), tenantID, docID)

	gotTenant, ok := invoice.TenantIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, tenantID, gotTenant)

	gotDoc, ok := invoice.DocumentIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, docID, gotDoc)
}

func TestValidationContext_Missing(t *testing.T) {
	ctx := context.Background()

	gotTenant, ok := invoice.TenantIDFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, uuid.UUID{}, gotTenant)

	gotDoc, ok := invoice.DocumentIDFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, uuid.UUID{}, gotDoc)
}

// --- Duplicate validator tests ---

func TestDuplicateInvoiceValidator_NoDuplicates(t *testing.T) {
	finder := &mockDuplicateFinder{matches: nil}
	v := invoice.DuplicateInvoiceValidator(finder)
	ctx := invoice.WithValidationContext(context.Background(), uuid.New(), uuid.New())

	inv := validInvoice()
	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed)
	assert.Contains(t, results[0].Message, "no duplicate")
}

func TestDuplicateInvoiceValidator_OneDuplicate(t *testing.T) {
	finder := &mockDuplicateFinder{
		matches: []port.DuplicateMatch{
			{DocumentName: "Invoice-ABC.pdf", CreatedAt: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)},
		},
	}
	v := invoice.DuplicateInvoiceValidator(finder)
	ctx := invoice.WithValidationContext(context.Background(), uuid.New(), uuid.New())

	inv := validInvoice()
	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.False(t, results[0].Passed)
	assert.Contains(t, results[0].Message, "Invoice-ABC.pdf")
	assert.Contains(t, results[0].Message, "2025-01-10")
	assert.Equal(t, "1 duplicate(s) found", results[0].ActualValue)
}

func TestDuplicateInvoiceValidator_MultipleDuplicates(t *testing.T) {
	finder := &mockDuplicateFinder{
		matches: []port.DuplicateMatch{
			{DocumentName: "Invoice-A.pdf", CreatedAt: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
			{DocumentName: "Invoice-B.pdf", CreatedAt: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)},
		},
	}
	v := invoice.DuplicateInvoiceValidator(finder)
	ctx := invoice.WithValidationContext(context.Background(), uuid.New(), uuid.New())

	inv := validInvoice()
	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.False(t, results[0].Passed)
	assert.Contains(t, results[0].Message, "Invoice-A.pdf")
	assert.Contains(t, results[0].Message, "Invoice-B.pdf")
	assert.Equal(t, "2 duplicate(s) found", results[0].ActualValue)
}

func TestDuplicateInvoiceValidator_EmptySellerGSTIN(t *testing.T) {
	finder := &mockDuplicateFinder{}
	v := invoice.DuplicateInvoiceValidator(finder)
	ctx := invoice.WithValidationContext(context.Background(), uuid.New(), uuid.New())

	inv := validInvoice()
	inv.Seller.GSTIN = ""
	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed)
	assert.Contains(t, results[0].Message, "skipping")
}

func TestDuplicateInvoiceValidator_EmptyInvoiceNumber(t *testing.T) {
	finder := &mockDuplicateFinder{}
	v := invoice.DuplicateInvoiceValidator(finder)
	ctx := invoice.WithValidationContext(context.Background(), uuid.New(), uuid.New())

	inv := validInvoice()
	inv.Invoice.InvoiceNumber = ""
	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed)
	assert.Contains(t, results[0].Message, "skipping")
}

func TestDuplicateInvoiceValidator_MissingContext(t *testing.T) {
	finder := &mockDuplicateFinder{}
	v := invoice.DuplicateInvoiceValidator(finder)
	ctx := context.Background() // no WithValidationContext

	inv := validInvoice()
	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed)
	assert.Contains(t, results[0].Message, "context missing")
}

func TestDuplicateInvoiceValidator_FinderError(t *testing.T) {
	finder := &mockDuplicateFinder{err: errors.New("db connection failed")}
	v := invoice.DuplicateInvoiceValidator(finder)
	ctx := invoice.WithValidationContext(context.Background(), uuid.New(), uuid.New())

	inv := validInvoice()
	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed)
	assert.Contains(t, results[0].Message, "unavailable")
}

func TestDuplicateInvoiceValidator_Metadata(t *testing.T) {
	finder := &mockDuplicateFinder{}
	v := invoice.DuplicateInvoiceValidator(finder)

	assert.Equal(t, "logic.invoice.duplicate", v.RuleKey())
	assert.Equal(t, "Logical: Duplicate Invoice Detection", v.RuleName())
	assert.Equal(t, domain.ValidationRuleCustom, v.RuleType())
	assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	assert.False(t, v.ReconciliationCritical())
}

// --- Integration checks ---

func TestAllBuiltinValidators_StillReturns56(t *testing.T) {
	all := invoice.AllBuiltinValidators()
	assert.Len(t, all, 56, "Duplicate validator should NOT be in AllBuiltinValidators()")
}

func TestDuplicateValidator_NoKeyConflict(t *testing.T) {
	builtinKeys := make(map[string]bool)
	for _, v := range invoice.AllBuiltinValidators() {
		builtinKeys[v.RuleKey()] = true
	}

	finder := &mockDuplicateFinder{}
	dupKey := invoice.DuplicateInvoiceValidator(finder).RuleKey()
	assert.False(t, builtinKeys[dupKey],
		"duplicate validator key %q conflicts with an existing builtin validator", dupKey)
}
