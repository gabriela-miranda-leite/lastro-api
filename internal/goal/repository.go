package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Repository handles data access for the goal domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) insert(ctx context.Context, userID string, input *CreateInput) (Goal, error) {
	var g Goal
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO monthly_savings (user_id, year, month, goal_amount)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, year, month, goal_amount::text, saved_amount::text, created_at, updated_at`,
		userID, input.Year, input.Month, input.GoalAmount,
	).Scan(&g.ID, &g.UserID, &g.Year, &g.Month, &g.GoalAmount, &g.SavedAmount, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Goal{}, ErrConflict
		}
		return Goal{}, fmt.Errorf("goal.repository.insert: %w", err)
	}
	return g, nil
}
