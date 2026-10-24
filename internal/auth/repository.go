package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrEmailConflict is returned when a user with the same email already exists.
var ErrEmailConflict = errors.New("email already in use")

// ErrInvalidCredentials is returned when email or password don't match.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrTokenNotFound is returned when the refresh token is not found or already expired.
var ErrTokenNotFound = errors.New("refresh token not found or expired")

// ErrTokenRevoked is returned when the refresh token was already revoked (reuse detected).
var ErrTokenRevoked = errors.New("refresh token already revoked")

// Repository handles data access for the auth domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type createUserParams struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
}

type userRow struct {
	ID    string
	Name  string
	Email string
}

func (r *Repository) createUser(ctx context.Context, p createUserParams) (userRow, error) {
	const q = `
		INSERT INTO users (id, name, email, password_hash)
		VALUES (gen_random_uuid(), $1, $2, $3)
		RETURNING id, name, email`

	var row userRow
	err := r.db.QueryRowContext(ctx, q, p.Name, p.Email, p.PasswordHash).
		Scan(&row.ID, &row.Name, &row.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return userRow{}, ErrEmailConflict
		}
		return userRow{}, fmt.Errorf("auth.repository.createUser: %w", err)
	}

	return row, nil
}

func (r *Repository) saveRefreshToken(ctx context.Context, userID, tokenHash string) error {
	const q = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() + interval '30 days')`

	if _, err := r.db.ExecContext(ctx, q, userID, tokenHash); err != nil {
		return fmt.Errorf("auth.repository.saveRefreshToken: %w", err)
	}

	return nil
}

type refreshTokenRow struct {
	ID     string
	UserID string
}

// findRefreshToken looks up a non-expired, non-revoked token by its hash.
// Returns ErrTokenRevoked if found but revoked, ErrTokenNotFound if absent or expired.
func (r *Repository) findRefreshToken(ctx context.Context, tokenHash string) (refreshTokenRow, error) {
	const q = `
		SELECT id, user_id, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND expires_at > now()`

	var row refreshTokenRow
	var revokedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, q, tokenHash).Scan(&row.ID, &row.UserID, &revokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return refreshTokenRow{}, ErrTokenNotFound
		}
		return refreshTokenRow{}, fmt.Errorf("auth.repository.findRefreshToken: %w", err)
	}
	if revokedAt.Valid {
		return refreshTokenRow{}, ErrTokenRevoked
	}
	return row, nil
}

func (r *Repository) revokeRefreshToken(ctx context.Context, tokenID string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, q, tokenID); err != nil {
		return fmt.Errorf("auth.repository.revokeRefreshToken: %w", err)
	}
	return nil
}

// revokeRefreshTokenByHash revokes a single token scoped to a user.
// If the token doesn't exist or belongs to another user it's a no-op (idempotent).
func (r *Repository) revokeRefreshTokenByHash(ctx context.Context, userID, tokenHash string) error {
	const q = `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND user_id = $2 AND revoked_at IS NULL`

	if _, err := r.db.ExecContext(ctx, q, tokenHash, userID); err != nil {
		return fmt.Errorf("auth.repository.revokeRefreshTokenByHash: %w", err)
	}
	return nil
}

func (r *Repository) revokeAllUserTokens(ctx context.Context, userID string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`
	if _, err := r.db.ExecContext(ctx, q, userID); err != nil {
		return fmt.Errorf("auth.repository.revokeAllUserTokens: %w", err)
	}
	return nil
}

func (r *Repository) findUserByID(ctx context.Context, userID string) (userRow, error) {
	const q = `SELECT id, name, email FROM users WHERE id = $1 AND deleted_at IS NULL`
	var row userRow
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&row.ID, &row.Name, &row.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userRow{}, ErrInvalidCredentials
		}
		return userRow{}, fmt.Errorf("auth.repository.findUserByID: %w", err)
	}
	return row, nil
}

type loginUserRow struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
}

func (r *Repository) savePasswordResetToken(ctx context.Context, userID, tokenHash string) error {
	const q = `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() + interval '30 minutes')`
	if _, err := r.db.ExecContext(ctx, q, userID, tokenHash); err != nil {
		return fmt.Errorf("auth.repository.savePasswordResetToken: %w", err)
	}
	return nil
}

func (r *Repository) findUserByEmail(ctx context.Context, email string) (loginUserRow, error) {
	const q = `
		SELECT id, name, email, password_hash
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`

	var row loginUserRow
	err := r.db.QueryRowContext(ctx, q, email).
		Scan(&row.ID, &row.Name, &row.Email, &row.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return loginUserRow{}, ErrInvalidCredentials
		}
		return loginUserRow{}, fmt.Errorf("auth.repository.findUserByEmail: %w", err)
	}

	return row, nil
}
