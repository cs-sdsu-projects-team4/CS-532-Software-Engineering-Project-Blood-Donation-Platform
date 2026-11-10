// Package config loads runtime configuration from environment variables.
//
// Everything the app needs to run comes from the environment (12-factor style).
// In Docker these are set by docker-compose; locally they come from your shell
// or a .env you source. We never read secrets from committed files.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime settings. Fields are exported (capitalized) so other
// packages can read them.
type Config struct {
	HTTPAddr    string // address the HTTP server listens on, e.g. ":8080"
	DatabaseURL string // Postgres connection string
	Env         string // "development", "production", etc.
}

// Load reads configuration from the environment and validates required values.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Env:         getEnv("APP_ENV", "development"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

// getEnv returns the value of an env var, or a fallback when it is unset/empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
