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

	tables := []string{
		"refresh_tokens",
		"couple_invites",
		"recurring_expenses",
		"monthly_savings",
		"transactions",
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE user_id = $1", table), //nolint:gosec // table names are hardcoded, not user input
			userID,
		); err != nil {
			return fmt.Errorf("purge user %s: delete %s: %w", userID, table, err)
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
