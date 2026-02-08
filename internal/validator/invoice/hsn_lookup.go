package invoice

import (
	"math"

	"satvos/internal/port"
)

// HSNRateEntry holds a valid GST rate and optional condition for an HSN code.
type HSNRateEntry struct {
	Rate          float64
	ConditionDesc string
}

// HSNLookup provides fast in-memory lookups for HSN code existence and rate validation.
// It is immutable after construction and safe for concurrent access.
type HSNLookup struct {
	byCode map[string][]HSNRateEntry
}

// NewHSNLookup builds an HSNLookup from a slice of HSNEntry loaded from the database.
func NewHSNLookup(entries []port.HSNEntry) *HSNLookup {
	m := make(map[string][]HSNRateEntry, len(entries))
	for idx := range entries {
		e := &entries[idx]
		m[e.Code] = append(m[e.Code], HSNRateEntry{
			Rate:          e.GSTRate,
			ConditionDesc: e.ConditionDesc,
		})
	}
	return &HSNLookup{byCode: m}
}

// Exists returns true if the HSN code (or a prefix of it) is in the master list.
// It checks exact match first, then falls back from 8→6→4 digit prefixes.
func (h *HSNLookup) Exists(code string) bool {
	if len(h.byCode) == 0 || code == "" {
		return false
	}
	if _, ok := h.byCode[code]; ok {
		return true
	}
	// Hierarchical prefix fallback: try shorter prefixes
	for _, prefixLen := range []int{6, 4} {
		if len(code) > prefixLen {
			if _, ok := h.byCode[code[:prefixLen]]; ok {
				return true
			}
		}
	}
	return false
}

// Rates returns valid rate entries for the given HSN code, with prefix fallback.
func (h *HSNLookup) Rates(code string) []HSNRateEntry {
	if len(h.byCode) == 0 || code == "" {
		return nil
	}
	if rates, ok := h.byCode[code]; ok {
		return rates
	}
	for _, prefixLen := range []int{6, 4} {
		if len(code) > prefixLen {
			if rates, ok := h.byCode[code[:prefixLen]]; ok {
				return rates
			}
		}
	}
	return nil
}

// RateMatches checks if the given GST rate matches any valid rate for this HSN code.
// Returns whether a match was found and the list of valid rates.
func (h *HSNLookup) RateMatches(code string, gstRate float64) (matched bool, validRates []HSNRateEntry) {
	validRates = h.Rates(code)
	if len(validRates) == 0 {
		return false, nil
	}
	for idx := range validRates {
		if math.Abs(validRates[idx].Rate-gstRate) < 0.01 {
			return true, validRates
		}
	}
	return false, validRates
}
