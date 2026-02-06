package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"satvos/internal/config"
)

func TestParserConfig_PrimaryConfig_LegacyFallback(t *testing.T) {
	cfg := config.ParserConfig{
		Provider:     "claude",
		APIKey:       "sk-legacy",
		DefaultModel: "claude-sonnet-4-20250514",
		MaxRetries:   3,
		TimeoutSecs:  30,
	}

	primary := cfg.PrimaryConfig()

	assert.Equal(t, "claude", primary.Provider)
	assert.Equal(t, "sk-legacy", primary.APIKey)
	assert.Equal(t, "claude-sonnet-4-20250514", primary.DefaultModel)
	assert.Equal(t, 3, primary.MaxRetries)
	assert.Equal(t, 30, primary.TimeoutSecs)
}

func TestParserConfig_PrimaryConfig_ExplicitPrimary(t *testing.T) {
	cfg := config.ParserConfig{
		Provider: "legacy-should-be-ignored",
		Primary: config.ParserProviderConfig{
			Provider:     "claude",
			APIKey:       "sk-primary",
			DefaultModel: "claude-opus-4-20250514",
		},
	}

	primary := cfg.PrimaryConfig()

	assert.Equal(t, "claude", primary.Provider)
	assert.Equal(t, "sk-primary", primary.APIKey)
	assert.Equal(t, "claude-opus-4-20250514", primary.DefaultModel)
}

func TestParserConfig_SecondaryConfig_NotConfigured(t *testing.T) {
	cfg := config.ParserConfig{
		Provider: "claude",
		APIKey:   "sk-test",
	}

	secondary := cfg.SecondaryConfig()

	assert.Nil(t, secondary)
}

func TestParserConfig_SecondaryConfig_Configured(t *testing.T) {
	cfg := config.ParserConfig{
		Primary: config.ParserProviderConfig{
			Provider: "claude",
			APIKey:   "sk-primary",
		},
		Secondary: config.ParserProviderConfig{
			Provider:     "gemini",
			APIKey:       "gk-secondary",
			DefaultModel: "gemini-2.0-flash",
		},
	}

	secondary := cfg.SecondaryConfig()

	assert.NotNil(t, secondary)
	assert.Equal(t, "gemini", secondary.Provider)
	assert.Equal(t, "gk-secondary", secondary.APIKey)
	assert.Equal(t, "gemini-2.0-flash", secondary.DefaultModel)
}

func TestParserConfig_TertiaryConfig_NotConfigured(t *testing.T) {
	cfg := config.ParserConfig{
		Provider: "claude",
		APIKey:   "sk-test",
	}

	tertiary := cfg.TertiaryConfig()

	assert.Nil(t, tertiary)
}

func TestParserConfig_TertiaryConfig_Configured(t *testing.T) {
	cfg := config.ParserConfig{
		Primary: config.ParserProviderConfig{
			Provider: "claude",
			APIKey:   "sk-primary",
		},
		Tertiary: config.ParserProviderConfig{
			Provider:     "openai",
			APIKey:       "sk-tertiary",
			DefaultModel: "gpt-4o",
		},
	}

	tertiary := cfg.TertiaryConfig()

	assert.NotNil(t, tertiary)
	assert.Equal(t, "openai", tertiary.Provider)
	assert.Equal(t, "sk-tertiary", tertiary.APIKey)
	assert.Equal(t, "gpt-4o", tertiary.DefaultModel)
}
