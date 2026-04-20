// Package config defines application configuration loaded from environment variables.
package config

import (
	"os"
	"strconv"
)

// Config holds the runtime configuration for the Xentry server.
type Config struct {
	// Addr is the TCP listen address, e.g. ":8576".
	Addr string
	// Database is the path to the SQLite database file.
	Database string
	// JWTSecret is the HMAC key used for signing JWT tokens.
	JWTSecret string
	// Env is the deployment environment: "development" or "production".
	Env string
	// DataDir is the filesystem directory for uploads, symbols, and minidumps.
	DataDir string
}

// Load reads configuration from environment variables, falling back to sensible defaults.
func Load() *Config {
	cfg := &Config{
		Addr:      envOrDefault("XENTRY_ADDR", ":8576"),
		Database:  envOrDefault("XENTRY_DB", "./xentry.db"),
		JWTSecret: envOrDefault("XENTRY_JWT_SECRET", "change-me-in-production"),
		Env:       envOrDefault("XENTRY_ENV", "development"),
		DataDir:   envOrDefault("XENTRY_DATA_DIR", "./data"),
	}
	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
