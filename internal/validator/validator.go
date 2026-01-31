package validator

import (
	"context"

	"satvos/internal/domain"
	"satvos/internal/validator/invoice"
)

// Validator is the interface for a single built-in validation rule.
type Validator interface {
	Validate(ctx context.Context, data *invoice.GSTInvoice) []invoice.ValidationResult
	RuleKey() string
	RuleName() string
	RuleType() domain.ValidationRuleType
	Severity() domain.ValidationSeverity
}
