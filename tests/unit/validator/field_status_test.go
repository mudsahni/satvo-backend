package validator_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"satvos/internal/domain"
	"satvos/internal/validator"
)

func TestComputeFieldStatuses_AllPassed(t *testing.T) {
	ruleID := uuid.New()
	rules := map[string]*domain.DocumentValidationRule{
		ruleID.String(): {
			ID:       ruleID,
			Severity: domain.ValidationSeverityError,
		},
	}

	results := []validator.ValidationResultEntry{
		{RuleID: ruleID, Passed: true, FieldPath: "seller.gstin", Message: "GSTIN valid"},
		{RuleID: ruleID, Passed: true, FieldPath: "buyer.gstin", Message: "GSTIN valid"},
	}

	statuses := validator.ComputeFieldStatuses(results, rules, nil)

	assert.Equal(t, domain.FieldStatusValid, statuses["seller.gstin"].Status)
	assert.Equal(t, domain.FieldStatusValid, statuses["buyer.gstin"].Status)
	assert.Empty(t, statuses["seller.gstin"].Messages)
}

func TestComputeFieldStatuses_ErrorField(t *testing.T) {
	ruleID := uuid.New()
	rules := map[string]*domain.DocumentValidationRule{
		ruleID.String(): {
			ID:       ruleID,
			Severity: domain.ValidationSeverityError,
		},
	}

	results := []validator.ValidationResultEntry{
		{RuleID: ruleID, Passed: false, FieldPath: "seller.gstin", Message: "GSTIN is missing"},
	}

	statuses := validator.ComputeFieldStatuses(results, rules, nil)

	assert.Equal(t, domain.FieldStatusInvalid, statuses["seller.gstin"].Status)
	assert.Contains(t, statuses["seller.gstin"].Messages, "GSTIN is missing")
}

func TestComputeFieldStatuses_WarningField(t *testing.T) {
	ruleID := uuid.New()
	rules := map[string]*domain.DocumentValidationRule{
		ruleID.String(): {
			ID:       ruleID,
			Severity: domain.ValidationSeverityWarning,
		},
	}

	results := []validator.ValidationResultEntry{
		{RuleID: ruleID, Passed: false, FieldPath: "invoice.currency", Message: "Currency is missing"},
	}

	statuses := validator.ComputeFieldStatuses(results, rules, nil)

	assert.Equal(t, domain.FieldStatusUnsure, statuses["invoice.currency"].Status)
	assert.Contains(t, statuses["invoice.currency"].Messages, "Currency is missing")
}

func TestComputeFieldStatuses_ErrorOverridesWarning(t *testing.T) {
	errorRuleID := uuid.New()
	warningRuleID := uuid.New()
	rules := map[string]*domain.DocumentValidationRule{
		errorRuleID.String(): {
			ID:       errorRuleID,
			Severity: domain.ValidationSeverityError,
		},
		warningRuleID.String(): {
			ID:       warningRuleID,
			Severity: domain.ValidationSeverityWarning,
		},
	}

	results := []validator.ValidationResultEntry{
		{RuleID: warningRuleID, Passed: false, FieldPath: "seller.gstin", Message: "warning msg"},
		{RuleID: errorRuleID, Passed: false, FieldPath: "seller.gstin", Message: "error msg"},
	}

	statuses := validator.ComputeFieldStatuses(results, rules, nil)

	assert.Equal(t, domain.FieldStatusInvalid, statuses["seller.gstin"].Status)
	assert.Len(t, statuses["seller.gstin"].Messages, 2)
}

func TestComputeFieldStatuses_ConfidenceLow(t *testing.T) {
	confidenceMap := map[string]float64{
		"seller.pan": 0.3,
	}

	statuses := validator.ComputeFieldStatuses(nil, nil, confidenceMap)

	assert.Equal(t, domain.FieldStatusUnsure, statuses["seller.pan"].Status)
	assert.Empty(t, statuses["seller.pan"].Messages)
}

func TestComputeFieldStatuses_ConfidenceHigh(t *testing.T) {
	confidenceMap := map[string]float64{
		"seller.name": 0.95,
	}

	statuses := validator.ComputeFieldStatuses(nil, nil, confidenceMap)

	assert.Equal(t, domain.FieldStatusValid, statuses["seller.name"].Status)
	assert.Empty(t, statuses["seller.name"].Messages)
}

func TestComputeFieldStatuses_EmptyInput(t *testing.T) {
	statuses := validator.ComputeFieldStatuses(nil, nil, nil)

	assert.Empty(t, statuses)
}
