package recurringjob

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// jobNamespace is the UUID v5 namespace used to derive deterministic client_ids.
var jobNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // UUID namespace DNS

// Job generates transactions for active recurring expenses on the 1st of each month.
type Job struct {
	db *sql.DB
}

// New creates a new Job.
func New(db *sql.DB) *Job {
	return &Job{db: db}
}

// Run executes the job for the given year/month.
// It is safe to call multiple times for the same month — idempotent via client_id UNIQUE constraint.
func (j *Job) Run(ctx context.Context, year int, month time.Month) error {
	rows, err := j.db.QueryContext(ctx, `
		SELECT
			re.id, re.user_id, re.category_id, re.name,
			re.amount, re.due_day,
			ro.active        AS override_active,
			ro.amount_override
		FROM recurring_expenses re
		LEFT JOIN recurring_overrides ro
		       ON ro.recurring_id = re.id
		      AND ro.year  = $1
		      AND ro.month = $2
		WHERE re.active = true
	`, year, int(month))
	if err != nil {
		return fmt.Errorf("recurringjob.Run: query: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	created, skipped := 0, 0
	for rows.Next() {
		var (
			recurringID    string
			userID         string
			categoryID     sql.NullString
			name           string
			amount         string // NUMERIC as text
			dueDay         int
			overrideActive sql.NullBool
			amountOverride sql.NullString
		)
		if err := rows.Scan(&recurringID, &userID, &categoryID, &name, &amount, &dueDay,
			&overrideActive, &amountOverride); err != nil {
			return fmt.Errorf("recurringjob.Run: scan: %w", err)
		}

		// override com active=false → pular
		if overrideActive.Valid && !overrideActive.Bool {
			skipped++
			continue
		}

		txAmount := amount
		if amountOverride.Valid {
			txAmount = amountOverride.String
		}

		date := resolveDate(year, month, dueDay)
		clientID := deterministicClientID(recurringID, year, int(month))

		var catArg interface{}
		if categoryID.Valid {
			catArg = categoryID.String
		}

		_, err := j.db.ExecContext(ctx, `
			INSERT INTO transactions
			  (client_id, user_id, category_id, amount, direction, description, is_shared, date)
			VALUES ($1, $2, $3, $4, 'out', $5, false, $6)
			ON CONFLICT (client_id) DO NOTHING
		`, clientID, userID, catArg, txAmount, name, date)
		if err != nil {
			return fmt.Errorf("recurringjob.Run: insert transaction for recurring %s: %w", recurringID, err)
		}
		created++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("recurringjob.Run: rows: %w", err)
	}

	slog.Info("recurringjob finished", "year", year, "month", month, "created", created, "skipped", skipped)
	return nil
}

// RunForCurrentMonth is the cron entry point — called on the 1st of each month.
func (j *Job) RunForCurrentMonth() {
	now := time.Now()
	if err := j.Run(context.Background(), now.Year(), now.Month()); err != nil {
		slog.Error("recurringjob failed", "error", err)
	}
}

// resolveDate returns the date for a recurring expense in the given month.
// If dueDay exceeds the last day of the month, uses the last day.
func resolveDate(year int, month time.Month, dueDay int) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := dueDay
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// deterministicClientID generates a UUID v5 from recurring_id + year + month,
// ensuring the same input always produces the same UUID (idempotency).
func deterministicClientID(recurringID string, year, month int) uuid.UUID {
	seed := fmt.Sprintf("%s-%d-%02d", recurringID, year, month)
	return uuid.NewSHA1(jobNamespace, []byte(seed))
}
