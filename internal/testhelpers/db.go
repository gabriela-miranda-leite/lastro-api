package testhelpers

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupTestDB returns a *sql.DB ready for integration tests.
// If TEST_DATABASE_URL is set (e.g. in CI), it connects directly to that instance.
// Otherwise it spins up a postgres:16-alpine container via testcontainers.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		db, err := sql.Open("pgx", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		runMigrations(t, db)
		return db
	}

	ctx := context.Background()

	container, err := tcpostgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		tcpostgres.WithDatabase("lastro_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	runMigrations(t, db)

	return db
}

func runMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	// resolve db/migrations relative to this file's location
	_, thisFile, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations")
	abs, err := filepath.Abs(migrationsDir)
	require.NoError(t, err)

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	require.NoError(t, err)

	m, err := migrate.NewWithDatabaseInstance("file://"+abs, "postgres", driver)
	require.NoError(t, err)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrations failed: %v", err)
	}
}
