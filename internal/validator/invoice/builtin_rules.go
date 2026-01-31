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
func (b *BuiltinValidator) RuleKey() string                    { return b.key }
func (b *BuiltinValidator) RuleName() string                   { return b.name }
func (b *BuiltinValidator) RuleType() domain.ValidationRuleType { return b.ruleType }
func (b *BuiltinValidator) Severity() domain.ValidationSeverity { return b.sev }

// AllBuiltinValidators returns all built-in validators for GST invoices.
func AllBuiltinValidators() []*BuiltinValidator {
	var all []*BuiltinValidator

	// Required field validators
	for _, v := range RequiredFieldValidators() {
		v := v // capture
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Format validators
	for _, v := range FormatValidators() {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Math validators
	for _, v := range MathValidators() {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Cross-field validators
	for _, v := range CrossFieldValidators() {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	// Logical validators
	for _, v := range LogicalValidators() {
		v := v
		all = append(all, &BuiltinValidator{
			key: v.RuleKey(), name: v.RuleName(),
			ruleType: v.RuleType(), sev: v.Severity(),
			fn: v.Validate,
		})
	}

	return all
}
