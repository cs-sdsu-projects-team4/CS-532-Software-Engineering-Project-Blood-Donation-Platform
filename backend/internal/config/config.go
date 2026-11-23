// Package config loads runtime configuration from environment variables.
//
// Everything the app needs to run comes from the environment (12-factor style).
// In Docker these are set by docker-compose; locally they come from your shell
// or a .env you source. We never read secrets from committed files.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all runtime settings. Fields are exported (capitalized) so other
// packages can read them.
type Config struct {
	HTTPAddr    string // address the HTTP server listens on, e.g. ":8080"
	DatabaseURL string // Postgres connection string
	Env         string // "development", "production", etc.

	// Auth. JWTSecret signs access/refresh tokens and must be set in every
	// environment (never commit a real value — see deploy/.env.example).
	JWTSecret       string
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Load reads configuration from the environment and validates required values.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Env:         getEnv("APP_ENV", "development"),

		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTIssuer:       getEnv("JWT_ISSUER", "blood-donation-api"),
		AccessTokenTTL:  getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getEnvDuration("REFRESH_TOKEN_TTL", 720*time.Hour), // 30 days
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

// getEnvDuration parses a Go duration string (e.g. "15m", "720h") from the
// environment, falling back when unset or unparseable.
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// getEnv returns the value of an env var, or a fallback when it is unset/empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
