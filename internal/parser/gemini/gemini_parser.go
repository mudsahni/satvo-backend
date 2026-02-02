package gemini

import (
	"context"
	"fmt"

	"satvos/internal/config"
	"satvos/internal/port"
)

// Parser implements port.DocumentParser using Google's Gemini API.
// This is currently a stub — the actual API integration is not yet implemented.
type Parser struct {
	apiKey string
	model  string
}

// NewParser creates a Gemini-based document parser.
func NewParser(cfg *config.ParserProviderConfig) *Parser {
	model := cfg.DefaultModel
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &Parser{
		apiKey: cfg.APIKey,
		model:  model,
	}
}

func (p *Parser) Parse(_ context.Context, _ port.ParseInput) (*port.ParseOutput, error) {
	return nil, fmt.Errorf("gemini parser not yet implemented")
}
