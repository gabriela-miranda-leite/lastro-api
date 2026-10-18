package config

import (
	"fmt"
	"os"
)

// Config holds all environment-sourced application settings.
type Config struct {
	DatabaseURL   string
	JWTSecret     string
	Env           string
	Port          string
	AuthRateLimit string // e.g. "5-M" (default); raise in dev for load tests
}

// Load reads required environment variables and fails fast if any are missing.
func Load() (*Config, error) {
	authRate := os.Getenv("AUTH_RATE_LIMIT")
	if authRate == "" {
		authRate = "5-M"
	}

	cfg := &Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		Env:           os.Getenv("ENV"),
		Port:          os.Getenv("PORT"),
		AuthRateLimit: authRate,
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("config: JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("config: JWT_SECRET must be at least 32 characters")
	}
	if cfg.Env == "" {
		return nil, fmt.Errorf("config: ENV is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, nil
}
