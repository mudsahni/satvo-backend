package csvexport

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"satvos/internal/domain"
	"satvos/internal/validator/invoice"
)

// UTF-8 BOM bytes for Excel compatibility on Windows.
var BOM = []byte{0xEF, 0xBB, 0xBF}

// columns defines the CSV header row (30 columns).
var columns = []string{
	"Document Name",
	"Parsing Status",
	"Review Status",
	"Validation Status",
	"Reconciliation Status",
	"Invoice Number",
	"Invoice Date",
	"Invoice Type",
	"Place of Supply",
	"Reverse Charge",
	"Seller Name",
	"Seller GSTIN",
	"Seller State",
	"Seller State Code",
	"Buyer Name",
	"Buyer GSTIN",
	"Buyer State",
	"Buyer State Code",
	"Taxable Amount",
	"CGST",
	"SGST",
	"IGST",
	"Cess",
	"Total",
	"Currency",
	"Due Date",
	"Line Item Count",
	"Reviewer Notes",
	"Parsed At",
	"Created At",
}

// Writer wraps csv.Writer for exporting documents as CSV.
type Writer struct {
	csv *csv.Writer
}

// NewWriter creates a Writer that writes CSV to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{csv: csv.NewWriter(w)}
}

// WriteHeader writes the 30-column header row.
func (w *Writer) WriteHeader() error {
	return w.csv.Write(columns)
}

// WriteDocuments converts a batch of documents to CSV rows and writes them.
func (w *Writer) WriteDocuments(docs []domain.Document) error {
	for i := range docs {
		row := documentToRow(&docs[i])
		if err := w.csv.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// Flush flushes the underlying csv.Writer buffer.
func (w *Writer) Flush() {
	w.csv.Flush()
}

// Error returns any error from the underlying csv.Writer.
func (w *Writer) Error() error {
	return w.csv.Error()
}

// documentToRow converts a single document to a 30-element string slice.
// If the document is not successfully parsed or StructuredData is invalid,
// metadata columns are filled and invoice columns are left empty.
func documentToRow(doc *domain.Document) []string {
	row := make([]string, len(columns))

	// Metadata columns (always filled)
	row[0] = doc.Name
	row[1] = string(doc.ParsingStatus)
	row[2] = string(doc.ReviewStatus)
	row[3] = string(doc.ValidationStatus)
	row[4] = string(doc.ReconciliationStatus)
	row[27] = doc.ReviewerNotes
	row[28] = formatTime(doc.ParsedAt)
	row[29] = doc.CreatedAt.Format(time.RFC3339)

	// Invoice columns: only if parsing completed and JSON is valid
	if doc.ParsingStatus != domain.ParsingStatusCompleted || len(doc.StructuredData) == 0 {
		return row
	}

	var inv invoice.GSTInvoice
	if err := json.Unmarshal(doc.StructuredData, &inv); err != nil {
		return row
	}

	row[5] = inv.Invoice.InvoiceNumber
	row[6] = inv.Invoice.InvoiceDate
	row[7] = inv.Invoice.InvoiceType
	row[8] = inv.Invoice.PlaceOfSupply
	row[9] = formatBool(inv.Invoice.ReverseCharge)
	row[10] = inv.Seller.Name
	row[11] = inv.Seller.GSTIN
	row[12] = inv.Seller.State
	row[13] = inv.Seller.StateCode
	row[14] = inv.Buyer.Name
	row[15] = inv.Buyer.GSTIN
	row[16] = inv.Buyer.State
	row[17] = inv.Buyer.StateCode
	row[18] = formatMoney(inv.Totals.TaxableAmount)
	row[19] = formatMoney(inv.Totals.CGST)
	row[20] = formatMoney(inv.Totals.SGST)
	row[21] = formatMoney(inv.Totals.IGST)
	row[22] = formatMoney(inv.Totals.Cess)
	row[23] = formatMoney(inv.Totals.Total)
	row[24] = inv.Invoice.Currency
	row[25] = inv.Invoice.DueDate
	row[26] = strconv.Itoa(len(inv.LineItems))

	return row
}

func formatMoney(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func formatBool(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// nonAlphanumeric matches characters that are not alphanumeric, hyphen, or underscore.
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// multiUnderscore matches consecutive underscores.
var multiUnderscore = regexp.MustCompile(`_{2,}`)

// SanitizeFilename cleans a collection name for use in Content-Disposition.
// Replaces non-alphanumeric chars (except - _) with _, collapses consecutive
// underscores, and truncates to 100 chars.
func SanitizeFilename(name string) string {
	s := nonAlphanumeric.ReplaceAllString(name, "_")
	s = multiUnderscore.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

// BuildFilename returns a sanitized filename for Content-Disposition header.
// Format: {sanitized_collection_name}_{YYYY-MM-DD}.csv
func BuildFilename(collectionName string) string {
	sanitized := SanitizeFilename(collectionName)
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s_%s.csv", sanitized, date)
}
