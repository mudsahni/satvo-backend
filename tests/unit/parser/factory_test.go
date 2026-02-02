package parser_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"satvos/internal/config"
	"satvos/internal/parser"
	"satvos/internal/port"
)

func TestFactory_RegisterAndCreate(t *testing.T) {
	parser.RegisterProvider("test-provider", func(cfg *config.ParserProviderConfig) (port.DocumentParser, error) {
		return &stubParser{model: cfg.DefaultModel}, nil
	})

	p, err := parser.NewParser(&config.ParserProviderConfig{
		Provider:     "test-provider",
		DefaultModel: "test-model",
	})

	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestFactory_UnknownProvider(t *testing.T) {
	p, err := parser.NewParser(&config.ParserProviderConfig{
		Provider: "nonexistent-provider-xyz",
	})

	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown parser provider")
}

// stubParser is a minimal DocumentParser for testing the factory.
type stubParser struct {
	model string
}

func (s *stubParser) Parse(_ context.Context, _ port.ParseInput) (*port.ParseOutput, error) {
	return &port.ParseOutput{ModelUsed: s.model}, nil
}
