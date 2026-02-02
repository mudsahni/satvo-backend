package invoice

import (
	"context"
	"fmt"
	"math"

	"satvos/internal/domain"
)

const mathTolerance = 1.00

// mathValidator checks arithmetic relationships between fields.
type mathValidator struct {
	ruleKey       string
	ruleName      string
	severity      domain.ValidationSeverity
	reconCritical bool
	validate      func(*GSTInvoice) []ValidationResult
}

func (v *mathValidator) RuleKey() string                     { return v.ruleKey }
func (v *mathValidator) RuleName() string                    { return v.ruleName }
func (v *mathValidator) RuleType() domain.ValidationRuleType { return domain.ValidationRuleSumCheck }
func (v *mathValidator) Severity() domain.ValidationSeverity { return v.severity }
func (v *mathValidator) ReconciliationCritical() bool        { return v.reconCritical }

func (v *mathValidator) Validate(_ context.Context, data *GSTInvoice) []ValidationResult {
	return v.validate(data)
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) <= mathTolerance
}

func mathResult(passed bool, fieldPath, expected, actual, ruleName string) ValidationResult {
	msg := fmt.Sprintf("%s: %s calculation matches", ruleName, fieldPath)
	if !passed {
		msg = fmt.Sprintf("%s: %s calculation mismatch (expected %s, got %s)", ruleName, fieldPath, expected, actual)
	}
	return ValidationResult{
		Passed: passed, FieldPath: fieldPath,
		ExpectedValue: expected, ActualValue: actual, Message: msg,
	}
}

func fmtf(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// MathValidators returns all mathematical validators.
func MathValidators() []*mathValidator {
	return []*mathValidator{
		{
			ruleKey: "math.line_item.taxable_amount", ruleName: "Math: Line Item Taxable Amount",
			severity: domain.ValidationSeverityError,
			validate: func(d *GSTInvoice) []ValidationResult {
				results := make([]ValidationResult, 0, len(d.LineItems))
				for i := range d.LineItems {
					item := &d.LineItems[i]
					fp := fmt.Sprintf("line_items[%d].taxable_amount", i)
					expected := (item.Quantity * item.UnitPrice) - item.Discount
					passed := approxEqual(item.TaxableAmount, expected)
					results = append(results, mathResult(passed, fp, fmtf(expected), fmtf(item.TaxableAmount), "Math: Line Item Taxable Amount"))
				}
				return results
			},
		},
		{
			ruleKey: "math.line_item.cgst_amount", ruleName: "Math: Line Item CGST Amount",
			severity: domain.ValidationSeverityError,
			validate: func(d *GSTInvoice) []ValidationResult {
				results := make([]ValidationResult, 0, len(d.LineItems))
				for i := range d.LineItems {
					item := &d.LineItems[i]
					fp := fmt.Sprintf("line_items[%d].cgst_amount", i)
					expected := item.TaxableAmount * item.CGSTRate / 100
					passed := approxEqual(item.CGSTAmount, expected)
					results = append(results, mathResult(passed, fp, fmtf(expected), fmtf(item.CGSTAmount), "Math: Line Item CGST Amount"))
				}
				return results
			},
		},
		{
			ruleKey: "math.line_item.sgst_amount", ruleName: "Math: Line Item SGST Amount",
			severity: domain.ValidationSeverityError,
			validate: func(d *GSTInvoice) []ValidationResult {
				results := make([]ValidationResult, 0, len(d.LineItems))
				for i := range d.LineItems {
					item := &d.LineItems[i]
					fp := fmt.Sprintf("line_items[%d].sgst_amount", i)
					expected := item.TaxableAmount * item.SGSTRate / 100
					passed := approxEqual(item.SGSTAmount, expected)
					results = append(results, mathResult(passed, fp, fmtf(expected), fmtf(item.SGSTAmount), "Math: Line Item SGST Amount"))
				}
				return results
			},
		},
		{
			ruleKey: "math.line_item.igst_amount", ruleName: "Math: Line Item IGST Amount",
			severity: domain.ValidationSeverityError,
			validate: func(d *GSTInvoice) []ValidationResult {
				results := make([]ValidationResult, 0, len(d.LineItems))
				for i := range d.LineItems {
					item := &d.LineItems[i]
					fp := fmt.Sprintf("line_items[%d].igst_amount", i)
					expected := item.TaxableAmount * item.IGSTRate / 100
					passed := approxEqual(item.IGSTAmount, expected)
					results = append(results, mathResult(passed, fp, fmtf(expected), fmtf(item.IGSTAmount), "Math: Line Item IGST Amount"))
				}
				return results
			},
		},
		{
			ruleKey: "math.line_item.total", ruleName: "Math: Line Item Total",
			severity: domain.ValidationSeverityError,
			validate: func(d *GSTInvoice) []ValidationResult {
				results := make([]ValidationResult, 0, len(d.LineItems))
				for i := range d.LineItems {
					item := &d.LineItems[i]
					fp := fmt.Sprintf("line_items[%d].total", i)
					expected := item.TaxableAmount + item.CGSTAmount + item.SGSTAmount + item.IGSTAmount
					passed := approxEqual(item.Total, expected)
					results = append(results, mathResult(passed, fp, fmtf(expected), fmtf(item.Total), "Math: Line Item Total"))
				}
				return results
			},
		},
		{
			ruleKey: "math.totals.subtotal", ruleName: "Math: Subtotal",
			severity: domain.ValidationSeverityError,
			validate: func(d *GSTInvoice) []ValidationResult {
				var sum float64
				for idx := range d.LineItems {
					item := &d.LineItems[idx]
					sum += item.TaxableAmount
				}
				passed := approxEqual(d.Totals.Subtotal, sum)
				return []ValidationResult{mathResult(passed, "totals.subtotal", fmtf(sum), fmtf(d.Totals.Subtotal), "Math: Subtotal")}
			},
		},
		{
			ruleKey: "math.totals.taxable_amount", ruleName: "Math: Taxable Amount",
			severity: domain.ValidationSeverityError, reconCritical: true,
			validate: func(d *GSTInvoice) []ValidationResult {
				expected := d.Totals.Subtotal - d.Totals.TotalDiscount
				passed := approxEqual(d.Totals.TaxableAmount, expected)
				return []ValidationResult{mathResult(passed, "totals.taxable_amount", fmtf(expected), fmtf(d.Totals.TaxableAmount), "Math: Taxable Amount")}
			},
		},
		{
			ruleKey: "math.totals.cgst", ruleName: "Math: Total CGST",
			severity: domain.ValidationSeverityError, reconCritical: true,
			validate: func(d *GSTInvoice) []ValidationResult {
				var sum float64
				for idx := range d.LineItems {
					item := &d.LineItems[idx]
					sum += item.CGSTAmount
				}
				passed := approxEqual(d.Totals.CGST, sum)
				return []ValidationResult{mathResult(passed, "totals.cgst", fmtf(sum), fmtf(d.Totals.CGST), "Math: Total CGST")}
			},
		},
		{
			ruleKey: "math.totals.sgst", ruleName: "Math: Total SGST",
			severity: domain.ValidationSeverityError, reconCritical: true,
			validate: func(d *GSTInvoice) []ValidationResult {
				var sum float64
				for idx := range d.LineItems {
					item := &d.LineItems[idx]
					sum += item.SGSTAmount
				}
				passed := approxEqual(d.Totals.SGST, sum)
				return []ValidationResult{mathResult(passed, "totals.sgst", fmtf(sum), fmtf(d.Totals.SGST), "Math: Total SGST")}
			},
		},
		{
			ruleKey: "math.totals.igst", ruleName: "Math: Total IGST",
			severity: domain.ValidationSeverityError, reconCritical: true,
			validate: func(d *GSTInvoice) []ValidationResult {
				var sum float64
				for idx := range d.LineItems {
					item := &d.LineItems[idx]
					sum += item.IGSTAmount
				}
				passed := approxEqual(d.Totals.IGST, sum)
				return []ValidationResult{mathResult(passed, "totals.igst", fmtf(sum), fmtf(d.Totals.IGST), "Math: Total IGST")}
			},
		},
		{
			ruleKey: "math.totals.grand_total", ruleName: "Math: Grand Total",
			severity: domain.ValidationSeverityError, reconCritical: true,
			validate: func(d *GSTInvoice) []ValidationResult {
				expected := d.Totals.TaxableAmount + d.Totals.CGST + d.Totals.SGST + d.Totals.IGST + d.Totals.Cess + d.Totals.RoundOff
				passed := approxEqual(d.Totals.Total, expected)
				return []ValidationResult{mathResult(passed, "totals.total", fmtf(expected), fmtf(d.Totals.Total), "Math: Grand Total")}
			},
		},
		{
			ruleKey: "math.totals.round_off", ruleName: "Math: Round Off",
			severity: domain.ValidationSeverityWarning,
			validate: func(d *GSTInvoice) []ValidationResult {
				passed := math.Abs(d.Totals.RoundOff) <= 0.50
				msg := "Math: Round Off: within acceptable range"
				if !passed {
					msg = fmt.Sprintf("Math: Round Off: abs(%.2f) > 0.50", d.Totals.RoundOff)
				}
				return []ValidationResult{{
					Passed: passed, FieldPath: "totals.round_off",
					ExpectedValue: "abs(round_off) <= 0.50", ActualValue: fmtf(d.Totals.RoundOff), Message: msg,
				}}
			},
		},
	}
}
