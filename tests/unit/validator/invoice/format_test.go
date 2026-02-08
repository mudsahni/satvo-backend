package invoice_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"satvos/internal/domain"
	"satvos/internal/validator/invoice"
)

func TestFormatValidators_Count(t *testing.T) {
	assert.Len(t, invoice.FormatValidators(), 12)
}

func TestFormatValidators_Metadata(t *testing.T) {
	for _, v := range invoice.FormatValidators() {
		assert.NotEmpty(t, v.RuleKey())
		assert.NotEmpty(t, v.RuleName())
		assert.Equal(t, domain.ValidationRuleRegex, v.RuleType())
	}
}

func TestFormat_SellerGSTIN(t *testing.T) {
	v := findFormatValidator("fmt.seller.gstin")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.GSTIN = "INVALID"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_short", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.GSTIN = "123"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.GSTIN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed) // empty skips
	})

	t.Run("is_reconciliation_critical", func(t *testing.T) {
		assert.True(t, v.ReconciliationCritical())
	})
}

func TestFormat_BuyerGSTIN(t *testing.T) {
	v := findFormatValidator("fmt.buyer.gstin")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.GSTIN = "NOTGSTIN"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.GSTIN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_SellerPAN(t *testing.T) {
	v := findFormatValidator("fmt.seller.pan")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.PAN = "123"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_lowercase", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.PAN = "abcde1234f"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.PAN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_BuyerPAN(t *testing.T) {
	v := findFormatValidator("fmt.buyer.pan")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_short", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.PAN = "SHORT"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.PAN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_SellerStateCode(t *testing.T) {
	v := findFormatValidator("fmt.seller.state_code")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_zero", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = "00"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_out_of_range", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = "99"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_letters", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = "AB"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_single_digit", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = "0" // only 1 char, must be exactly 2
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_boundary_01", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = "01"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_boundary_38", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = "38"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_boundary_39", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = "39"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("is_reconciliation_critical", func(t *testing.T) {
		assert.True(t, v.ReconciliationCritical())
	})
}

func TestFormat_BuyerStateCode(t *testing.T) {
	v := findFormatValidator("fmt.buyer.state_code")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.StateCode = "39"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.StateCode = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_InvoiceDate(t *testing.T) {
	v := findFormatValidator("fmt.invoice.date")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_dd_mm_yyyy", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice()) // "15/01/2025"
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_iso_format", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.InvoiceDate = "2025-01-15"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_dd_Mon_yyyy", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.InvoiceDate = "15 Jan 2025"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.InvoiceDate = "not-a-date"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.InvoiceDate = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_DueDate(t *testing.T) {
	v := findFormatValidator("fmt.invoice.due_date")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.DueDate = "bad"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.DueDate = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("severity_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})
}

func TestFormat_Currency(t *testing.T) {
	v := findFormatValidator("fmt.invoice.currency")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_INR", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_USD", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.Currency = "USD"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_case_insensitive", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.Currency = "inr"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed) // normalized to INR
	})

	t.Run("fail_unknown", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.Currency = "XYZ"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.Currency = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_IFSC(t *testing.T) {
	v := findFormatValidator("fmt.payment.ifsc")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_invalid", func(t *testing.T) {
		inv := validInvoice()
		inv.Payment.IFSCCode = "INVALID"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Payment.IFSCCode = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_AccountNumber(t *testing.T) {
	v := findFormatValidator("fmt.payment.account_no")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_valid", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice()) // "1234567890" (10 digits)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_too_short", func(t *testing.T) {
		inv := validInvoice()
		inv.Payment.AccountNumber = "12345" // 5 digits, min is 9
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_letters", func(t *testing.T) {
		inv := validInvoice()
		inv.Payment.AccountNumber = "abc123456"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Payment.AccountNumber = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestFormat_LineItemHSNSAC(t *testing.T) {
	v := findFormatValidator("fmt.line_item.hsn_sac")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_4_digit", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice()) // "8471"
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_8_digit", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].HSNSACCode = "84713010"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_too_short", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].HSNSACCode = "12" // 2 digits, min is 4
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_letters", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].HSNSACCode = "ABCD"
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("skip_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].HSNSACCode = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("multi_item", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems = append(inv.LineItems, invoice.LineItem{
			HSNSACCode: "999999",
		})
		results := v.Validate(ctx, inv)
		require.Len(t, results, 2)
		assert.True(t, results[0].Passed)
		assert.True(t, results[1].Passed)
	})
}

func TestFormatValidators_EmptyFieldsSkip(t *testing.T) {
	ctx := context.Background()
	// An invoice with all format-checked fields empty should pass all format validators
	inv := &invoice.GSTInvoice{
		LineItems: []invoice.LineItem{{}},
	}

	for _, v := range invoice.AllBuiltinValidators() {
		if v.RuleType() != domain.ValidationRuleRegex {
			continue
		}
		results := v.Validate(ctx, inv)
		for _, r := range results {
			assert.True(t, r.Passed, "format validator %s should skip empty fields", v.RuleKey())
		}
	}
}
