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

func (r *Repository) list(ctx context.Context, userID string, input *ListInput) ([]Goal, error) {
	query := `SELECT id, user_id, year, month, goal_amount::text, saved_amount::text, created_at, updated_at
	            FROM monthly_savings
	           WHERE user_id = $1`
	args := []interface{}{userID}
	idx := 2

	if input.Year != nil {
		query += fmt.Sprintf(" AND year = $%d", idx)
		args = append(args, *input.Year)
		idx++
	}
	if input.Month != nil {
		query += fmt.Sprintf(" AND month = $%d", idx)
		args = append(args, *input.Month)
	}
	query += " ORDER BY year DESC, month DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("goal.repository.list: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	var result []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.UserID, &g.Year, &g.Month,
			&g.GoalAmount, &g.SavedAmount, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("goal.repository.list: %w", err)
		}
		result = append(result, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("goal.repository.list: %w", err)
	}
	return result, nil
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
