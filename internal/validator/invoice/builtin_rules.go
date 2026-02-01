package invoice

import (
	"context"

	"satvos/internal/domain"
)

// BuiltinValidator wraps a validator function and its metadata for the registry.
type BuiltinValidator struct {
	key      string
	name     string
	ruleType domain.ValidationRuleType
	sev      domain.ValidationSeverity
	fn       func(context.Context, *GSTInvoice) []ValidationResult
}

func (b *BuiltinValidator) Validate(ctx context.Context, data *GSTInvoice) []ValidationResult {
	return b.fn(ctx, data)
}
func (b *BuiltinValidator) RuleKey() string                     { return b.key }
func (b *BuiltinValidator) RuleName() string                    { return b.name }
func (b *BuiltinValidator) RuleType() domain.ValidationRuleType { return b.ruleType }
func (b *BuiltinValidator) Severity() domain.ValidationSeverity { return b.sev }

// AllBuiltinValidators returns all built-in validators for GST invoices.
func AllBuiltinValidators() []*BuiltinValidator {
	reqVals := RequiredFieldValidators()
	fmtVals := FormatValidators()
	mathVals := MathValidators()
	xfVals := CrossFieldValidators()
	logVals := LogicalValidators()
	all := make([]*BuiltinValidator, 0, len(reqVals)+len(fmtVals)+len(mathVals)+len(xfVals)+len(logVals))

	// Required field validators
	for _, v := range reqVals {
		v := v // capture
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Format validators
	for _, v := range fmtVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Math validators
	for _, v := range mathVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Cross-field validators
	for _, v := range xfVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Logical validators
	for _, v := range logVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	return all
}
