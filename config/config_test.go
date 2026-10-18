package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/config"
)

func setEnv(t *testing.T, pairs ...string) {
	t.Helper()
	for i := 0; i < len(pairs); i += 2 {
		t.Setenv(pairs[i], pairs[i+1])
	}
}

func TestLoad_Success(t *testing.T) {
	setEnv(t,
		"DATABASE_URL", "postgres://user:pass@localhost/db",
		"JWT_SECRET", "supersecret",
		"ENV", "development",
	)
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("AUTH_RATE_LIMIT")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "postgres://user:pass@localhost/db", cfg.DatabaseURL)
	assert.Equal(t, "supersecret", cfg.JWTSecret)
	assert.Equal(t, "development", cfg.Env)
	assert.Equal(t, "8080", cfg.Port)         // default
	assert.Equal(t, "5-M", cfg.AuthRateLimit) // default
}

func TestLoad_CustomPort(t *testing.T) {
	setEnv(t,
		"DATABASE_URL", "postgres://x",
		"JWT_SECRET", "s",
		"ENV", "production",
		"PORT", "9090",
	)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
}

func TestLoad_CustomAuthRateLimit(t *testing.T) {
	setEnv(t,
		"DATABASE_URL", "postgres://x",
		"JWT_SECRET", "s",
		"ENV", "development",
		"AUTH_RATE_LIMIT", "100-M",
	)

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "100-M", cfg.AuthRateLimit)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	_ = os.Unsetenv("DATABASE_URL")
	t.Setenv("JWT_SECRET", "s")
	t.Setenv("ENV", "development")

	_, err := config.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	_ = os.Unsetenv("JWT_SECRET")
	t.Setenv("ENV", "development")

	_, err := config.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_MissingENV(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "s")
	_ = os.Unsetenv("ENV")

	_, err := config.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ENV")
}
