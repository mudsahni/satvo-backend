// Command seedhsn converts the GST HSN/SAC Excel file into a SQL seed file.
// Reads both HSN_Master_v1 (goods) and SAC_Master (services) sheets.
// Usage: go run ./cmd/seedhsn
// Output: db/seeds/hsn_codes.sql
package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

const batchSize = 500

type hsnEntry struct {
	code        string
	description string
	gstRate     float64
	parentCode  string // empty = NULL
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	xlsxPath := "AI Tool - GST_HSN Code summary_19.02.2025.xlsx"
	outPath := "db/seeds/hsn_codes.sql"

	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		return fmt.Errorf("open Excel file: %w", err)
	}
	defer func() { _ = f.Close() }()

	seen := make(map[string]bool)
	var entries []hsnEntry

	// Sheet 0: HSN_Master_v1 (goods)
	hsnEntries, err := parseHSNSheet(f, seen)
	if err != nil {
		return fmt.Errorf("parse HSN sheet: %w", err)
	}
	entries = append(entries, hsnEntries...)
	log.Printf("HSN sheet: %d entries", len(hsnEntries))

	// Sheet 2: SAC_Master (services)
	sacEntries, err := parseSACSheet(f, seen)
	if err != nil {
		return fmt.Errorf("parse SAC sheet: %w", err)
	}
	entries = append(entries, sacEntries...)
	log.Printf("SAC sheet: %d entries", len(sacEntries))

	// Write SQL file with batched multi-row INSERTs.
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() { _ = out.Close() }()

	w := func(s string) error { _, werr := fmt.Fprintln(out, s); return werr }

	for _, line := range []string{
		"-- HSN/SAC code seed data generated from Excel.",
		fmt.Sprintf("-- %d entries (HSN + SAC) in batches of %d.", len(entries), batchSize),
		"-- Run: make seed-hsn",
		"BEGIN;",
		"",
	} {
		if werr := w(line); werr != nil {
			return fmt.Errorf("write header: %w", werr)
		}
	}

	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		if err := writeBatch(out, entries[i:end]); err != nil {
			return fmt.Errorf("write batch at offset %d: %w", i, err)
		}
	}

	for _, line := range []string{"", "COMMIT;"} {
		if werr := w(line); werr != nil {
			return fmt.Errorf("write footer: %w", werr)
		}
	}

	log.Printf("Generated %d total entries (%d batches) in %s",
		len(entries), (len(entries)+batchSize-1)/batchSize, outPath)
	return nil
}

// parseHSNSheet reads the HSN_Master_v1 sheet (index 0).
// Columns: F(5)=4-digit, H(7)=4-digit desc, I(8)=6-digit, J(9)=6-digit desc,
// K(10)=8-digit, M(12)=8-digit desc, N(13)=GST rate (percentage formatted).
// Data starts at row index 5.
func parseHSNSheet(f *excelize.File, seen map[string]bool) ([]hsnEntry, error) {
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	var entries []hsnEntry
	for i := 5; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 14 {
			continue
		}

		gstRateStr := strings.TrimSpace(cellVal(row, 13))
		if gstRateStr == "" {
			continue
		}

		gstRateStr = strings.TrimSuffix(gstRateStr, "%")
		var gstRatePct float64
		if _, serr := fmt.Sscanf(gstRateStr, "%f", &gstRatePct); serr != nil {
			continue
		}

		if code := strings.TrimSpace(cellVal(row, 10)); code != "" && isNumeric(code) {
			entries = addEntry(entries, seen, code, strings.TrimSpace(cellVal(row, 12)), gstRatePct)
		}
		if code := strings.TrimSpace(cellVal(row, 8)); code != "" && isNumeric(code) {
			entries = addEntry(entries, seen, code, strings.TrimSpace(cellVal(row, 9)), gstRatePct)
		}
		if code := strings.TrimSpace(cellVal(row, 5)); code != "" && isNumeric(code) {
			entries = addEntry(entries, seen, code, strings.TrimSpace(cellVal(row, 7)), gstRatePct)
		}
	}
	return entries, nil
}

// parseSACSheet reads the SAC_Master sheet (index 2).
// Columns: A(0)=4-digit SAC, B(1)=4-digit desc, C(2)=6-digit SAC, D(3)=6-digit desc,
// E(4)=GST rate (free text like "18%", "Exempt", "5% (without ITC)", "12%-18%").
// Data starts at row index 3.
func parseSACSheet(f *excelize.File, seen map[string]bool) ([]hsnEntry, error) {
	rows, err := f.GetRows("SAC_Master")
	if err != nil {
		return nil, err
	}

	var entries []hsnEntry
	for i := 3; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 5 {
			continue
		}

		rateStr := strings.TrimSpace(cellVal(row, 4))
		rates := parseSACRate(rateStr)
		if len(rates) == 0 {
			continue
		}

		code6 := strings.TrimSpace(cellVal(row, 2))
		desc6 := strings.TrimSpace(cellVal(row, 3))
		code4 := strings.TrimSpace(cellVal(row, 0))
		desc4 := strings.TrimSpace(cellVal(row, 1))

		for _, rate := range rates {
			if code6 != "" && isNumeric(code6) {
				entries = addEntry(entries, seen, code6, desc6, rate)
			}
			if code4 != "" && isNumeric(code4) {
				entries = addEntry(entries, seen, code4, desc4, rate)
			}
		}
	}
	return entries, nil
}

// ratePattern matches a number followed by "%".
var ratePattern = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

// parseSACRate extracts GST rate(s) from free-text SAC rate strings.
// Examples:
//
//	"18%"                                     → [18]
//	"Exempt"                                  → [0]
//	"0%"                                      → [0]
//	"12%-18%"                                 → [12, 18]
//	"1% (without ITC) or 5% (without ITC)"   → [1, 5]
//	"5%(With ITC restriction) or 18%..."      → [5, 18]
func parseSACRate(s string) []float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	lower := strings.ToLower(s)
	if lower == "exempt" || lower == "nil" {
		return []float64{0}
	}

	matches := ratePattern.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[float64]bool)
	var rates []float64
	for _, m := range matches {
		var rate float64
		if _, err := fmt.Sscanf(m[1], "%f", &rate); err == nil && !seen[rate] {
			seen[rate] = true
			rates = append(rates, rate)
		}
	}
	return rates
}

func addEntry(entries []hsnEntry, seen map[string]bool, code, description string, gstRate float64) []hsnEntry {
	key := fmt.Sprintf("%s|%.2f", code, gstRate)
	if seen[key] {
		return entries
	}
	seen[key] = true

	parent := ""
	if len(code) > 4 {
		parent = code[:4]
	}
	return append(entries, hsnEntry{code: code, description: description, gstRate: gstRate, parentCode: parent})
}

func writeBatch(out *os.File, batch []hsnEntry) error {
	if len(batch) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("INSERT INTO hsn_codes (code, description, gst_rate, parent_code, effective_from) VALUES\n")

	for i := range batch {
		e := &batch[i]
		if i > 0 {
			b.WriteString(",\n")
		}

		parentVal := "NULL"
		if e.parentCode != "" {
			parentVal = fmt.Sprintf("'%s'", e.parentCode)
		}

		fmt.Fprintf(&b, "  ('%s', '%s', %.2f, %s, '2017-07-01')",
			escapeSQL(e.code), escapeSQL(e.description), e.gstRate, parentVal)
	}

	b.WriteString("\nON CONFLICT (code, gst_rate, condition_desc, effective_from) DO NOTHING;\n")

	_, err := out.WriteString(b.String())
	return err
}

func cellVal(row []string, idx int) string {
	if idx < len(row) {
		return row[idx]
	}
	return ""
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
