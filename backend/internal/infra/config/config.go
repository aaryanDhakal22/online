// Package config loads configuration from environment variables. Required
// vars cause a clean Fatal exit on startup; optional vars fall back to
// defaults. There is no file-based config and no remote secrets manager —
// Doppler (or the local .env file) injects everything as env vars.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
)

type AppConfig struct {
	Env    string // dev/staging/prod
	Domain string
}

type ServerConfig struct {
	Port string
	// RateLimitPerSec controls the per-IP token-bucket rate limit applied
	// in transport.AddDefaultMiddlewares. 0 disables the limiter (default
	// in dev so the pytest suite can fire sequential requests without
	// being throttled). Non-dev defaults to 10 req/s; override per-env
	// via RATE_LIMIT_PER_SEC in Doppler.
	RateLimitPerSec int
}

type LogConfig struct {
	Level zerolog.Level
	Style string // "json" or "console"
}

type AuthConfig struct {
	DefaultAPIKey   string
	AdminPassHash   string
}

// ScopeConfig is the (tenant_id, location_id) pair online/ stamps on every
// SQS publish. main/ IGNORES these for the online source — it routes each
// order to a tenant by the vendor body's store_id (falling back to
// store_name) — so they're optional: each is sent only when set. If/when we
// move to a multi-restaurant aggregator the pair gets resolved per-request
// instead of being a static env var.
type ScopeConfig struct {
	TenantID   string
	LocationID string
}

type SQSConfig struct {
	QueueURL string
	Region   string
	// Endpoint is an optional AWS endpoint override. Set in dev/staging to
	// point at a local ElasticMQ instance (e.g. http://elasticmq:9324).
	// In prod, leave empty so the SDK targets real AWS SQS.
	Endpoint string
}

type OTELConfig struct {
	Endpoint    string
	ServiceName string
}

type Config struct {
	App    AppConfig
	Server ServerConfig
	Log    LogConfig
	Auth   AuthConfig
	Scope  ScopeConfig
	SQS    SQSConfig
	OTEL   OTELConfig
	DBPath string

	logger *zerolog.Logger
}

// New loads config from env. The bootstrap logger is used to log resolution
// progress and to call Fatal on missing required vars (which os.Exit(1)s).
func New(logger *zerolog.Logger) *Config {
	cfg := &Config{logger: logger}

	cfg.App = AppConfig{
		Env:    cfg.GetEnv("APP_ENV", true, "dev"),
		Domain: cfg.GetEnv("DOMAIN", false, "localhost"),
	}
	defaultRate := 10
	if cfg.App.Env == "dev" {
		defaultRate = 0 // disabled — tests rely on this
	}
	cfg.Server = ServerConfig{
		Port:            cfg.GetEnv("SERVER_PORT", false, "1323"),
		RateLimitPerSec: cfg.GetEnvInt("RATE_LIMIT_PER_SEC", defaultRate),
	}
	// Dev forces ConsoleWriter on stderr regardless of LOG_STYLE — pretty
	// output is what dev needs and we don't want a leftover LOG_STYLE=json
	// in Doppler to silently downgrade the experience. Staging/prod read
	// LOG_STYLE normally and default to "json" for Loki / log shippers.
	// The OTLP log hook (see observability.Wire) runs on top of either
	// writer, so OTEL log shipping happens regardless of stderr format.
	var style string
	if cfg.App.Env == "dev" {
		style = "console"
	} else {
		style = cfg.GetEnv("LOG_STYLE", false, "json")
	}
	cfg.Log = LogConfig{
		Level: parseLevel(cfg.GetEnv("LOG_LEVEL", false, "info")),
		Style: style,
	}
	cfg.Auth = AuthConfig{
		DefaultAPIKey: cfg.GetEnv("DEFAULT_API_KEY", true, ""),
		AdminPassHash: cfg.GetEnv("ADMIN_PASS_HASH", true, ""),
	}
	// Optional, effectively dead config: main/ ignores these for the online
	// source (it routes by the vendor body's store_id, then store_name) and no
	// longer requires them. Still stamped on SQS messages when set.
	cfg.Scope = ScopeConfig{
		TenantID:   cfg.GetEnv("SCOPE_TENANT_ID", false, ""),
		LocationID: cfg.GetEnv("SCOPE_LOCATION_ID", false, ""),
	}
	// SQS_QUEUE_URL is required in every environment. Dev and staging
	// point it at the local ElasticMQ container; prod points at real AWS.
	cfg.SQS = SQSConfig{
		QueueURL: cfg.GetEnv("SQS_QUEUE_URL", true, ""),
		Region:   cfg.GetEnv("AWS_REGION", false, "us-east-1"),
		Endpoint: cfg.GetEnv("SQS_ENDPOINT", false, ""),
	}
	cfg.OTEL = OTELConfig{
		Endpoint:    cfg.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", false, ""),
		ServiceName: cfg.GetEnv("OTEL_SERVICE_NAME", false, "quicc-online"),
	}
	cfg.DBPath = cfg.GetEnv("DB_PATH", false, "/data/app.db")

	return cfg
}

// GetEnv reads an env var. If req is true and the var is unset/empty, the
// process logs Fatal and exits. Otherwise the default is returned.
func (c *Config) GetEnv(key string, req bool, def string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		if req {
			c.logger.Fatal().Str("env", key).Msg("required env var missing")
		}
		return def
	}
	return val
}

func (c *Config) GetEnvInt(key string, def int) int {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		c.logger.Warn().Err(err).Str("env", key).Str("value", val).
			Msg("invalid int; using default")
		return def
	}
	return n
}

func parseLevel(s string) zerolog.Level {
	level, err := zerolog.ParseLevel(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level %q; defaulting to info\n", s)
		return zerolog.InfoLevel
	}
	return level
}
