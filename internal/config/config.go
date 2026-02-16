// Package config provides centralized configuration loading and validation
// for the Gatify gateway.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all validated configuration for the Gatify gateway.
type Config struct {
	// ListenAddr is the address the HTTP server binds to (e.g., ":3000").
	ListenAddr string

	// BackendURL is the upstream target URL for the reverse proxy.
	BackendURL *url.URL

	// TrustProxy enables trusting X-Forwarded-For headers.
	TrustProxy bool

	// AdminAPIToken is the bearer token required for management API access.
	AdminAPIToken string

	// RateLimitRequests is the default global rate limit.
	RateLimitRequests int64

	// RateLimitWindow is the sliding window duration for rate limiting.
	RateLimitWindow time.Duration

	// RedisAddr is the Redis server address (host:port).
	RedisAddr string

	// DatabaseURL is the PostgreSQL/TimescaleDB connection string for analytics.
	// Empty string disables analytics persistence.
	DatabaseURL string

	// AllowedOrigins controls CORS allowed origins.
	// Empty means no CORS headers are set.
	AllowedOrigins []string

	// LogLevel controls the minimum log level (debug, info, warn, error).
	LogLevel string
}

// Load reads configuration from environment variables, applies defaults,
// and validates all required values.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:        getEnv("LISTEN_ADDR", ":3000"),
		TrustProxy:        getEnv("TRUST_PROXY", "false") == "true",
		AdminAPIToken:     strings.TrimSpace(getEnv("ADMIN_API_TOKEN", "")),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		DatabaseURL:       strings.TrimSpace(getEnv("DATABASE_URL", "")),
		LogLevel:          strings.ToLower(getEnv("LOG_LEVEL", "info")),
		RateLimitRequests: getEnvInt64("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
	}

	// Parse backend URL
	backendRaw := getEnv("BACKEND_URL", "http://localhost:8080")
	backendURL, err := url.Parse(backendRaw)
	if err != nil {
		return nil, fmt.Errorf("config: invalid BACKEND_URL %q: %w", backendRaw, err)
	}
	cfg.BackendURL = backendURL

	// Parse allowed origins
	originsRaw := strings.TrimSpace(getEnv("ALLOWED_ORIGINS", ""))
	if originsRaw != "" {
		for _, origin := range strings.Split(originsRaw, ",") {
			trimmed := strings.TrimSpace(origin)
			if trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that the configuration is consistent and safe.
func (c *Config) Validate() error {
	if c.BackendURL == nil {
		return fmt.Errorf("config: BACKEND_URL is required")
	}
	if c.BackendURL.Scheme != "http" && c.BackendURL.Scheme != "https" {
		return fmt.Errorf("config: BACKEND_URL scheme must be http or https, got %q", c.BackendURL.Scheme)
	}
	if c.BackendURL.Host == "" {
		return fmt.Errorf("config: BACKEND_URL must include a host")
	}
	if c.RedisAddr == "" {
		return fmt.Errorf("config: REDIS_ADDR is required")
	}
	if c.RateLimitRequests <= 0 {
		return fmt.Errorf("config: RATE_LIMIT_REQUESTS must be > 0")
	}
	if c.RateLimitWindow <= 0 {
		return fmt.Errorf("config: RATE_LIMIT_WINDOW_SECONDS must be > 0")
	}
	if c.AdminAPIToken == "change-me" {
		return fmt.Errorf("config: ADMIN_API_TOKEN must be changed from default value")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("config: LOG_LEVEL must be one of: debug, info, warn, error; got %q", c.LogLevel)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
