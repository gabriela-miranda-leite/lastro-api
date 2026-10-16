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

type loginUserRow struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
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
