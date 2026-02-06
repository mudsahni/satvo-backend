package parser_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"satvos/internal/parser"
)

func TestRateLimitError_ErrorString(t *testing.T) {
	underlying := fmt.Errorf("rate limited")
	rlErr := parser.NewRateLimitError("claude", underlying, 30)

	assert.Contains(t, rlErr.Error(), "claude")
	assert.Contains(t, rlErr.Error(), "rate limited")
	assert.Contains(t, rlErr.Error(), "30s")
}

func TestRateLimitError_Unwrap(t *testing.T) {
	underlying := fmt.Errorf("underlying error")
	rlErr := parser.NewRateLimitError("gemini", underlying, 60)

	assert.Equal(t, underlying, errors.Unwrap(rlErr))
}

func TestRateLimitError_ErrorsAs(t *testing.T) {
	underlying := fmt.Errorf("rate limited")
	rlErr := parser.NewRateLimitError("claude", underlying, 30)

	// Wrap it further
	wrapped := fmt.Errorf("parse failed: %w", rlErr)

	var target *parser.RateLimitError
	assert.True(t, errors.As(wrapped, &target))
	assert.Equal(t, "claude", target.Provider)
	assert.Equal(t, 30*time.Second, target.RetryAfter)
}

func TestNewRateLimitError_DefaultRetryAfter(t *testing.T) {
	rlErr := parser.NewRateLimitError("openai", fmt.Errorf("err"), 0)

	assert.Equal(t, 60*time.Second, rlErr.RetryAfter)
}

func TestNewRateLimitError_CustomRetryAfter(t *testing.T) {
	rlErr := parser.NewRateLimitError("openai", fmt.Errorf("err"), 30)

	assert.Equal(t, 30*time.Second, rlErr.RetryAfter)
}

func TestParseRetryAfterHeader(t *testing.T) {
	assert.Equal(t, 0, parser.ParseRetryAfterHeader(""))
	assert.Equal(t, 30, parser.ParseRetryAfterHeader("30"))
	assert.Equal(t, 0, parser.ParseRetryAfterHeader("invalid"))
	assert.Equal(t, 120, parser.ParseRetryAfterHeader("120"))
}
