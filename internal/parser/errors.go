package parser

import (
	"fmt"
	"strconv"
	"time"
)

// RateLimitError indicates a parser provider returned HTTP 429.
type RateLimitError struct {
	Err        error
	RetryAfter time.Duration
	Provider   string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%s rate limited (retry after %s): %v", e.Provider, e.RetryAfter, e.Err)
}

func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// NewRateLimitError creates a RateLimitError. If retryAfterSecs is 0, defaults to 60s.
func NewRateLimitError(provider string, err error, retryAfterSecs int) *RateLimitError {
	if retryAfterSecs <= 0 {
		retryAfterSecs = 60
	}
	return &RateLimitError{
		Err:        err,
		RetryAfter: time.Duration(retryAfterSecs) * time.Second,
		Provider:   provider,
	}
}

// ParseRetryAfterHeader parses a Retry-After header value into seconds.
// Returns 0 if the value is empty or not a valid integer.
func ParseRetryAfterHeader(val string) int {
	if val == "" {
		return 0
	}
	secs, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return secs
}
