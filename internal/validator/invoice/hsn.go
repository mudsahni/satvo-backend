package invoice

import (
	"context"
	"fmt"
	"strings"

	"satvos/internal/domain"
)

// HSNValidators returns validators that use an HSN lookup table for code existence
// and rate cross-validation. The lookup is captured by closure — no interface changes.
func HSNValidators(lookup *HSNLookup) []*BuiltinValidator {
	return []*BuiltinValidator{
		{
			key:      "logic.line_item.hsn_exists",
			name:     "Logical: HSN Code Exists in Master",
			ruleType: domain.ValidationRuleCustom,
			sev:      domain.ValidationSeverityWarning,
			fn:       hsnExistsValidator(lookup),
		},
		{
			key:      "xf.line_item.hsn_rate",
			name:     "Cross-field: HSN Code GST Rate Match",
			ruleType: domain.ValidationRuleCrossField,
			sev:      domain.ValidationSeverityWarning,
			fn:       hsnRateValidator(lookup),
		},
	}
}

func hsnExistsValidator(lookup *HSNLookup) func(context.Context, *GSTInvoice) []ValidationResult {
	return func(_ context.Context, inv *GSTInvoice) []ValidationResult {
		results := make([]ValidationResult, 0, len(inv.LineItems))
		for i := range inv.LineItems {
			item := &inv.LineItems[i]
			fp := fmt.Sprintf("line_items[%d].hsn_sac_code", i)

			if item.HSNSACCode == "" {
				results = append(results, ValidationResult{
					Passed: true, FieldPath: fp,
					Message: "Logical: HSN Code Exists in Master: HSN/SAC code is empty, skipping",
				})
				continue
			}

			exists := lookup.Exists(item.HSNSACCode)
			msg := fmt.Sprintf("Logical: HSN Code Exists in Master: %s found in HSN master list", fp)
			if !exists {
				msg = fmt.Sprintf("Logical: HSN Code Exists in Master: %s code %q not found in HSN master list", fp, item.HSNSACCode)
			}
			results = append(results, ValidationResult{
				Passed:        exists,
				FieldPath:     fp,
				ExpectedValue: "valid HSN/SAC code from master list",
				ActualValue:   item.HSNSACCode,
				Message:       msg,
			})
		}
		return results
	}
}

func hsnRateValidator(lookup *HSNLookup) func(context.Context, *GSTInvoice) []ValidationResult {
	return func(_ context.Context, inv *GSTInvoice) []ValidationResult {
		results := make([]ValidationResult, 0, len(inv.LineItems))
		for i := range inv.LineItems {
			item := &inv.LineItems[i]
			fp := fmt.Sprintf("line_items[%d]", i)

			if item.HSNSACCode == "" {
				results = append(results, ValidationResult{
					Passed: true, FieldPath: fp,
					Message: "Cross-field: HSN Code GST Rate Match: HSN/SAC code is empty, skipping",
				})
				continue
			}

			if !lookup.Exists(item.HSNSACCode) {
				results = append(results, ValidationResult{
					Passed: true, FieldPath: fp,
					Message: fmt.Sprintf("Cross-field: HSN Code GST Rate Match: HSN code %q not in master, skipping rate check", item.HSNSACCode),
				})
				continue
			}

			// Compute effective rate: IGST for interstate, CGST+SGST for intrastate
			effectiveRate := item.IGSTRate
			if effectiveRate == 0 {
				effectiveRate = item.CGSTRate + item.SGSTRate
			}

			matched, validRates := lookup.RateMatches(item.HSNSACCode, effectiveRate)

			if matched {
				results = append(results, ValidationResult{
					Passed:        true,
					FieldPath:     fp,
					ExpectedValue: formatExpectedRates(validRates),
					ActualValue:   fmtf(effectiveRate) + "%",
					Message:       fmt.Sprintf("Cross-field: HSN Code GST Rate Match: %s rate matches HSN %s", fp, item.HSNSACCode),
				})
			} else {
				results = append(results, ValidationResult{
					Passed:        false,
					FieldPath:     fp,
					ExpectedValue: formatExpectedRates(validRates),
					ActualValue:   fmtf(effectiveRate) + "%",
					Message:       fmt.Sprintf("Cross-field: HSN Code GST Rate Match: %s rate %s%% does not match expected rates for HSN %s", fp, fmtf(effectiveRate), item.HSNSACCode),
				})
			}
		}
		return results
	}
}

func formatExpectedRates(rates []HSNRateEntry) string {
	if len(rates) == 0 {
		return "no rates found"
	}
	parts := make([]string, 0, len(rates))
	for idx := range rates {
		r := &rates[idx]
		s := fmtf(r.Rate) + "%"
		if r.ConditionDesc != "" {
			s += " (" + r.ConditionDesc + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}
