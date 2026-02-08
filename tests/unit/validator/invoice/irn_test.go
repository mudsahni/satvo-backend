package invoice_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"satvos/internal/domain"
	"satvos/internal/validator/invoice"
)

func TestDeriveFinancialYear(t *testing.T) {
	tests := []struct {
		date     string
		expected string
	}{
		{"01-03-2025", "2024-25"},  // March → previous FY
		{"01/03/2025", "2024-25"},  // slash variant
		{"01-04-2025", "2025-26"},  // April → current FY
		{"15-12-2024", "2024-25"},  // December → current FY
		{"15/01/2025", "2024-25"},  // January → previous FY
		{"30-06-2023", "2023-24"},  // June
	}
	for _, tc := range tests {
		t.Run(tc.date, func(t *testing.T) {
			fy, err := invoice.DeriveFinancialYear(tc.date)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, fy)
		})
	}

	t.Run("invalid_date", func(t *testing.T) {
		_, err := invoice.DeriveFinancialYear("not-a-date")
		assert.Error(t, err)
	})
}

func TestComputeIRNHash(t *testing.T) {
	// Known input
	gstin := "29ABCDE1234F1Z5"
	invoiceNo := "INV-001"
	fy := "2024-25"

	hash := invoice.ComputeIRNHash(gstin, invoiceNo, fy)

	// Verify it's 64-char lowercase hex
	assert.Len(t, hash, 64)
	assert.Regexp(t, `^[0-9a-f]{64}$`, hash)

	// Verify against known SHA-256
	expected := sha256.Sum256([]byte(gstin + invoiceNo + fy))
	assert.Equal(t, fmt.Sprintf("%x", expected), hash)
}

func TestFmtInvoiceIRN(t *testing.T) {
	v := findFormatValidator("fmt.invoice.irn")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid_irn", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed, "empty IRN should skip")
	})

	t.Run("fail_too_short", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = "abc123"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("pass_uppercase_normalized", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = "FC3C01B73E8C2A0C57BFC195A264C1CDD6DA3ACF4FBDB5E5E06B90494D1B8568"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed, "uppercase IRN should pass after lowercase normalization")
	})

	t.Run("fail_non_hex", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = "zzzz01b73e8c2a0c57bfc195a264c1cdd6da3acf4fbdb5e5e06b90494d1b8568"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("severity_error", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityError, v.Severity())
	})
}

func TestFmtInvoiceAckNumber(t *testing.T) {
	v := findFormatValidator("fmt.invoice.ack_number")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_numeric", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.AcknowledgementNumber = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_alpha", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.AcknowledgementNumber = "ABC123"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("severity_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})
}

func TestFmtInvoiceAckDate(t *testing.T) {
	v := findFormatValidator("fmt.invoice.ack_date")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid_date", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.AcknowledgementDate = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid_date", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.AcknowledgementDate = "not-a-date"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("severity_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})
}

func TestXfInvoiceIRNHash(t *testing.T) {
	v := findCrossFieldValidator("xf.invoice.irn_hash")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct_hash", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_wrong_hash", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = "0000000000000000000000000000000000000000000000000000000000000000"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty_irn", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed, "should skip when IRN is empty")
	})

	t.Run("skip_empty_date", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.InvoiceDate = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed, "should skip when date is empty")
	})

	t.Run("skip_empty_gstin", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.GSTIN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed, "should skip when GSTIN is empty")
	})

	t.Run("is_reconciliation_critical", func(t *testing.T) {
		assert.True(t, v.ReconciliationCritical())
	})

	t.Run("severity_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})
}

func TestLogicInvoiceIRNExpected(t *testing.T) {
	v := findLogicalValidator("logic.invoice.irn_expected")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_irn_present", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("warn_missing_irn_b2b", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed, "should warn when IRN missing on B2B invoice")
	})

	t.Run("skip_no_seller_gstin", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.IRN = ""
		inv.Seller.GSTIN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed, "should skip when no seller GSTIN")
	})

	t.Run("severity_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})

	t.Run("not_reconciliation_critical", func(t *testing.T) {
		assert.False(t, v.ReconciliationCritical())
	})
}

func TestIRNValidatorCounts(t *testing.T) {
	assert.Len(t, invoice.IRNFormatValidators(), 3)
	assert.Len(t, invoice.IRNCrossFieldValidators(), 1)
	assert.Len(t, invoice.IRNLogicalValidators(), 1)
}
