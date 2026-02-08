package invoice_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"satvos/internal/domain"
	"satvos/internal/validator/invoice"
)

func TestRequiredValidators_Count(t *testing.T) {
	assert.Len(t, invoice.RequiredFieldValidators(), 12)
}

func TestRequiredValidators_Metadata(t *testing.T) {
	reconCriticalKeys := map[string]bool{
		"req.invoice.number":         true,
		"req.invoice.date":           true,
		"req.invoice.place_of_supply": true,
		"req.seller.name":            true,
		"req.seller.gstin":           true,
		"req.buyer.gstin":            true,
	}

	for _, v := range invoice.RequiredFieldValidators() {
		assert.NotEmpty(t, v.RuleKey())
		assert.NotEmpty(t, v.RuleName())
		assert.Equal(t, domain.ValidationRuleRequired, v.RuleType())

		if reconCriticalKeys[v.RuleKey()] {
			assert.True(t, v.ReconciliationCritical(), "%s should be recon-critical", v.RuleKey())
		}
	}
}

func TestRequired_InvoiceNumber(t *testing.T) {
	v := findRequiredValidator("req.invoice.number")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		inv := validInvoice()
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.InvoiceNumber = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
		assert.Equal(t, "invoice.invoice_number", results[0].FieldPath)
	})
}

func TestRequired_InvoiceDate(t *testing.T) {
	v := findRequiredValidator("req.invoice.date")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.InvoiceDate = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestRequired_PlaceOfSupply(t *testing.T) {
	v := findRequiredValidator("req.invoice.place_of_supply")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.PlaceOfSupply = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestRequired_Currency(t *testing.T) {
	v := findRequiredValidator("req.invoice.currency")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Invoice.Currency = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("severity_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})
}

func TestRequired_SellerName(t *testing.T) {
	v := findRequiredValidator("req.seller.name")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.Name = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestRequired_SellerGSTIN(t *testing.T) {
	v := findRequiredValidator("req.seller.gstin")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.GSTIN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestRequired_SellerStateCode(t *testing.T) {
	v := findRequiredValidator("req.seller.state_code")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Seller.StateCode = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("not_reconciliation_critical", func(t *testing.T) {
		assert.False(t, v.ReconciliationCritical())
	})
}

func TestRequired_BuyerName(t *testing.T) {
	v := findRequiredValidator("req.buyer.name")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.Name = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestRequired_BuyerGSTIN(t *testing.T) {
	v := findRequiredValidator("req.buyer.gstin")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.GSTIN = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestRequired_BuyerStateCode(t *testing.T) {
	v := findRequiredValidator("req.buyer.state_code")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.Buyer.StateCode = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})
}

func TestRequired_LineItemDescription(t *testing.T) {
	v := findRequiredValidator("req.line_item.description")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
		assert.Equal(t, "line_items[0].description", results[0].FieldPath)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].Description = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("multi_item_mixed", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems = append(inv.LineItems, invoice.LineItem{
			Description: "",
			HSNSACCode:  "8471",
		})
		results := v.Validate(ctx, inv)
		require.Len(t, results, 2)
		assert.True(t, results[0].Passed)
		assert.False(t, results[1].Passed)
		assert.Equal(t, "line_items[1].description", results[1].FieldPath)
	})

	t.Run("severity_warning", func(t *testing.T) {
		assert.Equal(t, domain.ValidationSeverityWarning, v.Severity())
	})
}

func TestRequired_LineItemHSNSAC(t *testing.T) {
	v := findRequiredValidator("req.line_item.hsn_sac")
	require.NotNil(t, v)
	ctx := context.Background()

	t.Run("pass_present", func(t *testing.T) {
		results := v.Validate(ctx, validInvoice())
		require.Len(t, results, 1)
		assert.True(t, results[0].Passed)
	})

	t.Run("fail_empty", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems[0].HSNSACCode = ""
		results := v.Validate(ctx, inv)
		require.Len(t, results, 1)
		assert.False(t, results[0].Passed)
	})

	t.Run("multi_item_mixed", func(t *testing.T) {
		inv := validInvoice()
		inv.LineItems = append(inv.LineItems, invoice.LineItem{
			Description: "Item 2",
			HSNSACCode:  "",
		})
		results := v.Validate(ctx, inv)
		require.Len(t, results, 2)
		assert.True(t, results[0].Passed)
		assert.False(t, results[1].Passed)
	})
}
