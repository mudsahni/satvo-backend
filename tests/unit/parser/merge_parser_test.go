package parser_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/parser"
	"satvos/internal/port"
	"satvos/internal/validator/invoice"
	"satvos/mocks"
)

func makeParseOutput(inv *invoice.GSTInvoice, conf *invoice.ConfidenceScores, model string) *port.ParseOutput {
	data, _ := json.Marshal(inv)
	confData, _ := json.Marshal(conf)
	return &port.ParseOutput{
		StructuredData:   data,
		ConfidenceScores: confData,
		ModelUsed:        model,
		PromptUsed:       "test prompt",
	}
}

func TestMergeParser_BothSucceed_Agreement(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	inv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{InvoiceNumber: "INV-001", InvoiceDate: "15/01/2025"},
		Seller:  invoice.Party{Name: "Seller Corp", GSTIN: "29ABCDE1234F1Z5"},
		Totals:  invoice.Totals{Total: 1000},
	}
	conf := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{InvoiceNumber: 0.8, InvoiceDate: 0.8},
		Seller:  invoice.PartyConfidence{Name: 0.8, GSTIN: 0.8},
		Totals:  invoice.TotalsConfidence{Total: 0.8},
	}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(makeParseOutput(&inv, &conf, "claude"), nil)
	secondary.On("Parse", mock.Anything, input).Return(makeParseOutput(&inv, &conf, "gemini"), nil)

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "claude", result.ModelUsed)
	assert.Equal(t, "gemini", result.SecondaryModel)
	assert.NotNil(t, result.FieldProvenance)
	assert.Equal(t, "agree", result.FieldProvenance["invoice.invoice_number"])
	assert.Equal(t, "agree", result.FieldProvenance["seller.gstin"])
	assert.Equal(t, "agree", result.FieldProvenance["totals.total"])

	// Confidence should be boosted on agreement
	var mergedConf invoice.ConfidenceScores
	err = json.Unmarshal(result.ConfidenceScores, &mergedConf)
	assert.NoError(t, err)
	assert.Greater(t, mergedConf.Invoice.InvoiceNumber, 0.8)
}

func TestMergeParser_BothSucceed_Disagreement(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	pInv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{InvoiceNumber: "INV-001"},
		Seller:  invoice.Party{Name: "Primary Seller"},
		Totals:  invoice.Totals{Total: 1000},
	}
	sInv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{InvoiceNumber: "INV-002"},
		Seller:  invoice.Party{Name: "Secondary Seller"},
		Totals:  invoice.Totals{Total: 2000},
	}
	pConf := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{InvoiceNumber: 0.9},
		Seller:  invoice.PartyConfidence{Name: 0.9},
		Totals:  invoice.TotalsConfidence{Total: 0.9},
	}
	sConf := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{InvoiceNumber: 0.7},
		Seller:  invoice.PartyConfidence{Name: 0.7},
		Totals:  invoice.TotalsConfidence{Total: 0.7},
	}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(makeParseOutput(&pInv, &pConf, "claude"), nil)
	secondary.On("Parse", mock.Anything, input).Return(makeParseOutput(&sInv, &sConf, "gemini"), nil)

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// On disagreement, primary value should be kept but confidence reduced
	var mergedData invoice.GSTInvoice
	err = json.Unmarshal(result.StructuredData, &mergedData)
	assert.NoError(t, err)
	assert.Equal(t, "INV-001", mergedData.Invoice.InvoiceNumber) // primary kept

	var mergedConf invoice.ConfidenceScores
	err = json.Unmarshal(result.ConfidenceScores, &mergedConf)
	assert.NoError(t, err)
	assert.Less(t, mergedConf.Invoice.InvoiceNumber, 0.9) // confidence reduced

	assert.Equal(t, "disagreement", result.FieldProvenance["invoice.invoice_number"])
}

func TestMergeParser_BothSucceed_OneEmpty(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	pInv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{InvoiceNumber: "INV-001"},
		Seller:  invoice.Party{Name: ""}, // primary has empty name
	}
	sInv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{InvoiceNumber: "INV-001"},
		Seller:  invoice.Party{Name: "Secondary Seller"}, // secondary has it
	}
	pConf := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{InvoiceNumber: 0.9},
		Seller:  invoice.PartyConfidence{Name: 0.0},
	}
	sConf := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{InvoiceNumber: 0.9},
		Seller:  invoice.PartyConfidence{Name: 0.85},
	}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(makeParseOutput(&pInv, &pConf, "claude"), nil)
	secondary.On("Parse", mock.Anything, input).Return(makeParseOutput(&sInv, &sConf, "gemini"), nil)

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)

	var mergedData invoice.GSTInvoice
	err = json.Unmarshal(result.StructuredData, &mergedData)
	assert.NoError(t, err)
	assert.Equal(t, "Secondary Seller", mergedData.Seller.Name) // filled from secondary
	assert.Equal(t, "secondary", result.FieldProvenance["seller.name"])
}

func TestMergeParser_BothSucceed_GSTINFormatHeuristic(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	pInv := invoice.GSTInvoice{
		Seller: invoice.Party{GSTIN: "invalid-gstin"},
	}
	sInv := invoice.GSTInvoice{
		Seller: invoice.Party{GSTIN: "29ABCDE1234F1Z5"}, // valid GSTIN format
	}
	pConf := invoice.ConfidenceScores{Seller: invoice.PartyConfidence{GSTIN: 0.7}}
	sConf := invoice.ConfidenceScores{Seller: invoice.PartyConfidence{GSTIN: 0.8}}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(makeParseOutput(&pInv, &pConf, "claude"), nil)
	secondary.On("Parse", mock.Anything, input).Return(makeParseOutput(&sInv, &sConf, "gemini"), nil)

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)

	var mergedData invoice.GSTInvoice
	err = json.Unmarshal(result.StructuredData, &mergedData)
	assert.NoError(t, err)
	// Secondary should be preferred because it matches GSTIN format
	assert.Equal(t, "29ABCDE1234F1Z5", mergedData.Seller.GSTIN)
	assert.Equal(t, "secondary_format", result.FieldProvenance["seller.gstin"])
}

func TestMergeParser_PrimaryFails(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	sInv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{InvoiceNumber: "INV-001"},
	}
	sConf := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{InvoiceNumber: 0.9},
	}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(nil, errors.New("primary API error"))
	secondary.On("Parse", mock.Anything, input).Return(makeParseOutput(&sInv, &sConf, "gemini"), nil)

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "secondary_only", result.FieldProvenance["_source"])
}

func TestMergeParser_SecondaryFails(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	pInv := invoice.GSTInvoice{
		Invoice: invoice.InvoiceHeader{InvoiceNumber: "INV-001"},
	}
	pConf := invoice.ConfidenceScores{
		Invoice: invoice.InvoiceConfidence{InvoiceNumber: 0.9},
	}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(makeParseOutput(&pInv, &pConf, "claude"), nil)
	secondary.On("Parse", mock.Anything, input).Return(nil, errors.New("secondary API error"))

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "primary_only", result.FieldProvenance["_source"])
}

func TestMergeParser_BothFail(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(nil, errors.New("primary error"))
	secondary.On("Parse", mock.Anything, input).Return(nil, errors.New("secondary error"))

	result, err := mp.Parse(context.Background(), input)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both parsers failed")
}

func TestMergeParser_LineItems_SecondaryHasMore(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	pInv := invoice.GSTInvoice{
		LineItems: []invoice.LineItem{
			{Description: "Item 1", Total: 100},
		},
	}
	sInv := invoice.GSTInvoice{
		LineItems: []invoice.LineItem{
			{Description: "Item 1", Total: 100},
			{Description: "Item 2", Total: 200},
		},
	}
	pConf := invoice.ConfidenceScores{}
	sConf := invoice.ConfidenceScores{}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(makeParseOutput(&pInv, &pConf, "claude"), nil)
	secondary.On("Parse", mock.Anything, input).Return(makeParseOutput(&sInv, &sConf, "gemini"), nil)

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)

	var mergedData invoice.GSTInvoice
	err = json.Unmarshal(result.StructuredData, &mergedData)
	assert.NoError(t, err)
	assert.Len(t, mergedData.LineItems, 2)
	assert.Equal(t, "secondary", result.FieldProvenance["line_items"])
}

func TestMergeParser_LineItems_PrimaryHasMoreOrEqual(t *testing.T) {
	primary := new(mocks.MockDocumentParser)
	secondary := new(mocks.MockDocumentParser)
	mp := parser.NewMergeParser(primary, secondary)

	pInv := invoice.GSTInvoice{
		LineItems: []invoice.LineItem{
			{Description: "Item 1", Total: 100},
			{Description: "Item 2", Total: 200},
		},
	}
	sInv := invoice.GSTInvoice{
		LineItems: []invoice.LineItem{
			{Description: "Item 1", Total: 100},
		},
	}
	pConf := invoice.ConfidenceScores{}
	sConf := invoice.ConfidenceScores{}

	input := port.ParseInput{FileBytes: []byte("test"), ContentType: "application/pdf", DocumentType: "invoice"}

	primary.On("Parse", mock.Anything, input).Return(makeParseOutput(&pInv, &pConf, "claude"), nil)
	secondary.On("Parse", mock.Anything, input).Return(makeParseOutput(&sInv, &sConf, "gemini"), nil)

	result, err := mp.Parse(context.Background(), input)

	assert.NoError(t, err)

	var mergedData invoice.GSTInvoice
	err = json.Unmarshal(result.StructuredData, &mergedData)
	assert.NoError(t, err)
	assert.Len(t, mergedData.LineItems, 2)
	assert.Equal(t, "primary", result.FieldProvenance["line_items"])
}
