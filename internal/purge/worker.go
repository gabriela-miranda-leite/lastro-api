package purge

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

const interval = time.Hour
const retentionDays = 30

// Worker runs the account purge job on a fixed interval.
type Worker struct {
	db *sql.DB
}

// New creates a new Worker.
func New(db *sql.DB) *Worker {
	return &Worker{db: db}
}

// Run starts the purge loop. It blocks until ctx is canceled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("purge worker started", "interval", interval, "retention_days", retentionDays)

	for {
		select {
		case <-ctx.Done():
			slog.Info("purge worker stopped")
			return
		case <-ticker.C:
			if err := w.purge(ctx); err != nil {
				slog.Error("purge worker error", "error", err)
			}
		}
	}
}

// RunOnce executes a single purge cycle. Exported for testing.
func (w *Worker) RunOnce(ctx context.Context) error {
	return w.purge(ctx)
}

func (w *Worker) purge(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	rows, err := w.db.QueryContext(ctx,
		`SELECT id FROM users WHERE deleted_at IS NOT NULL AND deleted_at < $1`,
		cutoff,
	)
	if err != nil {
		return fmt.Errorf("purge: query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("purge: scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("purge: rows: %w", err)
	}

	for _, userID := range ids {
		if err := w.purgeUser(ctx, userID); err != nil {
			slog.Error("purge: failed to purge user", "user_id", userID, "error", err)
			// continue to next user — don't abort the whole run
		}
	}

	if len(ids) > 0 {
		slog.Info("purge: completed", "users_purged", len(ids))
	}

	return nil
}

func (w *Worker) purgeUser(ctx context.Context, userID string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("purge user %s: begin: %w", userID, err)
	}
	defer func() { _ = tx.Rollback() }()

	type tableDelete struct {
		table  string
		column string
	}
	tables := []tableDelete{
		{"refresh_tokens", "user_id"},
		{"couple_invites", "inviter_id"},
		{"recurring_expenses", "user_id"},
		{"monthly_savings", "user_id"},
		{"transactions", "user_id"},
	}
	for _, td := range tables {
		//nolint:gosec // table/column names are hardcoded constants, not user input
		q := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", td.table, td.column)
		if _, err := tx.ExecContext(ctx, q, userID); err != nil {
			return fmt.Errorf("purge user %s: delete %s: %w", userID, td.table, err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM users WHERE id = $1`,
		userID,
	); err != nil {
		return fmt.Errorf("purge user %s: delete user: %w", userID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("purge user %s: commit: %w", userID, err)
	}

	slog.Info("purge: user purged", "user_id", userID)
	return nil
}
