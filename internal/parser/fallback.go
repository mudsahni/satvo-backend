package parser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"satvos/internal/port"
)

// circuitState tracks rate-limit backoff for a single parser.
type circuitState struct {
	mu      sync.RWMutex
	resetAt time.Time // zero value = closed (healthy)
}

func (c *circuitState) isOpenWithReset(now time.Time) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.resetAt, !c.resetAt.IsZero() && now.Before(c.resetAt)
}

func (c *circuitState) open(resetAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetAt = resetAt
}

// FallbackParser tries parsers in order, skipping those with open circuits.
// It implements port.DocumentParser.
type FallbackParser struct {
	parsers  []port.DocumentParser
	circuits []*circuitState
	names    []string
}

// NewFallbackParser creates a FallbackParser from an ordered list of parsers and their names.
func NewFallbackParser(parsers []port.DocumentParser, names []string) *FallbackParser {
	circuits := make([]*circuitState, len(parsers))
	for i := range circuits {
		circuits[i] = &circuitState{}
	}
	return &FallbackParser{
		parsers:  parsers,
		circuits: circuits,
		names:    names,
	}
}

func (f *FallbackParser) Parse(ctx context.Context, input port.ParseInput) (*port.ParseOutput, error) {
	now := time.Now()
	var lastErr error
	allRateLimited := true
	var earliestReset time.Time

	for i, p := range f.parsers {
		if resetAt, open := f.circuits[i].isOpenWithReset(now); open {
			log.Printf("parser.FallbackParser: skipping %s (circuit open until %s)", f.names[i], resetAt.Format(time.RFC3339))
			if earliestReset.IsZero() || resetAt.Before(earliestReset) {
				earliestReset = resetAt
			}
			continue
		}

		out, err := p.Parse(ctx, input)
		if err == nil {
			return out, nil
		}

		log.Printf("parser.FallbackParser: %s failed: %v", f.names[i], err)
		lastErr = err

		var rlErr *RateLimitError
		if errors.As(err, &rlErr) {
			resetAt := now.Add(rlErr.RetryAfter)
			f.circuits[i].open(resetAt)
			if earliestReset.IsZero() || resetAt.Before(earliestReset) {
				earliestReset = resetAt
			}
		} else {
			allRateLimited = false
		}
	}

	if lastErr == nil {
		// All parsers were skipped due to open circuits
		retryAfter := time.Until(earliestReset)
		if retryAfter < 0 {
			retryAfter = time.Second
		}
		return nil, NewRateLimitError("all", fmt.Errorf("all parsers rate limited"), int(retryAfter.Seconds()))
	}

	if allRateLimited {
		retryAfter := time.Until(earliestReset)
		if retryAfter < 0 {
			retryAfter = time.Second
		}
		return nil, NewRateLimitError("all", fmt.Errorf("all parsers rate limited"), int(retryAfter.Seconds()))
	}

	return nil, fmt.Errorf("all parsers failed: %w", lastErr)
}
