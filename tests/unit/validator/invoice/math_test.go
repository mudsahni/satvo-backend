package invoice_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"satvos/internal/domain"
	"satvos/internal/validator/invoice"
)

func TestMathValidators_Count(t *testing.T) {
	assert.Len(t, invoice.MathValidators(), 12) // includes round_off
}

func TestMathValidators_Metadata(t *testing.T) {
	for _, v := range invoice.MathValidators() {
		assert.NotEmpty(t, v.RuleKey())
		assert.NotEmpty(t, v.RuleName())
		assert.Equal(t, domain.ValidationRuleSumCheck, v.RuleType())
	}
}

func TestMath_LineItemTaxableAmount(t *testing.T) {
	v := findMathValidator("math.line_item.taxable_amount")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct", func(t *testing.T) {
		inv := validInvoice()
		// qty=10, price=100, discount=0 → taxable=1000
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_with_discount", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].Discount = 50
		inv.LineItems[0].TaxableAmount = 950 // 10*100 - 50
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].TaxableAmount = 9999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
		assert.Equal(t, "line_items[0].taxable_amount", results[0].FieldPath)
	})

	t.Run("multiple_items", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems = append(inv.LineItems, invoice.LineItem{
			Quantity: 5, UnitPrice: 200, Discount: 0, TaxableAmount: 1000,
		})
		results := v.Validate(ctx, inv)
		require.Len(t, results, 2)
		assert.True(t, results[0].Passed)
		assert.True(t, results[1].Passed)
	})

	t.Run("within_tolerance", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].TaxableAmount = 1000.99 // within ±1.00 tolerance
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestMath_LineItemCGSTAmount(t *testing.T) {
	v := findMathValidator("math.line_item.cgst_amount")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct", func(t *testing.T) {
		inv := validInvoice()
		// taxable=1000, cgst_rate=9 → 90
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_zero_rate", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].CGSTRate = 0
		inv.LineItems[0].CGSTAmount = 0
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].CGSTAmount = 999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestMath_LineItemSGSTAmount(t *testing.T) {
	v := findMathValidator("math.line_item.sgst_amount")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].SGSTAmount = 999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestMath_LineItemIGSTAmount(t *testing.T) {
	v := findMathValidator("math.line_item.igst_amount")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_zero_intrastate", func(t *testing.T) {
		inv := validInvoice()
		// IGST=0 for intrastate
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_interstate", func(t *testing.T) {
		inv := validInterstateInvoice()
		// taxable=1000, igst_rate=18 → 180
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInterstateInvoice()
		inv.LineItems[0].IGSTAmount = 999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestMath_LineItemTotal(t *testing.T) {
	v := findMathValidator("math.line_item.total")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_intrastate", func(t *testing.T) {
		inv := validInvoice()
		// taxable=1000 + cgst=90 + sgst=90 + igst=0 = 1180
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_interstate", func(t *testing.T) {
		inv := validInterstateInvoice()
		// taxable=1000 + igst=180 = 1180
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].Total = 9999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestMath_Subtotal(t *testing.T) {
	v := findMathValidator("math.totals.subtotal")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_wrong_subtotal", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.Subtotal = 9999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	// Regression: subtotal = sum(TaxableAmount), NOT sum(Total)
	t.Run("regression_sums_taxable_not_total", func(t *testing.T) {
		inv := validInvoice()
		// TaxableAmount=1000, Total=1180 — subtotal must match TaxableAmount
		inv.Totals.Subtotal = 1180 // would pass if bug used Total
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed, "subtotal should equal sum of TaxableAmount, not Total")
	})

	t.Run("multiple_items", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems = append(inv.LineItems, invoice.LineItem{
			TaxableAmount: 500,
			CGSTRate:      9, CGSTAmount: 45,
			SGSTRate: 9, SGSTAmount: 45,
			Total: 590,
		})
		inv.Totals.Subtotal = 1500 // 1000 + 500
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})
}

func TestMath_TaxableAmount(t *testing.T) {
	v := findMathValidator("math.totals.taxable_amount")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_no_discount", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_with_discount", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.TotalDiscount = 100
		inv.Totals.TaxableAmount = 900
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.TaxableAmount = 9999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("is_reconciliation_critical", func(t *testing.T) {
		assert.True(t, v.ReconciliationCritical())
	})
}

func TestMath_TotalCGST(t *testing.T) {
	v := findMathValidator("math.totals.cgst")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.CGST = 999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("is_reconciliation_critical", func(t *testing.T) {
		assert.True(t, v.ReconciliationCritical())
	})
}

func TestMath_TotalSGST(t *testing.T) {
	v := findMathValidator("math.totals.sgst")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.SGST = 999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestMath_TotalIGST(t *testing.T) {
	v := findMathValidator("math.totals.igst")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_zero_intrastate", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_interstate", func(t *testing.T) {
		inv := validInterstateInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInterstateInvoice()
		inv.Totals.IGST = 999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestMath_GrandTotal(t *testing.T) {
	v := findMathValidator("math.totals.grand_total")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_correct", func(t *testing.T) {
		inv := validInvoice()
		// taxable=1000 + cgst=90 + sgst=90 + igst=0 + cess=0 + roundoff=0 = 1180
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_with_cess", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.Cess = 10
		inv.Totals.Total = 1190
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_with_round_off", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.RoundOff = 0.20
		inv.Totals.Total = 1180.20
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_negative_round_off", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.RoundOff = -0.20
		inv.Totals.Total = 1179.80
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.Total = 9999
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("is_reconciliation_critical", func(t *testing.T) {
		assert.True(t, v.ReconciliationCritical())
	})
}

func TestMath_RoundOff(t *testing.T) {
	v := findMathValidator("math.totals.round_off")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_zero", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_exactly_0.50", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.RoundOff = 0.50
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("pass_negative_0.50", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.RoundOff = -0.50
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_0.51", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.RoundOff = 0.51
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("fail_negative_0.51", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.RoundOff = -0.51
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("severity_is_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})
}

func TestMath_Tolerance(t *testing.T) {
	v := findMathValidator("math.totals.cgst")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("within_tolerance_plus_1", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.CGST = 91.00 // off by 1.00, within tolerance
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("outside_tolerance", func(t *testing.T) {
		inv := validInvoice()
		inv.Totals.CGST = 91.01 // off by 1.01, outside tolerance
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestMath_MultipleLineItems(t *testing.T) {
	v := findMathValidator("math.totals.subtotal")
	require.NotNil(t, v)
	ctx := context.Background()

	inv := validInvoice()
	inv.LineItems = []invoice.LineItem{
		{Quantity: 10, UnitPrice: 100, TaxableAmount: 1000, CGSTRate: 9, CGSTAmount: 90, SGSTRate: 9, SGSTAmount: 90, Total: 1180},
		{Quantity: 5, UnitPrice: 200, TaxableAmount: 1000, CGSTRate: 9, CGSTAmount: 90, SGSTRate: 9, SGSTAmount: 90, Total: 1180},
		{Quantity: 2, UnitPrice: 500, TaxableAmount: 1000, CGSTRate: 9, CGSTAmount: 90, SGSTRate: 9, SGSTAmount: 90, Total: 1180},
	}
	inv.Totals.Subtotal = 3000

	results := v.Validate(ctx, inv)
	require.Len(t, results, 1)
	assert.True(t, results[0].Passed)
}

func TestMath_InterstateTaxation(t *testing.T) {
	ctx := context.Background()
	inv := validInterstateInvoice()

	// Verify all totals math validators pass for interstate
	keys := []string{
		"math.totals.subtotal", "math.totals.taxable_amount",
		"math.totals.cgst", "math.totals.sgst", "math.totals.igst",
		"math.totals.grand_total",
	}
	for _, key := range keys {
		v := findMathValidator(key)
		require.NotNil(t, v, "validator %s not found", key)
		results := v.Validate(ctx, inv)
		require.NotEmpty(t, results, "validator %s returned no results", key)
		assert.True(t, results[0].Passed, "validator %s failed for interstate invoice", key)
	}
}
