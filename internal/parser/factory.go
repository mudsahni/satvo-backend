package parser

import (
	"fmt"

	"satvos/internal/config"
	"satvos/internal/port"
)

// ProviderFactory is a function that creates a DocumentParser from a provider config.
type ProviderFactory func(cfg *config.ParserProviderConfig) (port.DocumentParser, error)

// registry of parser provider factories, populated by init() in each provider package
// or explicitly via RegisterProvider.
var providers = map[string]ProviderFactory{}

// RegisterProvider registers a parser provider factory by name.
func RegisterProvider(name string, factory ProviderFactory) {
	providers[name] = factory
}

// NewParser creates a DocumentParser from a provider config using the registered factory.
func NewParser(cfg *config.ParserProviderConfig) (port.DocumentParser, error) {
	factory, ok := providers[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown parser provider: %s", cfg.Provider)
	}
	return factory(cfg)
}
