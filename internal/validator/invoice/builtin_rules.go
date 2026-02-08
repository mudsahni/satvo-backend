package invoice

import (
	"context"

	"satvos/internal/domain"
)

// BuiltinValidator wraps a validator function and its metadata for the registry.
type BuiltinValidator struct {
	key           string
	name          string
	ruleType      domain.ValidationRuleType
	sev           domain.ValidationSeverity
	reconCritical bool
	fn            func(context.Context, *GSTInvoice) []ValidationResult
}

func (b *BuiltinValidator) Validate(ctx context.Context, data *GSTInvoice) []ValidationResult {
	return b.fn(ctx, data)
}
func (b *BuiltinValidator) RuleKey() string                     { return b.key }
func (b *BuiltinValidator) RuleName() string                    { return b.name }
func (b *BuiltinValidator) RuleType() domain.ValidationRuleType { return b.ruleType }
func (b *BuiltinValidator) Severity() domain.ValidationSeverity { return b.sev }
func (b *BuiltinValidator) ReconciliationCritical() bool        { return b.reconCritical }

// AllBuiltinValidators returns all built-in validators for GST invoices.
func AllBuiltinValidators() []*BuiltinValidator {
	reqVals := RequiredFieldValidators()
	fmtVals := FormatValidators()
	mathVals := MathValidators()
	xfVals := CrossFieldValidators()
	logVals := LogicalValidators()
	irnFmtVals := IRNFormatValidators()
	irnXfVals := IRNCrossFieldValidators()
	irnLogVals := IRNLogicalValidators()
	all := make([]*BuiltinValidator, 0, len(reqVals)+len(fmtVals)+len(mathVals)+len(xfVals)+len(logVals)+len(irnFmtVals)+len(irnXfVals)+len(irnLogVals))

	// Required field validators
	for _, v := range reqVals {
		v := v // capture
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	// Format validators
	for _, v := range fmtVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	// Math validators
	for _, v := range mathVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	// Cross-field validators
	for _, v := range xfVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	// Logical validators
	for _, v := range logVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	// IRN format validators
	for _, v := range irnFmtVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	// IRN cross-field validators
	for _, v := range irnXfVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	// IRN logical validators
	for _, v := range irnLogVals {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			reconCritical: v.ReconciliationCritical(),
			fn:            v.Validate,
		})
	}

	return all
}
