package csvexport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"satvos/internal/domain"
	"satvos/internal/validator/invoice"
)

func TestWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteHeader())
	w.Flush()
	require.NoError(t, w.Error())

	r := csv.NewReader(&buf)
	row, err := r.Read()
	require.NoError(t, err)

	assert.Len(t, row, 30)
	assert.Equal(t, "Document Name", row[0])
	assert.Equal(t, "Parsing Status", row[1])
	assert.Equal(t, "Created At", row[29])
}

func TestWriteDocuments_Completed(t *testing.T) {
	inv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{
			InvoiceNumber: "INV-001",
			InvoiceDate:   "2025-01-15",
			InvoiceType:   "Tax Invoice",
			PlaceOfSupply: "29-Karnataka",
			ReverseCharge: true,
			Currency:      "INR",
			DueDate:       "2025-02-15",
		},
		Seller: invoice.Party{
			Name:      "Seller Corp",
			GSTIN:     "29ABCDE1234F1Z5",
			State:     "Karnataka",
			StateCode: "29",
		},
		Buyer: invoice.Party{
			Name:      "Buyer Inc",
			GSTIN:     "07FGHIJ5678K2Z3",
			State:     "Delhi",
			StateCode: "07",
		},
		LineItems: []invoice.LineItem{
			{Description: "Item A", Total: 1000},
			{Description: "Item B", Total: 2000},
		},
		Totals: invoice.Totals{
			TaxableAmount: 10000.50,
			CGST:          900.25,
			SGST:          900.25,
			IGST:          0,
			Cess:          50.10,
			Total:         11851.10,
		},
	}

	structuredData, err := json.Marshal(inv)
	require.NoError(t, err)

	parsedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	createdAt := time.Date(2025, 1, 14, 8, 0, 0, 0, time.UTC)

	doc := domain.Document{
		ID:                   uuid.New(),
		Name:                 "Test Invoice",
		ParsingStatus:        domain.ParsingStatusCompleted,
		ReviewStatus:         domain.ReviewStatusPending,
		ValidationStatus:     domain.ValidationStatusValid,
		ReconciliationStatus: domain.ReconciliationStatusValid,
		StructuredData:       structuredData,
		ReviewerNotes:        "Looks good",
		ParsedAt:             &parsedAt,
		CreatedAt:            createdAt,
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteDocuments([]domain.Document{doc}))
	w.Flush()
	require.NoError(t, w.Error())

	r := csv.NewReader(&buf)
	row, err := r.Read()
	require.NoError(t, err)

	assert.Len(t, row, 30)
	assert.Equal(t, "Test Invoice", row[0])
	assert.Equal(t, "completed", row[1])
	assert.Equal(t, "pending", row[2])
	assert.Equal(t, "valid", row[3])
	assert.Equal(t, "valid", row[4])
	assert.Equal(t, "INV-001", row[5])
	assert.Equal(t, "2025-01-15", row[6])
	assert.Equal(t, "Tax Invoice", row[7])
	assert.Equal(t, "29-Karnataka", row[8])
	assert.Equal(t, "Yes", row[9])
	assert.Equal(t, "Seller Corp", row[10])
	assert.Equal(t, "29ABCDE1234F1Z5", row[11])
	assert.Equal(t, "Karnataka", row[12])
	assert.Equal(t, "29", row[13])
	assert.Equal(t, "Buyer Inc", row[14])
	assert.Equal(t, "07FGHIJ5678K2Z3", row[15])
	assert.Equal(t, "Delhi", row[16])
	assert.Equal(t, "07", row[17])
	assert.Equal(t, "10000.50", row[18])
	assert.Equal(t, "900.25", row[19])
	assert.Equal(t, "900.25", row[20])
	assert.Equal(t, "0.00", row[21])
	assert.Equal(t, "50.10", row[22])
	assert.Equal(t, "11851.10", row[23])
	assert.Equal(t, "INR", row[24])
	assert.Equal(t, "2025-02-15", row[25])
	assert.Equal(t, "2", row[26])
	assert.Equal(t, "Looks good", row[27])
	assert.Equal(t, "2025-01-15T10:30:00Z", row[28])
	assert.Equal(t, "2025-01-14T08:00:00Z", row[29])
}

func TestWriteDocuments_Unparsed(t *testing.T) {
	createdAt := time.Date(2025, 1, 14, 8, 0, 0, 0, time.UTC)
	doc := domain.Document{
		ID:                   uuid.New(),
		Name:                 "Pending Doc",
		ParsingStatus:        domain.ParsingStatusPending,
		ReviewStatus:         domain.ReviewStatusPending,
		ValidationStatus:     domain.ValidationStatusPending,
		ReconciliationStatus: domain.ReconciliationStatusPending,
		CreatedAt:            createdAt,
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteDocuments([]domain.Document{doc}))
	w.Flush()
	require.NoError(t, w.Error())

	r := csv.NewReader(&buf)
	row, err := r.Read()
	require.NoError(t, err)

	assert.Len(t, row, 30)
	assert.Equal(t, "Pending Doc", row[0])
	assert.Equal(t, "pending", row[1])
	// Invoice columns should be empty
	for i := 5; i <= 26; i++ {
		assert.Empty(t, row[i], "column %d should be empty for unparsed doc", i)
	}
	assert.Equal(t, "", row[28]) // parsed_at empty
	assert.Equal(t, "2025-01-14T08:00:00Z", row[29])
}

func TestWriteDocuments_MalformedJSON(t *testing.T) {
	createdAt := time.Date(2025, 1, 14, 8, 0, 0, 0, time.UTC)
	doc := domain.Document{
		ID:               uuid.New(),
		Name:             "Bad JSON",
		ParsingStatus:    domain.ParsingStatusCompleted,
		StructuredData:   json.RawMessage(`{invalid json`),
		ReviewStatus:     domain.ReviewStatusPending,
		ValidationStatus: domain.ValidationStatusPending,
		CreatedAt:        createdAt,
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteDocuments([]domain.Document{doc}))
	w.Flush()
	require.NoError(t, w.Error())

	r := csv.NewReader(&buf)
	row, err := r.Read()
	require.NoError(t, err)

	assert.Len(t, row, 30)
	assert.Equal(t, "Bad JSON", row[0])
	assert.Equal(t, "completed", row[1])
	// Invoice columns should be empty due to unmarshal failure
	for i := 5; i <= 26; i++ {
		assert.Empty(t, row[i], "column %d should be empty for malformed JSON", i)
	}
}

func TestWriteDocuments_MonetaryFormatting(t *testing.T) {
	inv := invoice.GSTInvoice{
		Totals: invoice.Totals{
			TaxableAmount: 1000,    // whole number
			CGST:          99.999,  // rounds to 2 decimal places
			SGST:          0.1,     // trailing zero
			Total:         1100.10, // exact
		},
	}
	structuredData, err := json.Marshal(inv)
	require.NoError(t, err)

	doc := domain.Document{
		Name:           "Money Test",
		ParsingStatus:  domain.ParsingStatusCompleted,
		StructuredData: structuredData,
		CreatedAt:      time.Now(),
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteDocuments([]domain.Document{doc}))
	w.Flush()

	r := csv.NewReader(&buf)
	row, err := r.Read()
	require.NoError(t, err)

	assert.Equal(t, "1000.00", row[18])  // TaxableAmount
	assert.Equal(t, "100.00", row[19])   // CGST (99.999 rounds)
	assert.Equal(t, "0.10", row[20])     // SGST
	assert.Equal(t, "1100.10", row[23])  // Total
}

func TestWriteDocuments_ReverseCharge(t *testing.T) {
	tests := []struct {
		name     string
		rc       bool
		expected string
	}{
		{"true", true, "Yes"},
		{"false", false, "No"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := invoice.GSTInvoice{
				Invoice: invoice.InvoiceHeader{ReverseCharge: tt.rc},
			}
			data, _ := json.Marshal(inv)
			doc := domain.Document{
				Name:           "RC Test",
				ParsingStatus:  domain.ParsingStatusCompleted,
				StructuredData: data,
				CreatedAt:      time.Now(),
			}

			var buf bytes.Buffer
			w := NewWriter(&buf)
			require.NoError(t, w.WriteDocuments([]domain.Document{doc}))
			w.Flush()

			r := csv.NewReader(&buf)
			row, err := r.Read()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, row[9])
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "Q3 Purchase Invoices", "Q3_Purchase_Invoices"},
		{"special chars", "FY 2024-25 / Q3 (Oct–Dec)", "FY_2024-25_Q3_Oct_Dec"},
		{"unicode", "कंपनी Invoices", "Invoices"},
		{"hyphens and underscores preserved", "my-collection_2025", "my-collection_2025"},
		{"consecutive underscores collapsed", "test___collection", "test_collection"},
		{"leading/trailing cleaned", "  hello  ", "hello"},
		{
			"long name truncated",
			"abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz-extra",
			"abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrs",
		},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeFilename(tt.input))
		})
	}
}

func TestBuildFilename(t *testing.T) {
	filename := BuildFilename("Q3 Purchase Invoices")
	today := time.Now().Format("2006-01-02")
	assert.Equal(t, "Q3_Purchase_Invoices_"+today+".csv", filename)
}
