package recurringjob_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/recurringjob"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

// seed helpers

func createUser(t *testing.T, db *sql.DB, email string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (name, email, password_hash) VALUES ('Test', $1, 'x') RETURNING id`, email,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func createRecurring(t *testing.T, db *sql.DB, userID, name string, amount float64, dueDay int) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO recurring_expenses (user_id, name, amount, due_day)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, name, amount, dueDay,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func createOverride(t *testing.T, db *sql.DB, recurringID string, year, month int, amountOverride *float64, active bool) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO recurring_overrides (recurring_id, year, month, amount_override, active)
		 VALUES ($1, $2, $3, $4, $5)`,
		recurringID, year, month, amountOverride, active,
	)
	require.NoError(t, err)
}

func countTransactions(t *testing.T, db *sql.DB, userID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&n))
	return n
}

func getTransaction(t *testing.T, db *sql.DB, userID string) (amount, description string, date time.Time) {
	t.Helper()
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT amount::text, description, date FROM transactions
		  WHERE user_id = $1 AND deleted_at IS NULL LIMIT 1`, userID,
	).Scan(&amount, &description, &date))
	return
}

// --- Tests ---

func TestJob_CreatesTransaction(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_basic@test.com")
	createRecurring(t, db, userID, "Aluguel", 1500, 5)

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	assert.Equal(t, 1, countTransactions(t, db, userID))
	amount, desc, date := getTransaction(t, db, userID)
	assert.Equal(t, "1500.00", amount)
	assert.Equal(t, "Aluguel", desc)
	assert.Equal(t, 5, date.Day())
	assert.Equal(t, time.October, date.Month())
}

func TestJob_Idempotent(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_idem@test.com")
	createRecurring(t, db, userID, "Netflix", 40, 10)

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.October))
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	assert.Equal(t, 1, countTransactions(t, db, userID))
}

func TestJob_OverrideActiveFalse_Skips(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_skip@test.com")
	recID := createRecurring(t, db, userID, "Gym", 80, 1)
	createOverride(t, db, recID, 2026, 10, nil, false)

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	assert.Equal(t, 0, countTransactions(t, db, userID))
}

func TestJob_OverrideAmountOverride_UsesOverrideValue(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_amt@test.com")
	recID := createRecurring(t, db, userID, "Condomínio", 500, 10)
	amt := 650.0
	createOverride(t, db, recID, 2026, 10, &amt, true)

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	amount, _, _ := getTransaction(t, db, userID)
	assert.Equal(t, "650.00", amount)
}

func TestJob_DueDayExceedsMonth_UsesLastDay(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_lastday@test.com")
	createRecurring(t, db, userID, "Internet", 100, 31)

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.February))

	_, _, date := getTransaction(t, db, userID)
	assert.Equal(t, 28, date.Day())
}

func TestJob_InactiveRecurring_NotGenerated(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_inactive@test.com")
	var id string
	require.NoError(t, db.QueryRowContext(context.Background(),
		`INSERT INTO recurring_expenses (user_id, name, amount, due_day, active)
		 VALUES ($1, 'Pausado', 200, 5, false) RETURNING id`, userID,
	).Scan(&id))

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	assert.Equal(t, 0, countTransactions(t, db, userID))
}

func TestJob_MultipleRecurrings(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_multi@test.com")
	createRecurring(t, db, userID, "A", 100, 1)
	createRecurring(t, db, userID, "B", 200, 15)
	createRecurring(t, db, userID, "C", 300, 20)

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	assert.Equal(t, 3, countTransactions(t, db, userID))
}

func TestJob_NextMonthUsesOriginalAmount(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	userID := createUser(t, db, "job_nextmonth@test.com")
	recID := createRecurring(t, db, userID, "Fixo", 500, 10)
	// override apenas em outubro
	amt := 650.0
	createOverride(t, db, recID, 2026, 10, &amt, true)

	job := recurringjob.New(db)
	require.NoError(t, job.Run(context.Background(), 2026, time.November))

	amount, _, _ := getTransaction(t, db, userID)
	assert.Equal(t, "500.00", amount)
}
