package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrInvalidPassword = errors.New("invalid password")

func (r *Repository) softDelete(ctx context.Context, userID, passwordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("user.repository.softDelete: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET deleted_at = now(), couple_id = null WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	); err != nil {
		return fmt.Errorf("user.repository.softDelete: soft delete: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	); err != nil {
		return fmt.Errorf("user.repository.softDelete: revoke tokens: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO audit_log (user_id, action, entity, entity_id) VALUES ($1, 'delete_account', 'user', $1)`,
		userID,
	); err != nil {
		return fmt.Errorf("user.repository.softDelete: audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("user.repository.softDelete: commit: %w", err)
	}

	return nil
}

func (r *Repository) findPasswordHash(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("user.repository.findPasswordHash: %w", err)
	}
	return hash, nil
}
