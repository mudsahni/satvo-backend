package invoice

import (
	"context"
	"fmt"
	"strings"

	"satvos/internal/domain"
	"satvos/internal/port"
)

// DuplicateInvoiceValidator returns a validator that checks whether another document in
// the same tenant already has the same (seller GSTIN + invoice number) combination.
func DuplicateInvoiceValidator(finder port.DuplicateInvoiceFinder) *BuiltinValidator {
	return &BuiltinValidator{
		key:      "logic.invoice.duplicate",
		name:     "Logical: Duplicate Invoice Detection",
		ruleType: domain.ValidationRuleCustom,
		sev:      domain.ValidationSeverityWarning,
		fn:       duplicateInvoiceValidator(finder),
	}
}

func duplicateInvoiceValidator(finder port.DuplicateInvoiceFinder) func(context.Context, *GSTInvoice) []ValidationResult {
	return func(ctx context.Context, inv *GSTInvoice) []ValidationResult {
		gstin := inv.Seller.GSTIN
		invoiceNum := inv.Invoice.InvoiceNumber

		if gstin == "" || invoiceNum == "" {
			return []ValidationResult{{
				Passed:    true,
				FieldPath: "invoice",
				Message:   "Logical: Duplicate Invoice Detection: seller GSTIN or invoice number is empty, skipping duplicate check",
			}}
		}

		tenantID, ok := TenantIDFromContext(ctx)
		if !ok {
			return []ValidationResult{{
				Passed:    true,
				FieldPath: "invoice",
				Message:   "Logical: Duplicate Invoice Detection: validation context missing, skipping duplicate check",
			}}
		}
		docID, ok := DocumentIDFromContext(ctx)
		if !ok {
			return []ValidationResult{{
				Passed:    true,
				FieldPath: "invoice",
				Message:   "Logical: Duplicate Invoice Detection: validation context missing, skipping duplicate check",
			}}
		}

		matches, err := finder.FindDuplicates(ctx, tenantID, docID, gstin, invoiceNum)
		if err != nil {
			return []ValidationResult{{
				Passed:    true,
				FieldPath: "invoice",
				Message:   "Logical: Duplicate Invoice Detection: duplicate check unavailable",
			}}
		}

		if len(matches) == 0 {
			return []ValidationResult{{
				Passed:        true,
				FieldPath:     "invoice",
				ExpectedValue: "no duplicate invoices",
				ActualValue:   "none found",
				Message:       "Logical: Duplicate Invoice Detection: no duplicate invoices found",
			}}
		}

		names := make([]string, 0, len(matches))
		for idx := range matches {
			m := &matches[idx]
			names = append(names, fmt.Sprintf("%q (uploaded %s)", m.DocumentName, m.CreatedAt.Format("2006-01-02")))
		}

		return []ValidationResult{{
			Passed:        false,
			FieldPath:     "invoice",
			ExpectedValue: "no duplicate invoices",
			ActualValue:   fmt.Sprintf("%d duplicate(s) found", len(matches)),
			Message: fmt.Sprintf(
				"Logical: Duplicate Invoice Detection: invoice %s from seller %s already exists in: %s",
				invoiceNum, gstin, strings.Join(names, ", "),
			),
		}}
	}
}
