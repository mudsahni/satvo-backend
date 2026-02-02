package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server ServerConfig
	DB     DBConfig
	JWT    JWTConfig
	S3     S3Config
	Log    LogConfig
	Parser ParserConfig
}

// ParserProviderConfig holds settings for a single LLM parser provider.
type ParserProviderConfig struct {
	Provider     string `mapstructure:"provider"`
	APIKey       string `mapstructure:"api_key"`
	DefaultModel string `mapstructure:"default_model"`
	MaxRetries   int    `mapstructure:"max_retries"`
	TimeoutSecs  int    `mapstructure:"timeout_secs"`
}

// ParserConfig holds LLM document parser settings with multi-provider support.
type ParserConfig struct {
	// Legacy flat fields (backwards-compatible)
	Provider     string `mapstructure:"provider"`
	APIKey       string `mapstructure:"api_key"`
	DefaultModel string `mapstructure:"default_model"`
	MaxRetries   int    `mapstructure:"max_retries"`
	TimeoutSecs  int    `mapstructure:"timeout_secs"`

	// Multi-provider fields
	Primary   ParserProviderConfig `mapstructure:"primary"`
	Secondary ParserProviderConfig `mapstructure:"secondary"`
}

// PrimaryConfig returns the primary parser provider config, falling back to legacy flat fields.
func (p *ParserConfig) PrimaryConfig() *ParserProviderConfig {
	if p.Primary.Provider != "" {
		return &p.Primary
	}
	return &ParserProviderConfig{
		Provider:     p.Provider,
		APIKey:       p.APIKey,
		DefaultModel: p.DefaultModel,
		MaxRetries:   p.MaxRetries,
		TimeoutSecs:  p.TimeoutSecs,
	}
}

// SecondaryConfig returns the secondary parser provider config, or nil if not configured.
func (p *ParserConfig) SecondaryConfig() *ParserProviderConfig {
	if p.Secondary.Provider != "" {
		return &p.Secondary
	}
	return nil
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	Environment  string        `mapstructure:"environment"`
}

// DBConfig holds PostgreSQL connection settings.
type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
	MaxOpen  int    `mapstructure:"max_open"`
	MaxIdle  int    `mapstructure:"max_idle"`
}

// DSN returns the PostgreSQL connection string.
func (d *DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// JWTConfig holds JWT signing and expiry settings.
type JWTConfig struct {
	Secret             string        `mapstructure:"secret"`
	AccessTokenExpiry  time.Duration `mapstructure:"access_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_expiry"`
	Issuer             string        `mapstructure:"issuer"`
}

// S3Config holds AWS S3 settings.
type S3Config struct {
	Region        string `mapstructure:"region"`
	Bucket        string `mapstructure:"bucket"`
	Endpoint      string `mapstructure:"endpoint"`
	AccessKey     string `mapstructure:"access_key"`
	SecretKey     string `mapstructure:"secret_key"`
	MaxFileSizeMB int64  `mapstructure:"max_file_size_mb"`
	PresignExpiry int64  `mapstructure:"presign_expiry"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads configuration from environment variables with the SATVOS_ prefix.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("SATVOS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Server defaults
	v.SetDefault("server.port", ":8080")
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.environment", "development")

	// DB defaults
	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.user", "satvos")
	v.SetDefault("db.password", "satvos_secret")
	v.SetDefault("db.name", "satvos_db")
	v.SetDefault("db.sslmode", "disable")
	v.SetDefault("db.max_open", 25)
	v.SetDefault("db.max_idle", 10)

	// JWT defaults
	v.SetDefault("jwt.secret", "change-me-in-production")
	v.SetDefault("jwt.access_expiry", "15m")
	v.SetDefault("jwt.refresh_expiry", "168h")
	v.SetDefault("jwt.issuer", "satvos")

	// S3 defaults
	v.SetDefault("s3.region", "us-east-1")
	v.SetDefault("s3.bucket", "satvos-uploads")
	v.SetDefault("s3.endpoint", "")
	v.SetDefault("s3.max_file_size_mb", 50)
	v.SetDefault("s3.presign_expiry", 3600)

	// Log defaults
	v.SetDefault("log.level", "debug")
	v.SetDefault("log.format", "console")

	// Parser defaults (legacy flat)
	v.SetDefault("parser.provider", "claude")
	v.SetDefault("parser.api_key", "")
	v.SetDefault("parser.default_model", "claude-sonnet-4-20250514")
	v.SetDefault("parser.max_retries", 2)
	v.SetDefault("parser.timeout_secs", 120)

	// Parser primary/secondary defaults
	v.SetDefault("parser.primary.provider", "")
	v.SetDefault("parser.primary.api_key", "")
	v.SetDefault("parser.primary.default_model", "")
	v.SetDefault("parser.primary.max_retries", 2)
	v.SetDefault("parser.primary.timeout_secs", 120)
	v.SetDefault("parser.secondary.provider", "")
	v.SetDefault("parser.secondary.api_key", "")
	v.SetDefault("parser.secondary.default_model", "")
	v.SetDefault("parser.secondary.max_retries", 2)
	v.SetDefault("parser.secondary.timeout_secs", 120)

	// Bind environment variables explicitly for nested keys
	envBindings := map[string]string{
		"server.port":          "SATVOS_SERVER_PORT",
		"server.read_timeout":  "SATVOS_SERVER_READ_TIMEOUT",
		"server.write_timeout": "SATVOS_SERVER_WRITE_TIMEOUT",
		"server.environment":   "SATVOS_SERVER_ENVIRONMENT",
		"db.host":              "SATVOS_DB_HOST",
		"db.port":              "SATVOS_DB_PORT",
		"db.user":              "SATVOS_DB_USER",
		"db.password":          "SATVOS_DB_PASSWORD",
		"db.name":              "SATVOS_DB_NAME",
		"db.sslmode":           "SATVOS_DB_SSLMODE",
		"db.max_open":          "SATVOS_DB_MAX_OPEN",
		"db.max_idle":          "SATVOS_DB_MAX_IDLE",
		"jwt.secret":           "SATVOS_JWT_SECRET",
		"jwt.access_expiry":    "SATVOS_JWT_ACCESS_EXPIRY",
		"jwt.refresh_expiry":   "SATVOS_JWT_REFRESH_EXPIRY",
		"jwt.issuer":           "SATVOS_JWT_ISSUER",
		"s3.region":            "SATVOS_S3_REGION",
		"s3.bucket":            "SATVOS_S3_BUCKET",
		"s3.endpoint":          "SATVOS_S3_ENDPOINT",
		"s3.access_key":        "SATVOS_S3_ACCESS_KEY",
		"s3.secret_key":        "SATVOS_S3_SECRET_KEY",
		"s3.max_file_size_mb":  "SATVOS_S3_MAX_FILE_SIZE_MB",
		"s3.presign_expiry":    "SATVOS_S3_PRESIGN_EXPIRY",
		"log.level":            "SATVOS_LOG_LEVEL",
		"log.format":           "SATVOS_LOG_FORMAT",
		"parser.provider":                "SATVOS_PARSER_PROVIDER",
		"parser.api_key":                 "SATVOS_PARSER_API_KEY",
		"parser.default_model":           "SATVOS_PARSER_DEFAULT_MODEL",
		"parser.max_retries":             "SATVOS_PARSER_MAX_RETRIES",
		"parser.timeout_secs":            "SATVOS_PARSER_TIMEOUT_SECS",
		"parser.primary.provider":        "SATVOS_PARSER_PRIMARY_PROVIDER",
		"parser.primary.api_key":         "SATVOS_PARSER_PRIMARY_API_KEY",
		"parser.primary.default_model":   "SATVOS_PARSER_PRIMARY_DEFAULT_MODEL",
		"parser.primary.max_retries":     "SATVOS_PARSER_PRIMARY_MAX_RETRIES",
		"parser.primary.timeout_secs":    "SATVOS_PARSER_PRIMARY_TIMEOUT_SECS",
		"parser.secondary.provider":      "SATVOS_PARSER_SECONDARY_PROVIDER",
		"parser.secondary.api_key":       "SATVOS_PARSER_SECONDARY_API_KEY",
		"parser.secondary.default_model": "SATVOS_PARSER_SECONDARY_DEFAULT_MODEL",
		"parser.secondary.max_retries":   "SATVOS_PARSER_SECONDARY_MAX_RETRIES",
		"parser.secondary.timeout_secs":  "SATVOS_PARSER_SECONDARY_TIMEOUT_SECS",
	}
	for key, env := range envBindings {
		_ = v.BindEnv(key, env)
	}

	cfg := &Config{}
	cfg.Server = ServerConfig{
		Port:         v.GetString("server.port"),
		ReadTimeout:  v.GetDuration("server.read_timeout"),
		WriteTimeout: v.GetDuration("server.write_timeout"),
		Environment:  v.GetString("server.environment"),
	}
	cfg.DB = DBConfig{
		Host:     v.GetString("db.host"),
		Port:     v.GetInt("db.port"),
		User:     v.GetString("db.user"),
		Password: v.GetString("db.password"),
		Name:     v.GetString("db.name"),
		SSLMode:  v.GetString("db.sslmode"),
		MaxOpen:  v.GetInt("db.max_open"),
		MaxIdle:  v.GetInt("db.max_idle"),
	}
	cfg.JWT = JWTConfig{
		Secret:             v.GetString("jwt.secret"),
		AccessTokenExpiry:  v.GetDuration("jwt.access_expiry"),
		RefreshTokenExpiry: v.GetDuration("jwt.refresh_expiry"),
		Issuer:             v.GetString("jwt.issuer"),
	}
	cfg.S3 = S3Config{
		Region:        v.GetString("s3.region"),
		Bucket:        v.GetString("s3.bucket"),
		Endpoint:      v.GetString("s3.endpoint"),
		AccessKey:     v.GetString("s3.access_key"),
		SecretKey:     v.GetString("s3.secret_key"),
		MaxFileSizeMB: v.GetInt64("s3.max_file_size_mb"),
		PresignExpiry: v.GetInt64("s3.presign_expiry"),
	}
	cfg.Log = LogConfig{
		Level:  v.GetString("log.level"),
		Format: v.GetString("log.format"),
	}
	cfg.Parser = ParserConfig{
		Provider:     v.GetString("parser.provider"),
		APIKey:       v.GetString("parser.api_key"),
		DefaultModel: v.GetString("parser.default_model"),
		MaxRetries:   v.GetInt("parser.max_retries"),
		TimeoutSecs:  v.GetInt("parser.timeout_secs"),
		Primary: ParserProviderConfig{
			Provider:     v.GetString("parser.primary.provider"),
			APIKey:       v.GetString("parser.primary.api_key"),
			DefaultModel: v.GetString("parser.primary.default_model"),
			MaxRetries:   v.GetInt("parser.primary.max_retries"),
			TimeoutSecs:  v.GetInt("parser.primary.timeout_secs"),
		},
		Secondary: ParserProviderConfig{
			Provider:     v.GetString("parser.secondary.provider"),
			APIKey:       v.GetString("parser.secondary.api_key"),
			DefaultModel: v.GetString("parser.secondary.default_model"),
			MaxRetries:   v.GetInt("parser.secondary.max_retries"),
			TimeoutSecs:  v.GetInt("parser.secondary.timeout_secs"),
		},
	}

	return cfg, nil
}
