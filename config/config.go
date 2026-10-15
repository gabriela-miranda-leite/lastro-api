package config

import (
	"fmt"
	"os"
)

// Config holds all environment-sourced application settings.
type Config struct {
	DatabaseURL string
	JWTSecret   string
	Env         string
	Port        string
}

// Load reads required environment variables and fails fast if any are missing.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		Env:         os.Getenv("ENV"),
		Port:        os.Getenv("PORT"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("config: JWT_SECRET is required")
	}
	if cfg.Env == "" {
		return nil, fmt.Errorf("config: ENV is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, nil
}
