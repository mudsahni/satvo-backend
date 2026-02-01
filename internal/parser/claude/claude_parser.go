package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"satvos/internal/config"
	"satvos/internal/port"
)

const (
	apiURL     = "https://api.anthropic.com/v1/messages"
	apiVersion = "2023-06-01"
)

// Parser implements port.DocumentParser using the Anthropic Messages API.
type Parser struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

// NewParser creates a Claude-based document parser.
func NewParser(cfg *config.ParserConfig) *Parser {
	return newParser(cfg, apiURL)
}

// NewParserWithEndpoint creates a parser pointing at a custom API endpoint (for testing).
func NewParserWithEndpoint(cfg *config.ParserConfig, endpoint string) *Parser {
	return newParser(cfg, endpoint)
}

func newParser(cfg *config.ParserConfig, endpoint string) *Parser {
	model := cfg.DefaultModel
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	timeout := time.Duration(cfg.TimeoutSecs) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &Parser{
		apiKey:   cfg.APIKey,
		model:    model,
		endpoint: endpoint,
		client:   &http.Client{Timeout: timeout},
	}
}

func (p *Parser) Parse(ctx context.Context, input port.ParseInput) (*port.ParseOutput, error) {
	prompt := buildPrompt(input.DocumentType)

	contentBlocks, err := buildContentBlocks(input, prompt)
	if err != nil {
		return nil, fmt.Errorf("building content blocks: %w", err)
	}

	reqBody := map[string]interface{}{
		"model":      p.model,
		"max_tokens": 16384,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": contentBlocks,
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling anthropic API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return parseResponse(respBody, p.model, prompt)
}

func buildContentBlocks(input port.ParseInput, prompt string) ([]map[string]interface{}, error) {
	encoded := base64.StdEncoding.EncodeToString(input.FileBytes)
	var blocks []map[string]interface{}

	switch input.ContentType {
	case "application/pdf":
		blocks = append(blocks, map[string]interface{}{
			"type": "document",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": "application/pdf",
				"data":       encoded,
			},
		})
	case "image/jpeg", "image/png":
		blocks = append(blocks, map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": input.ContentType,
				"data":       encoded,
			},
		})
	default:
		return nil, fmt.Errorf("unsupported content type for parsing: %s", input.ContentType)
	}

	blocks = append(blocks, map[string]interface{}{
		"type": "text",
		"text": prompt,
	})

	return blocks, nil
}

func buildPrompt(documentType string) string {
	return `You are a document data extraction assistant. Analyze the provided ` + documentType + ` document and extract ALL data into the following JSON structure.

IMPORTANT INSTRUCTIONS:
- The document may span multiple pages. Extract ALL line items from every page and every section (e.g., Genuine Parts, Other Parts, Labor, Other Labor, Services, Other Charges) into a single flat "line_items" array.
- It is critical that you extract EVERY line item. Do not skip, summarize, or omit any items.
- Normalize all dates to DD-MM-YYYY format. Strip timestamps, annotations like "(On or Before)", and other non-date text.
- State codes must be exactly 2 digits, zero-padded (e.g., "07" not "7").

Return ONLY valid JSON with no markdown formatting, no code fences, no explanation — just the raw JSON object.

Return two top-level keys: "data" and "confidence_scores".

The "data" object must follow this schema:
{
  "invoice": {
    "invoice_number": "",
    "invoice_date": "",
    "due_date": "",
    "invoice_type": "",
    "currency": "",
    "place_of_supply": "",
    "reverse_charge": false
  },
  "seller": {
    "name": "", "address": "",
    "gstin": "", "pan": "",
    "state": "", "state_code": ""
  },
  "buyer": {
    "name": "", "address": "",
    "gstin": "", "pan": "",
    "state": "", "state_code": ""
  },
  "line_items": [
    {
      "description": "",
      "hsn_sac_code": "",
      "quantity": 0, "unit": "",
      "unit_price": 0, "discount": 0,
      "taxable_amount": 0,
      "cgst_rate": 0, "cgst_amount": 0,
      "sgst_rate": 0, "sgst_amount": 0,
      "igst_rate": 0, "igst_amount": 0,
      "total": 0
    }
  ],
  "totals": {
    "subtotal": 0, "total_discount": 0,
    "taxable_amount": 0,
    "cgst": 0, "sgst": 0, "igst": 0, "cess": 0,
    "round_off": 0, "total": 0,
    "amount_in_words": ""
  },
  "payment": {
    "bank_name": "",
    "account_number": "",
    "ifsc_code": "",
    "payment_terms": ""
  },
  "notes": ""
}

The "confidence_scores" object should mirror the "data" structure but with float values between 0.0 and 1.0 indicating your confidence for each extracted field. Use 0.0 for fields not found in the document.

If a field is not present in the document, use empty string for text, 0 for numbers, and false for booleans.`
}

// apiResponse models the Anthropic Messages API response.
type apiResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func parseResponse(body []byte, model, prompt string) (*port.ParseOutput, error) {
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	text := resp.Content[0].Text

	// Parse the JSON response from the LLM
	var parsed struct {
		Data             json.RawMessage `json:"data"`
		ConfidenceScores json.RawMessage `json:"confidence_scores"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("parsing LLM JSON output: %w (raw: %s)", err, truncate(text, 500))
	}

	return &port.ParseOutput{
		StructuredData:   parsed.Data,
		ConfidenceScores: parsed.ConfidenceScores,
		ModelUsed:        model,
		PromptUsed:       prompt,
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
